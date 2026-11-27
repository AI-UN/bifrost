package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/fasthttp/router"
	"github.com/maximhq/bifrost/framework/configstore"
	configstoreTables "github.com/maximhq/bifrost/framework/configstore/tables"
	"github.com/maximhq/bifrost/plugins/guardrails"
	"github.com/maximhq/bifrost/transports/v2/bifrost-http/lib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// uintString renders a profile or rule id for a request body.
func uintString(id uint) string {
	return strconv.FormatUint(uint64(id), 10)
}

// sonicMarshal renders a test request body.
func sonicMarshal(v any) (string, error) {
	raw, err := json.Marshal(v)
	return string(raw), err
}

// newGuardrailTestHandler builds a handler over a real SQLite config store. The live-config
// handle is nil, which the handler treats as "no plugin loaded yet": a write is then applied
// on the next reload instead of failing the request.
func newGuardrailTestHandler(t *testing.T) (*GuardrailsHandler, configstore.ConfigStore) {
	t.Helper()
	store, err := configstore.NewConfigStore(context.Background(), &configstore.Config{
		Enabled: true,
		Type:    configstore.ConfigStoreTypeSQLite,
		Config:  &configstore.SQLiteConfig{Path: filepath.Join(t.TempDir(), "config.db")},
	}, testLogger{})
	require.NoError(t, err)
	t.Cleanup(func() { store.Close(context.Background()) })

	handler, err := NewGuardrailsHandler(store, nil)
	require.NoError(t, err)
	return handler, store
}

// newGuardrailRequestCtx builds a request context for a handler call.
func newGuardrailRequestCtx(body string, pathParams map[string]string) *fasthttp.RequestCtx {
	var req fasthttp.Request
	req.SetBodyString(body)
	ctx := &fasthttp.RequestCtx{}
	ctx.Init(&req, &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 12345}, nil)
	for key, value := range pathParams {
		ctx.SetUserValue(key, value)
	}
	return ctx
}

// createTestGuardrailProfile creates a regex profile and returns its id.
func createTestGuardrailProfile(t *testing.T, handler *GuardrailsHandler, body string) uint {
	t.Helper()
	ctx := newGuardrailRequestCtx(body, map[string]string{"provider": "regex"})
	handler.createProfile(ctx)
	require.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode(), "body: %s", ctx.Response.Body())
	config := decodeJSONResponse(t, ctx)["config"].(map[string]any)
	return uint(config["id"].(float64))
}

// createTestGuardrailRule creates a rule from the given name and profile references.
func createTestGuardrailRule(t *testing.T, handler *GuardrailsHandler, name string, refs []string) uint {
	t.Helper()
	body, err := sonicMarshal(map[string]any{
		"name":                      name,
		"enabled":                   true,
		"celExpression":             "true",
		"selectedGuardrailProfiles": refs,
	})
	require.NoError(t, err)
	ctx := newGuardrailRequestCtx(body, nil)
	handler.createGuardrailRule(ctx)
	require.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode(), "body: %s", ctx.Response.Body())
	rule := decodeJSONResponse(t, ctx)["rule"].(map[string]any)
	return uint(rule["id"].(float64))
}

