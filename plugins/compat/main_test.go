package compat

import (
	"context"
	"encoding/json"
	"testing"

	schemas "github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/modelcatalog"
)

func TestConfigUnmarshalConvertResponsesToChat(t *testing.T) {
	var enabled Config
	if err := json.Unmarshal([]byte(`{"convert_responses_to_chat":true}`), &enabled); err != nil {
		t.Fatalf("unmarshal enabled config: %v", err)
	}
	if !enabled.ConvertResponsesToChat {
		t.Fatal("expected convert_responses_to_chat to be enabled")
	}

	var omitted Config
	if err := json.Unmarshal([]byte(`{}`), &omitted); err != nil {
		t.Fatalf("unmarshal omitted config: %v", err)
	}
	if omitted.ConvertResponsesToChat {
		t.Fatal("expected convert_responses_to_chat to default to false when omitted")
	}
}

func TestPreLLMHook_ResponsesToChatOverrideForUnknownModel(t *testing.T) {
	plugin, err := Init(Config{}, nil, &modelcatalog.ModelCatalog{}, nil)
	if err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	ctx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
	ctx.SetValue(schemas.BifrostContextKeyCompatConvertResponsesToChat, true)

	req := &schemas.BifrostRequest{
		RequestType: schemas.ResponsesRequest,
		ResponsesRequest: &schemas.BifrostResponsesRequest{
			Provider: "xiaomi",
			Model:    "mimo-v2.5-pro",
			Input: []schemas.ResponsesMessage{
				{
					Role: schemas.Ptr(schemas.ResponsesInputMessageRoleUser),
					Content: &schemas.ResponsesMessageContent{
						ContentBlocks: []schemas.ResponsesMessageContentBlock{
							{
								Type: schemas.ResponsesInputMessageContentBlockTypeText,
								Text: schemas.Ptr("Say hello."),
							},
						},
					},
				},
			},
		},
	}

	_, _, err = plugin.PreLLMHook(ctx, req)
	if err != nil {
		t.Fatalf("PreLLMHook returned error: %v", err)
	}

	changeType, ok := ctx.Value(schemas.BifrostContextKeyChangeRequestType).(schemas.RequestType)
	if !ok {
		t.Fatal("expected change request type to be set")
	}
	if changeType != schemas.ChatCompletionRequest {
		t.Fatalf("expected change request type %q, got %q", schemas.ChatCompletionRequest, changeType)
	}
}

func TestPreLLMHook_ShouldConvertParamsOverrideClonesRequest(t *testing.T) {
	plugin, err := Init(Config{}, nil, &modelcatalog.ModelCatalog{}, nil)
	if err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	ctx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
	ctx.SetValue(schemas.BifrostContextKeyCompatShouldConvertParams, true)

	req := &schemas.BifrostRequest{
		RequestType: schemas.ChatCompletionRequest,
		ChatRequest: &schemas.BifrostChatRequest{
			Provider: schemas.Gemini,
			Model:    "gemini-2.5-pro",
			Input: []schemas.ChatMessage{
				{
					Role: schemas.ChatMessageRoleDeveloper,
					Content: &schemas.ChatMessageContent{
						ContentBlocks: []schemas.ChatContentBlock{
							{Type: schemas.ChatContentBlockTypeText, Text: schemas.Ptr("You are concise.")},
						},
					},
				},
			},
		},
	}

	modifiedReq, _, err := plugin.PreLLMHook(ctx, req)
	if err != nil {
		t.Fatalf("PreLLMHook returned error: %v", err)
	}
	if modifiedReq == req {
		t.Fatal("expected request clone when should_convert_params override is enabled")
	}
	if modifiedReq.ChatRequest.Input[0].Role != schemas.ChatMessageRoleSystem {
		t.Fatalf("expected converted role %q, got %q", schemas.ChatMessageRoleSystem, modifiedReq.ChatRequest.Input[0].Role)
	}
	if req.ChatRequest.Input[0].Role != schemas.ChatMessageRoleDeveloper {
		t.Fatalf("expected original request to remain %q, got %q", schemas.ChatMessageRoleDeveloper, req.ChatRequest.Input[0].Role)
	}
}

