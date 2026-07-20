package siliconflow

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var _ schemas.Provider = (*SiliconFlowProvider)(nil)

func TestNewSiliconFlowProvider(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		wantURL string
	}{
		{
			name:    "defaults to international API",
			wantURL: "https://api.siliconflow.com",
		},
		{
			name:    "trims trailing slashes",
			baseURL: "https://api.siliconflow.com///",
			wantURL: "https://api.siliconflow.com",
		},
		{
			name:    "preserves China region override",
			baseURL: "https://api.siliconflow.cn/",
			wantURL: "https://api.siliconflow.cn",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewSiliconFlowProvider(&schemas.ProviderConfig{
				NetworkConfig: schemas.NetworkConfig{BaseURL: tt.baseURL},
			}, siliconFlowSpeechTestLogger{})
			require.NoError(t, err)
			require.NotNil(t, provider)
			assert.Equal(t, tt.wantURL, provider.networkConfig.BaseURL)
			assert.Equal(t, schemas.SiliconFlow, provider.GetProviderKey())
			require.NotNil(t, provider.client)
			require.NotNil(t, provider.streamingClient)
			assert.NotSame(t, provider.client, provider.streamingClient)
		})
	}
}

func TestSiliconFlowProviderChatCompletion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"id":"chatcmpl-1","object":"chat.completion","created":1,"model":"deepseek-ai/DeepSeek-V3.2","choices":[{"index":0,"message":{"role":"assistant","content":"hello from SiliconFlow"},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}}`)
	}))
	defer server.Close()

	provider := newSiliconFlowProviderForHTTPTest(t, server.URL)
	ctx, cancel := schemas.NewBifrostContextWithCancel(context.Background())
	defer cancel()

	response, bifrostErr := provider.ChatCompletion(ctx, siliconFlowProviderTestKey(), siliconFlowChatTestRequest("hello"))
	require.Nil(t, bifrostErr)
	require.NotNil(t, response)
	require.Len(t, response.Choices, 1)
	require.NotNil(t, response.Choices[0].ChatNonStreamResponseChoice)
	require.NotNil(t, response.Choices[0].Message)
	require.NotNil(t, response.Choices[0].Message.Content)
	require.NotNil(t, response.Choices[0].Message.Content.ContentStr)
	assert.Equal(t, "hello from SiliconFlow", *response.Choices[0].Message.Content.ContentStr)
	assert.Equal(t, "deepseek-ai/DeepSeek-V3.2", response.Model)
	require.NotNil(t, response.Usage)
	assert.Equal(t, 5, response.Usage.TotalTokens)
}

func TestSiliconFlowProviderChatCompletionStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("test response writer does not support flushing")
			return
		}
		_, _ = fmt.Fprint(w, "data: {\"id\":\"chatcmpl-stream\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"deepseek-ai/DeepSeek-V3.2\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"hello \"},\"finish_reason\":null}]}\n\n")
		flusher.Flush()
		_, _ = fmt.Fprint(w, "data: {\"id\":\"chatcmpl-stream\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"deepseek-ai/DeepSeek-V3.2\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"stream\"},\"finish_reason\":\"stop\"}]}\n\n")
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	provider := newSiliconFlowProviderForHTTPTest(t, server.URL)
	ctx, cancel := schemas.NewBifrostContextWithCancel(context.Background())
	defer cancel()

	stream, bifrostErr := provider.ChatCompletionStream(
		ctx,
		siliconFlowProviderTestPostHook,
		nil,
		siliconFlowProviderTestKey(),
		siliconFlowChatTestRequest("stream please"),
	)
	require.Nil(t, bifrostErr)
	require.NotNil(t, stream)

	var content strings.Builder
	var sawFinish bool
	for chunk := range stream {
		require.NotNil(t, chunk)
		require.Nil(t, chunk.BifrostError)
		if chunk.BifrostChatResponse == nil {
			continue
		}
		for _, choice := range chunk.BifrostChatResponse.Choices {
			if choice.ChatStreamResponseChoice != nil && choice.Delta != nil && choice.Delta.Content != nil {
				content.WriteString(*choice.Delta.Content)
			}
			if choice.FinishReason != nil && *choice.FinishReason == string(schemas.BifrostFinishReasonStop) {
				sawFinish = true
			}
		}
	}
	assert.Equal(t, "hello stream", content.String())
	assert.True(t, sawFinish)
}

func TestSiliconFlowProviderResponsesUsesChatFallback(t *testing.T) {
	var requestedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"id":"chatcmpl-response","object":"chat.completion","created":1,"model":"deepseek-ai/DeepSeek-V3.2","choices":[{"index":0,"message":{"role":"assistant","content":"fallback output"},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":2,"total_tokens":4}}`)
	}))
	defer server.Close()

	provider := newSiliconFlowProviderForHTTPTest(t, server.URL)
	ctx, cancel := schemas.NewBifrostContextWithCancel(context.Background())
	defer cancel()
	messageType := schemas.ResponsesMessageTypeMessage
	role := schemas.ResponsesInputMessageRoleUser
	input := "hello"

	response, bifrostErr := provider.Responses(ctx, siliconFlowProviderTestKey(), &schemas.BifrostResponsesRequest{
		Provider: schemas.SiliconFlow,
		Model:    "deepseek-ai/DeepSeek-V3.2",
		Input: []schemas.ResponsesMessage{{
			Type: &messageType,
			Role: &role,
			Content: &schemas.ResponsesMessageContent{
				ContentStr: &input,
			},
		}},
	})
	require.Nil(t, bifrostErr)
	require.NotNil(t, response)
	assert.Equal(t, "/v1/chat/completions", requestedPath)
	require.NotEmpty(t, response.Output)
	require.NotNil(t, response.Output[0].Content)
	require.NotEmpty(t, response.Output[0].Content.ContentBlocks)
	require.NotNil(t, response.Output[0].Content.ContentBlocks[0].Text)
	assert.Equal(t, "fallback output", *response.Output[0].Content.ContentBlocks[0].Text)
}

