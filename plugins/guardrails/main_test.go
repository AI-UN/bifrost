package guardrails

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/stretchr/testify/require"
)

const testPolicyName = "email-policy"

// testLogger counts warnings so a test can tell a fail-open path from a silent one.
type testLogger struct{ warnings int }

func (*testLogger) Debug(string, ...any)                   {}
func (*testLogger) Info(string, ...any)                    {}
func (l *testLogger) Warn(string, ...any)                  { l.warnings++ }
func (*testLogger) Error(string, ...any)                   {}
func (*testLogger) Fatal(string, ...any)                   {}
func (*testLogger) SetLevel(schemas.LogLevel)              {}
func (*testLogger) SetOutputType(schemas.LoggerOutputType) {}
func (*testLogger) LogHTTPRequest(schemas.LogLevel, string) schemas.LogEventBuilder {
	return nil
}

func newTestContext() *schemas.BifrostContext {
	return schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
}

func staticLoader(profiles []Profile, rules []Rule) Loader {
	return func(context.Context) ([]Profile, []Rule, error) { return profiles, rules, nil }
}

func mustInit(t *testing.T, logger schemas.Logger, profiles []Profile, rules []Rule) *Plugin {
	t.Helper()
	plugin, err := Init(context.Background(), &Config{}, logger, staticLoader(profiles, rules), []byte("guardrails-test-hmac-key-32-bytes"))
	require.NoError(t, err)
	return plugin
}

// regexProfileJSON builds a regex profile whose patterns are configured by the caller.
func regexProfileJSON(tb testing.TB, patterns ...RegexPattern) json.RawMessage {
	tb.Helper()
	raw, err := json.Marshal(RegexConfig{Patterns: patterns})
	require.NoError(tb, err)
	return raw
}

func enabledRegexProfile(t *testing.T, id uint, patterns ...RegexPattern) Profile {
	t.Helper()
	return Profile{ID: id, ProviderName: ProviderRegex, PolicyName: testPolicyName, Enabled: true, Config: regexProfileJSON(t, patterns...)}
}

func chatRequest(content string) *schemas.BifrostRequest {
	return &schemas.BifrostRequest{
		RequestType: schemas.ChatCompletionRequest,
		ChatRequest: &schemas.BifrostChatRequest{
			Provider: schemas.OpenAI,
			Model:    "gpt-4o",
			Input: []schemas.ChatMessage{{
				Role:    schemas.ChatMessageRoleUser,
				Content: &schemas.ChatMessageContent{ContentStr: schemas.Ptr(content)},
			}},
		},
	}
}

func emailPattern(action string) RegexPattern {
	return RegexPattern{Pattern: `[\w.+-]+@[\w-]+\.[\w.]+`, EntityType: "EMAIL_ADDRESS", Action: action}
}

// metadata returns the guardrail metadata recorded on a context.
func metadata(t *testing.T, ctx *schemas.BifrostContext) *schemas.BifrostGuardrailMetadata {
	t.Helper()
	found, ok := schemas.GuardrailMetadataFromContext(ctx)
	require.True(t, ok, "expected guardrail metadata on the context")
	return found
}

func TestPreRequestHookNoRulesDoesNotAllocate(t *testing.T) {
	plugin := mustInit(t, &testLogger{}, nil, nil)
	ctx := newTestContext()
	req := chatRequest("hello")

	allocs := testing.AllocsPerRun(100, func() { _ = plugin.PreRequestHook(ctx, req) })
	require.Zero(t, allocs, "a deployment with no rules must not allocate on the request path")
}

func TestPreRequestHookDisabledRuleDoesNotAllocate(t *testing.T) {
	profile := enabledRegexProfile(t, 1, emailPattern(ActionBlock))
	rule := Rule{ID: 1, Enabled: false, ProfileIDs: []uint{1}}
	plugin := mustInit(t, &testLogger{}, []Profile{profile}, []Rule{rule})
	ctx := newTestContext()
	req := chatRequest("hello")

	allocs := testing.AllocsPerRun(100, func() { _ = plugin.PreRequestHook(ctx, req) })
	require.Zero(t, allocs)
}

func TestRuleSelectionUsesCELVariables(t *testing.T) {
	profile := enabledRegexProfile(t, 1, emailPattern(ActionDetectOnly))
	rule := Rule{ID: 1, Name: "openai only", Enabled: true, CELExpression: `provider == "openai"`, ProfileIDs: []uint{1}}
	plugin := mustInit(t, &testLogger{}, []Profile{profile}, []Rule{rule})

	t.Run("matching request runs the profile", func(t *testing.T) {
		ctx := newTestContext()
		require.NoError(t, plugin.PreRequestHook(ctx, chatRequest("write to alex@example.com")))
		require.Len(t, metadata(t, ctx).Detections, 1)
	})

	t.Run("non-matching request runs nothing", func(t *testing.T) {
		ctx := newTestContext()
		req := chatRequest("write to alex@example.com")
		req.ChatRequest.Provider = schemas.Anthropic
		require.NoError(t, plugin.PreRequestHook(ctx, req))
		_, ok := schemas.GuardrailMetadataFromContext(ctx)
		require.False(t, ok)
	})
}

