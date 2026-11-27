package guardrails

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/cel-go/cel"
	"github.com/maximhq/bifrost/core/schemas"
)

var _ schemas.LLMPlugin = (*Plugin)(nil)

// guardrailInterventionErrorType is the error type the log store's guardrail-outcome filter
// reads (framework/logstore: error_details.error.type).
const guardrailInterventionErrorType = "guardrail_intervention"

// Plugin evaluates guardrail rules against LLM request input.
type Plugin struct {
	logger schemas.Logger
	load   Loader
	// hmacKey keys the hash redaction strategy. It is never logged.
	hmacKey []byte
	// reloadMu serializes load, build and swap, so a slow reload that read an older config
	// cannot publish it over the snapshot a later reload already installed.
	reloadMu sync.Mutex
	snapshot atomic.Pointer[snapshot]
}

// snapshot is the immutable compiled configuration. Reload builds a new one and swaps it in,
// so a request never sees a half-built rule set.
type snapshot struct {
	rules []*compiledRule
}

// compiledRule is an enabled, in-scope rule with its compiled CEL program.
type compiledRule struct {
	id       uint
	name     string
	sampling int
	timeout  int
	program  cel.Program
	profiles []*compiledProfile
}

// compiledProfile is an enabled profile with its detector built at reload time.
type compiledProfile struct {
	id         uint
	provider   string
	policyName string
	action     string
	strategy   string
	failOpen   bool
	timeout    int
	// enforcing reports whether the profile must fail closed: a detect-only profile, or one that
	// opted into fail-open, monitors; anything else must block when the text cannot be inspected.
	enforcing bool
	detector  detector
}

// profileRun is one profile selected for this request, with the absolute deadline it must
// finish by and the first rule that selected it (the rule the guardrail metadata names).
type profileRun struct {
	profile  *compiledProfile
	rule     *compiledRule
	deadline time.Time
}

// verdict is what an inspection leaves for PreLLMHook: the policies that blocked and the entity
// types they found. It never holds matched text. requestID binds it to the request that produced
// it, so a verdict left on a context that another request also reads is not applied to that one.
type verdict struct {
	requestID string
	policies  []string
	entities  []string
	// detail replaces the entity list in the block message when the block is not about entities.
	detail string
	failed bool
}

// verdictKey holds the request phase's *verdict.
type verdictKey struct{}

// inspectMarker is the small handle the last inspection leaves on the context: the request it
// ran for and the shape of the text it covered. A later attempt whose text is unchanged reuses
// that inspection instead of running the detectors again.
type inspectMarker struct {
	requestID   string
	segments    int
	bytes       int
	sum         uint64
	unsupported bool // sampled out while the text could not be walked; nothing to fingerprint
}

// markerKey holds the last inspection's inspectMarker.
type markerKey struct{}

// Init builds the plugin and its first snapshot. hmacKey keys the hash redaction strategy;
// when it is nil a random key is generated for this process, so hashes are not comparable
// across restarts.
func Init(ctx context.Context, cfg *Config, logger schemas.Logger, load Loader, hmacKey []byte) (*Plugin, error) {
	if logger == nil {
		return nil, errors.New("guardrails: a logger is required")
	}
	if len(hmacKey) == 0 {
		generated := make([]byte, 32)
		if _, err := rand.Read(generated); err != nil {
			return nil, fmt.Errorf("guardrails: generate hash key: %w", err)
		}
		hmacKey = generated
		logger.Warn("[Guardrails] no deployment encryption key is configured, so redaction hashes use a per-process key and change on restart")
	}
	p := &Plugin{logger: logger, load: load, hmacKey: hmacKey}
	if err := p.Reload(ctx); err != nil {
		return nil, err
	}
	return p, nil
}

// GetName returns the plugin name.
func (p *Plugin) GetName() string { return PluginName }

// Cleanup releases nothing: the plugin owns no goroutines or connections beyond the shared
// HTTP client.
func (p *Plugin) Cleanup() error { return nil }

