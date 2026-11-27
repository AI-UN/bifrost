// This file contains the guardrail profile and rule CRUD endpoints. Profiles are the
// configured detectors (secrets, custom regex, Presidio); rules select which requests they
// inspect. Both live in the config store and are applied by the guardrails plugin, which
// every successful write asks to rebuild its in-memory snapshot.
package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/fasthttp/router"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
	configstoreTables "github.com/maximhq/bifrost/framework/configstore/tables"
	"github.com/maximhq/bifrost/plugins/guardrails"
	"github.com/maximhq/bifrost/transports/v2/bifrost-http/lib"
	"github.com/valyala/fasthttp"
	"gorm.io/gorm"
)

// guardrailDefaultTimeout bounds a verify or entity lookup that has no profile timeout of its
// own, so an unreachable Analyzer fails the request instead of hanging it.
const guardrailDefaultTimeout = 30

// guardrailProviderNames lists the providers this build runs, in the order the API reports
// them. A path segment that is not one of these is a 404, never a new provider.
var guardrailProviderNames = []string{
	guardrails.ProviderSecrets,
	guardrails.ProviderRegex,
	guardrails.ProviderPresidio,
}

// GuardrailsHandler manages HTTP requests for guardrail profiles and rules.
type GuardrailsHandler struct {
	configStore configstore.ConfigStore
	// config locates the live plugin after a write. It is optional so the handler can be
	// built without a server (then a write is applied on the next reload).
	config *lib.Config
}

// NewGuardrailsHandler creates a guardrails handler. The config store is required: profiles
// and rules are persisted before the plugin is asked to apply them.
func NewGuardrailsHandler(configStore configstore.ConfigStore, config *lib.Config) (*GuardrailsHandler, error) {
	if configStore == nil {
		return nil, fmt.Errorf("config store is required")
	}
	return &GuardrailsHandler{configStore: configStore, config: config}, nil
}

// RegisterRoutes registers the guardrail endpoints.
//
// Static paths are registered before /api/guardrails/{provider}: the router returns the first
// child that matches a path segment, so a parameter sibling registered first would swallow
// "rules" or "presidio" as a provider name.
func (h *GuardrailsHandler) RegisterRoutes(r *router.Router, middlewares ...schemas.BifrostHTTPMiddleware) {
	wrap := func(handler fasthttp.RequestHandler) fasthttp.RequestHandler {
		return lib.ChainMiddlewares(handler, middlewares...)
	}

	r.Handle(fasthttp.MethodGet, "/api/guardrails", wrap(h.getGuardrails))
	r.Handle(fasthttp.MethodGet, "/api/guardrails/secrets/rules", wrap(h.getSecretRules))
	r.Handle(fasthttp.MethodPost, "/api/guardrails/presidio/supported-entities", wrap(h.getPresidioEntities))
	r.Handle(fasthttp.MethodGet, "/api/guardrails/rules", wrap(h.getGuardrailRules))
	r.Handle(fasthttp.MethodPost, "/api/guardrails/rules", wrap(h.createGuardrailRule))
	r.Handle(fasthttp.MethodPut, "/api/guardrails/rules/{rule_id}", wrap(h.updateGuardrailRule))
	r.Handle(fasthttp.MethodDelete, "/api/guardrails/rules/{rule_id}", wrap(h.deleteGuardrailRule))
	r.Handle(fasthttp.MethodPatch, "/api/guardrails/rules/{rule_id}/toggle", wrap(h.toggleGuardrailRule))

	r.Handle(fasthttp.MethodGet, "/api/guardrails/{provider}", wrap(h.getProviderProfiles))
	r.Handle(fasthttp.MethodPost, "/api/guardrails/{provider}", wrap(h.createProfile))
	r.Handle(fasthttp.MethodPut, "/api/guardrails/{provider}", wrap(h.updateProfile))
	r.Handle(fasthttp.MethodDelete, "/api/guardrails/{provider}", wrap(h.deleteProfile))
	r.Handle(fasthttp.MethodPost, "/api/guardrails/{provider}/verify", wrap(h.verifyProfile))
}

// guardrailProviderGroup is one provider's profiles in the GET shapes.
type guardrailProviderGroup struct {
	Name    string           `json:"name"`
	Configs []map[string]any `json:"configs"`
}