func TestRuleSelectionReadsHeadersAndTeam(t *testing.T) {
	profile := enabledRegexProfile(t, 1, emailPattern(ActionDetectOnly))
	rule := Rule{ID: 1, Enabled: true, CELExpression: `team == "team-platform" && headers["x-environment"] == "production"`, ProfileIDs: []uint{1}}
	plugin := mustInit(t, &testLogger{}, []Profile{profile}, []Rule{rule})

	ctx := newTestContext()
	ctx.SetValue(schemas.BifrostContextKeyGuardrailHeaders, map[string]string{"x-environment": "production"})
	ctx.SetValue(schemas.BifrostContextKeyGovernanceTeamID, "team-platform")
	require.NoError(t, plugin.PreRequestHook(ctx, chatRequest("alex@example.com")))
	require.Len(t, metadata(t, ctx).Detections, 1)
}

func TestRuleSelectionBindsIdentityToGovernanceIDs(t *testing.T) {
	profile := enabledRegexProfile(t, 1, emailPattern(ActionDetectOnly))
	rule := Rule{
		ID: 1, Enabled: true,
		CELExpression: `virtual_key == "vk-id" && team == "team-id" && customer == "customer-id" && user == "user-id"`,
		ProfileIDs:    []uint{1},
	}
	plugin := mustInit(t, &testLogger{}, []Profile{profile}, []Rule{rule})

	ctx := newTestContext()
	// The raw credential the caller presented and the display names are not the documented IDs.
	ctx.SetValue(schemas.BifrostContextKeyVirtualKey, "sk-bf-raw-credential")
	ctx.SetValue(schemas.BifrostContextKeyGovernanceTeamName, "Platform Team")
	ctx.SetValue(schemas.BifrostContextKeyGovernanceCustomerName, "Acme")
	ctx.SetValue(schemas.BifrostContextKeyGovernanceVirtualKeyID, "vk-id")
	ctx.SetValue(schemas.BifrostContextKeyGovernanceTeamID, "team-id")
	ctx.SetValue(schemas.BifrostContextKeyGovernanceCustomerID, "customer-id")
	ctx.SetValue(schemas.BifrostContextKeyUserID, "user-id")
	require.NoError(t, plugin.PreRequestHook(ctx, chatRequest("alex@example.com")))
	require.Len(t, metadata(t, ctx).Detections, 1)

	// A rule written against an ID must not fire on a lookalike credential or display name.
	ctx = newTestContext()
	ctx.SetValue(schemas.BifrostContextKeyVirtualKey, "vk-id")
	ctx.SetValue(schemas.BifrostContextKeyGovernanceTeamName, "team-id")
	ctx.SetValue(schemas.BifrostContextKeyGovernanceCustomerName, "customer-id")
	ctx.SetValue(schemas.BifrostContextKeyUserName, "user-id")
	require.NoError(t, plugin.PreRequestHook(ctx, chatRequest("alex@example.com")))
	_, ok := schemas.GuardrailMetadataFromContext(ctx)
	require.False(t, ok)
}

func TestRuleSelectionSamplingBounds(t *testing.T) {
	profile := enabledRegexProfile(t, 1, emailPattern(ActionDetectOnly))
	tests := map[string]struct {
		rate int
		runs bool
	}{
		"zero never runs":     {rate: 0, runs: false},
		"hundred always runs": {rate: 100, runs: true},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			rule := Rule{ID: 1, Enabled: true, SamplingRate: schemas.Ptr(test.rate), ProfileIDs: []uint{1}}
			plugin := mustInit(t, &testLogger{}, []Profile{profile}, []Rule{rule})
			ctx := newTestContext()
			for range 5 {
				require.NoError(t, plugin.PreRequestHook(ctx, chatRequest("alex@example.com")))
			}
			_, ok := schemas.GuardrailMetadataFromContext(ctx)
			require.Equal(t, test.runs, ok)
		})
	}
}

func TestRuleRuntimeErrorIsNoMatch(t *testing.T) {
	profile := enabledRegexProfile(t, 1, emailPattern(ActionBlock))
	// Division by zero only fails when evaluated: the expression compiles to bool.
	rule := Rule{ID: 1, Enabled: true, CELExpression: `headers.size() / (params.size() - params.size()) == 1`, ProfileIDs: []uint{1}}
	logger := &testLogger{}
	plugin := mustInit(t, logger, []Profile{profile}, []Rule{rule})

	ctx := newTestContext()
	require.NoError(t, plugin.PreRequestHook(ctx, chatRequest("alex@example.com")))
	_, ok := schemas.GuardrailMetadataFromContext(ctx)
	require.False(t, ok)
	require.Positive(t, logger.warnings)
}

func TestRuleMissingHeaderIsNoMatch(t *testing.T) {
	profile := enabledRegexProfile(t, 1, emailPattern(ActionBlock))
	rule := Rule{ID: 1, Enabled: true, CELExpression: `headers["x-absent"] == "yes"`, ProfileIDs: []uint{1}}
	logger := &testLogger{}
	plugin := mustInit(t, logger, []Profile{profile}, []Rule{rule})

	ctx := newTestContext()
	require.NoError(t, plugin.PreRequestHook(ctx, chatRequest("alex@example.com")))
	_, ok := schemas.GuardrailMetadataFromContext(ctx)
	require.False(t, ok)
	// A missing map key is an ordinary no-match, not a warning-worthy failure.
	require.Zero(t, logger.warnings)
}