// TestGuardrailRoutesStaticPathsWinOverProviderParam guards the registration order: the
// router returns the first matching child, so a static path registered after {provider} would
// be served as a profile group for a provider named "rules".
func TestGuardrailRoutesStaticPathsWinOverProviderParam(t *testing.T) {
	handler, _ := newGuardrailTestHandler(t)
	r := router.New()
	handler.RegisterRoutes(r) // panics on a route conflict, which is the other failure mode

	request := func(method, path string) *fasthttp.RequestCtx {
		var req fasthttp.Request
		req.Header.SetMethod(method)
		req.SetRequestURI(path)
		ctx := &fasthttp.RequestCtx{}
		ctx.Init(&req, &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 12345}, nil)
		r.Handler(ctx)
		return ctx
	}

	ctx := request(fasthttp.MethodGet, "/api/guardrails/rules")
	require.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode(), "body: %s", ctx.Response.Body())
	assert.Contains(t, string(ctx.Response.Body()), `"rules"`, "the rules listing must not be served as a provider group")

	ctx = request(fasthttp.MethodGet, "/api/guardrails/secrets/rules")
	require.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode(), "body: %s", ctx.Response.Body())
	assert.Contains(t, string(ctx.Response.Body()), `"rules"`)

	ctx = request(fasthttp.MethodPost, "/api/guardrails/presidio/supported-entities")
	assert.Equal(t, fasthttp.StatusBadRequest, ctx.Response.StatusCode(),
		"an empty analyzer_url is a validation failure, not an unknown provider: %s", ctx.Response.Body())

	ctx = request(fasthttp.MethodGet, "/api/guardrails/bedrock")
	assert.Equal(t, fasthttp.StatusNotFound, ctx.Response.StatusCode())

	ctx = request(fasthttp.MethodGet, "/api/guardrails/regex")
	require.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode(), "body: %s", ctx.Response.Body())
	assert.Contains(t, string(ctx.Response.Body()), `"configs":[]`)
}

// TestGuardrailProfileCRUD covers create, read, merge-on-update and delete.
func TestGuardrailProfileCRUD(t *testing.T) {
	handler, store := newGuardrailTestHandler(t)

	id := createTestGuardrailProfile(t, handler, `{
		"name": "pii",
		"config": {
			"timeout": 15,
			"patterns": [{"pattern": "a+", "entity_type": "A", "action": "redact"}]
		}
	}`)

	ctx := newGuardrailRequestCtx("", map[string]string{"provider": "regex"})
	handler.getProviderProfiles(ctx)
	require.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode())
	group := decodeJSONResponse(t, ctx)
	assert.Equal(t, "regex", group["name"])
	configs := group["configs"].([]any)
	require.Len(t, configs, 1)
	profile := configs[0].(map[string]any)
	assert.Equal(t, "pii", profile["name"])
	assert.Equal(t, true, profile["enabled"])
	assert.Equal(t, float64(15), profile["timeout"], "timeout is served at the top level, not inside config")

	// PUT merges: a config key the request omits keeps its stored value.
	updateBody := `{"id": ` + uintString(id) + `, "name": "pii2", "config": {
		"patterns": [
			{"pattern": "a+", "entity_type": "A"},
			{"pattern": "b+", "entity_type": "B"}
		]
	}}`
	ctx = newGuardrailRequestCtx(updateBody, map[string]string{"provider": "regex"})
	handler.updateProfile(ctx)
	require.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode(), "body: %s", ctx.Response.Body())
	updated := decodeJSONResponse(t, ctx)["config"].(map[string]any)
	assert.Equal(t, "pii2", updated["name"])
	assert.Equal(t, float64(15), updated["timeout"], "an omitted timeout keeps the stored value")
	assert.Len(t, updated["patterns"].([]any), 2)

	stored, err := store.GetGuardrailProvider(context.Background(), id)
	require.NoError(t, err)
	assert.Contains(t, stored.ConfigJSON, `"b+"`, "the new patterns are what was stored")

	ctx = newGuardrailRequestCtx(`{"id": `+uintString(id)+`}`, map[string]string{"provider": "regex"})
	handler.deleteProfile(ctx)
	require.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode(), "body: %s", ctx.Response.Body())
	_, err = store.GetGuardrailProvider(context.Background(), id)
	assert.ErrorIs(t, err, configstore.ErrNotFound)
}