// getGuardrails returns every provider group, always the fixed three.
func (h *GuardrailsHandler) getGuardrails(ctx *fasthttp.RequestCtx) {
	profiles, err := h.configStore.GetGuardrailProviders(ctx)
	if err != nil {
		logger.Error("failed to retrieve guardrail profiles: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, "Failed to retrieve guardrail profiles")
		return
	}
	groups := make([]guardrailProviderGroup, 0, len(guardrailProviderNames))
	for _, provider := range guardrailProviderNames {
		group := guardrailProviderGroup{Name: provider, Configs: []map[string]any{}}
		for _, profile := range profiles {
			if profile.ProviderName != provider {
				continue
			}
			view, err := guardrailProfileView(profile)
			if err != nil {
				logger.Error("failed to render guardrail profile %d: %v", profile.ID, err)
				SendError(ctx, fasthttp.StatusInternalServerError, "Failed to retrieve guardrail profiles")
				return
			}
			group.Configs = append(group.Configs, view)
		}
		groups = append(groups, group)
	}
	SendJSON(ctx, groups)
}

// getProviderProfiles returns one provider's profiles.
func (h *GuardrailsHandler) getProviderProfiles(ctx *fasthttp.RequestCtx) {
	provider, ok := guardrailProviderFromPath(ctx)
	if !ok {
		SendError(ctx, fasthttp.StatusNotFound, guardrailUnknownProviderMessage(ctx))
		return
	}
	profiles, err := h.configStore.GetGuardrailProviders(ctx)
	if err != nil {
		logger.Error("failed to retrieve guardrail profiles: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, "Failed to retrieve guardrail profiles")
		return
	}
	group := guardrailProviderGroup{Name: provider, Configs: []map[string]any{}}
	for _, profile := range profiles {
		if profile.ProviderName != provider {
			continue
		}
		view, err := guardrailProfileView(profile)
		if err != nil {
			logger.Error("failed to render guardrail profile %d: %v", profile.ID, err)
			SendError(ctx, fasthttp.StatusInternalServerError, "Failed to retrieve guardrail profiles")
			return
		}
		group.Configs = append(group.Configs, view)
	}
	SendJSON(ctx, group)
}

// guardrailProfileRequest is the profile write body. config holds the provider settings with
// an optional timeout, which is stored in its own column.
type guardrailProfileRequest struct {
	ID      uint                       `json:"id"`
	Name    *string                    `json:"name"`
	Enabled *bool                      `json:"enabled"`
	Config  map[string]json.RawMessage `json:"config"`
}

// createProfile creates one profile for the provider in the path.
func (h *GuardrailsHandler) createProfile(ctx *fasthttp.RequestCtx) {
	provider, ok := guardrailProviderFromPath(ctx)
	if !ok {
		SendError(ctx, fasthttp.StatusNotFound, guardrailUnknownProviderMessage(ctx))
		return
	}
	var req guardrailProfileRequest
	if err := sonic.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, "Invalid JSON")
		return
	}
	if req.Name == nil || strings.TrimSpace(*req.Name) == "" {
		SendError(ctx, fasthttp.StatusBadRequest, "name field is required")
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	timeout, config, err := normalizeGuardrailProfileConfig(req.Config, 0)
	if err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, err.Error())
		return
	}
	profile, err := newGuardrailProfile(provider, 0, strings.TrimSpace(*req.Name), enabled, timeout, config)
	if err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, err.Error())
		return
	}
	if err := h.configStore.CreateGuardrailProvider(ctx, &profile); err != nil {
		logger.Error("failed to create guardrail profile: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, "Failed to create guardrail profile")
		return
	}
	view, err := guardrailProfileView(profile)
	if err != nil {
		logger.Error("failed to render guardrail profile %d: %v", profile.ID, err)
		SendError(ctx, fasthttp.StatusInternalServerError, "Failed to create guardrail profile")
		return
	}
	lib.RecordAuditChange(ctx, "guardrail_profile.created", "guardrail_profile",
		strconv.FormatUint(uint64(profile.ID), 10), profile.PolicyName, nil, view)
	if err := h.reloadGuardrails(ctx); err != nil {
		sendSavedNotActivated(ctx, err)
		return
	}
	SendJSON(ctx, map[string]any{"config": view})
}

// updateProfile merges a partial profile update onto the stored profile.
func (h *GuardrailsHandler) updateProfile(ctx *fasthttp.RequestCtx) {
	provider, ok := guardrailProviderFromPath(ctx)
	if !ok {
		SendError(ctx, fasthttp.StatusNotFound, guardrailUnknownProviderMessage(ctx))
		return
	}
	var req guardrailProfileRequest
	if err := sonic.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, "Invalid JSON")
		return
	}
	if req.ID == 0 {
		SendError(ctx, fasthttp.StatusBadRequest, "id field is required")
		return
	}
	existing, err := h.configStore.GetGuardrailProvider(ctx, req.ID)
	if err != nil {
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, fasthttp.StatusNotFound, "Guardrail profile not found")
			return
		}
		logger.Error("failed to get guardrail profile: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, "Failed to retrieve guardrail profile")
		return
	}
	// A profile belongs to one provider: writing it through another provider's path would
	// silently change what it validates against.
	if existing.ProviderName != provider {
		SendError(ctx, fasthttp.StatusNotFound, "Guardrail profile not found")
		return
	}
	before, err := guardrailProfileView(*existing)
	if err != nil {
		logger.Error("failed to render guardrail profile %d: %v", existing.ID, err)
		SendError(ctx, fasthttp.StatusInternalServerError, "Failed to retrieve guardrail profile")
		return
	}

	stored, err := decodeGuardrailConfig(existing.ConfigJSON)
	if err != nil {
		logger.Error("failed to decode guardrail profile %d: %v", existing.ID, err)
		SendError(ctx, fasthttp.StatusInternalServerError, "Failed to retrieve guardrail profile")
		return
	}
	merged, err := mergeGuardrailConfig(stored, req.Config)
	if err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, err.Error())
		return
	}
	timeout, config, err := normalizeGuardrailProfileConfig(merged, existing.Timeout)
	if err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, err.Error())
		return
	}
	policyName := existing.PolicyName
	if req.Name != nil && strings.TrimSpace(*req.Name) != "" {
		policyName = strings.TrimSpace(*req.Name)
	}
	enabled := existing.Enabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	profile, err := newGuardrailProfile(provider, existing.ID, policyName, enabled, timeout, config)
	if err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, err.Error())
		return
	}
	// The stored hash records what config.json last wrote, which is what lets a startup
	// reconcile leave this edit alone in split mode.
	profile.ConfigHash = existing.ConfigHash
	if err := h.configStore.UpdateGuardrailProvider(ctx, &profile); err != nil {
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, fasthttp.StatusNotFound, "Guardrail profile not found")
			return
		}
		logger.Error("failed to update guardrail profile: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, "Failed to update guardrail profile")
		return
	}
	after, err := guardrailProfileView(profile)
	if err != nil {
		logger.Error("failed to render guardrail profile %d: %v", profile.ID, err)
		SendError(ctx, fasthttp.StatusInternalServerError, "Failed to update guardrail profile")
		return
	}
	lib.RecordAuditChange(ctx, "guardrail_profile.updated", "guardrail_profile",
		strconv.FormatUint(uint64(profile.ID), 10), profile.PolicyName, before, after)
	if err := h.reloadGuardrails(ctx); err != nil {
		sendSavedNotActivated(ctx, err)
		return
	}
	SendJSON(ctx, map[string]any{"config": after})
}