// Reload rebuilds the snapshot from the loader. A failed reload keeps the previous snapshot,
// so a bad write never leaves the gateway unprotected. Reloads are serialized: two writers race,
// and the one that read the older config must not install it over the newer one.
func (p *Plugin) Reload(ctx context.Context) error {
	if p.load == nil {
		return nil
	}
	p.reloadMu.Lock()
	defer p.reloadMu.Unlock()
	profiles, rules, err := p.load(ctx)
	if err != nil {
		return fmt.Errorf("guardrails: load config: %w", err)
	}
	snapshot, err := p.build(profiles, rules)
	if err != nil {
		return err
	}
	p.snapshot.Store(snapshot)
	return nil
}

// build compiles profiles and rules into an immutable snapshot.
func (p *Plugin) build(profiles []Profile, rules []Rule) (*snapshot, error) {
	compiled := make(map[uint]*compiledProfile, len(profiles))
	for _, profile := range profiles {
		if !profile.Enabled {
			continue
		}
		cp, err := p.compileProfile(profile)
		if err != nil {
			return nil, err
		}
		compiled[cp.id] = cp
	}

	building := &snapshot{rules: make([]*compiledRule, 0, len(rules))}
	for _, rule := range rules {
		if !rule.Enabled || !ruleInScope(rule) {
			continue
		}
		program, err := compileCEL(rule.CELExpression)
		if err != nil {
			// Validators reject a rule that will not compile at write time, so this is config
			// that predates them; skipping it keeps the rest of the rule set live.
			p.logger.Warn("[Guardrails] rule %d (%s) skipped: %v", rule.ID, rule.Name, err)
			continue
		}
		refs := make([]*compiledProfile, 0, len(rule.ProfileIDs))
		for _, id := range rule.ProfileIDs {
			if cp := compiled[id]; cp != nil {
				refs = append(refs, cp)
			}
		}
		if len(refs) == 0 {
			continue
		}
		building.rules = append(building.rules, &compiledRule{
			id:       rule.ID,
			name:     rule.Name,
			sampling: samplingRate(rule),
			timeout:  ruleTimeout(rule),
			program:  program,
			profiles: refs,
		})
	}
	sort.Slice(building.rules, func(i, j int) bool { return building.rules[i].id < building.rules[j].id })
	return building, nil
}

// ruleInScope reports whether a rule targets this build's phase: the LLM input.
func ruleInScope(rule Rule) bool {
	if target := rule.Target; target != "" && target != TargetLLM {
		return false
	}
	if applyTo := rule.ApplyTo; applyTo != "" && applyTo != ApplyToInput {
		return false
	}
	return true
}

// compileProfile validates a profile and builds its detector.
func (p *Plugin) compileProfile(profile Profile) (*compiledProfile, error) {
	if err := ValidateProfile(profile); err != nil {
		return nil, fmt.Errorf("guardrails: profile %d (%s): %w", profile.ID, profile.PolicyName, err)
	}
	compiled := &compiledProfile{
		id:         profile.ID,
		provider:   profile.ProviderName,
		policyName: profile.PolicyName,
		timeout:    profileTimeout(profile),
	}
	switch profile.ProviderName {
	case ProviderSecrets:
		var cfg SecretsConfig
		if err := decodeConfig(profile.Config, &cfg); err != nil {
			return nil, err
		}
		compiled.action = defaultAction(cfg.Action, ActionBlock)
		compiled.strategy = cfg.RedactionStrategy
		compiled.enforcing = compiled.action != ActionDetectOnly
		detector, err := newSecretsDetector(cfg)
		if err != nil {
			return nil, fmt.Errorf("guardrails: profile %d (%s): %w", profile.ID, profile.PolicyName, err)
		}
		compiled.detector = detector
	case ProviderRegex:
		var cfg RegexConfig
		if err := decodeConfig(profile.Config, &cfg); err != nil {
			return nil, err
		}
		// A regex pattern carries its own action; there is no profile-level one, so the profile
		// fails closed when any pattern would block or redact.
		for _, pattern := range cfg.Patterns {
			if defaultAction(pattern.Action, ActionBlock) != ActionDetectOnly {
				compiled.enforcing = true
			}
		}
		detector, err := newRegexDetector(cfg)
		if err != nil {
			return nil, fmt.Errorf("guardrails: profile %d (%s): %w", profile.ID, profile.PolicyName, err)
		}
		compiled.detector = detector
	case ProviderPresidio:
		var cfg PresidioConfig
		if err := decodeConfig(profile.Config, &cfg); err != nil {
			return nil, err
		}
		compiled.action = defaultAction(cfg.Action, ActionDetectOnly)
		compiled.strategy = cfg.RedactionStrategy
		compiled.failOpen = cfg.FailOpen
		compiled.enforcing = compiled.action != ActionDetectOnly && !compiled.failOpen
		detector, err := newPresidioDetector(cfg)
		if err != nil {
			return nil, fmt.Errorf("guardrails: profile %d (%s): %w", profile.ID, profile.PolicyName, err)
		}
		compiled.detector = detector
	}
	return compiled, nil
}

