package siliconflow

import (
	"maps"
	"reflect"
	"testing"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrepareSiliconFlowChatRequest(t *testing.T) {

	tests := []struct {
		name      string
		request   *schemas.BifrostChatRequest
		wantExtra map[string]interface{}
	}{
		{
			name: "nil request is returned unchanged",
		},
		{
			name: "nil params are returned unchanged",
			request: &schemas.BifrostChatRequest{
				Model: "Qwen/Qwen3-32B",
			},
		},
		{
			name: "enabled true enables thinking",
			request: &schemas.BifrostChatRequest{
				Model: "Qwen/Qwen3-32B",
				Params: &schemas.ChatParameters{
					Reasoning:   &schemas.ChatReasoning{Enabled: new(true)},
					ExtraParams: map[string]interface{}{"preserved": "value"},
				},
			},
			wantExtra: map[string]interface{}{"preserved": "value", "enable_thinking": true},
		},
		{
			name: "enabled false disables thinking",
			request: &schemas.BifrostChatRequest{
				Model: "Qwen/Qwen3-32B",
				Params: &schemas.ChatParameters{
					Reasoning:   &schemas.ChatReasoning{Enabled: new(false)},
					ExtraParams: map[string]interface{}{"preserved": "value"},
				},
			},
			wantExtra: map[string]interface{}{"preserved": "value", "enable_thinking": false},
		},
		{
			name: "none effort disables thinking",
			request: &schemas.BifrostChatRequest{
				Model: "Qwen/Qwen3-32B",
				Params: &schemas.ChatParameters{
					Reasoning:   &schemas.ChatReasoning{Effort: new("none")},
					ExtraParams: map[string]interface{}{"preserved": "value"},
				},
			},
			wantExtra: map[string]interface{}{"preserved": "value", "enable_thinking": false},
		},
		{
			name: "high effort enables thinking",
			request: &schemas.BifrostChatRequest{
				Model: "Qwen/Qwen3-32B",
				Params: &schemas.ChatParameters{
					Reasoning:   &schemas.ChatReasoning{Effort: new("high")},
					ExtraParams: map[string]interface{}{"preserved": "value"},
				},
			},
			wantExtra: map[string]interface{}{"preserved": "value", "enable_thinking": true},
		},
		{
			name: "max tokens becomes thinking budget",
			request: &schemas.BifrostChatRequest{
				Model: "Qwen/Qwen3-32B",
				Params: &schemas.ChatParameters{
					Reasoning:   &schemas.ChatReasoning{MaxTokens: new(2048)},
					ExtraParams: map[string]interface{}{"preserved": "value"},
				},
			},
			wantExtra: map[string]interface{}{"preserved": "value", "thinking_budget": 2048},
		},
		{
			name: "deepseek v3.1 tools force thinking off",
			request: &schemas.BifrostChatRequest{
				Model: "deepseek-ai/DeepSeek-V3.1-Terminus",
				Params: &schemas.ChatParameters{
					Reasoning: &schemas.ChatReasoning{Enabled: new(true)},
					Tools: []schemas.ChatTool{
						{
							Type: schemas.ChatToolTypeFunction,
							Function: &schemas.ChatToolFunction{
								Name: "lookup",
							},
						},
					},
					ExtraParams: map[string]interface{}{"preserved": "value"},
				},
			},
			wantExtra: map[string]interface{}{"preserved": "value", "enable_thinking": false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.request == nil {
				assert.Nil(t, prepareSiliconFlowChatRequest(nil))
				return
			}

			originalRequest := tt.request
			requestSnapshot := *originalRequest
			if originalRequest.Params == nil {
				got := prepareSiliconFlowChatRequest(originalRequest)
				assert.Same(t, originalRequest, got)
				assert.Equal(t, requestSnapshot, *originalRequest)
				return
			}

			originalParams := originalRequest.Params
			paramsSnapshot := *originalParams
			originalReasoning := originalParams.Reasoning
			originalExtraParams := originalParams.ExtraParams
			extraParamsSnapshot := maps.Clone(originalExtraParams)
			originalExtraParamsPointer := reflect.ValueOf(originalExtraParams).Pointer()

			got := prepareSiliconFlowChatRequest(originalRequest)
			require.NotNil(t, got)
			assert.NotSame(t, originalRequest, got)
			require.NotNil(t, got.Params)
			assert.NotSame(t, originalParams, got.Params)
			assert.Nil(t, got.Params.Reasoning)
			assert.Equal(t, tt.wantExtra, got.Params.ExtraParams)
			assert.NotEqual(t, originalExtraParamsPointer, reflect.ValueOf(got.Params.ExtraParams).Pointer())

			assert.Same(t, originalRequest, tt.request)
			assert.Same(t, originalParams, originalRequest.Params)
			assert.Same(t, originalReasoning, originalRequest.Params.Reasoning)
			assert.Equal(t, requestSnapshot, *originalRequest)
			assert.Equal(t, paramsSnapshot, *originalParams)
			assert.Equal(t, extraParamsSnapshot, originalExtraParams)
			assert.Equal(t, originalExtraParamsPointer, reflect.ValueOf(originalRequest.Params.ExtraParams).Pointer())
		})
	}
}