// deleteProfile deletes a profile that no rule references.
func (h *GuardrailsHandler) deleteProfile(ctx *fasthttp.RequestCtx) {
	provider, ok := guardrailProviderFromPath(ctx)
	if !ok {
		SendError(ctx, fasthttp.StatusNotFound, guardrailUnknownProviderMessage(ctx))
		return
	}
	var req struct {
		ID uint `json:"id"`
	}
	if err := sonic.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, "Invalid JSON")
		return
	}
	if req.ID == 0 {
		SendError(ctx, fasthttp.StatusBadRequest, "id field is required")
		return
	}
	existing, err := h.configStore.GetGuardrailProvider(ctx, req.ID)
	if err != nil {
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, fasthttp.StatusNotFound, "Guardrail profile not found")
			return
		}
		logger.Error("failed to get guardrail profile: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, "Failed to retrieve guardrail profile")
		return
	}
	if existing.ProviderName != provider {
		SendError(ctx, fasthttp.StatusNotFound, "Guardrail profile not found")
		return
	}
	rules, _, err := h.configStore.GetGuardrailRules(ctx, configstore.GuardrailRulesQueryParams{})
	if err != nil {
		logger.Error("failed to retrieve guardrail rules: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, "Failed to retrieve guardrail rules")
		return
	}
	for _, rule := range rules {
		if slices.Contains(rule.ProviderConfigIDs, req.ID) {
			SendError(ctx, fasthttp.StatusConflict,
				fmt.Sprintf("Guardrail profile is used by rule %q; remove it from that rule first", rule.Name))
			return
		}
	}
	before, err := guardrailProfileView(*existing)
	if err != nil {
		logger.Error("failed to render guardrail profile %d: %v", existing.ID, err)
		SendError(ctx, fasthttp.StatusInternalServerError, "Failed to delete guardrail profile")
		return
	}
	// The reference check above is a courtesy so a caller sees the conflict without a write; the
	// delete itself re-checks inside its transaction, so a rule created concurrently cannot
	// commit a reference to a profile that is gone.
	var inUse *guardrailProfileInUseError
	err = h.configStore.ExecuteTransaction(ctx, func(tx *gorm.DB) error {
		rules, _, err := h.configStore.GetGuardrailRules(ctx, configstore.GuardrailRulesQueryParams{}, tx)
		if err != nil {
			return err
		}
		for _, rule := range rules {
			if slices.Contains(rule.ProviderConfigIDs, req.ID) {
				return &guardrailProfileInUseError{ruleName: rule.Name}
			}
		}
		return h.configStore.DeleteGuardrailProvider(ctx, req.ID, tx)
	})
	if err != nil {
		if errors.As(err, &inUse) {
			SendError(ctx, fasthttp.StatusConflict, inUse.Error())
			return
		}
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, fasthttp.StatusNotFound, "Guardrail profile not found")
			return
		}
		logger.Error("failed to delete guardrail profile: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, "Failed to delete guardrail profile")
		return
	}
	lib.RecordAuditChange(ctx, "guardrail_profile.deleted", "guardrail_profile",
		strconv.FormatUint(uint64(req.ID), 10), existing.PolicyName, before, nil)
	if err := h.reloadGuardrails(ctx); err != nil {
		sendSavedNotActivated(ctx, err)
		return
	}
	SendJSON(ctx, map[string]string{"message": "Guardrail profile deleted successfully"})
}

// guardrailProfileInUseError reports a profile a rule still runs, so the delete can answer 409
// from inside its transaction without matching on message text.
type guardrailProfileInUseError struct{ ruleName string }

func (e *guardrailProfileInUseError) Error() string {
	return fmt.Sprintf("Guardrail profile is used by rule %q; remove it from that rule first", e.ruleName)
}

// guardrailVerifyRequest is the verify body. An id names the stored profile whose secret and
// settings fill in whatever the request omits.
type guardrailVerifyRequest struct {
	ID     uint                       `json:"id"`
	Name   string                     `json:"name"`
	Config map[string]json.RawMessage `json:"config"`
}

// verifyProfile checks a profile without saving it: its config is valid and, for presidio,
// the Analyzer answers /health.
func (h *GuardrailsHandler) verifyProfile(ctx *fasthttp.RequestCtx) {
	provider, ok := guardrailProviderFromPath(ctx)
	if !ok {
		SendError(ctx, fasthttp.StatusNotFound, guardrailUnknownProviderMessage(ctx))
		return
	}
	var req guardrailVerifyRequest
	if err := sonic.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, "Invalid JSON")
		return
	}
	profile, err := h.profileFromRequest(ctx, provider, req.ID, req.Name, req.Config)
	if err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, err.Error())
		return
	}
	callCtx, cancel := guardrailCallContext(ctx, profile.Timeout)
	defer cancel()
	if err := guardrails.VerifyProfile(callCtx, guardrails.Profile{
		ID:           profile.ID,
		ProviderName: profile.ProviderName,
		PolicyName:   profile.PolicyName,
		Enabled:      profile.Enabled,
		Timeout:      profile.Timeout,
		Config:       json.RawMessage(profile.ConfigJSON),
	}); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("Verification failed: %v", err))
		return
	}
	SendJSON(ctx, map[string]bool{"ok": true})
}

// getSecretRules serves the gitleaks rule catalog the secrets profile picks from.
func (h *GuardrailsHandler) getSecretRules(ctx *fasthttp.RequestCtx) {
	SendJSON(ctx, map[string]any{"rules": guardrails.SecretsRuleCatalog()})
}

// getPresidioEntities asks a Presidio Analyzer which entities it supports. The lookup runs
// here because an internal Analyzer is often unreachable from a browser.
func (h *GuardrailsHandler) getPresidioEntities(ctx *fasthttp.RequestCtx) {
	var req struct {
		ID     uint                       `json:"id"`
		Config map[string]json.RawMessage `json:"config"`
	}
	if err := sonic.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, "Invalid JSON")
		return
	}
	// Reject a misspelled key before anything is decoded: the decoder matches field names
	// case-insensitively, so it would otherwise accept API_KEY as api_key.
	if err := lib.ValidateGuardrailConfigKeys(guardrails.ProviderPresidio, req.Config); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, err.Error())
		return
	}
	config, err := h.mergeStoredConfig(ctx, req.ID, guardrails.ProviderPresidio, req.Config)
	if err != nil {
		if errors.Is(err, errGuardrailAnalyzerMoved) {
			SendError(ctx, fasthttp.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, fasthttp.StatusNotFound, "Guardrail profile not found")
			return
		}
		logger.Error("failed to load guardrail profile %d: %v", req.ID, err)
		SendError(ctx, fasthttp.StatusInternalServerError, "Failed to retrieve guardrail profile")
		return
	}
	raw, err := sonic.Marshal(config)
	if err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, "config must be a JSON object")
		return
	}
	var presidio guardrails.PresidioConfig
	if err := sonic.Unmarshal(raw, &presidio); err != nil {
		// Fixed message: the decoder's own error quotes surrounding bytes of the merged config,
		// which carries the stored credential.
		SendError(ctx, fasthttp.StatusBadRequest, "config does not decode as a presidio profile")
		return
	}
	callCtx, cancel := guardrailCallContext(ctx, 0)
	defer cancel()
	entities, err := guardrails.PresidioSupportedEntities(callCtx, presidio)
	if err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("Failed to load supported entities: %v", err))
		return
	}
	SendJSON(ctx, map[string][]string{"entities": entities})
}