// TestGuardrailProfileUnknownProvider checks that a path segment outside the three providers
// is a 404 on every profile route.
func TestGuardrailProfileUnknownProvider(t *testing.T) {
	handler, _ := newGuardrailTestHandler(t)
	for _, call := range []func(*fasthttp.RequestCtx){
		handler.getProviderProfiles,
		handler.createProfile,
		handler.updateProfile,
		handler.deleteProfile,
		handler.verifyProfile,
	} {
		ctx := newGuardrailRequestCtx(`{"id":1,"name":"x"}`, map[string]string{"provider": "bedrock"})
		call(ctx)
		assert.Equal(t, fasthttp.StatusNotFound, ctx.Response.StatusCode(), "body: %s", ctx.Response.Body())
	}
}

// TestGuardrailProfileRejectsUnsupportedConfig covers a value this build does not run and a
// key the provider does not accept.
func TestGuardrailProfileRejectsUnsupportedConfig(t *testing.T) {
	handler, _ := newGuardrailTestHandler(t)

	ctx := newGuardrailRequestCtx(`{"name": "p", "config": {"patterns": [
		{"pattern": "a+", "action": "block", "redaction_mode": "logs_only"}
	]}}`, map[string]string{"provider": "regex"})
	handler.createProfile(ctx)
	assert.Equal(t, fasthttp.StatusBadRequest, ctx.Response.StatusCode())
	assert.Contains(t, string(ctx.Response.Body()), "not supported in this build")

	ctx = newGuardrailRequestCtx(`{"name": "p", "config": {"paterns": []}}`,
		map[string]string{"provider": "regex"})
	handler.createProfile(ctx)
	assert.Equal(t, fasthttp.StatusBadRequest, ctx.Response.StatusCode())
	assert.Contains(t, string(ctx.Response.Body()), "paterns", "a typo must be named, not silently ignored")

	// A non-http analyzer URL is rejected, so the gateway cannot be pointed at another scheme.
	ctx = newGuardrailRequestCtx(`{"name": "p", "config": {"analyzer_url": "file:///etc/passwd"}}`,
		map[string]string{"provider": "presidio"})
	handler.createProfile(ctx)
	assert.Equal(t, fasthttp.StatusBadRequest, ctx.Response.StatusCode())
}

// TestGuardrailProfileAPIKeyRedactedAndPreserved checks that reads never return the key and
// that writing a read-back key does not overwrite the stored one.
func TestGuardrailProfileAPIKeyRedactedAndPreserved(t *testing.T) {
	handler, store := newGuardrailTestHandler(t)
	const apiKey = "sk-live-0123456789abcdef"

	ctx := newGuardrailRequestCtx(`{"name": "holdings", "config": {
		"analyzer_url": "http://127.0.0.1:5002",
		"api_key": "`+apiKey+`"
	}}`, map[string]string{"provider": "presidio"})
	handler.createProfile(ctx)
	require.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode(), "body: %s", ctx.Response.Body())
	created := decodeJSONResponse(t, ctx)["config"].(map[string]any)
	id := uint(created["id"].(float64))
	assert.NotContains(t, string(ctx.Response.Body()), apiKey, "a create response must not echo the key")

	ctx = newGuardrailRequestCtx("", map[string]string{"provider": "presidio"})
	handler.getProviderProfiles(ctx)
	require.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode())
	assert.NotContains(t, string(ctx.Response.Body()), apiKey, "a read must not return the key")
	read := decodeJSONResponse(t, ctx)["configs"].([]any)[0].(map[string]any)
	redactedKey, ok := read["api_key"].(map[string]any)
	require.True(t, ok, "api_key is served as a redacted SecretVar object")
	assert.Contains(t, redactedKey["value"], "*")

	// Writing the redacted value back keeps the stored key.
	body, err := sonicMarshal(map[string]any{
		"id": id,
		"config": map[string]any{
			"api_key":  redactedKey,
			"language": "en",
		},
	})
	require.NoError(t, err)
	ctx = newGuardrailRequestCtx(body, map[string]string{"provider": "presidio"})
	handler.updateProfile(ctx)
	require.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode(), "body: %s", ctx.Response.Body())

	stored, err := store.GetGuardrailProvider(context.Background(), id)
	require.NoError(t, err)
	assert.Contains(t, stored.ConfigJSON, apiKey, "the stored key survives a redacted write")
	assert.NotContains(t, stored.ConfigJSON, "********")

	// A new key replaces the stored one.
	body, err = sonicMarshal(map[string]any{"id": id, "config": map[string]any{"api_key": "sk-live-replaced"}})
	require.NoError(t, err)
	ctx = newGuardrailRequestCtx(body, map[string]string{"provider": "presidio"})
	handler.updateProfile(ctx)
	require.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode(), "body: %s", ctx.Response.Body())
	stored, err = store.GetGuardrailProvider(context.Background(), id)
	require.NoError(t, err)
	assert.Contains(t, stored.ConfigJSON, "sk-live-replaced")
}

