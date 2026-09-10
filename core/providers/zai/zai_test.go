package zai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testLogger struct{}

var noopPostHookRunner schemas.PostHookRunner = func(_ *schemas.BifrostContext, result *schemas.BifrostResponse, err *schemas.BifrostError) (*schemas.BifrostResponse, *schemas.BifrostError) {
	return result, err
}

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

func testChatRequest(provider schemas.ModelProvider) *schemas.BifrostChatRequest {
	content := "Hello"
	user := "user-123456"
	effort := "high"
	enabled := true
	maxTokens := 64
	temperature := 0.3
	toolChoice := string(schemas.ChatToolChoiceTypeAuto)
	responseFormat := any(map[string]any{"type": "json_object"})
	return &schemas.BifrostChatRequest{
		Provider: provider,
		Model:    "glm-5",
		Input: []schemas.ChatMessage{{
			Role:    schemas.ChatMessageRoleUser,
			Content: &schemas.ChatMessageContent{ContentStr: &content},
		}},
		Params: &schemas.ChatParameters{
			MaxCompletionTokens: &maxTokens,
			Temperature:         &temperature,
			User:                &user,
			Reasoning: &schemas.ChatReasoning{
				Enabled: &enabled,
				Effort:  &effort,
			},
			ToolChoice:     &schemas.ChatToolChoice{ChatToolChoiceStr: &toolChoice},
			ResponseFormat: &responseFormat,
			ExtraParams: map[string]any{
				"clear_thinking": false,
				"do_sample":      true,
				"request_id":     "request-123456",
				"tool_stream":    true,
			},
		},
	}
}

func newTestProvider(t *testing.T, provider schemas.ModelProvider, baseURL string) *ZAIProvider {
	t.Helper()
	config := &schemas.ProviderConfig{NetworkConfig: schemas.NetworkConfig{
		BaseURL:                        baseURL,
		DefaultRequestTimeoutInSeconds: 5,
		StreamIdleTimeoutInSeconds:     5,
		MaxConnsPerHost:                2,
	}}
	var result *ZAIProvider
	var err error
	if provider == schemas.Zhipu {
		result, err = NewZhipuProvider(config, testLogger{})
	} else {
		result, err = NewZAIProvider(config, testLogger{})
	}
	require.NoError(t, err)
	return result
}

func TestProviderProfiles(t *testing.T) {
	international, err := NewZAIProvider(&schemas.ProviderConfig{}, testLogger{})
	require.NoError(t, err)
	assert.Equal(t, schemas.ZAI, international.GetProviderKey())
	assert.Equal(t, defaultInternationalBaseURL, international.networkConfig.BaseURL)

	china, err := NewZhipuProvider(&schemas.ProviderConfig{}, testLogger{})
	require.NoError(t, err)
	assert.Equal(t, schemas.Zhipu, china.GetProviderKey())
	assert.Equal(t, defaultChinaBaseURL, china.networkConfig.BaseURL)
}
func TestResponsesUseResponsesPermissions(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var requestBody map[string]interface{}
		require.NoError(t, json.Unmarshal(body, &requestBody))
		if requestBody["stream"] == true {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, "data: {\"id\":\"chat-1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}]}\n\n")
			_, _ = io.WriteString(w, "data: [DONE]\n\n")
			return
		}
		_, _ = w.Write([]byte(`{"id":"chat-1","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`))
	}))
	defer server.Close()

	provider := newTestProvider(t, schemas.ZAI, server.URL)
	provider.customProviderConfig = &schemas.CustomProviderConfig{AllowedRequests: &schemas.AllowedRequests{
		Responses:       true,
		ResponsesStream: true,
	}}
	request := testResponsesRequest()
	response, bifrostErr := provider.Responses(
		schemas.NewBifrostContext(context.Background(), schemas.NoDeadline),
		schemas.Key{Value: *schemas.NewSecretVar("test-key")},
		request,
	)
	require.Nil(t, bifrostErr)
	require.NotNil(t, response)

	stream, bifrostErr := provider.ResponsesStream(
		schemas.NewBifrostContext(context.Background(), schemas.NoDeadline),
		noopPostHookRunner,
		nil,
		schemas.Key{Value: *schemas.NewSecretVar("test-key")},
		request,
	)
	require.Nil(t, bifrostErr)
	for range stream {
	}
	assert.Equal(t, int32(2), calls.Load())

	provider.customProviderConfig.AllowedRequests = &schemas.AllowedRequests{
		ChatCompletion:       true,
		ChatCompletionStream: true,
	}
	response, bifrostErr = provider.Responses(
		schemas.NewBifrostContext(context.Background(), schemas.NoDeadline),
		schemas.Key{Value: *schemas.NewSecretVar("test-key")},
		request,
	)
	assert.Nil(t, response)
	require.NotNil(t, bifrostErr)
	_, bifrostErr = provider.ResponsesStream(
		schemas.NewBifrostContext(context.Background(), schemas.NoDeadline),
		noopPostHookRunner,
		nil,
		schemas.Key{Value: *schemas.NewSecretVar("test-key")},
		request,
	)
	require.NotNil(t, bifrostErr)
	assert.Equal(t, int32(2), calls.Load())
}