// guardrailRuleRequest is a rule write body. The settings this build does not run are accepted
// only at their defaults, so a rule authored against them fails loudly instead of silently
// behaving differently.
type guardrailRuleRequest struct {
	Name                        string   `json:"name"`
	Description                 string   `json:"description"`
	Enabled                     *bool    `json:"enabled"`
	Target                      string   `json:"target"`
	CELExpression               string   `json:"celExpression"`
	ApplyTo                     string   `json:"applyTo"`
	SamplingRate                *int     `json:"samplingRate"`
	Timeout                     int      `json:"timeout"`
	SelectedGuardrailProfiles   []string `json:"selectedGuardrailProfiles"`
	Query                       any      `json:"query"`
	OutputMode                  *string  `json:"outputMode"`
	InputTiming                 *string  `json:"inputTiming"`
	EvaluationMode              *string  `json:"evaluationMode"`
	ProfileExecutionMode        *string  `json:"profileExecutionMode"`
	StreamReplayEventIntervalMs *int     `json:"streamReplayEventIntervalMs"`
}

// guardrailRuleView is a rule in the API shape: camelCase fields and the profiles it runs as
// "<provider>:<id>" references, in execution order.
type guardrailRuleView struct {
	ID                        uint      `json:"id"`
	Name                      string    `json:"name"`
	Description               string    `json:"description"`
	Enabled                   bool      `json:"enabled"`
	Target                    string    `json:"target"`
	CELExpression             string    `json:"celExpression"`
	ApplyTo                   string    `json:"applyTo"`
	SamplingRate              int       `json:"samplingRate"`
	Timeout                   int       `json:"timeout"`
	SelectedGuardrailProfiles []string  `json:"selectedGuardrailProfiles"`
	Query                     any       `json:"query"`
	CreatedAt                 time.Time `json:"createdAt"`
	UpdatedAt                 time.Time `json:"updatedAt"`
}

// getGuardrailRules lists rules by id. count is the total row count, so a client can page.
func (h *GuardrailsHandler) getGuardrailRules(ctx *fasthttp.RequestCtx) {
	limit, offset := 0, 0
	if raw := string(ctx.QueryArgs().Peek("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			SendError(ctx, fasthttp.StatusBadRequest, "limit must be a non-negative integer")
			return
		}
		limit = parsed
	}
	if raw := string(ctx.QueryArgs().Peek("offset")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			SendError(ctx, fasthttp.StatusBadRequest, "offset must be a non-negative integer")
			return
		}
		offset = parsed
	}
	limit, offset = ClampPaginationParams(limit, offset)

	rules, total, err := h.configStore.GetGuardrailRules(ctx, configstore.GuardrailRulesQueryParams{Limit: limit, Offset: offset})
	if err != nil {
		logger.Error("failed to retrieve guardrail rules: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, "Failed to retrieve guardrail rules")
		return
	}
	providers, err := h.guardrailProviders(ctx)
	if err != nil {
		logger.Error("failed to retrieve guardrail profiles: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, "Failed to retrieve guardrail rules")
		return
	}
	views := make([]guardrailRuleView, 0, len(rules))
	for _, rule := range rules {
		views = append(views, guardrailRuleViewOf(rule, providers))
	}
	SendJSON(ctx, map[string]any{
		"count":  total,
		"limit":  limit,
		"offset": offset,
		"rules":  views,
	})
}

// createGuardrailRule creates a rule.
func (h *GuardrailsHandler) createGuardrailRule(ctx *fasthttp.RequestCtx) {
	var req guardrailRuleRequest
	if err := sonic.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, "Invalid JSON")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		SendError(ctx, fasthttp.StatusBadRequest, "name field is required")
		return
	}
	// The profile references are resolved inside the write's transaction, so a profile deleted
	// concurrently cannot commit a rule that points at nothing.
	var rule configstoreTables.TableGuardrailRule
	var providers map[uint]string
	err := h.configStore.ExecuteTransaction(ctx, func(tx *gorm.DB) error {
		var err error
		if providers, err = h.guardrailProviders(ctx, tx); err != nil {
			return err
		}
		rule, err = h.ruleFromRequest(0, req, true, providers)
		if err != nil {
			return &guardrailInvalidError{err: err}
		}
		return h.configStore.CreateGuardrailRule(ctx, &rule, tx)
	})
	if err != nil {
		var invalid *guardrailInvalidError
		if errors.As(err, &invalid) {
			SendError(ctx, fasthttp.StatusBadRequest, invalid.Error())
			return
		}
		logger.Error("failed to create guardrail rule: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, "Failed to create guardrail rule")
		return
	}
	view := guardrailRuleViewOf(rule, providers)
	lib.RecordAuditChange(ctx, "guardrail_rule.created", "guardrail_rule",
		strconv.FormatUint(uint64(rule.ID), 10), rule.Name, nil, view)
	if err := h.reloadGuardrails(ctx); err != nil {
		sendSavedNotActivated(ctx, err)
		return
	}
	SendJSON(ctx, map[string]any{"rule": view})
}