func TestPreLLMHook_UsesCustomProviderAllowedRequestsForResponsesFallback(t *testing.T) {
	plugin, err := Init(
		Config{ConvertResponsesToChat: true},
		nil,
		nil,
		func(provider schemas.ModelProvider) *schemas.CustomProviderConfig {
			if provider != "xiaomi" {
				return nil
			}
			return &schemas.CustomProviderConfig{
				AllowedRequests: &schemas.AllowedRequests{
					Responses:            false,
					ResponsesStream:      false,
					ChatCompletion:       true,
					ChatCompletionStream: true,
				},
			}
		},
	)
	if err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	ctx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
	req := &schemas.BifrostRequest{
		RequestType: schemas.ResponsesRequest,
		ResponsesRequest: &schemas.BifrostResponsesRequest{
			Provider: "xiaomi",
			Model:    "mimo-v2.5-pro",
		},
	}

	_, _, err = plugin.PreLLMHook(ctx, req)
	if err != nil {
		t.Fatalf("PreLLMHook returned error: %v", err)
	}

	changeType, ok := ctx.Value(schemas.BifrostContextKeyChangeRequestType).(schemas.RequestType)
	if !ok {
		t.Fatal("expected change request type to be set from custom provider allowed_requests")
	}
	if changeType != schemas.ChatCompletionRequest {
		t.Fatalf("expected change request type %q, got %q", schemas.ChatCompletionRequest, changeType)
	}
}

func TestPreLLMHook_DoesNotConvertWhenCustomProviderAllowsResponses(t *testing.T) {
	plugin, err := Init(
		Config{ConvertResponsesToChat: true},
		nil,
		&modelcatalog.ModelCatalog{},
		func(provider schemas.ModelProvider) *schemas.CustomProviderConfig {
			if provider != "xiaomi" {
				return nil
			}
			return &schemas.CustomProviderConfig{
				AllowedRequests: &schemas.AllowedRequests{
					Responses:            true,
					ResponsesStream:      true,
					ChatCompletion:       true,
					ChatCompletionStream: true,
				},
			}
		},
	)
	if err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	ctx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
	req := &schemas.BifrostRequest{
		RequestType: schemas.ResponsesRequest,
		ResponsesRequest: &schemas.BifrostResponsesRequest{
			Provider: "xiaomi",
			Model:    "mimo-v2.5-pro",
		},
	}

	_, _, err = plugin.PreLLMHook(ctx, req)
	if err != nil {
		t.Fatalf("PreLLMHook returned error: %v", err)
	}

	if _, ok := ctx.Value(schemas.BifrostContextKeyChangeRequestType).(schemas.RequestType); ok {
		t.Fatal("did not expect change request type when custom provider explicitly allows responses")
	}
}

func TestPreLLMHook_UsesStreamingAllowedRequestsForResponsesFallback(t *testing.T) {
	plugin, err := Init(
		Config{ConvertResponsesToChat: true},
		nil,
		nil,
		func(provider schemas.ModelProvider) *schemas.CustomProviderConfig {
			if provider != "xiaomi" {
				return nil
			}
			return &schemas.CustomProviderConfig{
				AllowedRequests: &schemas.AllowedRequests{
					Responses:            true,
					ResponsesStream:      false,
					ChatCompletion:       true,
					ChatCompletionStream: true,
				},
			}
		},
	)
	if err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	ctx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
	req := &schemas.BifrostRequest{
		RequestType: schemas.ResponsesStreamRequest,
		ResponsesRequest: &schemas.BifrostResponsesRequest{
			Provider: "xiaomi",
			Model:    "mimo-v2.5-pro",
		},
	}

	_, _, err = plugin.PreLLMHook(ctx, req)
	if err != nil {
		t.Fatalf("PreLLMHook returned error: %v", err)
	}

	changeType, ok := ctx.Value(schemas.BifrostContextKeyChangeRequestType).(schemas.RequestType)
	if !ok {
		t.Fatal("expected streaming responses request to fall back to chat")
	}
	if changeType != schemas.ChatCompletionRequest {
		t.Fatalf("expected change request type %q, got %q", schemas.ChatCompletionRequest, changeType)
	}
}