func testResponsesRequest() *schemas.BifrostResponsesRequest {
	role := schemas.ResponsesInputMessageRoleUser
	content := "hello"
	return &schemas.BifrostResponsesRequest{
		Provider: schemas.ZAI,
		Model:    "glm-5",
		Input: []schemas.ResponsesMessage{{
			Role:    &role,
			Content: &schemas.ResponsesMessageContent{ContentStr: &content},
		}},
	}
}

func TestChatCompletionUsesNativeRequestForBothProfiles(t *testing.T) {
	for _, providerName := range []schemas.ModelProvider{schemas.ZAI, schemas.Zhipu} {
		t.Run(string(providerName), func(t *testing.T) {
			var captured map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, pathChatCompletions, r.URL.Path)
				assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
				body, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				require.NoError(t, json.Unmarshal(body, &captured))
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"id":"chat-1","object":"chat.completion","created":1,"model":"glm-5","choices":[{"index":0,"finish_reason":"stop","message":{"role":"assistant","content":"hello","reasoning_content":"reason"}}],"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}}`))
			}))
			defer server.Close()

			provider := newTestProvider(t, providerName, server.URL)
			ctx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
			response, bifrostErr := provider.ChatCompletion(ctx, schemas.Key{Value: *schemas.NewSecretVar("test-key")}, testChatRequest(providerName))
			require.Nil(t, bifrostErr)
			require.NotNil(t, response)
			require.Len(t, response.Choices, 1)
			require.NotNil(t, response.Choices[0].ChatNonStreamResponseChoice)
			require.NotNil(t, response.Choices[0].Message.ChatAssistantMessage)
			assert.Equal(t, "reason", *response.Choices[0].Message.ChatAssistantMessage.Reasoning)

			assert.Equal(t, "glm-5", captured["model"])
			assert.Equal(t, float64(64), captured["max_tokens"])
			assert.NotContains(t, captured, "max_completion_tokens")
			assert.Equal(t, "high", captured["reasoning_effort"])
			assert.Equal(t, map[string]any{"type": "enabled"}, captured["thinking"])
			assert.Equal(t, "user-123456", captured["user_id"])
			assert.NotContains(t, captured, "user")
			assert.Equal(t, true, captured["do_sample"])
			assert.Equal(t, true, captured["tool_stream"])
			assert.Equal(t, false, captured["clear_thinking"])
			assert.Equal(t, "request-123456", captured["request_id"])
			assert.Equal(t, false, captured["stream"])
		})
	}
}

func TestChatCompletionPreservesFileURLsForBothProfiles(t *testing.T) {
	for _, providerName := range []schemas.ModelProvider{schemas.ZAI, schemas.Zhipu} {
		t.Run(string(providerName), func(t *testing.T) {
			var captured map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				require.NoError(t, json.Unmarshal(body, &captured))
				_, _ = w.Write([]byte(`{"id":"chat-1","choices":[{"index":0,"finish_reason":"stop","message":{"role":"assistant","content":"ok"}}]}`))
			}))
			defer server.Close()

			request := testChatRequest(providerName)
			request.Input[0].Content = &schemas.ChatMessageContent{
				ContentBlocks: []schemas.ChatContentBlock{{
					Type: schemas.ChatContentBlockTypeFile,
					File: &schemas.ChatInputFile{
						FileURL:  schemas.Ptr("https://example.com/document.pdf"),
						Filename: schemas.Ptr("document.pdf"),
					},
				}},
			}
			provider := newTestProvider(t, providerName, server.URL)
			response, bifrostErr := provider.ChatCompletion(
				schemas.NewBifrostContext(context.Background(), schemas.NoDeadline),
				schemas.Key{Value: *schemas.NewSecretVar("test-key")},
				request,
			)
			require.Nil(t, bifrostErr)
			require.NotNil(t, response)
			assertFileURLPreserved(t, captured)
		})
	}
}

func assertFileURLPreserved(t *testing.T, captured map[string]any) {
	t.Helper()
	messages, ok := captured["messages"].([]any)
	require.True(t, ok)
	require.Len(t, messages, 1)
	message, ok := messages[0].(map[string]any)
	require.True(t, ok)
	content, ok := message["content"].([]any)
	require.True(t, ok)
	require.Len(t, content, 1)
	block, ok := content[0].(map[string]any)
	require.True(t, ok)
	file, ok := block["file"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "https://example.com/document.pdf", file["file_url"])
	assert.Equal(t, "document.pdf", file["filename"])
}