// TestGuardrailProfileVerifyWithoutAnalyzer covers the in-process providers: verification only
// has to prove the config is usable, so no Analyzer is contacted.
func TestGuardrailProfileVerifyWithoutAnalyzer(t *testing.T) {
	handler, _ := newGuardrailTestHandler(t)
	id := createTestGuardrailProfile(t, handler, `{"name": "p", "config": {"patterns": [{"pattern": "a+"}]}}`)

	// With the id and no config, the stored profile is what gets verified.
	ctx := newGuardrailRequestCtx(`{"id": `+uintString(id)+`}`, map[string]string{"provider": "regex"})
	handler.verifyProfile(ctx)
	require.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode(), "body: %s", ctx.Response.Body())
	assert.Equal(t, true, decodeJSONResponse(t, ctx)["ok"])

	// A pattern that does not compile fails the check.
	ctx = newGuardrailRequestCtx(`{"name": "p", "config": {"patterns": [{"pattern": "("}]}}`,
		map[string]string{"provider": "regex"})
	handler.verifyProfile(ctx)
	assert.Equal(t, fasthttp.StatusBadRequest, ctx.Response.StatusCode())
}

// TestGuardrailProfileDeleteReferencedIs409 checks that a profile a rule runs cannot be
// deleted out from under it.
func TestGuardrailProfileDeleteReferencedIs409(t *testing.T) {
	handler, _ := newGuardrailTestHandler(t)
	id := createTestGuardrailProfile(t, handler, `{"name": "p", "config": {"patterns": [{"pattern": "a+"}]}}`)
	ruleID := createTestGuardrailRule(t, handler, "all", []string{"regex:" + uintString(id)})

	ctx := newGuardrailRequestCtx(`{"id": `+uintString(id)+`}`, map[string]string{"provider": "regex"})
	handler.deleteProfile(ctx)
	require.Equal(t, fasthttp.StatusConflict, ctx.Response.StatusCode(), "body: %s", ctx.Response.Body())
	assert.Contains(t, string(ctx.Response.Body()), "all", "the blocking rule is named")

	ctx = newGuardrailRequestCtx("", map[string]string{"rule_id": uintString(ruleID)})
	handler.deleteGuardrailRule(ctx)
	require.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode(), "body: %s", ctx.Response.Body())

	ctx = newGuardrailRequestCtx(`{"id": `+uintString(id)+`}`, map[string]string{"provider": "regex"})
	handler.deleteProfile(ctx)
	assert.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode(), "body: %s", ctx.Response.Body())
}

