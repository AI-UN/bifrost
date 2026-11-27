package guardrails

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"sync"
	"time"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/maximhq/bifrost/core/schemas"
)

// celEnv builds the rule-selection environment. Variable names and types are the enterprise
// guardrails contract, so a rule written there keeps working here.
var celEnv = sync.OnceValues(func() (*cel.Env, error) {
	return cel.NewEnv(
		cel.Variable("headers", cel.MapType(cel.StringType, cel.StringType)),
		cel.Variable("params", cel.MapType(cel.StringType, cel.StringType)),
		cel.Variable("virtual_key", cel.StringType),
		cel.Variable("customer", cel.StringType),
		cel.Variable("team", cel.StringType),
		cel.Variable("user", cel.StringType),
		cel.Variable("model", cel.StringType),
		cel.Variable("provider", cel.StringType),
		cel.Variable("service_tier", cel.StringType),
	)
})

// compileCEL compiles a rule's expression. An empty expression matches every request and
// compiles to a nil program, which rule selection treats as match-all.
func compileCEL(expression string) (cel.Program, error) {
	if strings.TrimSpace(expression) == "" {
		return nil, nil
	}
	env, err := celEnv()
	if err != nil {
		return nil, fmt.Errorf("CEL environment: %w", err)
	}
	ast, issues := env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("CEL compile error: %w", issues.Err())
	}
	if ast.OutputType() != cel.BoolType {
		return nil, fmt.Errorf("CEL expression must return bool, got %s", ast.OutputType())
	}
	program, err := env.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("CEL program: %w", err)
	}
	return program, nil
}

// evaluateCEL reports whether the program matched. A runtime error that means "this value is
// absent" is a no-match, as is an unknown; anything else is returned so the caller can warn.
func evaluateCEL(program cel.Program, variables map[string]any) (bool, error) {
	if program == nil {
		return true, nil
	}
	out, _, err := program.Eval(variables)
	if err != nil {
		if strings.Contains(err.Error(), "no such key") {
			return false, nil
		}
		return false, err
	}
	if types.IsUnknown(out) {
		return false, nil
	}
	matched, ok := out.Value().(bool)
	if !ok {
		return false, fmt.Errorf("CEL expression did not return bool, got %T", out.Value())
	}
	return matched, nil
}

// celVariables builds the activation for one request. The maps are passed through, not
// copied: BifrostContextKeyGuardrailHeaders and BifrostContextKeyRequestHeaders are both
// documented as lowercase-keyed by whoever sets them.
//
// The identity variables are the IDs governance resolved, which is what the documented CEL
// contract names: the raw virtual-key credential would make a rule key off the secret the
// caller presented, and a display name would miss traffic the documented ID matches.
func celVariables(ctx *schemas.BifrostContext, req *schemas.BifrostRequest) map[string]any {
	headers, _ := ctx.Value(schemas.BifrostContextKeyGuardrailHeaders).(map[string]string)
	if headers == nil {
		// The HTTP transport captures every inbound header (lowercased) before plugins run,
		// so an HTTP request needs no guardrail-specific copy; in-process callers that want
		// their own view set BifrostContextKeyGuardrailHeaders instead.
		headers, _ = ctx.Value(schemas.BifrostContextKeyRequestHeaders).(map[string]string)
	}
	params, _ := ctx.Value(schemas.BifrostContextKeyRequestQuery).(map[string]string)

	provider, model, serviceTier := requestFields(req)
	return map[string]any{
		"headers":      headers,
		"params":       params,
		"virtual_key":  contextString(ctx, schemas.BifrostContextKeyGovernanceVirtualKeyID),
		"customer":     contextString(ctx, schemas.BifrostContextKeyGovernanceCustomerID),
		"team":         contextString(ctx, schemas.BifrostContextKeyGovernanceTeamID),
		"user":         contextString(ctx, schemas.BifrostContextKeyUserID),
		"model":        model,
		"provider":     provider,
		"service_tier": serviceTier,
	}
}

// contextString reads a string value governance resolved.
func contextString(ctx *schemas.BifrostContext, key schemas.BifrostContextKey) string {
	value, _ := ctx.Value(key).(string)
	return value
}