func TestBlockShortCircuitError(t *testing.T) {
	profile := enabledRegexProfile(t, 1, emailPattern(ActionBlock))
	rule := Rule{ID: 7, Name: "block email", Enabled: true, ProfileIDs: []uint{1}}
	plugin := mustInit(t, &testLogger{}, []Profile{profile}, []Rule{rule})

	ctx := newTestContext()
	request := chatRequest("contact alex@example.com now")
	require.NoError(t, plugin.PreRequestHook(ctx, request))

	// A blocked request is not rewritten, and the input is left as the caller sent it.
	require.Equal(t, "contact alex@example.com now", *request.ChatRequest.Input[0].Content.ContentStr)

	_, shortCircuit, err := plugin.PreLLMHook(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, shortCircuit)
	require.NotNil(t, shortCircuit.Error)

	blocked := shortCircuit.Error
	require.NotNil(t, blocked.Type)
	require.Equal(t, guardrailInterventionErrorType, *blocked.Type)
	require.NotNil(t, blocked.StatusCode)
	require.Equal(t, http.StatusBadRequest, *blocked.StatusCode)
	require.NotNil(t, blocked.AllowFallbacks)
	require.False(t, *blocked.AllowFallbacks)
	require.NotNil(t, blocked.Error)
	require.Equal(t, guardrailInterventionErrorType, *blocked.Error.Type)
	require.Equal(t, "Blocked by "+testPolicyName+" policy: EMAIL_ADDRESS", blocked.Error.Message)
	require.NotContains(t, blocked.Error.Message, "alex@example.com")
}

func TestBlockRecordsInterventionWithoutMatchedText(t *testing.T) {
	profile := enabledRegexProfile(t, 1, emailPattern(ActionBlock))
	rule := Rule{ID: 9, Name: "block email", Enabled: true, ProfileIDs: []uint{1}}
	plugin := mustInit(t, &testLogger{}, []Profile{profile}, []Rule{rule})

	ctx := newTestContext()
	require.NoError(t, plugin.PreRequestHook(ctx, chatRequest("contact alex@example.com now")))

	found := metadata(t, ctx)
	require.Empty(t, found.Detections)
	require.Len(t, found.Interventions, 1)
	intervention := found.Interventions[0]
	require.Equal(t, ApplyToInput, intervention.Phase)
	require.Equal(t, testPolicyName, intervention.GuardrailName)
	require.Equal(t, ProviderRegex, intervention.GuardrailProvider)
	require.Equal(t, []string{"EMAIL_ADDRESS"}, intervention.Assessments)
	require.Equal(t, uint(9), *intervention.RuleID)
	require.Equal(t, "block email", intervention.RuleName)
	require.NotContains(t, intervention.Reason, "alex@example.com")
}

func TestDetectOnlyLeavesRequestAlone(t *testing.T) {
	profile := enabledRegexProfile(t, 1, emailPattern(ActionDetectOnly))
	rule := Rule{ID: 1, Enabled: true, ProfileIDs: []uint{1}}
	plugin := mustInit(t, &testLogger{}, []Profile{profile}, []Rule{rule})

	ctx := newTestContext()
	request := chatRequest("contact alex@example.com")
	require.NoError(t, plugin.PreRequestHook(ctx, request))
	require.Equal(t, "contact alex@example.com", *request.ChatRequest.Input[0].Content.ContentStr)

	found := metadata(t, ctx)
	require.Empty(t, found.Interventions)
	require.Len(t, found.Detections, 1)
	require.Equal(t, schemas.GuardrailDetectionActionDetect, found.Detections[0].Action)
	require.Equal(t, []string{"EMAIL_ADDRESS"}, found.Detections[0].Assessments)

	_, shortCircuit, err := plugin.PreLLMHook(ctx, request)
	require.NoError(t, err)
	require.Nil(t, shortCircuit)
}

func TestRedactRewritesRequestAndRecordsRedactionData(t *testing.T) {
	profile := enabledRegexProfile(t, 1, RegexPattern{
		Pattern:           `[\w.+-]+@[\w-]+\.[\w.]+`,
		EntityType:        "EMAIL_ADDRESS",
		Action:            ActionRedact,
		RedactionStrategy: StrategyReplace,
	})
	rule := Rule{ID: 1, Enabled: true, ProfileIDs: []uint{1}}
	plugin := mustInit(t, &testLogger{}, []Profile{profile}, []Rule{rule})

	ctx := newTestContext()
	request := chatRequest("contact alex@example.com and bob@example.com")
	require.NoError(t, plugin.PreRequestHook(ctx, request))

	require.Equal(t, "contact [EMAIL_ADDRESS] and [EMAIL_ADDRESS]", *request.ChatRequest.Input[0].Content.ContentStr)

	redaction, ok := schemas.RedactionDataFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, map[string]string{
		"alex@example.com": "[EMAIL_ADDRESS]",
		"bob@example.com":  "[EMAIL_ADDRESS]",
	}, redaction.LiteralReplacements.Input)

	found := metadata(t, ctx)
	require.Len(t, found.Detections, 1)
	require.Equal(t, schemas.GuardrailDetectionActionRedact, found.Detections[0].Action)
	require.NotContains(t, found.Detections[0].Reason, "alex@example.com")

	_, shortCircuit, err := plugin.PreLLMHook(ctx, request)
	require.NoError(t, err)
	require.Nil(t, shortCircuit)
}