// TestGuardrailRuleCRUD covers create, list, replace, toggle and delete.
func TestGuardrailRuleCRUD(t *testing.T) {
	handler, _ := newGuardrailTestHandler(t)
	profileID := createTestGuardrailProfile(t, handler, `{"name": "p", "config": {"patterns": [{"pattern": "a+"}]}}`)
	ref := "regex:" + uintString(profileID)

	id := createTestGuardrailRule(t, handler, "all-requests", []string{ref})

	ctx := newGuardrailRequestCtx("", nil)
	handler.getGuardrailRules(ctx)
	require.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode(), "body: %s", ctx.Response.Body())
	listing := decodeJSONResponse(t, ctx)
	assert.Equal(t, float64(1), listing["count"])
	rules := listing["rules"].([]any)
	require.Len(t, rules, 1)
	rule := rules[0].(map[string]any)
	assert.Equal(t, "llm", rule["target"])
	assert.Equal(t, "input", rule["applyTo"])
	assert.Equal(t, float64(100), rule["samplingRate"], "an unset sampling rate means every request")
	assert.Equal(t, []any{ref}, rule["selectedGuardrailProfiles"])
	assert.Equal(t, "true", rule["celExpression"])
	assert.Contains(t, rule, "createdAt")

	updateBody, err := sonicMarshal(map[string]any{
		"name":                      "renamed",
		"enabled":                   true,
		"celExpression":             `model == "gpt-4o"`,
		"applyTo":                   "input",
		"target":                    "llm",
		"samplingRate":              25,
		"timeout":                   45,
		"selectedGuardrailProfiles": []string{ref},
	})
	require.NoError(t, err)
	ctx = newGuardrailRequestCtx(updateBody, map[string]string{"rule_id": uintString(id)})
	handler.updateGuardrailRule(ctx)
	require.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode(), "body: %s", ctx.Response.Body())
	updated := decodeJSONResponse(t, ctx)["rule"].(map[string]any)
	assert.Equal(t, "renamed", updated["name"])
	assert.Equal(t, float64(25), updated["samplingRate"])
	assert.Equal(t, float64(45), updated["timeout"])

	ctx = newGuardrailRequestCtx(`{"enabled": false}`, map[string]string{"rule_id": uintString(id)})
	handler.toggleGuardrailRule(ctx)
	require.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode(), "body: %s", ctx.Response.Body())
	assert.Equal(t, false, decodeJSONResponse(t, ctx)["rule"].(map[string]any)["enabled"])

	ctx = newGuardrailRequestCtx("", map[string]string{"rule_id": uintString(id)})
	handler.deleteGuardrailRule(ctx)
	require.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode(), "body: %s", ctx.Response.Body())

	ctx = newGuardrailRequestCtx("", nil)
	handler.getGuardrailRules(ctx)
	assert.Equal(t, float64(0), decodeJSONResponse(t, ctx)["count"])
}

// TestGuardrailRuleListPagination checks that count is the total, which is what a client pages
// against, while rules holds the requested page.
func TestGuardrailRuleListPagination(t *testing.T) {
	handler, _ := newGuardrailTestHandler(t)
	profileID := createTestGuardrailProfile(t, handler, `{"name": "p", "config": {"patterns": [{"pattern": "a+"}]}}`)
	ref := "regex:" + uintString(profileID)
	createTestGuardrailRule(t, handler, "first", []string{ref})
	createTestGuardrailRule(t, handler, "second", []string{ref})

	ctx := newGuardrailRequestCtx("", nil)
	ctx.Request.SetRequestURI("/api/guardrails/rules?limit=1&offset=1")
	handler.getGuardrailRules(ctx)
	require.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode(), "body: %s", ctx.Response.Body())
	listing := decodeJSONResponse(t, ctx)
	assert.Equal(t, float64(2), listing["count"], "count is the total row count")
	assert.Equal(t, float64(1), listing["limit"])
	assert.Equal(t, float64(1), listing["offset"])
	assert.Len(t, listing["rules"].([]any), 1)
}