func TestSiliconFlowProviderUnsupportedOperations(t *testing.T) {
	provider := &SiliconFlowProvider{}
	key := schemas.Key{}
	tests := []struct {
		name   string
		invoke func() *schemas.BifrostError
	}{
		{name: "TranscriptionStream", invoke: func() *schemas.BifrostError {
			_, err := provider.TranscriptionStream(nil, nil, nil, key, nil)
			return err
		}},
		{name: "ImageVariation", invoke: func() *schemas.BifrostError { _, err := provider.ImageVariation(nil, key, nil); return err }},
		{name: "BatchDelete", invoke: func() *schemas.BifrostError { _, err := provider.BatchDelete(nil, nil, nil); return err }},
		{name: "BatchResults", invoke: func() *schemas.BifrostError { _, err := provider.BatchResults(nil, nil, nil); return err }},
		{name: "FileRetrieve", invoke: func() *schemas.BifrostError { _, err := provider.FileRetrieve(nil, nil, nil); return err }},
		{name: "FileDelete", invoke: func() *schemas.BifrostError { _, err := provider.FileDelete(nil, nil, nil); return err }},
		{name: "FileContent", invoke: func() *schemas.BifrostError { _, err := provider.FileContent(nil, nil, nil); return err }},
		{name: "VideoList", invoke: func() *schemas.BifrostError { _, err := provider.VideoList(nil, key, nil); return err }},
		{name: "VideoDelete", invoke: func() *schemas.BifrostError { _, err := provider.VideoDelete(nil, key, nil); return err }},
		{name: "VideoRemix", invoke: func() *schemas.BifrostError { _, err := provider.VideoRemix(nil, key, nil); return err }},
		{name: "CountTokens", invoke: func() *schemas.BifrostError { _, err := provider.CountTokens(nil, key, nil); return err }},
		{name: "OCR", invoke: func() *schemas.BifrostError { _, err := provider.OCR(nil, key, nil); return err }},
		{name: "Compaction", invoke: func() *schemas.BifrostError { _, err := provider.Compaction(nil, key, nil); return err }},
		{name: "Passthrough", invoke: func() *schemas.BifrostError { _, err := provider.Passthrough(nil, key, nil); return err }},
		{name: "CachedContentCreate", invoke: func() *schemas.BifrostError { _, err := provider.CachedContentCreate(nil, key, nil); return err }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bifrostErr := tt.invoke()
			require.NotNil(t, bifrostErr)
			require.NotNil(t, bifrostErr.Error)
			assert.NotEmpty(t, bifrostErr.Error.Message)
		})
	}
}

func newSiliconFlowProviderForHTTPTest(t *testing.T, baseURL string) *SiliconFlowProvider {
	t.Helper()
	provider, err := NewSiliconFlowProvider(&schemas.ProviderConfig{
		NetworkConfig: schemas.NetworkConfig{
			BaseURL:                        baseURL,
			DefaultRequestTimeoutInSeconds: 5,
			StreamIdleTimeoutInSeconds:     5,
		},
	}, siliconFlowSpeechTestLogger{})
	require.NoError(t, err)
	return provider
}

func siliconFlowProviderTestKey() schemas.Key {
	return schemas.Key{Value: *schemas.NewSecretVar("test-key")}
}

func siliconFlowChatTestRequest(content string) *schemas.BifrostChatRequest {
	return &schemas.BifrostChatRequest{
		Provider: schemas.SiliconFlow,
		Model:    "deepseek-ai/DeepSeek-V3.2",
		Input: []schemas.ChatMessage{{
			Role: schemas.ChatMessageRoleUser,
			Content: &schemas.ChatMessageContent{
				ContentStr: schemas.Ptr(content),
			},
		}},
	}
}

func siliconFlowProviderTestPostHook(
	_ *schemas.BifrostContext,
	result *schemas.BifrostResponse,
	bifrostErr *schemas.BifrostError,
) (*schemas.BifrostResponse, *schemas.BifrostError) {
	return result, bifrostErr
}