// updateGuardrailRule replaces a rule's settings.
func (h *GuardrailsHandler) updateGuardrailRule(ctx *fasthttp.RequestCtx) {
	id, ok := guardrailRuleIDFromPath(ctx)
	if !ok {
		SendError(ctx, fasthttp.StatusBadRequest, "invalid rule id")
		return
	}
	var req guardrailRuleRequest
	if err := sonic.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, "Invalid JSON")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		SendError(ctx, fasthttp.StatusBadRequest, "name field is required")
		return
	}
	existing, err := h.configStore.GetGuardrailRule(ctx, id)
	if err != nil {
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, fasthttp.StatusNotFound, "Guardrail rule not found")
			return
		}
		logger.Error("failed to get guardrail rule: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, "Failed to retrieve guardrail rule")
		return
	}
	var rule configstoreTables.TableGuardrailRule
	var providers map[uint]string
	err = h.configStore.ExecuteTransaction(ctx, func(tx *gorm.DB) error {
		var err error
		if providers, err = h.guardrailProviders(ctx, tx); err != nil {
			return err
		}
		if rule, err = h.ruleFromRequest(id, req, existing.Enabled, providers); err != nil {
			return &guardrailInvalidError{err: err}
		}
		// created_at is immutable and the store preserves it; keep the response consistent with
		// what a later read returns.
		rule.CreatedAt = existing.CreatedAt
		// The stored hash records what config.json last wrote, so a startup reconcile can tell a
		// declared rule that has not changed from one this edit replaced.
		rule.ConfigHash = existing.ConfigHash
		return h.configStore.UpdateGuardrailRule(ctx, &rule, tx)
	})
	if err != nil {
		var invalid *guardrailInvalidError
		if errors.As(err, &invalid) {
			SendError(ctx, fasthttp.StatusBadRequest, invalid.Error())
			return
		}
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, fasthttp.StatusNotFound, "Guardrail rule not found")
			return
		}
		logger.Error("failed to update guardrail rule: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, "Failed to update guardrail rule")
		return
	}
	before := guardrailRuleViewOf(*existing, providers)
	view := guardrailRuleViewOf(rule, providers)
	lib.RecordAuditChange(ctx, "guardrail_rule.updated", "guardrail_rule",
		strconv.FormatUint(uint64(rule.ID), 10), rule.Name, before, view)
	if err := h.reloadGuardrails(ctx); err != nil {
		sendSavedNotActivated(ctx, err)
		return
	}
	SendJSON(ctx, map[string]any{"rule": view})
}

// deleteGuardrailRule deletes a rule.
func (h *GuardrailsHandler) deleteGuardrailRule(ctx *fasthttp.RequestCtx) {
	id, ok := guardrailRuleIDFromPath(ctx)
	if !ok {
		SendError(ctx, fasthttp.StatusBadRequest, "invalid rule id")
		return
	}
	existing, err := h.configStore.GetGuardrailRule(ctx, id)
	if err != nil {
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, fasthttp.StatusNotFound, "Guardrail rule not found")
			return
		}
		logger.Error("failed to get guardrail rule: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, "Failed to retrieve guardrail rule")
		return
	}
	providers, err := h.guardrailProviders(ctx)
	if err != nil {
		logger.Error("failed to retrieve guardrail profiles: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, "Failed to retrieve guardrail rule")
		return
	}
	before := guardrailRuleViewOf(*existing, providers)
	if err := h.configStore.DeleteGuardrailRule(ctx, id); err != nil {
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, fasthttp.StatusNotFound, "Guardrail rule not found")
			return
		}
		logger.Error("failed to delete guardrail rule: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, "Failed to delete guardrail rule")
		return
	}
	lib.RecordAuditChange(ctx, "guardrail_rule.deleted", "guardrail_rule",
		strconv.FormatUint(uint64(id), 10), existing.Name, before, nil)
	if err := h.reloadGuardrails(ctx); err != nil {
		sendSavedNotActivated(ctx, err)
		return
	}
	SendJSON(ctx, map[string]string{"message": "Guardrail rule deleted successfully"})
}

// toggleGuardrailRule enables or disables a rule without touching its settings.
func (h *GuardrailsHandler) toggleGuardrailRule(ctx *fasthttp.RequestCtx) {
	id, ok := guardrailRuleIDFromPath(ctx)
	if !ok {
		SendError(ctx, fasthttp.StatusBadRequest, "invalid rule id")
		return
	}
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if err := sonic.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, "Invalid JSON")
		return
	}
	if req.Enabled == nil {
		SendError(ctx, fasthttp.StatusBadRequest, "enabled field is required")
		return
	}
	rule, err := h.configStore.GetGuardrailRule(ctx, id)
	if err != nil {
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, fasthttp.StatusNotFound, "Guardrail rule not found")
			return
		}
		logger.Error("failed to get guardrail rule: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, "Failed to retrieve guardrail rule")
		return
	}
	providers, err := h.guardrailProviders(ctx)
	if err != nil {
		logger.Error("failed to retrieve guardrail profiles: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, "Failed to retrieve guardrail rule")
		return
	}
	before := guardrailRuleViewOf(*rule, providers)
	wasEnabled := rule.Enabled
	rule.Enabled = *req.Enabled
	if err := h.configStore.UpdateGuardrailRule(ctx, rule); err != nil {
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, fasthttp.StatusNotFound, "Guardrail rule not found")
			return
		}
		logger.Error("failed to update guardrail rule: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, "Failed to update guardrail rule")
		return
	}
	view := guardrailRuleViewOf(*rule, providers)
	event := "guardrail_rule.enabled"
	if !rule.Enabled {
		event = "guardrail_rule.disabled"
	}
	if wasEnabled == rule.Enabled {
		event = "guardrail_rule.updated"
	}
	lib.RecordAuditChange(ctx, event, "guardrail_rule",
		strconv.FormatUint(uint64(rule.ID), 10), rule.Name, before, view)
	if err := h.reloadGuardrails(ctx); err != nil {
		sendSavedNotActivated(ctx, err)
		return
	}
	SendJSON(ctx, map[string]any{"rule": view})
}