// TestGuardrailRuleRejectsUnsupportedFields covers the settings this build accepts only at
// their defaults.
func TestGuardrailRuleRejectsUnsupportedFields(t *testing.T) {
	handler, _ := newGuardrailTestHandler(t)
	profileID := createTestGuardrailProfile(t, handler, `{"name": "p", "config": {"patterns": [{"pattern": "a+"}]}}`)
	ref := "regex:" + uintString(profileID)

	for _, body := range []string{
		`{"name": "r", "celExpression": "true", "outputMode": "monitor"}`,
		`{"name": "r", "celExpression": "true", "inputTiming": "parallel"}`,
		`{"name": "r", "celExpression": "true", "evaluationMode": "per_turn"}`,
		`{"name": "r", "celExpression": "true", "profileExecutionMode": "chained"}`,
		`{"name": "r", "celExpression": "true", "streamReplayEventIntervalMs": 5}`,
		`{"name": "r", "celExpression": "true", "target": "mcp"}`,
		`{"name": "r", "celExpression": "true", "applyTo": "output"}`,
	} {
		ctx := newGuardrailRequestCtx(body, nil)
		handler.createGuardrailRule(ctx)
		assert.Equal(t, fasthttp.StatusBadRequest, ctx.Response.StatusCode(), "body %s", body)
		assert.Contains(t, string(ctx.Response.Body()), "not supported in this build", "body %s", body)
	}

	// The defaults are accepted, so a client that sends the whole view round-trips.
	body := `{"name": "r", "celExpression": "true", "target": "llm", "applyTo": "input",
		"outputMode": "enforce", "inputTiming": "pre_call", "evaluationMode": "bundled",
		"profileExecutionMode": "independent", "streamReplayEventIntervalMs": 0,
		"selectedGuardrailProfiles": ["` + ref + `"]}`
	ctx := newGuardrailRequestCtx(body, nil)
	handler.createGuardrailRule(ctx)
	require.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode(), "body: %s", ctx.Response.Body())

	// A malformed CEL expression is rejected at write time, not at first evaluation.
	ctx = newGuardrailRequestCtx(`{"name": "r", "celExpression": "model =="}`, nil)
	handler.createGuardrailRule(ctx)
	assert.Equal(t, fasthttp.StatusBadRequest, ctx.Response.StatusCode())
}

// TestGuardrailRuleRejectsBadProfileRefs checks that a reference must name a profile that
// exists, of the provider the reference claims, once.
func TestGuardrailRuleRejectsBadProfileRefs(t *testing.T) {
	handler, _ := newGuardrailTestHandler(t)
	profileID := createTestGuardrailProfile(t, handler, `{"name": "p", "config": {"patterns": [{"pattern": "a+"}]}}`)
	regexRef := "regex:" + uintString(profileID)

	for _, refs := range [][]string{
		{"regex:9999"},
		{"secrets:" + uintString(profileID)},
		{regexRef, regexRef},
		{uintString(profileID)},
	} {
		body, err := sonicMarshal(map[string]any{
			"name":                      "r",
			"celExpression":             "true",
			"selectedGuardrailProfiles": refs,
		})
		require.NoError(t, err)
		ctx := newGuardrailRequestCtx(body, nil)
		handler.createGuardrailRule(ctx)
		assert.Equal(t, fasthttp.StatusBadRequest, ctx.Response.StatusCode(), "refs %v", refs)
	}
}

