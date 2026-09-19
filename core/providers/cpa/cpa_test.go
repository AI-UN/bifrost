package cpa

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	schemas "github.com/maximhq/bifrost/core/schemas"
)

type testLogger struct{}

func (testLogger) Debug(string, ...any)                   {}
func (testLogger) Info(string, ...any)                    {}
func (testLogger) Warn(string, ...any)                    {}
func (testLogger) Error(string, ...any)                   {}
func (testLogger) Fatal(string, ...any)                   {}
func (testLogger) SetLevel(schemas.LogLevel)              {}
func (testLogger) SetOutputType(schemas.LoggerOutputType) {}
func (testLogger) LogHTTPRequest(schemas.LogLevel, string) schemas.LogEventBuilder {
	return schemas.NoopLogEvent
}

// fakeCPA stands in for a CLI Proxy API server: it records the path of every
// request it receives and answers with a body in the dialect that path belongs to,
// so a delegated call completes end to end instead of failing in the response
// converter.
type fakeCPA struct {
	server *httptest.Server

	mu     sync.Mutex
	paths  []string
	status int
	body   string
}

func newFakeCPA(t *testing.T) *fakeCPA {
	t.Helper()
	f := &fakeCPA{}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.paths = append(f.paths, r.URL.Path)
		status, body := f.status, f.body
		f.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if status != 0 {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
			return
		}
		_, _ = w.Write([]byte(dialectResponseFor(r.URL.Path)))
	}))
	t.Cleanup(f.server.Close)
	return f
}

// lastPath returns the path of the most recent request, or "" when the provider
// never reached the server.
func (f *fakeCPA) lastPath() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.paths) == 0 {
		return ""
	}
	return f.paths[len(f.paths)-1]
}

func (f *fakeCPA) failWith(status int, body string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.status, f.body = status, body
}

func dialectResponseFor(path string) string {
	switch {
	case path == "/v1/messages":
		return `{"id":"msg_1","type":"message","role":"assistant","model":"m","content":[{"type":"text","text":"hi"}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`
	case path == "/v1/messages/count_tokens":
		return `{"input_tokens":7}`
	case strings.HasSuffix(path, ":generateContent"):
		return `{"candidates":[{"content":{"role":"model","parts":[{"text":"hi"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":1,"candidatesTokenCount":1,"totalTokenCount":2}}`
	case strings.HasSuffix(path, ":countTokens"):
		return `{"totalTokens":7}`
	case path == "/v1/responses" || path == "/v1/responses/compact":
		return `{"id":"resp_1","object":"response","created_at":1,"model":"m","status":"completed","output":[{"type":"message","id":"msg_1","status":"completed","role":"assistant","content":[{"type":"output_text","text":"hi"}]}],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`
	case path == "/v1/images/generations" || path == "/v1/images/edits":
		return `{"created":1,"data":[{"b64_json":"aGk="}]}`
	case path == "/v1/models":
		return `{"object":"list","data":[{"id":"gpt-5.6-sol","object":"model","owned_by":"openai"}]}`
	default:
		return `{"id":"c1","object":"chat.completion","created":1,"model":"m","choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`
	}
}

func newTestProvider(t *testing.T, baseURL string) *CPAProvider {
	t.Helper()
	provider, err := NewCPAProvider(&schemas.ProviderConfig{
		NetworkConfig: schemas.NetworkConfig{
			BaseURL:                        baseURL,
			DefaultRequestTimeoutInSeconds: 5,
			StreamIdleTimeoutInSeconds:     5,
			MaxConnsPerHost:                1,
			AllowPrivateNetwork:            true,
		},
		ConcurrencyAndBufferSize: schemas.ConcurrencyAndBufferSize{Concurrency: 1, BufferSize: 1},
	}, testLogger{})
	if err != nil {
		t.Fatalf("NewCPAProvider: %v", err)
	}
	return provider
}

func testCtx(integration string) *schemas.BifrostContext {
	ctx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
	if integration != "" {
		ctx.SetValue(schemas.BifrostContextKeyIntegrationType, integration)
	}
	return ctx
}