// ruleFromRequest validates a rule body and builds the row to store. defaultEnabled applies
// when the body omits enabled: a new rule starts enabled, an edited one keeps its state.
func (h *GuardrailsHandler) ruleFromRequest(id uint, req guardrailRuleRequest, defaultEnabled bool, providers map[uint]string) (configstoreTables.TableGuardrailRule, error) {
	if err := validateGuardrailRuleUnsupported(req); err != nil {
		return configstoreTables.TableGuardrailRule{}, err
	}
	profileIDs, err := resolveGuardrailProfileRefs(req.SelectedGuardrailProfiles, providers)
	if err != nil {
		return configstoreTables.TableGuardrailRule{}, err
	}
	target := req.Target
	if target == "" {
		target = guardrails.TargetLLM
	}
	applyTo := req.ApplyTo
	if applyTo == "" {
		applyTo = guardrails.ApplyToInput
	}
	rule := configstoreTables.TableGuardrailRule{
		ID:                id,
		Name:              strings.TrimSpace(req.Name),
		Description:       req.Description,
		Enabled:           defaultEnabled,
		Target:            target,
		CELExpression:     req.CELExpression,
		ApplyTo:           applyTo,
		SamplingRate:      req.SamplingRate,
		Timeout:           req.Timeout,
		ProviderConfigIDs: profileIDs,
	}
	if req.Enabled != nil {
		rule.Enabled = *req.Enabled
	}
	if req.Query != nil {
		query, err := sonic.Marshal(req.Query)
		if err != nil {
			return configstoreTables.TableGuardrailRule{}, fmt.Errorf("query does not encode: %w", err)
		}
		rule.QueryJSON = string(query)
	}
	if err := guardrails.ValidateRule(guardrails.Rule{
		ID:            rule.ID,
		Name:          rule.Name,
		Description:   rule.Description,
		Enabled:       rule.Enabled,
		Target:        rule.Target,
		CELExpression: rule.CELExpression,
		ApplyTo:       rule.ApplyTo,
		SamplingRate:  rule.SamplingRate,
		Timeout:       rule.Timeout,
		ProfileIDs:    rule.ProviderConfigIDs,
	}); err != nil {
		return configstoreTables.TableGuardrailRule{}, err
	}
	return rule, nil
}

// validateGuardrailRuleUnsupported rejects a rule that asks for behaviour this build does not
// implement. Silently ignoring the field would give the caller a rule that does something
// else than what it says.
func validateGuardrailRuleUnsupported(req guardrailRuleRequest) error {
	return lib.GuardrailRuleSettings{
		OutputMode:                  req.OutputMode,
		InputTiming:                 req.InputTiming,
		EvaluationMode:              req.EvaluationMode,
		ProfileExecutionMode:        req.ProfileExecutionMode,
		StreamReplayEventIntervalMs: req.StreamReplayEventIntervalMs,
	}.Validate()
}

// guardrailRuleViewOf renders a stored rule.
func guardrailRuleViewOf(rule configstoreTables.TableGuardrailRule, providers map[uint]string) guardrailRuleView {
	refs := make([]string, 0, len(rule.ProviderConfigIDs))
	for _, id := range rule.ProviderConfigIDs {
		// A reference whose profile was pruned from config.json names nothing that runs, so it
		// is left out rather than rendered as an unknown provider.
		if provider, ok := providers[id]; ok {
			refs = append(refs, provider+":"+strconv.FormatUint(uint64(id), 10))
		}
	}
	sampling := 100
	if rule.SamplingRate != nil {
		sampling = *rule.SamplingRate
	}
	target := rule.Target
	if target == "" {
		target = guardrails.TargetLLM
	}
	applyTo := rule.ApplyTo
	if applyTo == "" {
		applyTo = guardrails.ApplyToInput
	}
	view := guardrailRuleView{
		ID:                        rule.ID,
		Name:                      rule.Name,
		Description:               rule.Description,
		Enabled:                   rule.Enabled,
		Target:                    target,
		CELExpression:             rule.CELExpression,
		ApplyTo:                   applyTo,
		SamplingRate:              sampling,
		Timeout:                   rule.Timeout,
		SelectedGuardrailProfiles: refs,
		CreatedAt:                 rule.CreatedAt,
		UpdatedAt:                 rule.UpdatedAt,
	}
	if raw := strings.TrimSpace(rule.QueryJSON); raw != "" {
		var query any
		if err := sonic.Unmarshal([]byte(raw), &query); err == nil {
			view.Query = query
		}
	}
	return view
}

// profileFromRequest builds a profile from a request body, filling the gaps from the stored
// profile an id names. It does not write anything.
func (h *GuardrailsHandler) profileFromRequest(ctx context.Context, provider string, id uint, name string, config map[string]json.RawMessage) (configstoreTables.TableGuardrailProvider, error) {
	policyName := strings.TrimSpace(name)
	fallbackTimeout := 0
	enabled := true
	merged, err := mergeGuardrailConfig(nil, config)
	if err != nil {
		return configstoreTables.TableGuardrailProvider{}, err
	}
	if id != 0 {
		stored, err := h.configStore.GetGuardrailProvider(ctx, id)
		if err != nil {
			if errors.Is(err, configstore.ErrNotFound) {
				return configstoreTables.TableGuardrailProvider{}, fmt.Errorf("guardrail profile %d not found", id)
			}
			return configstoreTables.TableGuardrailProvider{}, err
		}
		if stored.ProviderName != provider {
			return configstoreTables.TableGuardrailProvider{}, fmt.Errorf("guardrail profile %d not found", id)
		}
		storedConfig, err := decodeGuardrailConfig(stored.ConfigJSON)
		if err != nil {
			return configstoreTables.TableGuardrailProvider{}, err
		}
		merged, err = mergeGuardrailConfig(storedConfig, config)
		if err != nil {
			return configstoreTables.TableGuardrailProvider{}, err
		}
		if policyName == "" {
			policyName = stored.PolicyName
		}
		fallbackTimeout = stored.Timeout
		enabled = stored.Enabled
	}
	timeout, normalized, err := normalizeGuardrailProfileConfig(merged, fallbackTimeout)
	if err != nil {
		return configstoreTables.TableGuardrailProvider{}, err
	}
	return newGuardrailProfile(provider, id, policyName, enabled, timeout, normalized)
}

// mergeStoredConfig overlays a request config on the stored profile an id names, applying the
// stored-credential rules mergeGuardrailConfig enforces.
func (h *GuardrailsHandler) mergeStoredConfig(ctx context.Context, id uint, provider string, incoming map[string]json.RawMessage) (map[string]json.RawMessage, error) {
	if id == 0 {
		return mergeGuardrailConfig(nil, incoming)
	}
	stored, err := h.configStore.GetGuardrailProvider(ctx, id)
	if err != nil {
		return nil, err
	}
	if stored.ProviderName != provider {
		return nil, configstore.ErrNotFound
	}
	config, err := decodeGuardrailConfig(stored.ConfigJSON)
	if err != nil {
		return nil, err
	}
	return mergeGuardrailConfig(config, incoming)
}