func TestRedactUsesLongestLiteralFirst(t *testing.T) {
	// The shorter value is a prefix-free substring of the longer one; a naive map order could
	// replace it first and leave a mangled placeholder.
	profile := enabledRegexProfile(t, 1, RegexPattern{Pattern: `alex@example\.com`, EntityType: "EMAIL_ADDRESS", Action: ActionRedact})
	rule := Rule{ID: 1, Enabled: true, ProfileIDs: []uint{1}}
	plugin := mustInit(t, &testLogger{}, []Profile{profile}, []Rule{rule})

	ctx := newTestContext()
	request := chatRequest("alex@example.com")
	require.NoError(t, plugin.PreRequestHook(ctx, request))
	require.Equal(t, "[EMAIL_ADDRESS]", *request.ChatRequest.Input[0].Content.ContentStr)
}

func TestRedactRawBodyUsesRewriter(t *testing.T) {
	profile := enabledRegexProfile(t, 1, RegexPattern{Pattern: `[\w.+-]+@[\w-]+\.[\w.]+`, EntityType: "EMAIL_ADDRESS", Action: ActionRedact})
	rule := Rule{ID: 1, Enabled: true, ProfileIDs: []uint{1}}
	plugin := mustInit(t, &testLogger{}, []Profile{profile}, []Rule{rule})

	ctx := newTestContext()
	ctx.SetValue(schemas.BifrostContextKeyUseRawRequestBody, true)
	calls := 0
	ctx.SetValue(schemas.BifrostContextKeyRawRequestBodyTextRewriter, schemas.RawRequestBodyTextRewriter(func(rawBody []byte, replacements map[string]string) ([]byte, error) {
		calls++
		rewritten := string(rawBody)
		for literal, replacement := range replacements {
			rewritten = strings.ReplaceAll(rewritten, literal, replacement)
		}
		return []byte(rewritten), nil
	}))

	request := chatRequest("contact alex@example.com")
	request.ChatRequest.RawRequestBody = []byte(`{"messages":[{"content":"contact alex@example.com"}]}`)
	require.NoError(t, plugin.PreRequestHook(ctx, request))

	require.Equal(t, 1, calls)
	require.Equal(t, `{"messages":[{"content":"contact [EMAIL_ADDRESS]"}]}`, string(request.ChatRequest.RawRequestBody))
	require.Equal(t, "contact [EMAIL_ADDRESS]", *request.ChatRequest.Input[0].Content.ContentStr)

	_, shortCircuit, err := plugin.PreLLMHook(ctx, request)
	require.NoError(t, err)
	require.Nil(t, shortCircuit)
}

func TestRedactRawBodyWithoutRewriterBlocks(t *testing.T) {
	profile := enabledRegexProfile(t, 1, RegexPattern{Pattern: `[\w.+-]+@[\w-]+\.[\w.]+`, EntityType: "EMAIL_ADDRESS", Action: ActionRedact})
	rule := Rule{ID: 1, Enabled: true, ProfileIDs: []uint{1}}
	plugin := mustInit(t, &testLogger{}, []Profile{profile}, []Rule{rule})

	ctx := newTestContext()
	ctx.SetValue(schemas.BifrostContextKeyUseRawRequestBody, true)
	request := chatRequest("contact alex@example.com")
	request.ChatRequest.RawRequestBody = []byte(`{"messages":[{"content":"contact alex@example.com"}]}`)
	require.NoError(t, plugin.PreRequestHook(ctx, request))

	_, shortCircuit, err := plugin.PreLLMHook(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, shortCircuit)
	require.Equal(t, "Blocked by "+testPolicyName+" policy: detection unavailable", shortCircuit.Error.Error.Message)
}

func TestRedactRewriterErrorBlocks(t *testing.T) {
	profile := enabledRegexProfile(t, 1, RegexPattern{Pattern: `[\w.+-]+@[\w-]+\.[\w.]+`, EntityType: "EMAIL_ADDRESS", Action: ActionRedact})
	rule := Rule{ID: 1, Enabled: true, ProfileIDs: []uint{1}}
	plugin := mustInit(t, &testLogger{}, []Profile{profile}, []Rule{rule})

	ctx := newTestContext()
	ctx.SetValue(schemas.BifrostContextKeyUseRawRequestBody, true)
	ctx.SetValue(schemas.BifrostContextKeyRawRequestBodyTextRewriter, schemas.RawRequestBodyTextRewriter(func([]byte, map[string]string) ([]byte, error) {
		return nil, &schemas.RawRequestLiteralsNotFoundError{Literals: []string{"alex@example.com"}}
	}))

	request := chatRequest("contact alex@example.com")
	request.ChatRequest.RawRequestBody = []byte(`{"messages":[]}`)
	require.NoError(t, plugin.PreRequestHook(ctx, request))

	blocked, ok := ctx.Value(verdictKey{}).(*verdict)
	require.True(t, ok)
	require.True(t, blocked.failed)
	require.NotContains(t, blocked.message(), "alex@example.com")
	// The request is refused, so the log path is the only place the value could survive: it must
	// still carry the map the inspection applied.
	redaction, ok := schemas.RedactionDataFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, map[string]string{"alex@example.com": "[EMAIL_ADDRESS]"}, redaction.LiteralReplacements.Input)
}