func TestChatCompletionNormalizesObjectToolArguments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"id":"chat-1","choices":[{"index":0,"finish_reason":"tool_calls","message":{"role":"assistant","tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"lookup","arguments":{"city":"Paris"}}}]}}]}`)
	}))
	defer server.Close()

	provider := newTestProvider(t, schemas.ZAI, server.URL)
	response, bifrostErr := provider.ChatCompletion(
		schemas.NewBifrostContext(context.Background(), schemas.NoDeadline),
		schemas.Key{Value: *schemas.NewSecretVar("test-key")},
		testChatRequest(schemas.ZAI),
	)
	require.Nil(t, bifrostErr)
	require.NotNil(t, response)
	require.Len(t, response.Choices, 1)
	require.NotNil(t, response.Choices[0].Message.ChatAssistantMessage)
	require.Len(t, response.Choices[0].Message.ChatAssistantMessage.ToolCalls, 1)
	assert.Equal(t, `{"city":"Paris"}`, response.Choices[0].Message.ChatAssistantMessage.ToolCalls[0].Function.Arguments)
}

func TestChatCompletionStreamUsesNativeRequestAndPreservesReasoning(t *testing.T) {
	bodySeen := make(chan map[string]any, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var captured map[string]any
		require.NoError(t, json.Unmarshal(body, &captured))
		bodySeen <- captured

		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		require.True(t, ok)
		_, _ = io.WriteString(w, "data: {\"id\":\"chat-1\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"glm-5\",\"choices\":[{\"index\":0,\"finish_reason\":null,\"delta\":{\"role\":\"assistant\",\"reasoning_content\":\"reason\"}}]}\n\n")
		flusher.Flush()
		_, _ = io.WriteString(w, "data: {\"id\":\"chat-1\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"glm-5\",\"choices\":[{\"index\":0,\"finish_reason\":\"stop\",\"delta\":{\"content\":\"hello\"}}],\"usage\":{\"prompt_tokens\":2,\"completion_tokens\":3,\"total_tokens\":5}}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	provider := newTestProvider(t, schemas.ZAI, server.URL)
	ctx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
	request := testChatRequest(schemas.ZAI)
	request.Input[0].Content = &schemas.ChatMessageContent{
		ContentBlocks: []schemas.ChatContentBlock{{
			Type: schemas.ChatContentBlockTypeFile,
			File: &schemas.ChatInputFile{
				FileURL:  schemas.Ptr("https://example.com/document.pdf"),
				Filename: schemas.Ptr("document.pdf"),
			},
		}},
	}
	stream, bifrostErr := provider.ChatCompletionStream(ctx, noopPostHookRunner, nil, schemas.Key{Value: *schemas.NewSecretVar("test-key")}, request)
	require.Nil(t, bifrostErr)

	var reasoning string
	var content string
	var finalUsage *schemas.BifrostLLMUsage
	for chunk := range stream {
		require.Nil(t, chunk.BifrostError)
		if chunk.BifrostChatResponse == nil {
			continue
		}
		if chunk.BifrostChatResponse.Usage != nil {
			finalUsage = chunk.BifrostChatResponse.Usage
		}
		if len(chunk.BifrostChatResponse.Choices) == 0 || chunk.BifrostChatResponse.Choices[0].ChatStreamResponseChoice == nil {
			continue
		}
		delta := chunk.BifrostChatResponse.Choices[0].Delta
		if delta != nil && delta.Reasoning != nil {
			reasoning += *delta.Reasoning
		}
		if delta != nil && delta.Content != nil {
			content += *delta.Content
		}
	}
	assert.Equal(t, "reason", reasoning)
	assert.Equal(t, "hello", content)
	require.NotNil(t, finalUsage)
	assert.Equal(t, 5, finalUsage.TotalTokens)

	captured := <-bodySeen
	assert.Equal(t, true, captured["stream"])
	assert.Equal(t, true, captured["tool_stream"])
	assert.Equal(t, float64(64), captured["max_tokens"])
	assert.NotContains(t, captured, "stream_options")
	assertFileURLPreserved(t, captured)
}
func TestChatCompletionStreamAccumulatesToolCallFragments(t *testing.T) {
	toolChunk1 := `{"id":"chat-1","object":"chat.completion.chunk","created":1,"model":"glm-5","choices":[{"index":0,"finish_reason":null,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"q\":"}}]}}]}`
	toolChunk2 := `{"id":"chat-1","object":"chat.completion.chunk","created":1,"model":"glm-5","choices":[{"index":0,"finish_reason":null,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"glm\"}"}}]}}]}`
	toolChunk3 := `{"id":"chat-1","object":"chat.completion.chunk","created":1,"model":"glm-5","choices":[{"index":0,"finish_reason":"tool_calls","delta":{}}],"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}}`
	for _, payload := range []string{toolChunk1, toolChunk2, toolChunk3} {
		var decoded map[string]any
		require.NoError(t, json.Unmarshal([]byte(payload), &decoded))
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for _, payload := range []string{toolChunk1, toolChunk2, toolChunk3} {
			_, _ = io.WriteString(w, "data: "+payload+"\n\n")
		}
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	provider := newTestProvider(t, schemas.ZAI, server.URL)
	stream, bifrostErr := provider.ChatCompletionStream(
		schemas.NewBifrostContext(context.Background(), schemas.NoDeadline),
		noopPostHookRunner,
		nil,
		schemas.Key{Value: *schemas.NewSecretVar("test-key")},
		testChatRequest(schemas.ZAI),
	)
	require.Nil(t, bifrostErr)

	var fragments []schemas.ChatAssistantMessageToolCall
	for chunk := range stream {
		require.Nil(t, chunk.BifrostError)
		if chunk.BifrostChatResponse == nil || len(chunk.BifrostChatResponse.Choices) == 0 {
			continue
		}
		choice := chunk.BifrostChatResponse.Choices[0].ChatStreamResponseChoice
		if choice == nil || choice.Delta == nil {
			continue
		}
		fragments = append(fragments, choice.Delta.ToolCalls...)
	}
	require.Len(t, fragments, 2)
	require.NotNil(t, fragments[0].ID)
	assert.Equal(t, "call_1", *fragments[0].ID)
	require.NotNil(t, fragments[0].Function.Name)
	assert.Equal(t, "lookup", *fragments[0].Function.Name)
	assert.Equal(t, `{"q":`, fragments[0].Function.Arguments)
	assert.Equal(t, `"glm"}`, fragments[1].Function.Arguments)
}

func TestUnsupportedToolChoiceFailsBeforeDispatch(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls.Add(1)
	}))
	defer server.Close()

	provider := newTestProvider(t, schemas.ZAI, server.URL)
	request := testChatRequest(schemas.ZAI)
	required := string(schemas.ChatToolChoiceTypeRequired)
	request.Params.ToolChoice = &schemas.ChatToolChoice{ChatToolChoiceStr: &required}

	ctx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
	response, bifrostErr := provider.ChatCompletion(ctx, schemas.Key{Value: *schemas.NewSecretVar("test-key")}, request)
	assert.Nil(t, response)
	require.NotNil(t, bifrostErr)
	require.NotNil(t, bifrostErr.StatusCode)
	assert.Equal(t, http.StatusBadRequest, *bifrostErr.StatusCode)
	assert.Zero(t, calls.Load())
}
func TestChatRequestValidation(t *testing.T) {
	t.Run("rejects invalid user and request id lengths", func(t *testing.T) {
		provider := newTestProvider(t, schemas.ZAI, "http://127.0.0.1:1")
		for _, test := range []struct {
			name   string
			mutate func(*schemas.BifrostChatRequest)
		}{
			{name: "short user", mutate: func(request *schemas.BifrostChatRequest) {
				value := "short"
				request.Params.User = &value
			}},
			{name: "long user", mutate: func(request *schemas.BifrostChatRequest) {
				value := string(make([]byte, 129))
				request.Params.User = &value
			}},
			{name: "short request id", mutate: func(request *schemas.BifrostChatRequest) {
				request.Params.ExtraParams["request_id"] = "short"
			}},
			{name: "non-string request id", mutate: func(request *schemas.BifrostChatRequest) {
				request.Params.ExtraParams["request_id"] = 123
			}},
		} {
			t.Run(test.name, func(t *testing.T) {
				request := testChatRequest(schemas.ZAI)
				test.mutate(request)
				response, bifrostErr := provider.ChatCompletion(
					schemas.NewBifrostContext(context.Background(), schemas.NoDeadline),
					schemas.Key{Value: *schemas.NewSecretVar("test-key")},
					request,
				)
				assert.Nil(t, response)
				require.NotNil(t, bifrostErr)
				require.NotNil(t, bifrostErr.StatusCode)
				assert.Equal(t, http.StatusBadRequest, *bifrostErr.StatusCode)
			})
		}
	})

	t.Run("rejects unsupported reasoning max tokens", func(t *testing.T) {
		provider := newTestProvider(t, schemas.ZAI, "http://127.0.0.1:1")
		request := testChatRequest(schemas.ZAI)
		request.Params.Reasoning.MaxTokens = new(64)
		_, bifrostErr := provider.ChatCompletion(
			schemas.NewBifrostContext(context.Background(), schemas.NoDeadline),
			schemas.Key{Value: *schemas.NewSecretVar("test-key")},
			request,
		)
		require.NotNil(t, bifrostErr)
		require.NotNil(t, bifrostErr.StatusCode)
		assert.Equal(t, http.StatusBadRequest, *bifrostErr.StatusCode)
	})

	t.Run("disables thinking when reasoning is explicitly disabled", func(t *testing.T) {
		var captured map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(body, &captured))
			_, _ = w.Write([]byte(`{"id":"chat-1","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`))
		}))
		defer server.Close()
		provider := newTestProvider(t, schemas.ZAI, server.URL)
		request := testChatRequest(schemas.ZAI)
		enabled := false
		request.Params.Reasoning.Enabled = &enabled
		response, bifrostErr := provider.ChatCompletion(
			schemas.NewBifrostContext(context.Background(), schemas.NoDeadline),
			schemas.Key{Value: *schemas.NewSecretVar("test-key")},
			request,
		)
		require.Nil(t, bifrostErr)
		require.NotNil(t, response)
		assert.Equal(t, map[string]any{"type": "disabled"}, captured["thinking"])
		assert.NotContains(t, captured, "reasoning_effort")
	})
}