// guardrailProviders maps every profile id to its provider, which is what a rule's
// "<provider>:<id>" references and views need. A caller passing the write's transaction sees the
// profiles that transaction can commit against.
func (h *GuardrailsHandler) guardrailProviders(ctx context.Context, tx ...*gorm.DB) (map[uint]string, error) {
	profiles, err := h.configStore.GetGuardrailProviders(ctx, tx...)
	if err != nil {
		return nil, err
	}
	providers := make(map[uint]string, len(profiles))
	for _, profile := range profiles {
		providers[profile.ID] = profile.ProviderName
	}
	return providers, nil
}

// guardrailInvalidError marks a request that fails validation inside a write transaction, so the
// handler can answer 400 rather than reporting the rollback as a server fault.
type guardrailInvalidError struct{ err error }

func (e *guardrailInvalidError) Error() string { return e.err.Error() }
func (e *guardrailInvalidError) Unwrap() error { return e.err }

// reloadGuardrails asks the running plugin to rebuild its snapshot and reports whether the stored
// policy is now the active one. A missing plugin is not an error: the next reload, or a restart,
// applies the stored state, and the write itself succeeded.
func (h *GuardrailsHandler) reloadGuardrails(ctx context.Context) error {
	if h.config == nil {
		return nil
	}
	plugin, err := lib.FindPluginAs[*guardrails.Plugin](h.config, guardrails.PluginName)
	if err != nil || plugin == nil {
		return nil
	}
	return plugin.Reload(ctx)
}

// sendSavedNotActivated answers a write that reached the store but whose policy the plugin could
// not activate, so a caller can tell "stored" from "enforcing" instead of assuming success.
func sendSavedNotActivated(ctx *fasthttp.RequestCtx, err error) {
	logger.Error("guardrail policy was saved but could not be activated: %v", err)
	SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("saved but failed to activate: %v", err))
}

// guardrailProfileView flattens a stored profile into the API shape. Only canonical keys are
// rendered, and api_key is always redacted: a config stored under a non-canonical spelling (a
// row written before the write paths compared keys exactly) is dropped rather than echoed, so a
// secret can never leave the API under a name the redaction does not know.
func guardrailProfileView(profile configstoreTables.TableGuardrailProvider) (map[string]any, error) {
	config, err := decodeGuardrailConfig(profile.ConfigJSON)
	if err != nil {
		return nil, err
	}
	keys, err := lib.GuardrailConfigKeys(profile.ProviderName)
	if err != nil {
		// An unknown provider has no known salvagable shape; the row's config stays hidden.
		keys = map[string]bool{}
	}
	view := make(map[string]any, len(keys)+4)
	for key := range keys {
		if key == "api_key" {
			continue
		}
		raw, ok := config[key]
		if !ok {
			continue
		}
		var value any
		if err := sonic.Unmarshal(raw, &value); err != nil {
			return nil, fmt.Errorf("config key %q does not decode: %w", key, err)
		}
		view[key] = value
	}
	if raw, ok := config["api_key"]; ok && keys["api_key"] {
		if secret := guardrailSecretValue(raw); secret != nil {
			view["api_key"] = secret.Redacted()
		}
	}
	view["id"] = profile.ID
	view["name"] = profile.PolicyName
	view["enabled"] = profile.Enabled
	view["timeout"] = profile.Timeout
	return view, nil
}

// decodeGuardrailConfig decodes a stored provider config object.
func decodeGuardrailConfig(raw string) (map[string]json.RawMessage, error) {
	if strings.TrimSpace(raw) == "" {
		return map[string]json.RawMessage{}, nil
	}
	var config map[string]json.RawMessage
	if err := sonic.Unmarshal([]byte(raw), &config); err != nil {
		return nil, fmt.Errorf("stored config does not decode: %w", err)
	}
	if config == nil {
		config = map[string]json.RawMessage{}
	}
	return config, nil
}

// errGuardrailAnalyzerMoved rejects an Analyzer override that would send a saved credential to a
// new destination. The message never quotes config bytes.
var errGuardrailAnalyzerMoved = errors.New(
	"analyzer_url changed: supply a new api_key for the new Analyzer, because the stored key is bound to the destination it was saved for")

// mergeGuardrailConfig overlays incoming config keys on the stored ones. A stored Analyzer
// credential belongs to the URL it was saved with, so moving that URL without supplying a fresh
// key is rejected rather than replaying the saved bearer key at another host. A redacted or empty
// api_key keeps the stored secret, so a client that read a redacted profile can write it back
// unchanged.
func mergeGuardrailConfig(stored, incoming map[string]json.RawMessage) (map[string]json.RawMessage, error) {
	if url, ok := guardrailConfigString(incoming, "analyzer_url"); ok {
		if previous, had := guardrailConfigString(stored, "analyzer_url"); had && previous != url {
			if _, storedKey := stored["api_key"]; storedKey {
				if secret := guardrailSecretValue(incoming["api_key"]); secret == nil || secret.ShouldPreserveStored() {
					return nil, errGuardrailAnalyzerMoved
				}
			}
		}
	}
	merged := make(map[string]json.RawMessage, len(stored)+len(incoming))
	for key, raw := range stored {
		merged[key] = raw
	}
	for key, raw := range incoming {
		if key == "api_key" && guardrailSecretValue(raw).ShouldPreserveStored() {
			continue
		}
		merged[key] = raw
	}
	return merged, nil
}

// guardrailConfigString reads a string-valued config entry.
func guardrailConfigString(config map[string]json.RawMessage, key string) (string, bool) {
	raw, ok := config[key]
	if !ok {
		return "", false
	}
	var value string
	if err := sonic.Unmarshal(raw, &value); err != nil {
		return "", false
	}
	return value, true
}

// guardrailSecretValue reads a config secret, which is either the SecretVar object a redacted
// read returned or the plain string a write sends ("env.NAME", "vault.path", or a literal).
func guardrailSecretValue(raw json.RawMessage) *schemas.SecretVar {
	var secret schemas.SecretVar
	if err := sonic.Unmarshal(raw, &secret); err != nil {
		return nil
	}
	return &secret
}