func TestBlockingProfileStillPublishesRedactions(t *testing.T) {
	blocking := enabledRegexProfile(t, 1, emailPattern(ActionBlock))
	redacting := enabledRegexProfile(t, 2, RegexPattern{Pattern: `\+44\d{10}`, EntityType: "PHONE_NUMBER", Action: ActionRedact})
	rule := Rule{ID: 1, Enabled: true, ProfileIDs: []uint{1, 2}}
	plugin := mustInit(t, &testLogger{}, []Profile{blocking, redacting}, []Rule{rule})

	const input = "mail alex@example.com or call +447700900123"
	ctx := newTestContext()
	request := chatRequest(input)
	require.NoError(t, plugin.PreRequestHook(ctx, request))
	require.NotNil(t, verdictForRequest(ctx))

	// One policy blocked, so nothing is rewritten, but the phone number another policy redacts
	// must not reach the log as the caller sent it.
	redaction, ok := schemas.RedactionDataFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, map[string]string{"+447700900123": "[PHONE_NUMBER]"}, redaction.LiteralReplacements.Input)
	require.Equal(t, input, *request.ChatRequest.Input[0].Content.ContentStr)
}

// countingDetector counts calls, so a test can tell a re-inspection from a plain replay.
type countingDetector struct {
	calls int
	found []finding
}

func (d *countingDetector) Detect(context.Context, []string) ([]finding, error) {
	d.calls++
	return d.found, nil
}

func TestPreLLMHookInspectsTextALaterTurnAdded(t *testing.T) {
	profile := enabledRegexProfile(t, 1, emailPattern(ActionBlock))
	rule := Rule{ID: 1, Enabled: true, ProfileIDs: []uint{1}}
	plugin := mustInit(t, &testLogger{}, []Profile{profile}, []Rule{rule})

	ctx := newTestContext()
	request := chatRequest("hello")
	require.NoError(t, plugin.PreRequestHook(ctx, request))
	_, shortCircuit, err := plugin.PreLLMHook(ctx, request)
	require.NoError(t, err)
	require.Nil(t, shortCircuit)

	// An MCP continuation turn appends the tool result to the same request and calls the provider
	// without another PreRequestHook.
	request.ChatRequest.Input = append(request.ChatRequest.Input, schemas.ChatMessage{
		Role:    schemas.ChatMessageRoleTool,
		Content: &schemas.ChatMessageContent{ContentStr: schemas.Ptr("mail alex@example.com")},
	})
	_, shortCircuit, err = plugin.PreLLMHook(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, shortCircuit, "text added after PreRequestHook must be inspected")
	require.Equal(t, "Blocked by "+testPolicyName+" policy: EMAIL_ADDRESS", shortCircuit.Error.Error.Message)
	require.NotContains(t, shortCircuit.Error.Error.Message, "alex@example.com")
}

func TestPreLLMHookRedactsTextALaterTurnAdded(t *testing.T) {
	profile := enabledRegexProfile(t, 1, RegexPattern{Pattern: `[\w.+-]+@[\w-]+\.[\w.]+`, EntityType: "EMAIL_ADDRESS", Action: ActionRedact})
	rule := Rule{ID: 1, Enabled: true, ProfileIDs: []uint{1}}
	plugin := mustInit(t, &testLogger{}, []Profile{profile}, []Rule{rule})

	ctx := newTestContext()
	request := chatRequest("hello")
	require.NoError(t, plugin.PreRequestHook(ctx, request))

	request.ChatRequest.Input = append(request.ChatRequest.Input, schemas.ChatMessage{
		Role:    schemas.ChatMessageRoleTool,
		Content: &schemas.ChatMessageContent{ContentStr: schemas.Ptr("mail alex@example.com")},
	})
	_, shortCircuit, err := plugin.PreLLMHook(ctx, request)
	require.NoError(t, err)
	require.Nil(t, shortCircuit)
	require.Equal(t, "mail [EMAIL_ADDRESS]", *request.ChatRequest.Input[1].Content.ContentStr)
}

func TestPreLLMHookInspectsInjectedPromptTemplate(t *testing.T) {
	// The prompts plugin merges its template in this phase, after PreRequestHook looked at the
	// request; the template's text must be inspected before it reaches the provider.
	profile := enabledRegexProfile(t, 1, emailPattern(ActionBlock))
	rule := Rule{ID: 1, Enabled: true, ProfileIDs: []uint{1}}
	plugin := mustInit(t, &testLogger{}, []Profile{profile}, []Rule{rule})

	ctx := newTestContext()
	request := chatRequest("hello")
	require.NoError(t, plugin.PreRequestHook(ctx, request))

	request.ChatRequest.Input = append([]schemas.ChatMessage{{
		Role:    schemas.ChatMessageRoleSystem,
		Content: &schemas.ChatMessageContent{ContentStr: schemas.Ptr("escalate to alex@example.com")},
	}}, request.ChatRequest.Input...)
	_, shortCircuit, err := plugin.PreLLMHook(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, shortCircuit)
}

func TestPreLLMHookSkipsDetectionWhenTextIsUnchanged(t *testing.T) {
	profile := enabledRegexProfile(t, 1, emailPattern(ActionDetectOnly))
	rule := Rule{ID: 1, Enabled: true, ProfileIDs: []uint{1}}
	plugin := mustInit(t, &testLogger{}, []Profile{profile}, []Rule{rule})
	counter := &countingDetector{}
	plugin.snapshot.Load().rules[0].profiles[0].detector = counter

	ctx := newTestContext()
	request := chatRequest("hello")
	require.NoError(t, plugin.PreRequestHook(ctx, request))
	require.Equal(t, 1, counter.calls)

	for range 3 {
		_, shortCircuit, err := plugin.PreLLMHook(ctx, request)
		require.NoError(t, err)
		require.Nil(t, shortCircuit)
	}
	require.Equal(t, 1, counter.calls, "a fallback attempt with unchanged text must not run a detector")
}