func TestChatSendsUserIDLongerThanOpenAILimit(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &captured))
		_, _ = w.Write([]byte(`{"id":"chat-1","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`))
	}))
	defer server.Close()

	// 100 runes: rejected by OpenAI's 64 character cap, valid for Z.AI's 6-128 range.
	user := strings.Repeat("u", 100)
	provider := newTestProvider(t, schemas.ZAI, server.URL)
	request := testChatRequest(schemas.ZAI)
	request.Params.User = &user
	response, bifrostErr := provider.ChatCompletion(
		schemas.NewBifrostContext(context.Background(), schemas.NoDeadline),
		schemas.Key{Value: *schemas.NewSecretVar("test-key")},
		request,
	)
	require.Nil(t, bifrostErr)
	require.NotNil(t, response)
	assert.Equal(t, user, captured["user_id"])
	assert.NotContains(t, captured, "user")
}

func TestChatToolArgumentsPreserveLargeIntegers(t *testing.T) {
	const arguments = `{"amount":9007199254740993,"nested":{"id":18446744073709551615}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"id":"chat-1","choices":[{"index":0,"finish_reason":"tool_calls","message":{"role":"assistant","tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"pay","arguments":`+arguments+`}}]}}]}`)
	}))
	defer server.Close()

	provider := newTestProvider(t, schemas.ZAI, server.URL)
	response, bifrostErr := provider.ChatCompletion(
		schemas.NewBifrostContext(context.Background(), schemas.NoDeadline),
		schemas.Key{Value: *schemas.NewSecretVar("test-key")},
		testChatRequest(schemas.ZAI),
	)
	require.Nil(t, bifrostErr)
	require.NotNil(t, response)
	require.Len(t, response.Choices, 1)
	require.NotNil(t, response.Choices[0].Message.ChatAssistantMessage)
	require.Len(t, response.Choices[0].Message.ChatAssistantMessage.ToolCalls, 1)
	assert.Equal(t, arguments, response.Choices[0].Message.ChatAssistantMessage.ToolCalls[0].Function.Arguments)
}

