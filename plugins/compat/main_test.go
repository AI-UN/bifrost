package compat

import (
	"context"
	"testing"

	schemas "github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/modelcatalog"
)

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