func testKey() schemas.Key {
	return schemas.Key{Value: schemas.SecretVar{Val: "test-key"}, Models: schemas.WhiteList{"*"}}
}

func chatRequest(model string) *schemas.BifrostChatRequest {
	msg := "hi"
	return &schemas.BifrostChatRequest{
		Provider: schemas.CPA,
		Model:    model,
		Input: []schemas.ChatMessage{{
			Role:    schemas.ChatMessageRoleUser,
			Content: &schemas.ChatMessageContent{ContentStr: &msg},
		}},
	}
}

func responsesRequest(model string) *schemas.BifrostResponsesRequest {
	return &schemas.BifrostResponsesRequest{
		Provider: schemas.CPA,
		Model:    model,
		Input: []schemas.ResponsesMessage{{
			Role:    schemas.Ptr(schemas.ResponsesInputMessageRoleUser),
			Content: &schemas.ResponsesMessageContent{ContentStr: schemas.Ptr("hi")},
		}},
	}
}

// TestChatCompletionDialectFollowsIngress is the core contract: the CPA endpoint a
// chat request lands on is decided by the protocol the client spoke, not by the
// model name. Both halves matter — a Claude Code request for a GPT model must still
// go to /v1/messages, and a Codex request for a Claude model must still go to
// /v1/chat/completions, because CPA owns the model-to-upstream mapping.
func TestChatCompletionDialectFollowsIngress(t *testing.T) {
	tests := []struct {
		name        string
		integration string
		model       string
		wantPath    string
	}{
		{"anthropic ingress, claude model", "anthropic", "claude-opus-4-8", "/v1/messages"},
		{"anthropic ingress, gpt model", "anthropic", "gpt-5.6-sol", "/v1/messages"},
		{"openai ingress, claude model", "openai", "claude-opus-4-8", "/v1/chat/completions"},
		{"openai ingress, gpt model", "openai", "gpt-5.6-sol", "/v1/chat/completions"},
		{"genai ingress", "genai", "gemini-3-pro", "/v1beta/models/gemini-3-pro:generateContent"},
		{"no integration falls back to openai", "", "claude-opus-4-8", "/v1/chat/completions"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fake := newFakeCPA(t)
			provider := newTestProvider(t, fake.server.URL)

			resp, bifrostErr := provider.ChatCompletion(testCtx(test.integration), testKey(), chatRequest(test.model))
			if bifrostErr != nil {
				t.Fatalf("ChatCompletion: %v", bifrostErr.Error.Message)
			}
			if resp == nil || len(resp.Choices) == 0 {
				t.Fatalf("expected a parsed chat response, got %#v", resp)
			}
			if got := fake.lastPath(); got != test.wantPath {
				t.Errorf("path = %q, want %q", got, test.wantPath)
			}
		})
	}
}

// TestResponsesDialectFollowsIngress pins the same rule on the Responses surface,
// where the OpenAI dialect has its own CPA route rather than sharing one.
func TestResponsesDialectFollowsIngress(t *testing.T) {
	tests := []struct {
		integration string
		wantPath    string
	}{
		{"openai", "/v1/responses"},
		{"anthropic", "/v1/messages"},
		{"genai", "/v1beta/models/gemini-3-pro:generateContent"},
	}

	for _, test := range tests {
		t.Run(test.integration, func(t *testing.T) {
			fake := newFakeCPA(t)
			provider := newTestProvider(t, fake.server.URL)

			_, bifrostErr := provider.Responses(testCtx(test.integration), testKey(), responsesRequest("gemini-3-pro"))
			if bifrostErr != nil {
				t.Fatalf("Responses: %v", bifrostErr.Error.Message)
			}
			if got := fake.lastPath(); got != test.wantPath {
				t.Errorf("path = %q, want %q", got, test.wantPath)
			}
		})
	}
}