func TestRequiredThinkingModelsRejectDisabledReasoning(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls.Add(1)
	}))
	defer server.Close()

	for _, test := range []struct {
		name     string
		provider schemas.ModelProvider
		model    string
		mutate   func(*schemas.ChatReasoning)
	}{
		{name: "glm-5.3 enabled false", provider: schemas.ZAI, model: "glm-5.3", mutate: func(reasoning *schemas.ChatReasoning) {
			reasoning.Enabled = schemas.Ptr(false)
		}},
		{name: "glm-5.3-flash enabled false", provider: schemas.ZAI, model: "glm-5.3-flash", mutate: func(reasoning *schemas.ChatReasoning) {
			reasoning.Enabled = schemas.Ptr(false)
		}},
		{name: "provider prefixed model", provider: schemas.Zhipu, model: "zhipu/glm-5.3", mutate: func(reasoning *schemas.ChatReasoning) {
			reasoning.Enabled = schemas.Ptr(false)
		}},
		{name: "effort none", provider: schemas.ZAI, model: "glm-5.3", mutate: func(reasoning *schemas.ChatReasoning) {
			reasoning.Enabled = nil
			reasoning.Effort = schemas.Ptr("none")
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			provider := newTestProvider(t, test.provider, server.URL)
			request := testChatRequest(test.provider)
			request.Model = test.model
			test.mutate(request.Params.Reasoning)
			response, bifrostErr := provider.ChatCompletion(
				schemas.NewBifrostContext(context.Background(), schemas.NoDeadline),
				schemas.Key{Value: *schemas.NewSecretVar("test-key")},
				request,
			)
			assert.Nil(t, response)
			require.NotNil(t, bifrostErr)
			require.NotNil(t, bifrostErr.StatusCode)
			assert.Equal(t, http.StatusBadRequest, *bifrostErr.StatusCode)
			assert.Contains(t, bifrostErr.Error.Message, "requires thinking")
		})
	}
	assert.Equal(t, int32(0), calls.Load())
}
func TestChatPreservesNativeReasoningEffort(t *testing.T) {
	var captured map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &captured))
		_, _ = w.Write([]byte(`{"id":"chat-1","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`))
	}))
	defer server.Close()

	provider := newTestProvider(t, schemas.ZAI, server.URL)
	request := testChatRequest(schemas.ZAI)
	request.Model = "glm-5.3"
	request.Params.Reasoning.Effort = new("max")
	response, bifrostErr := provider.ChatCompletion(
		schemas.NewBifrostContext(context.Background(), schemas.NoDeadline),
		schemas.Key{Value: *schemas.NewSecretVar("test-key")},
		request,
	)
	require.Nil(t, bifrostErr)
	require.NotNil(t, response)
	assert.Equal(t, "max", captured["reasoning_effort"])
}