// normalizeGuardrailProfileConfig moves the timeout into its own column and stores api_key as
// the reference itself, so a resolved secret is never written into the config JSON.
func normalizeGuardrailProfileConfig(config map[string]json.RawMessage, fallbackTimeout int) (int, map[string]json.RawMessage, error) {
	if config == nil {
		config = map[string]json.RawMessage{}
	}
	timeout := fallbackTimeout
	if raw, ok := config["timeout"]; ok {
		delete(config, "timeout")
		if err := sonic.Unmarshal(raw, &timeout); err != nil {
			return 0, nil, fmt.Errorf("timeout must be a whole number of seconds")
		}
		if timeout < 0 {
			return 0, nil, fmt.Errorf("timeout must not be negative")
		}
	}
	if raw, ok := config["api_key"]; ok {
		secret := guardrailSecretValue(raw)
		if secret == nil {
			return 0, nil, fmt.Errorf("api_key must be a string or a secret reference")
		}
		stored := secret.GetRawRef()
		if !secret.IsFromSecret() {
			stored = secret.GetValue()
		}
		encoded, err := sonic.Marshal(stored)
		if err != nil {
			return 0, nil, fmt.Errorf("api_key does not encode: %w", err)
		}
		config["api_key"] = encoded
	}
	return timeout, config, nil
}

// newGuardrailProfile validates a profile and builds the row to store.
func newGuardrailProfile(provider string, id uint, policyName string, enabled bool, timeout int, config map[string]json.RawMessage) (configstoreTables.TableGuardrailProvider, error) {
	if err := validateGuardrailProfileConfig(provider, config); err != nil {
		return configstoreTables.TableGuardrailProvider{}, err
	}
	raw, err := sonic.Marshal(config)
	if err != nil {
		return configstoreTables.TableGuardrailProvider{}, fmt.Errorf("config does not encode: %w", err)
	}
	profile := configstoreTables.TableGuardrailProvider{
		ID:           id,
		ProviderName: provider,
		PolicyName:   policyName,
		Enabled:      enabled,
		Timeout:      timeout,
		ConfigJSON:   string(raw),
	}
	if err := guardrails.ValidateProfile(guardrails.Profile{
		ID:           id,
		ProviderName: provider,
		PolicyName:   policyName,
		Enabled:      enabled,
		Timeout:      timeout,
		Config:       raw,
	}); err != nil {
		return configstoreTables.TableGuardrailProvider{}, err
	}
	return profile, nil
}

// validateGuardrailProfileConfig rejects a config key the provider does not accept, so a typo
// is a 400 instead of silently ignored configuration. Keys are compared exactly: the struct
// decoder is case-insensitive, so API_KEY would otherwise be stored beside api_key and echoed
// unmasked.
func validateGuardrailProfileConfig(provider string, config map[string]json.RawMessage) error {
	if err := lib.ValidateGuardrailConfigKeys(provider, config); err != nil {
		return err
	}
	raw, err := sonic.Marshal(config)
	if err != nil {
		return fmt.Errorf("config does not encode: %w", err)
	}
	var target any
	switch provider {
	case guardrails.ProviderSecrets:
		target = &guardrails.SecretsConfig{}
	case guardrails.ProviderRegex:
		target = &guardrails.RegexConfig{}
	case guardrails.ProviderPresidio:
		target = &guardrails.PresidioConfig{}
	default:
		return fmt.Errorf("unknown guardrail provider %q", provider)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("config is not valid for the %s provider: %w", provider, err)
	}
	return nil
}

// resolveGuardrailProfileRefs turns "<provider>:<id>" references into the ordered profile IDs
// a rule runs, rejecting unknown, duplicated and mismatched-provider references.
func resolveGuardrailProfileRefs(refs []string, providers map[uint]string) ([]uint, error) {
	ids := make([]uint, 0, len(refs))
	seen := make(map[uint]bool, len(refs))
	for _, ref := range refs {
		provider, idPart, ok := strings.Cut(strings.TrimSpace(ref), ":")
		if !ok {
			return nil, fmt.Errorf("profile reference %q must be \"<provider>:<id>\"", ref)
		}
		parsed, err := strconv.ParseUint(strings.TrimSpace(idPart), 10, 64)
		if err != nil || parsed == 0 {
			return nil, fmt.Errorf("profile reference %q must name a numeric profile id", ref)
		}
		id := uint(parsed)
		storedProvider, exists := providers[id]
		if !exists {
			return nil, fmt.Errorf("profile reference %q names no guardrail profile", ref)
		}
		if storedProvider != provider {
			return nil, fmt.Errorf("profile reference %q: profile %d is a %s profile", ref, id, storedProvider)
		}
		if seen[id] {
			return nil, fmt.Errorf("profile reference %q is duplicated", ref)
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids, nil
}

// guardrailProviderFromPath returns the provider the path names, false when this build does
// not run it.
func guardrailProviderFromPath(ctx *fasthttp.RequestCtx) (string, bool) {
	provider, ok := ctx.UserValue("provider").(string)
	if !ok || !slices.Contains(guardrailProviderNames, provider) {
		return "", false
	}
	return provider, true
}

// guardrailUnknownProviderMessage names what was requested, so a client can tell a typo from a
// build that lacks a provider.
func guardrailUnknownProviderMessage(ctx *fasthttp.RequestCtx) string {
	provider, _ := ctx.UserValue("provider").(string)
	if provider == "" {
		return "Guardrail provider not found"
	}
	return fmt.Sprintf("Guardrail provider %q is not available: use one of %s",
		provider, strings.Join(guardrailProviderNames, ", "))
}

// guardrailRuleIDFromPath parses the {rule_id} path parameter.
func guardrailRuleIDFromPath(ctx *fasthttp.RequestCtx) (uint, bool) {
	raw, ok := ctx.UserValue("rule_id").(string)
	if !ok {
		return 0, false
	}
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		return 0, false
	}
	return uint(id), true
}

// guardrailCallContext bounds an outbound Analyzer call. A profile's own timeout applies;
// without one the default keeps an unreachable Analyzer from holding the request open.
func guardrailCallContext(ctx context.Context, timeout int) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		timeout = guardrailDefaultTimeout
	}
	return context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
}