// PreRequestHook selects the rules for this request, runs their profiles' detectors once, and
// either redacts the request in place or leaves a block verdict for PreLLMHook. With no rules
// configured it is one atomic load and a length check.
func (p *Plugin) PreRequestHook(ctx *schemas.BifrostContext, req *schemas.BifrostRequest) error {
	current := p.snapshot.Load()
	if current == nil || len(current.rules) == 0 {
		return nil
	}
	p.inspect(ctx, req, current, false)
	return nil
}

// runOutcome is one profile's findings, split by the action each resolves to. A failed run is
// recorded instead of findings when the profile could not be evaluated at all.
type runOutcome struct {
	run     profileRun
	failure error
	block   []finding
	redact  []finding
	detect  []finding
}

// inspect is the detection and rewrite pass. reinspect marks the PreLLMHook pass, where a later
// turn changed the request's text.
func (p *Plugin) inspect(ctx *schemas.BifrostContext, req *schemas.BifrostRequest, current *snapshot, reinspect bool) {
	runs := current.selectProfiles(ctx, req, p.logger, reinspect)
	if len(runs) == 0 {
		if !reinspect {
			// Sampling decided this request is not inspected. Record that, so the PreLLMHook pass
			// does not treat a missing marker as new text and sample the rule again.
			if segments, err := collectSegments(ctx, req); err == nil {
				p.recordInspection(ctx, segments)
			} else {
				ctx.SetValue(markerKey{}, inspectMarker{requestID: requestID(ctx), unsupported: true})
			}
		}
		return
	}
	segments, err := collectSegments(ctx, req)
	if err != nil {
		p.blockUnsupportedText(ctx, req, runs)
		return
	}
	// Deferred so the marker records the text as it leaves this pass, after any rewrite.
	defer p.recordInspection(ctx, segments)

	texts := segments.detectTexts()
	if len(texts) == 0 {
		return
	}

	outcomes := make([]runOutcome, 0, len(runs))
	for _, run := range runs {
		detectCtx, cancel := context.WithDeadline(ctx, run.deadline)
		found, err := run.profile.detector.Detect(detectCtx, texts)
		cancel()
		if err != nil {
			// A detector fails on an expired deadline, an unreachable Analyzer, or text it cannot
			// finish scanning. A profile that only monitors logs and passes; an enforcing one
			// fails closed.
			p.logger.Warn("[Guardrails] %s profile %q could not evaluate the request: %v", run.profile.provider, run.profile.policyName, err)
			if !run.profile.enforcing {
				continue
			}
			outcomes = append(outcomes, runOutcome{run: run, failure: err})
			continue
		}
		if len(found) == 0 {
			continue
		}
		outcome := runOutcome{run: run}
		for _, hit := range found {
			switch effectiveAction(hit, run.profile) {
			case ActionBlock:
				outcome.block = append(outcome.block, hit)
			case ActionRedact:
				outcome.redact = append(outcome.redact, hit)
			default:
				outcome.detect = append(outcome.detect, hit)
			}
		}
		outcomes = append(outcomes, outcome)
	}

	blocking := make([]runOutcome, 0, len(outcomes))
	rest := make([]runOutcome, 0, len(outcomes))
	for _, outcome := range outcomes {
		if outcome.failure != nil || len(outcome.block) > 0 {
			blocking = append(blocking, outcome)
			continue
		}
		rest = append(rest, outcome)
	}

	// Replacements come from every profile that redacts, blocked or not: the log path must never
	// keep the originals of a value this request was going to redact.
	replacements := make(map[string]string)
	var redactor *profileRun
	for i := range outcomes {
		outcome := &outcomes[i]
		for _, hit := range outcome.redact {
			if hit.literal == "" {
				continue
			}
			if _, ok := replacements[hit.literal]; ok {
				continue
			}
			replacements[hit.literal] = placeholder(hit.entityType, effectiveStrategy(hit, outcome.run.profile), hit.literal, p.hmacKey)
			if redactor == nil {
				redactor = &outcome.run
			}
		}
	}

	if len(blocking) > 0 {
		// A blocked request never reaches a provider, so it is not rewritten; its redaction map
		// is still published before the verdict, or the incident record holds the originals.
		p.publishRedactions(ctx, replacements)
		for _, outcome := range blocking {
			p.recordIntervention(ctx, outcome)
		}
		for _, outcome := range rest {
			p.recordDetection(ctx, outcome, outcome.detect, schemas.GuardrailDetectionActionDetect)
		}
		ctx.SetValue(verdictKey{}, newVerdict(ctx, blocking))
		return
	}

	if len(replacements) > 0 {
		applied := segments.rewrite(replacements)
		if err := p.syncRawBody(ctx, req, applied); err != nil {
			// The raw body is what the provider would actually send, so a redaction it cannot
			// carry must block rather than forward the original. The map is published anyway:
			// the request is refused, and the log path still redacts what was inspected.
			p.logger.Warn("[Guardrails] raw request body could not be redacted: %v", err)
			p.publishRedactions(ctx, applied)
			ctx.SetValue(verdictKey{}, &verdict{requestID: requestID(ctx), policies: []string{redactor.profile.policyName}, failed: true})
			return
		}
		p.publishRedactions(ctx, applied)
	}
	for _, outcome := range rest {
		p.recordDetection(ctx, outcome, outcome.detect, schemas.GuardrailDetectionActionDetect)
		p.recordDetection(ctx, outcome, outcome.redact, schemas.GuardrailDetectionActionRedact)
	}
}