func TestPreLLMHookReinspectsWhenTextChangesInPlace(t *testing.T) {
	// A later turn can replace text without changing the segment count or the total length, so
	// the fingerprint must cover the content too.
	profile := enabledRegexProfile(t, 1, emailPattern(ActionBlock))
	rule := Rule{ID: 1, Enabled: true, ProfileIDs: []uint{1}}
	plugin := mustInit(t, &testLogger{}, []Profile{profile}, []Rule{rule})

	ctx := newTestContext()
	request := chatRequest("mail alex at example!")
	require.NoError(t, plugin.PreRequestHook(ctx, request))
	_, shortCircuit, err := plugin.PreLLMHook(ctx, request)
	require.NoError(t, err)
	require.Nil(t, shortCircuit)

	// Same length as the first text, and still one segment.
	*request.ChatRequest.Input[0].Content.ContentStr = "mail alex@example.com"
	_, shortCircuit, err = plugin.PreLLMHook(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, shortCircuit, "text of the same size but different content must be inspected again")
}

func TestPreLLMHookDoesNotSampleTheSameRequestTwice(t *testing.T) {
	profile := enabledRegexProfile(t, 1, emailPattern(ActionBlock))
	rule := Rule{ID: 1, Enabled: true, SamplingRate: schemas.Ptr(0), ProfileIDs: []uint{1}}
	plugin := mustInit(t, &testLogger{}, []Profile{profile}, []Rule{rule})

	ctx := newTestContext()
	request := chatRequest("mail alex@example.com")
	require.NoError(t, plugin.PreRequestHook(ctx, request))
	_, shortCircuit, err := plugin.PreLLMHook(ctx, request)
	require.NoError(t, err)
	require.Nil(t, shortCircuit, "sampling decides once per request, so a replayed attempt must not run the rule")

	// A later turn is new text, and is inspected: sampling never excuses added input.
	request.ChatRequest.Input = append(request.ChatRequest.Input, schemas.ChatMessage{
		Role:    schemas.ChatMessageRoleTool,
		Content: &schemas.ChatMessageContent{ContentStr: schemas.Ptr("bob@example.com")},
	})
	_, shortCircuit, err = plugin.PreLLMHook(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, shortCircuit)
}

// slowDetector uses up its whole deadline, like an Analyzer that never answers.
type slowDetector struct{}