func TestRerankSerializesStructuredDocuments(t *testing.T) {
	var captured map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &captured))
		_, _ = w.Write([]byte(`{"id":"rank-1","results":[{"index":0,"relevance_score":0.9}]}`))
	}))
	defer server.Close()

	provider := newTestProvider(t, schemas.Zhipu, server.URL)
	response, bifrostErr := provider.Rerank(
		schemas.NewBifrostContext(context.Background(), schemas.NoDeadline),
		schemas.Key{Value: *schemas.NewSecretVar("test-key")},
		&schemas.BifrostRerankRequest{
			Model: "rerank",
			Query: "q",
			Documents: []schemas.RerankDocument{{
				Data: map[string]interface{}{"z": 1, "a": "first"},
			}},
		},
	)
	require.Nil(t, bifrostErr)
	require.NotNil(t, response)
	require.Len(t, captured["documents"], 1)
	assert.Equal(t, `{"a":"first","z":1}`, captured["documents"].([]interface{})[0])
}

func TestRerankRejectsAmbiguousDocuments(t *testing.T) {
	provider := newTestProvider(t, schemas.Zhipu, "http://127.0.0.1:1")
	response, bifrostErr := provider.Rerank(
		schemas.NewBifrostContext(context.Background(), schemas.NoDeadline),
		schemas.Key{Value: *schemas.NewSecretVar("test-key")},
		&schemas.BifrostRerankRequest{
			Model:     "rerank",
			Query:     "q",
			Documents: []schemas.RerankDocument{{Text: "text", Data: map[string]interface{}{"title": "data"}}},
		},
	)
	assert.Nil(t, response)
	require.NotNil(t, bifrostErr)
	require.NotNil(t, bifrostErr.StatusCode)
	assert.Equal(t, http.StatusBadRequest, *bifrostErr.StatusCode)
}