// blockUnsupportedText handles a request whose text the walker cannot read: an enforcing profile
// must not let it reach a provider uninspected, while a monitoring one only warns. The reason
// names the request type, never its content.
func (p *Plugin) blockUnsupportedText(ctx *schemas.BifrostContext, req *schemas.BifrostRequest, runs []profileRun) {
	blocking := make([]runOutcome, 0, len(runs))
	for _, run := range runs {
		if run.profile.enforcing {
			blocking = append(blocking, runOutcome{run: run, failure: errUnsupportedShape})
		}
	}
	requestType := ""
	if req != nil {
		requestType = string(req.RequestType)
	}
	p.logger.Warn("[Guardrails] %s request text cannot be inspected: %d of %d profile(s) block", requestType, len(blocking), len(runs))
	if len(blocking) == 0 {
		return
	}
	for _, outcome := range blocking {
		p.recordIntervention(ctx, outcome)
	}
	blocked := newVerdict(ctx, blocking)
	blocked.detail = "unsupported request type " + requestType
	ctx.SetValue(verdictKey{}, blocked)
}

// publishRedactions hands the replacements to the log path and the trace, so neither stores a
// value this request redacts. A later turn's map merges into what earlier turns published: one
// inspection replacing another's would un-redact the first turn's values in the log.
func (p *Plugin) publishRedactions(ctx *schemas.BifrostContext, replacements map[string]string) {
	if len(replacements) == 0 {
		return
	}
	data, _ := schemas.RedactionDataFromContext(ctx)
	data.LiteralReplacements.MergePhase(schemas.RedactionPhaseInput, replacements)
	schemas.SetRedactionDataOnContext(ctx, data)
	p.recordTraceRedactions(ctx, replacements)
}