// requestFields reads the model, provider and requested service tier the rule variables see.
func requestFields(req *schemas.BifrostRequest) (provider, model, serviceTier string) {
	if req == nil {
		return "", "", ""
	}
	switch {
	case req.ChatRequest != nil:
		provider, model = string(req.ChatRequest.Provider), req.ChatRequest.Model
		if params := req.ChatRequest.Params; params != nil && params.ServiceTier != nil {
			serviceTier = string(*params.ServiceTier)
		}
	case req.ResponsesRequest != nil:
		provider, model = string(req.ResponsesRequest.Provider), req.ResponsesRequest.Model
		if params := req.ResponsesRequest.Params; params != nil && params.ServiceTier != nil {
			serviceTier = string(*params.ServiceTier)
		}
	case req.CountTokensRequest != nil:
		provider, model = string(req.CountTokensRequest.Provider), req.CountTokensRequest.Model
	case req.TextCompletionRequest != nil:
		provider, model = string(req.TextCompletionRequest.Provider), req.TextCompletionRequest.Model
	case req.EmbeddingRequest != nil:
		provider, model = string(req.EmbeddingRequest.Provider), req.EmbeddingRequest.Model
	case req.SpeechRequest != nil:
		provider, model = string(req.SpeechRequest.Provider), req.SpeechRequest.Model
	case req.ImageGenerationRequest != nil:
		provider, model = string(req.ImageGenerationRequest.Provider), req.ImageGenerationRequest.Model
	case req.ImageEditRequest != nil:
		provider, model = string(req.ImageEditRequest.Provider), req.ImageEditRequest.Model
	case req.PassthroughRequest != nil:
		provider, model = string(req.PassthroughRequest.Provider), req.PassthroughRequest.Model
	}
	return provider, model, serviceTier
}

// sampled reports whether a rule runs for this request. A nil or 100 rate always runs and a
// 0 rate never does, so both ends are deterministic.
func sampled(rate int) bool {
	if rate >= 100 {
		return true
	}
	if rate <= 0 {
		return false
	}
	return rand.IntN(100) < rate
}

// selectProfiles evaluates every rule and returns the profiles to run: each profile once, in
// rule order then profile order. A rule's timeout is one absolute deadline its profiles share,
// so a chain of slow profiles cannot multiply the budget it was configured with; a profile's
// own timeout caps its share further.
//
// reinspect is the PreLLMHook pass: sampling decides once per request whether a rule runs, so a
// rule that already ran must not be sampled out again when a later turn changed the input.
func (s *snapshot) selectProfiles(ctx *schemas.BifrostContext, req *schemas.BifrostRequest, logger schemas.Logger, reinspect bool) []profileRun {
	variables := celVariables(ctx, req)
	runs := make([]profileRun, 0, 2)
	selected := make(map[uint]int, 2)
	now := time.Now()
	for _, rule := range s.rules {
		matched, err := evaluateCEL(rule.program, variables)
		if err != nil {
			// A rule whose expression fails at runtime is treated as not matching.
			logger.Warn("[Guardrails] rule %d (%s) CEL evaluation failed, treating it as not matching: %v", rule.id, rule.name, err)
			continue
		}
		if !matched || (!reinspect && !sampled(rule.sampling)) {
			continue
		}
		ruleDeadline := now.Add(time.Duration(rule.timeout) * time.Second)
		for _, profile := range rule.profiles {
			deadline := ruleDeadline
			if profileDeadline := now.Add(time.Duration(profile.timeout) * time.Second); profileDeadline.Before(deadline) {
				deadline = profileDeadline
			}
			if at, ok := selected[profile.id]; ok {
				if deadline.Before(runs[at].deadline) {
					runs[at].deadline = deadline
				}
				continue
			}
			selected[profile.id] = len(runs)
			runs = append(runs, profileRun{profile: profile, rule: rule, deadline: deadline})
		}
	}
	return runs
}

// samplingRate resolves a rule's sampling rate.
func samplingRate(r Rule) int {
	if r.SamplingRate == nil {
		return 100
	}
	return *r.SamplingRate
}