func TestPreLLMHook_DoesNotConvertWhenCustomProviderAllowsResponsesStream(t *testing.T) {
	plugin, err := Init(
		Config{ConvertResponsesToChat: true},
		nil,
		nil,
		func(provider schemas.ModelProvider) *schemas.CustomProviderConfig {
			if provider != "xiaomi" {
				return nil
			}
			return &schemas.CustomProviderConfig{
				AllowedRequests: &schemas.AllowedRequests{
					Responses:            false,
					ResponsesStream:      true,
					ChatCompletion:       true,
					ChatCompletionStream: true,
				},
			}
		},
	)
	if err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	ctx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
	req := &schemas.BifrostRequest{
		RequestType: schemas.ResponsesStreamRequest,
		ResponsesRequest: &schemas.BifrostResponsesRequest{
			Provider: "xiaomi",
			Model:    "mimo-v2.5-pro",
		},
	}

	_, _, err = plugin.PreLLMHook(ctx, req)
	if err != nil {
		t.Fatalf("PreLLMHook returned error: %v", err)
	}

	if _, ok := ctx.Value(schemas.BifrostContextKeyChangeRequestType).(schemas.RequestType); ok {
		t.Fatal("did not expect conversion when custom provider explicitly allows streaming responses")
	}
}

func TestPreLLMHook_DoesNotConvertResponsesStreamWithoutChatStreamSupport(t *testing.T) {
	plugin, err := Init(
		Config{ConvertResponsesToChat: true},
		nil,
		nil,
		func(provider schemas.ModelProvider) *schemas.CustomProviderConfig {
			if provider != "xiaomi" {
				return nil
			}
			return &schemas.CustomProviderConfig{
				AllowedRequests: &schemas.AllowedRequests{
					Responses:            false,
					ResponsesStream:      false,
					ChatCompletion:       true,
					ChatCompletionStream: false,
				},
			}
		},
	)
	if err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	ctx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
	req := &schemas.BifrostRequest{
		RequestType: schemas.ResponsesStreamRequest,
		ResponsesRequest: &schemas.BifrostResponsesRequest{
			Provider: "xiaomi",
			Model:    "mimo-v2.5-pro",
		},
	}

	_, _, err = plugin.PreLLMHook(ctx, req)
	if err != nil {
		t.Fatalf("PreLLMHook returned error: %v", err)
	}

	if _, ok := ctx.Value(schemas.BifrostContextKeyChangeRequestType).(schemas.RequestType); ok {
		t.Fatal("did not expect conversion without streaming chat support")
	}
}

// chatOnlyResolver models a third-party OpenAI-compatible provider (e.g. Xiaomi Mimo)
// that serves chat completions but has no Responses API.
func chatOnlyResolver(schemas.ModelProvider) *schemas.CustomProviderConfig {
	return &schemas.CustomProviderConfig{
		BaseProviderType: schemas.OpenAI,
		AllowedRequests: &schemas.AllowedRequests{
			ChatCompletion:       true,
			ChatCompletionStream: true,
		},
	}
}