// TestImageGenerationDialectFollowsModelFamily pins the one operation that must NOT
// follow the ingress protocol. CPA's two image surfaces accept disjoint model sets,
// so routing by ingress would send Gemini image models to an endpoint that rejects
// them with a 400.
func TestImageGenerationDialectFollowsModelFamily(t *testing.T) {
	tests := []struct {
		name        string
		integration string
		model       string
		wantPath    string
	}{
		{"codex image model on openai ingress", "openai", "gpt-image-2", "/v1/images/generations"},
		{"codex image model on anthropic ingress", "anthropic", "gpt-image-2", "/v1/images/generations"},
		{"xai image model", "openai", "grok-imagine-image", "/v1/images/generations"},
		{"gemini image model on openai ingress", "openai", "gemini-3-pro-image", "/v1beta/models/gemini-3-pro-image:generateContent"},
		{"gemini image model on genai ingress", "genai", "gemini-3-pro-image", "/v1beta/models/gemini-3-pro-image:generateContent"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fake := newFakeCPA(t)
			provider := newTestProvider(t, fake.server.URL)

			_, bifrostErr := provider.ImageGeneration(testCtx(test.integration), testKey(), &schemas.BifrostImageGenerationRequest{
				Provider: schemas.CPA,
				Model:    test.model,
				Input:    &schemas.ImageGenerationInput{Prompt: "a cat"},
				// Non-nil Params mirrors what the HTTP handlers build.
				// gemini.ToGeminiImageGenerationRequest dereferences Params before its
				// own nil check (core/providers/gemini/images.go:423 vs :429), so a nil
				// Params panics in there — a pre-existing upstream defect, unrelated to
				// this provider.
				Params: &schemas.ImageGenerationParameters{},
			})
			if bifrostErr != nil {
				t.Fatalf("ImageGeneration: %v", bifrostErr.Error.Message)
			}
			if got := fake.lastPath(); got != test.wantPath {
				t.Errorf("path = %q, want %q", got, test.wantPath)
			}
		})
	}
}

// TestImagenImageOperationsRejectedWithoutRequest guards the silent-failure case.
// Imagen publishes only :predict, and CPA's Gemini route dispatches nothing for that
// action and writes no body, so the client would see an empty 200. The request must
// therefore never leave Bifrost.
func TestImagenImageOperationsRejectedWithoutRequest(t *testing.T) {
	fake := newFakeCPA(t)
	provider := newTestProvider(t, fake.server.URL)

	_, bifrostErr := provider.ImageGeneration(testCtx("genai"), testKey(), &schemas.BifrostImageGenerationRequest{
		Provider: schemas.CPA,
		Model:    "imagen-4.0-generate-001",
		Input:    &schemas.ImageGenerationInput{Prompt: "a cat"},
	})
	if bifrostErr == nil {
		t.Fatal("expected an unsupported-operation error for an Imagen model")
	}
	if bifrostErr.ExtraFields.Provider != schemas.CPA {
		t.Errorf("error provider = %q, want %q", bifrostErr.ExtraFields.Provider, schemas.CPA)
	}
	if got := fake.lastPath(); got != "" {
		t.Errorf("Imagen request reached the server at %q; it must be rejected locally", got)
	}
}

// TestPinnedOpenAIDialectRoutes covers the operations CPA publishes on exactly one
// dialect, where following the ingress protocol would 404. Remote compaction is the
// notable one: it is a real upstream Codex call, not a local rewrite.
func TestPinnedOpenAIDialectRoutes(t *testing.T) {
	t.Run("compaction", func(t *testing.T) {
		fake := newFakeCPA(t)
		provider := newTestProvider(t, fake.server.URL)

		_, bifrostErr := provider.Compaction(testCtx("anthropic"), testKey(), &schemas.BifrostCompactionRequest{
			Provider: schemas.CPA,
			Model:    "gpt-5.6-sol",
			Input: []schemas.ResponsesMessage{{
				Role:    schemas.Ptr(schemas.ResponsesInputMessageRoleUser),
				Content: &schemas.ResponsesMessageContent{ContentStr: schemas.Ptr("hi")},
			}},
		})
		if bifrostErr != nil {
			t.Fatalf("Compaction: %v", bifrostErr.Error.Message)
		}
		if got := fake.lastPath(); got != "/v1/responses/compact" {
			t.Errorf("path = %q, want /v1/responses/compact", got)
		}
	})

	t.Run("list models queries one dialect only", func(t *testing.T) {
		fake := newFakeCPA(t)
		provider := newTestProvider(t, fake.server.URL)

		_, bifrostErr := provider.ListModels(testCtx("anthropic"), []schemas.Key{testKey()}, &schemas.BifrostListModelsRequest{Provider: schemas.CPA})
		if bifrostErr != nil {
			t.Fatalf("ListModels: %v", bifrostErr.Error.Message)
		}
		fake.mu.Lock()
		paths := append([]string(nil), fake.paths...)
		fake.mu.Unlock()
		if len(paths) != 1 || paths[0] != "/v1/models" {
			t.Errorf("paths = %v, want exactly [/v1/models]", paths)
		}
	})

	t.Run("count tokens uses the anthropic route for openai ingress", func(t *testing.T) {
		fake := newFakeCPA(t)
		provider := newTestProvider(t, fake.server.URL)

		_, bifrostErr := provider.CountTokens(testCtx("openai"), testKey(), responsesRequest("gpt-5.6-sol"))
		if bifrostErr != nil {
			t.Fatalf("CountTokens: %v", bifrostErr.Error.Message)
		}
		if got := fake.lastPath(); got != "/v1/messages/count_tokens" {
			t.Errorf("path = %q, want /v1/messages/count_tokens", got)
		}
	})
}