func (*slowDetector) Detect(ctx context.Context, _ []string) ([]finding, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

// expiryDetector records whether it was handed a deadline that had already passed.
type expiryDetector struct{ sawExpired bool }

func (d *expiryDetector) Detect(ctx context.Context, _ []string) ([]finding, error) {
	d.sawExpired = ctx.Err() != nil
	return nil, ctx.Err()
}

func TestRuleDeadlineIsSharedAcrossItsProfiles(t *testing.T) {
	// A rule's timeout is one absolute budget its profiles share: a detector that spends all of
	// it must not leave the next profile in the same rule a fresh one.
	first := enabledRegexProfile(t, 1, emailPattern(ActionBlock))
	second := enabledRegexProfile(t, 2, emailPattern(ActionBlock))
	rule := Rule{ID: 1, Enabled: true, Timeout: 1, ProfileIDs: []uint{1, 2}}
	plugin := mustInit(t, &testLogger{}, []Profile{first, second}, []Rule{rule})
	compiled := plugin.snapshot.Load().rules[0].profiles
	compiled[0].detector = &slowDetector{}
	expired := &expiryDetector{}
	compiled[1].detector = expired

	ctx := newTestContext()
	require.NoError(t, plugin.PreRequestHook(ctx, chatRequest("hello")))
	require.True(t, expired.sawExpired, "the second profile must inherit the rule's spent budget")
}

func TestUnsupportedRequestTextFailsClosed(t *testing.T) {
	rule := Rule{ID: 1, Enabled: true, ProfileIDs: []uint{1}}

	t.Run("enforcing profile blocks", func(t *testing.T) {
		logger := &testLogger{}
		plugin := mustInit(t, logger, []Profile{enabledRegexProfile(t, 1, emailPattern(ActionBlock))}, []Rule{rule})
		ctx := newTestContext()
		request := chatRequest("hello")
		runs := plugin.snapshot.Load().selectProfiles(ctx, request, logger, false)
		require.Len(t, runs, 1)

		plugin.blockUnsupportedText(ctx, request, runs)

		blocked := verdictForRequest(ctx)
		require.NotNil(t, blocked)
		require.Contains(t, blocked.message(), string(request.RequestType))
		require.NotContains(t, blocked.message(), "hello")
		require.Positive(t, logger.warnings)
	})

	t.Run("monitoring profile passes", func(t *testing.T) {
		logger := &testLogger{}
		plugin := mustInit(t, logger, []Profile{enabledRegexProfile(t, 1, emailPattern(ActionDetectOnly))}, []Rule{rule})
		ctx := newTestContext()
		request := chatRequest("hello")
		runs := plugin.snapshot.Load().selectProfiles(ctx, request, logger, false)

		plugin.blockUnsupportedText(ctx, request, runs)

		require.Nil(t, verdictForRequest(ctx))
		require.Positive(t, logger.warnings)
	})
}

// A request sampling skipped stays skipped on PreLLMHook even when its text cannot be walked:
// re-inspection ignores sampling, so without a marker it would refuse a request sampling let through.
func TestSampledOutUnsupportedRequestIsNotBlockedLater(t *testing.T) {
	zero := 0
	rule := Rule{ID: 1, Enabled: true, ProfileIDs: []uint{1}, SamplingRate: &zero}
	plugin := mustInit(t, &testLogger{}, []Profile{enabledRegexProfile(t, 1, emailPattern(ActionBlock))}, []Rule{rule})
	ctx := newTestContext()
	request := &schemas.BifrostRequest{
		RequestType:        schemas.PassthroughRequest,
		PassthroughRequest: &schemas.BifrostPassthroughRequest{BodySource: schemas.NewBytesBodySource([]byte("x"))},
	}

	require.NoError(t, plugin.PreRequestHook(ctx, request))
	_, shortCircuit, err := plugin.PreLLMHook(ctx, request)
	require.NoError(t, err)
	require.Nil(t, shortCircuit)
}

func TestReloadSerializesOverlappingBuilds(t *testing.T) {
	profile := enabledRegexProfile(t, 1, emailPattern(ActionBlock))
	rule := Rule{ID: 1, Enabled: true, ProfileIDs: []uint{1}}
	plugin := mustInit(t, &testLogger{}, nil, nil)

	entered := make(chan struct{})
	secondStarted := make(chan struct{})
	release := make(chan struct{})
	var mu sync.Mutex
	loads := 0
	plugin.load = func(context.Context) ([]Profile, []Rule, error) {
		mu.Lock()
		loads++
		n := loads
		mu.Unlock()
		if n == 1 {
			// The first reload read the older config, then stalled before publishing it.
			close(entered)
			<-release
			return nil, nil, nil
		}
		close(secondStarted)
		return []Profile{profile}, []Rule{rule}, nil
	}

	go func() { _ = plugin.Reload(context.Background()) }()
	<-entered
	go func() { _ = plugin.Reload(context.Background()) }()
	select {
	case <-secondStarted:
	case <-time.After(100 * time.Millisecond):
	}
	close(release)

	require.Eventually(t, func() bool {
		ctx := newTestContext()
		if err := plugin.PreRequestHook(ctx, chatRequest("alex@example.com")); err != nil {
			return false
		}
		return verdictForRequest(ctx) != nil
	}, 2*time.Second, 10*time.Millisecond, "the reload that read the newer config must win")
}

func TestVerdictIsBoundToItsRequest(t *testing.T) {
	profile := enabledRegexProfile(t, 1, emailPattern(ActionBlock))
	rule := Rule{ID: 1, Enabled: true, ProfileIDs: []uint{1}}
	plugin := mustInit(t, &testLogger{}, []Profile{profile}, []Rule{rule})

	ctx := newTestContext()
	ctx.SetValue(schemas.BifrostContextKeyRequestID, "req-1")
	require.NoError(t, plugin.PreRequestHook(ctx, chatRequest("mail alex@example.com")))
	_, shortCircuit, err := plugin.PreLLMHook(ctx, chatRequest("hello"))
	require.NoError(t, err)
	require.NotNil(t, shortCircuit)

	// A later request is served from the same context: it must not inherit that verdict.
	ctx.SetValue(schemas.BifrostContextKeyRequestID, "req-2")
	_, shortCircuit, err = plugin.PreLLMHook(ctx, chatRequest("hello"))
	require.NoError(t, err)
	require.Nil(t, shortCircuit)
}

func TestPassthroughBodyIsScannedAndRewritten(t *testing.T) {
	profile := enabledRegexProfile(t, 1, RegexPattern{Pattern: `[\w.+-]+@[\w-]+\.[\w.]+`, EntityType: "EMAIL_ADDRESS", Action: ActionRedact})
	rule := Rule{ID: 1, Enabled: true, ProfileIDs: []uint{1}}
	plugin := mustInit(t, &testLogger{}, []Profile{profile}, []Rule{rule})

	ctx := newTestContext()
	ctx.SetValue(schemas.BifrostContextKeyRawRequestBodyTextRewriter, schemas.RawRequestBodyTextRewriter(func(rawBody []byte, replacements map[string]string) ([]byte, error) {
		rewritten := string(rawBody)
		for literal, replacement := range replacements {
			rewritten = strings.ReplaceAll(rewritten, literal, replacement)
		}
		return []byte(rewritten), nil
	}))

	request := &schemas.BifrostRequest{
		RequestType:        schemas.PassthroughRequest,
		PassthroughRequest: &schemas.BifrostPassthroughRequest{Provider: schemas.OpenAI, Body: []byte(`{"input":"mail alex@example.com"}`)},
	}
	require.NoError(t, plugin.PreRequestHook(ctx, request))
	require.Equal(t, `{"input":"mail [EMAIL_ADDRESS]"}`, string(request.PassthroughRequest.Body))
}

func TestPresidioFailureIsFailClosedByDefault(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	config, err := json.Marshal(PresidioConfig{AnalyzerURL: server.URL, Action: ActionBlock})
	require.NoError(t, err)
	profile := Profile{ID: 1, ProviderName: ProviderPresidio, PolicyName: testPolicyName, Enabled: true, Config: config}
	rule := Rule{ID: 1, Enabled: true, ProfileIDs: []uint{1}}
	plugin := mustInit(t, &testLogger{}, []Profile{profile}, []Rule{rule})

	ctx := newTestContext()
	require.NoError(t, plugin.PreRequestHook(ctx, chatRequest("hello")))
	blocked, ok := ctx.Value(verdictKey{}).(*verdict)
	require.True(t, ok)
	require.True(t, blocked.failed)
}

func TestPresidioFailureFailOpenContinues(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	config, err := json.Marshal(PresidioConfig{AnalyzerURL: server.URL, Action: ActionBlock, FailOpen: true})
	require.NoError(t, err)
	profile := Profile{ID: 1, ProviderName: ProviderPresidio, PolicyName: testPolicyName, Enabled: true, Config: config}
	rule := Rule{ID: 1, Enabled: true, ProfileIDs: []uint{1}}
	logger := &testLogger{}
	plugin := mustInit(t, logger, []Profile{profile}, []Rule{rule})

	ctx := newTestContext()
	require.NoError(t, plugin.PreRequestHook(ctx, chatRequest("hello")))
	_, ok := ctx.Value(verdictKey{}).(*verdict)
	require.False(t, ok)
	require.Positive(t, logger.warnings)
}

func TestPresidioDetectOnlyFailureDoesNotBlock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	config, err := json.Marshal(PresidioConfig{AnalyzerURL: server.URL})
	require.NoError(t, err)
	profile := Profile{ID: 1, ProviderName: ProviderPresidio, PolicyName: testPolicyName, Enabled: true, Config: config}
	rule := Rule{ID: 1, Enabled: true, ProfileIDs: []uint{1}}
	plugin := mustInit(t, &testLogger{}, []Profile{profile}, []Rule{rule})

	ctx := newTestContext()
	require.NoError(t, plugin.PreRequestHook(ctx, chatRequest("hello")))
	_, ok := ctx.Value(verdictKey{}).(*verdict)
	require.False(t, ok)
}