// TestGuardrailProfileRejectsNonCanonicalConfigKeys covers a case variant of a config key. Go's
// decoder matches field names case-insensitively, so API_KEY used to be stored beside api_key and
// returned unmasked under that spelling.
func TestGuardrailProfileRejectsNonCanonicalConfigKeys(t *testing.T) {
	handler, _ := newGuardrailTestHandler(t)
	const apiKey = "sk-live-0123456789abcdef"
	presidio := map[string]string{"provider": "presidio"}
	body := `{"name":"p","config":{"analyzer_url":"http://127.0.0.1:5002","API_KEY":"` + apiKey + `"}}`

	for name, invoke := range map[string]func(*fasthttp.RequestCtx){
		"create": handler.createProfile,
		"verify": handler.verifyProfile,
	} {
		ctx := newGuardrailRequestCtx(body, presidio)
		invoke(ctx)
		assert.Equal(t, fasthttp.StatusBadRequest, ctx.Response.StatusCode(), "%s must reject the spelling: %s", name, ctx.Response.Body())
		assert.Contains(t, string(ctx.Response.Body()), "API_KEY", "%s must name the key", name)
		assert.NotContains(t, string(ctx.Response.Body()), apiKey, "%s must not echo the value", name)
	}

	// supported-entities validates the same way, before any Analyzer call.
	ctx := newGuardrailRequestCtx(body, nil)
	handler.getPresidioEntities(ctx)
	assert.Equal(t, fasthttp.StatusBadRequest, ctx.Response.StatusCode(), "body: %s", ctx.Response.Body())
	assert.Contains(t, string(ctx.Response.Body()), "API_KEY")

	// An update of a stored profile is rejected the same way.
	id := createTestGuardrailProfile(t, handler, `{"name":"p","config":{"patterns":[{"pattern":"a+"}]}}`)
	update := `{"id":` + uintString(id) + `,"config":{"PATTERNS":[]}}`
	ctx = newGuardrailRequestCtx(update, map[string]string{"provider": "regex"})
	handler.updateProfile(ctx)
	assert.Equal(t, fasthttp.StatusBadRequest, ctx.Response.StatusCode(), "body: %s", ctx.Response.Body())
	assert.Contains(t, string(ctx.Response.Body()), "PATTERNS")
}

// TestGuardrailProfileViewHidesNonCanonicalKeys covers a row written before the write paths
// compared keys exactly: the view is built from the provider's canonical keys, so the secret
// cannot be echoed under a spelling the redaction does not know.
func TestGuardrailProfileViewHidesNonCanonicalKeys(t *testing.T) {
	handler, store := newGuardrailTestHandler(t)
	const apiKey = "sk-live-0123456789abcdef"
	stored := configstoreTables.TableGuardrailProvider{
		ID:           3,
		ProviderName: "presidio",
		PolicyName:   "legacy",
		Enabled:      true,
		ConfigJSON:   `{"analyzer_url":"http://127.0.0.1:5002","API_KEY":"` + apiKey + `"}`,
	}
	require.NoError(t, store.CreateGuardrailProvider(context.Background(), &stored))

	ctx := newGuardrailRequestCtx("", map[string]string{"provider": "presidio"})
	handler.getProviderProfiles(ctx)
	require.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode(), "body: %s", ctx.Response.Body())
	assert.NotContains(t, string(ctx.Response.Body()), apiKey, "a non-canonical secret must never be echoed")
	assert.NotContains(t, string(ctx.Response.Body()), "API_KEY", "a non-canonical key is not rendered")
	configs := decodeJSONResponse(t, ctx)["configs"].([]any)
	require.Len(t, configs, 1)
	assert.Equal(t, "http://127.0.0.1:5002", configs[0].(map[string]any)["analyzer_url"], "canonical keys still render")
}