// TestSubProvidersReportCPAIdentity pins the identity normalization. Each
// sub-provider defaults to reporting its own name, and that name is what lands in
// unsupported-operation errors and in operation gating. Without the custom provider
// key a CPA request would be attributed to openai/anthropic/gemini.
func TestSubProvidersReportCPAIdentity(t *testing.T) {
	fake := newFakeCPA(t)
	provider := newTestProvider(t, fake.server.URL)

	if got := provider.GetProviderKey(); got != schemas.CPA {
		t.Errorf("CPAProvider.GetProviderKey() = %q, want %q", got, schemas.CPA)
	}
	for name, sub := range map[string]schemas.Provider{
		"openai":    provider.openai,
		"anthropic": provider.anthropic,
		"gemini":    provider.gemini,
	} {
		if got := sub.GetProviderKey(); got != schemas.CPA {
			t.Errorf("%s sub-provider GetProviderKey() = %q, want %q", name, got, schemas.CPA)
		}
	}
}

// TestDelegatedUnsupportedErrorsReportCPA: a sub-provider that does not implement an
// operation builds the error from its own provider key, so this is the end-to-end
// check that the key rewrite reaches error metadata. Gemini image streaming is the
// real instance — CPA has no streaming image route for Gemini models.
func TestDelegatedUnsupportedErrorsReportCPA(t *testing.T) {
	fake := newFakeCPA(t)
	provider := newTestProvider(t, fake.server.URL)

	_, bifrostErr := provider.ImageGenerationStream(testCtx("genai"), nil, nil, testKey(), &schemas.BifrostImageGenerationRequest{
		Provider: schemas.CPA,
		Model:    "gemini-3-pro-image",
		Input:    &schemas.ImageGenerationInput{Prompt: "a cat"},
		Params:   &schemas.ImageGenerationParameters{},
	})
	if bifrostErr == nil {
		t.Fatal("expected streaming image generation on a Gemini model to be unsupported")
	}
	if bifrostErr.ExtraFields.Provider != schemas.CPA {
		t.Errorf("error provider = %q, want %q", bifrostErr.ExtraFields.Provider, schemas.CPA)
	}
	if got := fake.lastPath(); got != "" {
		t.Errorf("unsupported operation reached the server at %q", got)
	}
}

// TestUpstreamErrorsSurface: an upstream rejection must come back as an error rather
// than as an empty success, for every dialect.
func TestUpstreamErrorsSurface(t *testing.T) {
	for _, integration := range []string{"openai", "anthropic", "genai"} {
		t.Run(integration, func(t *testing.T) {
			fake := newFakeCPA(t)
			fake.failWith(http.StatusBadRequest, `{"error":{"message":"model not found","type":"invalid_request_error"}}`)
			provider := newTestProvider(t, fake.server.URL)

			resp, bifrostErr := provider.ChatCompletion(testCtx(integration), testKey(), chatRequest("gpt-5.6-sol"))
			if bifrostErr == nil {
				t.Fatalf("expected an error from the upstream 400, got response %#v", resp)
			}
			if !strings.Contains(bifrostErr.Error.Message, "model not found") {
				t.Errorf("error message = %q, want it to carry the upstream message", bifrostErr.Error.Message)
			}
		})
	}
}