// recordInspection stores what the last inspection covered, so the next attempt can tell
// "same request, same text" from "a later turn added input".
func (p *Plugin) recordInspection(ctx *schemas.BifrostContext, segments *segmentSet) {
	if segments == nil {
		return
	}
	count, bytes, sum := segments.fingerprint()
	ctx.SetValue(markerKey{}, inspectMarker{requestID: requestID(ctx), segments: count, bytes: bytes, sum: sum})
}

// inspected reports whether this request's text is the text the last inspection covered.
func inspected(ctx *schemas.BifrostContext, req *schemas.BifrostRequest) bool {
	marker, ok := ctx.Value(markerKey{}).(inspectMarker)
	if !ok || marker.requestID != requestID(ctx) {
		return false
	}
	segments, err := collectSegments(ctx, req)
	if err != nil || segments == nil {
		// Only a sampled-out request records an unsupported marker; without it PreLLMHook would
		// re-select with sampling ignored and refuse a request sampling had let through.
		return err != nil && marker.unsupported
	}
	count, bytes, sum := segments.fingerprint()
	return count == marker.segments && bytes == marker.bytes && sum == marker.sum
}

// requestID returns the ID the request is logged and correlated under.
func requestID(ctx *schemas.BifrostContext) string {
	id, _ := ctx.Value(schemas.BifrostContextKeyRequestID).(string)
	return id
}

// verdictForRequest returns the verdict only when it belongs to the request this context is
// serving: a nil-context SDK call can leave an earlier request's verdict on a shared context.
func verdictForRequest(ctx *schemas.BifrostContext) *verdict {
	blocked, _ := ctx.Value(verdictKey{}).(*verdict)
	if blocked == nil || blocked.requestID != requestID(ctx) {
		return nil
	}
	return blocked
}

// recordTraceRedactions publishes the replacements to the request's trace, so exported spans
// carry placeholders instead of the values.
func (p *Plugin) recordTraceRedactions(ctx *schemas.BifrostContext, replacements map[string]string) {
	tracer, _ := ctx.Value(schemas.BifrostContextKeyTracer).(schemas.Tracer)
	if tracer == nil {
		return
	}
	traceID, _ := ctx.Value(schemas.BifrostContextKeyTraceID).(string)
	if traceID == "" {
		return
	}
	tracer.SetTraceRedactionReplacements(traceID, schemas.RedactionPhaseInput, replacements)
}

// recordDetection appends one detection entry naming the rule, profile and entity types. It
// never records matched text.
func (p *Plugin) recordDetection(ctx *schemas.BifrostContext, outcome runOutcome, hits []finding, action string) {
	entities := uniqueEntityTypes(hits)
	if len(entities) == 0 {
		return
	}
	var ruleID *uint
	ruleName := ""
	if outcome.run.rule != nil {
		id := outcome.run.rule.id
		ruleID = &id
		ruleName = outcome.run.rule.name
	}
	schemas.AppendGuardrailDetectionOnContext(ctx, schemas.BifrostGuardrailDetection{
		Phase:             ApplyToInput,
		RuleID:            ruleID,
		RuleName:          ruleName,
		GuardrailName:     outcome.run.profile.policyName,
		GuardrailProvider: outcome.run.profile.provider,
		Reason:            "Detected " + strings.Join(entities, ", ") + " in request input",
		Assessments:       entities,
		Action:            action,
	})
}

// recordIntervention appends one blocking entry for a profile.
func (p *Plugin) recordIntervention(ctx *schemas.BifrostContext, outcome runOutcome) {
	var ruleID *uint
	ruleName := ""
	if outcome.run.rule != nil {
		id := outcome.run.rule.id
		ruleID = &id
		ruleName = outcome.run.rule.name
	}
	entities := uniqueEntityTypes(outcome.block)
	reason := "Detection failed for " + outcome.run.profile.policyName + " policy"
	if len(entities) > 0 {
		reason = "Blocked by " + outcome.run.profile.policyName + " policy: " + strings.Join(entities, ", ")
	}
	schemas.AppendGuardrailInterventionOnContext(ctx, schemas.BifrostGuardrailIntervention{
		Phase:             ApplyToInput,
		RuleID:            ruleID,
		RuleName:          ruleName,
		GuardrailName:     outcome.run.profile.policyName,
		GuardrailProvider: outcome.run.profile.provider,
		Reason:            reason,
		Assessments:       entities,
	})
}