// TestGuardrailAnalyzerCredentialBoundToDestination covers the stored Analyzer credential: it
// belongs to the URL it was saved with, so moving the URL without a freshly supplied key is a 400
// on every path that can reach the Analyzer.
func TestGuardrailAnalyzerCredentialBoundToDestination(t *testing.T) {
	handler, store := newGuardrailTestHandler(t)
	const apiKey = "sk-live-0123456789abcdef"
	presidio := map[string]string{"provider": "presidio"}

	ctx := newGuardrailRequestCtx(`{"name":"holdings","config":{
		"analyzer_url":"http://127.0.0.1:5002","api_key":"`+apiKey+`"}}`, presidio)
	handler.createProfile(ctx)
	require.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode(), "body: %s", ctx.Response.Body())
	created := decodeJSONResponse(t, ctx)["config"].(map[string]any)
	id := uint(created["id"].(float64))
	redactedKey := created["api_key"]

	// A moved URL with the key omitted is refused.
	ctx = newGuardrailRequestCtx(`{"id":`+uintString(id)+`,"config":{"analyzer_url":"http://other.example:5002"}}`, presidio)
	handler.updateProfile(ctx)
	require.Equal(t, fasthttp.StatusBadRequest, ctx.Response.StatusCode(), "body: %s", ctx.Response.Body())
	assert.Contains(t, string(ctx.Response.Body()), "analyzer_url changed")

	// Replaying the redacted key the read returned is refused too: it is the stored credential.
	body, err := sonicMarshal(map[string]any{
		"id":     id,
		"config": map[string]any{"analyzer_url": "http://other.example:5002", "api_key": redactedKey},
	})
	require.NoError(t, err)
	ctx = newGuardrailRequestCtx(body, presidio)
	handler.updateProfile(ctx)
	assert.Equal(t, fasthttp.StatusBadRequest, ctx.Response.StatusCode(), "body: %s", ctx.Response.Body())

	// Verify and the entity lookup refuse the same pair before contacting anything.
	ctx = newGuardrailRequestCtx(`{"id":`+uintString(id)+`,"config":{"analyzer_url":"http://other.example:5002"}}`, presidio)
	handler.verifyProfile(ctx)
	assert.Equal(t, fasthttp.StatusBadRequest, ctx.Response.StatusCode(), "body: %s", ctx.Response.Body())

	ctx = newGuardrailRequestCtx(`{"id":`+uintString(id)+`,"config":{"analyzer_url":"http://other.example:5002"}}`, nil)
	handler.getPresidioEntities(ctx)
	assert.Equal(t, fasthttp.StatusBadRequest, ctx.Response.StatusCode(), "body: %s", ctx.Response.Body())

	// Nothing was written by the refused updates.
	stored, err := store.GetGuardrailProvider(context.Background(), id)
	require.NoError(t, err)
	assert.Contains(t, stored.ConfigJSON, "127.0.0.1:5002")
	assert.Contains(t, stored.ConfigJSON, apiKey)

	// A freshly supplied key moves the destination.
	ctx = newGuardrailRequestCtx(`{"id":`+uintString(id)+`,"config":{
		"analyzer_url":"http://other.example:5002","api_key":"sk-live-fresh"}}`, presidio)
	handler.updateProfile(ctx)
	require.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode(), "body: %s", ctx.Response.Body())
	stored, err = store.GetGuardrailProvider(context.Background(), id)
	require.NoError(t, err)
	assert.Contains(t, stored.ConfigJSON, "other.example:5002")
	assert.Contains(t, stored.ConfigJSON, "sk-live-fresh")
	assert.NotContains(t, stored.ConfigJSON, apiKey)
}

// TestGuardrailWriteReportsActivationFailure covers a Store write that succeeds while the plugin
// cannot activate it: the caller must be told the policy is stored but not enforcing, instead of
// reading a success response.
func TestGuardrailWriteReportsActivationFailure(t *testing.T) {
	handler, store := newGuardrailTestHandler(t)
	var fail atomic.Bool
	plugin, err := guardrails.Init(context.Background(), &guardrails.Config{}, testLogger{},
		func(context.Context) ([]guardrails.Profile, []guardrails.Rule, error) {
			if fail.Load() {
				return nil, nil, errors.New("guardrail store unavailable")
			}
			return nil, nil, nil
		}, nil)
	require.NoError(t, err)
	handler.config = &lib.Config{}
	require.NoError(t, handler.config.ReloadPlugin(plugin))

	fail.Store(true)
	ctx := newGuardrailRequestCtx(`{"name":"p","config":{"patterns":[{"pattern":"a+"}]}}`,
		map[string]string{"provider": "regex"})
	handler.createProfile(ctx)
	require.Equal(t, fasthttp.StatusInternalServerError, ctx.Response.StatusCode(), "body: %s", ctx.Response.Body())
	assert.Contains(t, string(ctx.Response.Body()), "saved but failed to activate")

	// The write itself is stored: the next reload applies it.
	profiles, storeErr := store.GetGuardrailProviders(context.Background())
	require.NoError(t, storeErr)
	assert.Len(t, profiles, 1)
}