// TestNewCPAProviderRequiresBaseURL: CPA is always self-hosted, so silently falling
// back to a built-in default would send subscription traffic to api.openai.com.
func TestNewCPAProviderRequiresBaseURL(t *testing.T) {
	for _, baseURL := range []string{"", "   "} {
		_, err := NewCPAProvider(&schemas.ProviderConfig{
			NetworkConfig: schemas.NetworkConfig{BaseURL: baseURL},
		}, testLogger{})
		if err == nil {
			t.Fatalf("NewCPAProvider(%q) = nil error, want a configuration error", baseURL)
		}
	}
}

func TestPassthroughDialect(t *testing.T) {
	tests := []struct {
		path string
		want dialect
	}{
		{"/v1/messages", dialectAnthropic},
		{"/v1/messages/count_tokens", dialectAnthropic},
		{"/v1beta/models/gemini-3-pro:generateContent", dialectGemini},
		{"/v1beta", dialectGemini},
		{"/v1/chat/completions", dialectOpenAI},
		{"/v1/responses", dialectOpenAI},
		{"", dialectOpenAI},
	}
	for _, test := range tests {
		if got := passthroughDialect(test.path); got != test.want {
			t.Errorf("passthroughDialect(%q) = %v, want %v", test.path, got, test.want)
		}
	}
}

// TestDialectConfigIsolation: the sub-provider constructors mutate the config they
// are handed, so they must not share one. A shared config would leave whichever
// provider is built last owning everyone's base URL.
func TestDialectConfigIsolation(t *testing.T) {
	original := &schemas.ProviderConfig{
		NetworkConfig: schemas.NetworkConfig{
			BaseURL:      "http://cpa.internal:8317",
			ExtraHeaders: map[string]string{"x-test": "1"},
		},
		CustomProviderConfig: &schemas.CustomProviderConfig{
			AllowedRequests:       &schemas.AllowedRequests{ChatCompletion: true},
			DoesNotSendDoneMarker: true,
		},
	}

	openaiCfg := dialectConfig(original, "http://cpa.internal:8317", schemas.OpenAI)
	geminiCfg := dialectConfig(original, "http://cpa.internal:8317/v1beta", schemas.Gemini)

	if openaiCfg.NetworkConfig.BaseURL == geminiCfg.NetworkConfig.BaseURL {
		t.Fatal("dialect configs share a base URL; each sub-provider needs its own")
	}
	if original.NetworkConfig.BaseURL != "http://cpa.internal:8317" {
		t.Errorf("original config was mutated: base URL = %q", original.NetworkConfig.BaseURL)
	}
	if original.CustomProviderConfig.CustomProviderKey != "" {
		t.Errorf("original custom provider config was mutated: key = %q", original.CustomProviderConfig.CustomProviderKey)
	}
	for _, cfg := range []*schemas.ProviderConfig{openaiCfg, geminiCfg} {
		if cfg.CustomProviderConfig.CustomProviderKey != string(schemas.CPA) {
			t.Errorf("custom provider key = %q, want cpa", cfg.CustomProviderConfig.CustomProviderKey)
		}
		if !cfg.CustomProviderConfig.DoesNotSendDoneMarker {
			t.Error("operator-configured DoesNotSendDoneMarker was dropped")
		}
		if cfg.CustomProviderConfig.AllowedRequests == nil || !cfg.CustomProviderConfig.AllowedRequests.ChatCompletion {
			t.Error("operator-configured AllowedRequests was dropped")
		}
	}
	if openaiCfg.CustomProviderConfig.BaseProviderType != schemas.OpenAI {
		t.Errorf("openai base provider type = %q", openaiCfg.CustomProviderConfig.BaseProviderType)
	}
	if geminiCfg.CustomProviderConfig.BaseProviderType != schemas.Gemini {
		t.Errorf("gemini base provider type = %q", geminiCfg.CustomProviderConfig.BaseProviderType)
	}
}