// The fallback relies on the provider re-assembling Responses events. Anthropic Messages
// and Gemini GenerateContent arrive as ResponsesRequest too, but their routes cannot render
// chat chunks, so only callers on the OpenAI Responses wire may be downgraded.
func TestPreLLMHook_ResponsesToChatFallbackIsScopedToOpenAIWire(t *testing.T) {
	for _, tc := range []struct {
		integration string
		wantConvert bool
	}{
		{integration: "", wantConvert: true},
		{integration: "openai", wantConvert: true},
		{integration: "anthropic", wantConvert: false},
		{integration: "genai", wantConvert: false},
		{integration: "bedrock", wantConvert: false},
		{integration: "cohere", wantConvert: false},
	} {
		for _, requestType := range []schemas.RequestType{schemas.ResponsesRequest, schemas.ResponsesStreamRequest} {
			t.Run(tc.integration+"/"+string(requestType), func(t *testing.T) {
				plugin, err := Init(Config{ConvertResponsesToChat: true}, nil, nil, chatOnlyResolver)
				if err != nil {
					t.Fatalf("Init returned error: %v", err)
				}

				ctx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
				if tc.integration != "" {
					ctx.SetValue(schemas.BifrostContextKeyIntegrationType, tc.integration)
				}

				if _, _, err = plugin.PreLLMHook(ctx, &schemas.BifrostRequest{
					RequestType:      requestType,
					ResponsesRequest: &schemas.BifrostResponsesRequest{Provider: "xiaomi", Model: "mimo-v2.5-pro"},
				}); err != nil {
					t.Fatalf("PreLLMHook returned error: %v", err)
				}

				changeType, converted := ctx.Value(schemas.BifrostContextKeyChangeRequestType).(schemas.RequestType)
				if converted != tc.wantConvert {
					t.Fatalf("converted = %v, want %v", converted, tc.wantConvert)
				}
				if converted && changeType != schemas.ChatCompletionRequest {
					t.Fatalf("change request type = %q, want %q", changeType, schemas.ChatCompletionRequest)
				}
			})
		}
	}
}

// x-bf-compat is a deliberate per-request opt-in, so it overrides the wire scoping.
func TestPreLLMHook_ResponsesToChatOverrideBypassesWireScoping(t *testing.T) {
	plugin, err := Init(Config{}, nil, nil, chatOnlyResolver)
	if err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	ctx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
	ctx.SetValue(schemas.BifrostContextKeyIntegrationType, "anthropic")
	ctx.SetValue(schemas.BifrostContextKeyCompatConvertResponsesToChat, true)

	if _, _, err = plugin.PreLLMHook(ctx, &schemas.BifrostRequest{
		RequestType:      schemas.ResponsesStreamRequest,
		ResponsesRequest: &schemas.BifrostResponsesRequest{Provider: "xiaomi", Model: "mimo-v2.5-pro"},
	}); err != nil {
		t.Fatalf("PreLLMHook returned error: %v", err)
	}

	if _, ok := ctx.Value(schemas.BifrostContextKeyChangeRequestType).(schemas.RequestType); !ok {
		t.Fatal("expected the explicit x-bf-compat override to force conversion")
	}
}

// Azure DeepSeek reaches chat completions through a provider that re-assembles Responses
// events (openai.HandleOpenAIChatCompletionStreaming), so it is intentionally not scoped
// to the OpenAI wire — Claude Code on /anthropic/v1/messages still needs its reasoning.
func TestPreLLMHook_AzureDeepSeekConvertsOnAnthropicWire(t *testing.T) {
	plugin, err := Init(Config{AzureDeepseek: true}, nil, nil, nil)
	if err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	ctx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
	ctx.SetValue(schemas.BifrostContextKeyIntegrationType, "anthropic")
	ctx.SetValue(schemas.BifrostContextKeyUserAgent, "claude-cli/1.0.0")

	if _, _, err = plugin.PreLLMHook(ctx, &schemas.BifrostRequest{
		RequestType:      schemas.ResponsesStreamRequest,
		ResponsesRequest: &schemas.BifrostResponsesRequest{Provider: schemas.Azure, Model: "deepseek-r1"},
	}); err != nil {
		t.Fatalf("PreLLMHook returned error: %v", err)
	}

	if _, ok := ctx.Value(schemas.BifrostContextKeyChangeRequestType).(schemas.RequestType); !ok {
		t.Fatal("expected Azure DeepSeek to still convert for coding harnesses")
	}
}