func TestChinaEmbeddingAndRerankEndpoints(t *testing.T) {
	t.Run("embedding", func(t *testing.T) {
		var path string
		var captured map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path = r.URL.Path
			assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(body, &captured))
			_, _ = w.Write([]byte(`{"object":"list","data":[{"object":"embedding","index":0,"embedding":[0.1,0.2]}],"model":"embedding-3","usage":{"prompt_tokens":2,"total_tokens":2}}`))
		}))
		defer server.Close()

		provider := newTestProvider(t, schemas.Zhipu, server.URL)
		text := "hello"
		response, bifrostErr := provider.Embedding(
			schemas.NewBifrostContext(context.Background(), schemas.NoDeadline),
			schemas.Key{Value: *schemas.NewSecretVar("test-key")},
			&schemas.BifrostEmbeddingRequest{Provider: schemas.Zhipu, Model: "embedding-3", Input: &schemas.EmbeddingInput{Text: &text}},
		)
		require.Nil(t, bifrostErr)
		require.NotNil(t, response)
		assert.Equal(t, "/embeddings", path)
		assert.Equal(t, "embedding-3", captured["model"])
		assert.Equal(t, "hello", captured["input"])
		require.Len(t, response.Data, 1)
		assert.Equal(t, 2, response.Usage.TotalTokens)
	})

	t.Run("rerank", func(t *testing.T) {
		var path string
		var captured map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path = r.URL.Path
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(body, &captured))
			_, _ = w.Write([]byte(`{"id":"rank-1","results":[{"index":1,"relevance_score":0.9},{"index":0,"relevance_score":0.2}],"usage":{"prompt_tokens":11,"total_tokens":11}}`))
		}))
		defer server.Close()

		provider := newTestProvider(t, schemas.Zhipu, server.URL)
		provider.sendBackRawRequest = true
		provider.sendBackRawResponse = true
		response, bifrostErr := provider.Rerank(
			schemas.NewBifrostContext(context.Background(), schemas.NoDeadline),
			schemas.Key{Value: *schemas.NewSecretVar("test-key")},
			&schemas.BifrostRerankRequest{
				Provider: schemas.Zhipu,
				Model:    "rerank",
				Query:    "q",
				Documents: []schemas.RerankDocument{
					{Text: "a"},
					{Text: "b"},
				},
				Params: &schemas.RerankParameters{
					TopN:            new(2),
					ReturnDocuments: new(true),
					ExtraParams:     map[string]interface{}{"return_raw_scores": true},
				},
			},
		)
		require.Nil(t, bifrostErr)
		require.NotNil(t, response)
		assert.Equal(t, "/rerank", path)
		assert.Equal(t, []any{"a", "b"}, captured["documents"])
		assert.Equal(t, "rerank", captured["model"])
		assert.Equal(t, "q", captured["query"])
		assert.Equal(t, float64(2), captured["top_n"])
		assert.Equal(t, true, captured["return_documents"])
		assert.Equal(t, true, captured["return_raw_scores"])
		assert.Len(t, captured, 6)
		require.NotNil(t, response.Usage)
		assert.Equal(t, 11, response.Usage.TotalTokens)
		require.NotNil(t, response.ExtraFields.RawRequest)
		require.NotNil(t, response.ExtraFields.RawResponse)
		require.Len(t, response.Results, 2)
		assert.Equal(t, 1, response.Results[0].Index)
		assert.Equal(t, 0.9, response.Results[0].RelevanceScore)
	})
}
func TestCountTokensCapturesRawRequestAndResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"model":"glm-5","usage":{"prompt_tokens":7,"total_tokens":7}}`))
	}))
	defer server.Close()

	provider := newTestProvider(t, schemas.ZAI, server.URL)
	provider.sendBackRawRequest = true
	provider.sendBackRawResponse = true
	response, bifrostErr := provider.CountTokens(
		schemas.NewBifrostContext(context.Background(), schemas.NoDeadline),
		schemas.Key{Value: *schemas.NewSecretVar("test-key")},
		testResponsesRequest(),
	)
	require.Nil(t, bifrostErr)
	require.NotNil(t, response)
	assert.Equal(t, 7, response.InputTokens)
	require.NotNil(t, response.ExtraFields.RawRequest)
	require.NotNil(t, response.ExtraFields.RawResponse)
}

func TestNativeConversionsRejectLargePayloadPassthrough(t *testing.T) {
	for _, test := range []struct {
		name string
		call func(*schemas.BifrostContext) *schemas.BifrostError
	}{
		{name: "chat", call: func(ctx *schemas.BifrostContext) *schemas.BifrostError {
			provider := newTestProvider(t, schemas.ZAI, "http://127.0.0.1:1")
			_, err := provider.ChatCompletion(ctx, schemas.Key{}, testChatRequest(schemas.ZAI))
			return err
		}},
		{name: "chat stream", call: func(ctx *schemas.BifrostContext) *schemas.BifrostError {
			provider := newTestProvider(t, schemas.ZAI, "http://127.0.0.1:1")
			_, err := provider.ChatCompletionStream(ctx, noopPostHookRunner, nil, schemas.Key{}, testChatRequest(schemas.ZAI))
			return err
		}},
		{name: "rerank", call: func(ctx *schemas.BifrostContext) *schemas.BifrostError {
			provider := newTestProvider(t, schemas.Zhipu, "http://127.0.0.1:1")
			_, err := provider.Rerank(ctx, schemas.Key{}, &schemas.BifrostRerankRequest{Model: "rerank"})
			return err
		}},
		{name: "count tokens", call: func(ctx *schemas.BifrostContext) *schemas.BifrostError {
			provider := newTestProvider(t, schemas.ZAI, "http://127.0.0.1:1")
			_, err := provider.CountTokens(ctx, schemas.Key{}, testResponsesRequest())
			return err
		}},
		{name: "image", call: func(ctx *schemas.BifrostContext) *schemas.BifrostError {
			provider := newTestProvider(t, schemas.ZAI, "http://127.0.0.1:1")
			_, err := provider.ImageGeneration(ctx, schemas.Key{}, &schemas.BifrostImageGenerationRequest{Model: "glm-image", Input: &schemas.ImageGenerationInput{Prompt: "mountain"}})
			return err
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
			ctx.SetValue(schemas.BifrostContextKeyLargePayloadMode, true)
			ctx.SetValue(schemas.BifrostContextKeyLargePayloadReader, strings.NewReader(`{"model":"zai/glm-5"}`))
			bifrostErr := test.call(ctx)
			require.NotNil(t, bifrostErr)
			require.NotNil(t, bifrostErr.StatusCode)
			assert.Equal(t, http.StatusBadRequest, *bifrostErr.StatusCode)
		})
	}
}

func TestRerankNilRequestReturnsBadRequest(t *testing.T) {
	provider := newTestProvider(t, schemas.Zhipu, "http://127.0.0.1:1")
	_, bifrostErr := provider.Rerank(
		schemas.NewBifrostContext(context.Background(), schemas.NoDeadline),
		schemas.Key{Value: *schemas.NewSecretVar("test-key")},
		nil,
	)
	require.NotNil(t, bifrostErr)
	require.NotNil(t, bifrostErr.StatusCode)
	assert.Equal(t, http.StatusBadRequest, *bifrostErr.StatusCode)
}

func TestImageGenerationUsesRegionalCapabilityProfiles(t *testing.T) {
	tests := []struct {
		provider  schemas.ModelProvider
		model     string
		watermark bool
	}{
		{provider: schemas.ZAI, model: "cogview-4-250304"},
		{provider: schemas.Zhipu, model: "cogview-4", watermark: true},
	}
	for _, test := range tests {
		t.Run(string(test.provider), func(t *testing.T) {
			var captured map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/images/generations", r.URL.Path)
				body, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				require.NoError(t, json.Unmarshal(body, &captured))
				_, _ = w.Write([]byte(`{"created":1,"data":[{"url":"https://example.com/image.png"}]}`))
			}))
			defer server.Close()

			params := &schemas.ImageGenerationParameters{
				Quality: new("high"),
				Size:    new("1024x1024"),
				User:    new("user-123456"),
			}
			if test.watermark {
				params.ExtraParams = map[string]interface{}{"watermark_enabled": true}
			}
			provider := newTestProvider(t, test.provider, server.URL)
			response, bifrostErr := provider.ImageGeneration(
				schemas.NewBifrostContext(context.Background(), schemas.NoDeadline),
				schemas.Key{Value: *schemas.NewSecretVar("test-key")},
				&schemas.BifrostImageGenerationRequest{
					Provider: test.provider,
					Model:    test.model,
					Input:    &schemas.ImageGenerationInput{Prompt: "a mountain"},
					Params:   params,
				},
			)
			require.Nil(t, bifrostErr)
			require.NotNil(t, response)
			require.Len(t, response.Data, 1)
			assert.Equal(t, "a mountain", captured["prompt"])
			assert.Equal(t, test.model, captured["model"])
			assert.Equal(t, "high", captured["quality"])
			assert.Equal(t, "1024x1024", captured["size"])
			assert.Equal(t, "user-123456", captured["user_id"])
			if test.watermark {
				assert.Equal(t, true, captured["watermark_enabled"])
			} else {
				assert.NotContains(t, captured, "watermark_enabled")
			}
			assert.Equal(t, 5+boolToInt(test.watermark), len(captured))
		})
	}
}

func TestImageGenerationRejectsCrossRegionCapabilities(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls.Add(1)
	}))
	defer server.Close()

	tests := []struct {
		name     string
		provider schemas.ModelProvider
		model    string
		params   *schemas.ImageGenerationParameters
	}{
		{
			name:     "international rejects China model",
			provider: schemas.ZAI,
			model:    "cogview-4",
		},
		{
			name:     "China rejects international model",
			provider: schemas.Zhipu,
			model:    "cogview-4-250304",
		},
		{
			name:     "international rejects watermark",
			provider: schemas.ZAI,
			model:    "glm-image",
			params: &schemas.ImageGenerationParameters{
				ExtraParams: map[string]interface{}{"watermark_enabled": true},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			provider := newTestProvider(t, test.provider, server.URL)
			_, bifrostErr := provider.ImageGeneration(
				schemas.NewBifrostContext(context.Background(), schemas.NoDeadline),
				schemas.Key{Value: *schemas.NewSecretVar("test-key")},
				&schemas.BifrostImageGenerationRequest{
					Provider: test.provider,
					Model:    test.model,
					Input:    &schemas.ImageGenerationInput{Prompt: "a mountain"},
					Params:   test.params,
				},
			)
			require.NotNil(t, bifrostErr)
			require.NotNil(t, bifrostErr.StatusCode)
			assert.Equal(t, http.StatusBadRequest, *bifrostErr.StatusCode)
		})
	}
	assert.Zero(t, calls.Load())
}

func TestImageModelGateNormalizesProviderPrefix(t *testing.T) {
	assert.True(t, internationalProfile.supportsImageModel("zai/glm-image"))
	assert.True(t, chinaProfile.supportsImageModel("zhipu/cogview-3-flash"))
	// Date-suffixed ids stay region specific rather than collapsing onto a base name.
	assert.False(t, internationalProfile.supportsImageModel("cogview-4"))
	assert.False(t, chinaProfile.supportsImageModel("cogview-4-250304"))
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