func TestReloadKeepsPreviousSnapshotOnError(t *testing.T) {
	profile := enabledRegexProfile(t, 1, emailPattern(ActionDetectOnly))
	rule := Rule{ID: 1, Enabled: true, ProfileIDs: []uint{1}}
	loads := 0
	load := func(context.Context) ([]Profile, []Rule, error) {
		loads++
		if loads > 1 {
			return nil, nil, errors.New("database unavailable")
		}
		return []Profile{profile}, []Rule{rule}, nil
	}
	plugin, err := Init(context.Background(), &Config{}, &testLogger{}, load, nil)
	require.NoError(t, err)

	require.Error(t, plugin.Reload(context.Background()))

	ctx := newTestContext()
	require.NoError(t, plugin.PreRequestHook(ctx, chatRequest("alex@example.com")))
	require.Len(t, metadata(t, ctx).Detections, 1)
}

func TestReloadDisablesWhenRulesAreRemoved(t *testing.T) {
	profile := enabledRegexProfile(t, 1, emailPattern(ActionBlock))
	rule := Rule{ID: 1, Enabled: true, ProfileIDs: []uint{1}}
	plugin := mustInit(t, &testLogger{}, []Profile{profile}, []Rule{rule})

	plugin.load = staticLoader(nil, nil)
	require.NoError(t, plugin.Reload(context.Background()))

	ctx := newTestContext()
	require.NoError(t, plugin.PreRequestHook(ctx, chatRequest("alex@example.com")))
	_, ok := ctx.Value(verdictKey{}).(*verdict)
	require.False(t, ok)
}

func TestInitGeneratesHashKeyWhenNoneIsConfigured(t *testing.T) {
	logger := &testLogger{}
	profile := enabledRegexProfile(t, 1, RegexPattern{
		Pattern:           `[\w.+-]+@[\w-]+\.[\w.]+`,
		EntityType:        "EMAIL_ADDRESS",
		Action:            ActionRedact,
		RedactionStrategy: StrategyHash,
	})
	rule := Rule{ID: 1, Enabled: true, ProfileIDs: []uint{1}}
	plugin, err := Init(context.Background(), &Config{}, logger, staticLoader([]Profile{profile}, []Rule{rule}), nil)
	require.NoError(t, err)
	require.Len(t, plugin.hmacKey, 32)
	require.Positive(t, logger.warnings)
}

func BenchmarkPreRequestHookNoRules(b *testing.B) {
	plugin, err := Init(context.Background(), &Config{}, &testLogger{}, staticLoader(nil, nil), []byte("guardrails-test-hmac-key-32-bytes"))
	require.NoError(b, err)
	ctx := newTestContext()
	req := chatRequest("hello")

	b.ReportAllocs()
	for b.Loop() {
		if err := plugin.PreRequestHook(ctx, req); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPreRequestHookRedactHit(b *testing.B) {
	profile := Profile{ID: 1, ProviderName: ProviderRegex, PolicyName: testPolicyName, Enabled: true, Config: regexProfileJSON(b, RegexPattern{
		Pattern:           `[\w.+-]+@[\w-]+\.[\w.]+`,
		EntityType:        "EMAIL_ADDRESS",
		Action:            ActionRedact,
		RedactionStrategy: StrategyReplace,
	})}
	rule := Rule{ID: 1, Enabled: true, CELExpression: `provider == "openai"`, ProfileIDs: []uint{1}}
	plugin, err := Init(context.Background(), &Config{}, &testLogger{}, staticLoader([]Profile{profile}, []Rule{rule}), []byte("guardrails-test-hmac-key-32-bytes"))
	require.NoError(b, err)

	b.ReportAllocs()
	for b.Loop() {
		ctx := newTestContext()
		if err := plugin.PreRequestHook(ctx, chatRequest("contact alex@example.com")); err != nil {
			b.Fatal(err)
		}
	}
}