// newVerdict collects the blocking policies and their entity types, sorted so the message a
// caller sees is deterministic, and binds the result to the request that produced it.
func newVerdict(ctx *schemas.BifrostContext, blocking []runOutcome) *verdict {
	policies := make([]string, 0, len(blocking))
	entities := make([]string, 0, len(blocking))
	failed := false
	for _, outcome := range blocking {
		policies = append(policies, outcome.run.profile.policyName)
		entities = append(entities, uniqueEntityTypes(outcome.block)...)
		if outcome.failure != nil {
			failed = true
		}
	}
	sort.Strings(entities)
	return &verdict{requestID: requestID(ctx), policies: dedupeStrings(policies), entities: dedupeStrings(entities), failed: failed}
}

// message renders the block message. It names policies and entity types only, never a value.
func (v *verdict) message() string {
	detail := v.detail
	if detail == "" {
		detail = strings.Join(v.entities, ", ")
	}
	if detail == "" {
		detail = "detection unavailable"
	}
	return "Blocked by " + strings.Join(v.policies, ", ") + " policy: " + detail
}

// bifrostError builds the short-circuit error: HTTP 400, type guardrail_intervention, and no
// fallbacks, because a fallback would send the same blocked input to another provider.
func (v *verdict) bifrostError() *schemas.BifrostError {
	errorType := guardrailInterventionErrorType
	statusCode := http.StatusBadRequest
	allowFallbacks := false
	return &schemas.BifrostError{
		Type:           &errorType,
		IsBifrostError: true,
		StatusCode:     &statusCode,
		Error:          &schemas.ErrorField{Type: &errorType, Message: v.message()},
		AllowFallbacks: &allowFallbacks,
	}
}

// PreLLMHook short-circuits a request an inspection blocked, and inspects text a later turn
// added. PreRequestHook runs once, but the prompts plugin inserts its template in this phase and
// an MCP agent continuation turn appends tool results to the same request, so both reach the
// provider through here without a second PreRequestHook. While the request's text is unchanged
// (an ordinary fallback attempt) this costs one fingerprint and runs no detector.
func (p *Plugin) PreLLMHook(ctx *schemas.BifrostContext, req *schemas.BifrostRequest) (*schemas.BifrostRequest, *schemas.LLMPluginShortCircuit, error) {
	if blocked := verdictForRequest(ctx); blocked != nil {
		return req, &schemas.LLMPluginShortCircuit{Error: blocked.bifrostError()}, nil
	}
	current := p.snapshot.Load()
	if current == nil || len(current.rules) == 0 || inspected(ctx, req) {
		return req, nil, nil
	}
	p.inspect(ctx, req, current, true)
	if blocked := verdictForRequest(ctx); blocked != nil {
		return req, &schemas.LLMPluginShortCircuit{Error: blocked.bifrostError()}, nil
	}
	return req, nil, nil
}

// PostLLMHook passes the response through: this build inspects input only.
func (p *Plugin) PostLLMHook(_ *schemas.BifrostContext, resp *schemas.BifrostResponse, bifrostErr *schemas.BifrostError) (*schemas.BifrostResponse, *schemas.BifrostError, error) {
	return resp, bifrostErr, nil
}

// uniqueEntityTypes returns the sanitized, de-duplicated entity types of hits, in first-seen
// order.
func uniqueEntityTypes(hits []finding) []string {
	entities := make([]string, 0, len(hits))
	seen := make(map[string]struct{}, len(hits))
	for _, hit := range hits {
		entity := sanitizeEntityType(hit.entityType)
		if _, ok := seen[entity]; ok {
			continue
		}
		seen[entity] = struct{}{}
		entities = append(entities, entity)
	}
	return entities
}

// dedupeStrings keeps the first occurrence of each value.
func dedupeStrings(values []string) []string {
	deduped := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		deduped = append(deduped, value)
	}
	return deduped
}
