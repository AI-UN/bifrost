package deepinfra_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/core/internal/llmtests"
	deepinfraprovider "github.com/maximhq/bifrost/core/providers/deepinfra"
	"github.com/maximhq/bifrost/core/schemas"
)

// TestDeepinfra is the live provider harness entrypoint discovered by
// `make test-core PROVIDER=deepinfra`.
func TestDeepinfra(t *testing.T) {
	t.Parallel()
	if strings.TrimSpace(os.Getenv("DEEPINFRA_API_KEY")) == "" {
		t.Skip("Skipping DeepInfra tests because DEEPINFRA_API_KEY is not set")
	}

	client, ctx, cancel, err := llmtests.SetupTest()
	if err != nil {
		t.Fatalf("Error initializing test setup: %v", err)
	}
	defer cancel()
	defer client.Shutdown()

	testConfig := llmtests.ComprehensiveTestConfig{
		Provider:             schemas.DeepInfra,
		ChatModel:            "deepseek-ai/DeepSeek-V3.1",
		TextModel:            "deepseek-ai/DeepSeek-V3.1",
		EmbeddingModel:       "Qwen/Qwen3-Embedding-8B",
		RerankModel:          "Qwen/Qwen3-Reranker-8B",
		TranscriptionModel:   "openai/whisper-large-v3",
		SpeechSynthesisModel: "hexgrad/Kokoro-82M",
		ImageGenerationModel: "stabilityai/sdxl-turbo",
		Scenarios: llmtests.TestScenarios{
			TextCompletion:        true,
			TextCompletionStream:  true,
			SimpleChat:            true,
			CompletionStream:      true,
			MultiTurnConversation: true,
			ToolCalls:             true,
			ToolCallsStreaming:    true,
			CompleteEnd2End:       true,
			Embedding:             true,
			ListModels:            true,
			Rerank:                true,
			Transcription:         true,
			SpeechSynthesis:       true,
			SpeechSynthesisStream: true,
			ImageGeneration:       true,
		},
	}
	t.Run("DeepInfraTests", func(t *testing.T) {
		llmtests.RunAllComprehensiveTests(t, client, ctx, testConfig)
	})
}

// TestDeepInfraProviderEndpointRouting pins every operation to the DeepInfra endpoint it is
// supposed to reach. The split between the OpenAI-compatible surface and the native surface
// is the whole design of this provider, so a silently rewritten path is the regression this
// guards: hitting /v1/audio/speech for a streaming request, for instance, would return a
// finished clip instead of incremental audio.
func TestDeepInfraProviderEndpointRouting(t *testing.T) {
	tests := []struct {
		name         string
		expectedPath string
		run          func(t *testing.T, provider *deepinfraprovider.DeepInfraProvider, ctx *schemas.BifrostContext, key schemas.Key)
	}{
		{
			name:         "ChatCompletion",
			expectedPath: "/v1/chat/completions",
			run: func(t *testing.T, provider *deepinfraprovider.DeepInfraProvider, ctx *schemas.BifrostContext, key schemas.Key) {
				resp, err := provider.ChatCompletion(ctx, key, &schemas.BifrostChatRequest{
					Provider: schemas.DeepInfra,
					Model:    "deepseek-ai/DeepSeek-V3.1",
					Input:    []schemas.ChatMessage{llmtests.CreateBasicChatMessage("hello")},
				})
				if err != nil {
					t.Fatalf("ChatCompletion returned error: %v", llmtests.GetErrorMessage(err))
				}
				if resp == nil || len(resp.Choices) == 0 {
					t.Fatalf("unexpected chat response: %#v", resp)
				}
			},
		},
		{
			name:         "TextCompletion",
			expectedPath: "/v1/completions",
			run: func(t *testing.T, provider *deepinfraprovider.DeepInfraProvider, ctx *schemas.BifrostContext, key schemas.Key) {
				prompt := "A is for apple and B is for"
				resp, err := provider.TextCompletion(ctx, key, &schemas.BifrostTextCompletionRequest{
					Provider: schemas.DeepInfra,
					Model:    "deepseek-ai/DeepSeek-V3.1",
					Input:    &schemas.TextCompletionInput{PromptStr: &prompt},
				})
				if err != nil {
					t.Fatalf("TextCompletion returned error: %v", llmtests.GetErrorMessage(err))
				}
				if resp == nil || len(resp.Choices) == 0 {
					t.Fatalf("unexpected text completion response: %#v", resp)
				}
			},
		},
		{
			// DeepInfra publishes no /v1/responses, so the Responses API must land on the
			// chat endpoint rather than 404 against a path that does not exist.
			name:         "ResponsesFallsBackToChat",
			expectedPath: "/v1/chat/completions",
			run: func(t *testing.T, provider *deepinfraprovider.DeepInfraProvider, ctx *schemas.BifrostContext, key schemas.Key) {
				resp, err := provider.Responses(ctx, key, &schemas.BifrostResponsesRequest{
					Provider: schemas.DeepInfra,
					Model:    "deepseek-ai/DeepSeek-V3.1",
					Input:    []schemas.ResponsesMessage{llmtests.CreateBasicResponsesMessage("hello")},
				})
				if err != nil {
					t.Fatalf("Responses returned error: %v", llmtests.GetErrorMessage(err))
				}
				if resp == nil {
					t.Fatal("expected a responses payload")
				}
			},
		},
		{
			name:         "Embedding",
			expectedPath: "/v1/embeddings",
			run: func(t *testing.T, provider *deepinfraprovider.DeepInfraProvider, ctx *schemas.BifrostContext, key schemas.Key) {
				if _, err := provider.Embedding(ctx, key, &schemas.BifrostEmbeddingRequest{
					Provider: schemas.DeepInfra,
					Model:    "Qwen/Qwen3-Embedding-8B",
					Input:    &schemas.EmbeddingInput{Texts: []string{"hello"}},
				}); err != nil {
					t.Fatalf("Embedding returned error: %v", llmtests.GetErrorMessage(err))
				}
			},
		},
		{
			// Unary speech uses the OpenAI-compatible endpoint, not the native one.
			name:         "Speech",
			expectedPath: "/v1/audio/speech",
			run: func(t *testing.T, provider *deepinfraprovider.DeepInfraProvider, ctx *schemas.BifrostContext, key schemas.Key) {
				voice := "af_bella"
				resp, err := provider.Speech(ctx, key, &schemas.BifrostSpeechRequest{
					Provider: schemas.DeepInfra,
					Model:    "hexgrad/Kokoro-82M",
					Input:    &schemas.SpeechInput{Input: "hello"},
					Params:   &schemas.SpeechParameters{VoiceConfig: &schemas.SpeechVoiceInput{Voice: &voice}},
				})
				if err != nil {
					t.Fatalf("Speech returned error: %v", llmtests.GetErrorMessage(err))
				}
				if resp == nil || len(resp.Audio) == 0 {
					t.Fatalf("expected audio bytes, got %#v", resp)
				}
			},
		},
		{
			name:         "Transcription",
			expectedPath: "/v1/audio/transcriptions",
			run: func(t *testing.T, provider *deepinfraprovider.DeepInfraProvider, ctx *schemas.BifrostContext, key schemas.Key) {
				resp, err := provider.Transcription(ctx, key, &schemas.BifrostTranscriptionRequest{
					Provider: schemas.DeepInfra,
					Model:    "openai/whisper-large-v3",
					Input:    &schemas.TranscriptionInput{File: []byte("RIFFfake")},
				})
				if err != nil {
					t.Fatalf("Transcription returned error: %v", llmtests.GetErrorMessage(err))
				}
				if resp == nil || resp.Text != "hello world" {
					t.Fatalf("unexpected transcription response: %#v", resp)
				}
			},
		},
		{
			name:         "ImageGeneration",
			expectedPath: "/v1/images/generations",
			run: func(t *testing.T, provider *deepinfraprovider.DeepInfraProvider, ctx *schemas.BifrostContext, key schemas.Key) {
				if _, err := provider.ImageGeneration(ctx, key, &schemas.BifrostImageGenerationRequest{
					Provider: schemas.DeepInfra,
					Model:    "stabilityai/sdxl-turbo",
					Input:    &schemas.ImageGenerationInput{Prompt: "a burger on the beach"},
				}); err != nil {
					t.Fatalf("ImageGeneration returned error: %v", llmtests.GetErrorMessage(err))
				}
			},
		},
		{
			// Rerank has no OpenAI-shaped endpoint on DeepInfra; it is served natively,
			// keyed by model name in the path.
			name:         "RerankUsesNativeInferenceEndpoint",
			expectedPath: "/v1/inference/Qwen/Qwen3-Reranker-8B",
			run: func(t *testing.T, provider *deepinfraprovider.DeepInfraProvider, ctx *schemas.BifrostContext, key schemas.Key) {
				if _, err := provider.Rerank(ctx, key, &schemas.BifrostRerankRequest{
					Provider:  schemas.DeepInfra,
					Model:     "Qwen/Qwen3-Reranker-8B",
					Query:     "capital of France",
					Documents: []schemas.RerankDocument{{Text: "Berlin"}, {Text: "Paris"}, {Text: "Rome"}},
				}); err != nil {
					t.Fatalf("Rerank returned error: %v", llmtests.GetErrorMessage(err))
				}
			},
		},
		{
			name:         "VideoGeneration",
			expectedPath: "/v1/videos",
			run: func(t *testing.T, provider *deepinfraprovider.DeepInfraProvider, ctx *schemas.BifrostContext, key schemas.Key) {
				resp, err := provider.VideoGeneration(ctx, key, &schemas.BifrostVideoGenerationRequest{
					Provider: schemas.DeepInfra,
					Model:    "video-model",
					Input:    &schemas.VideoGenerationInput{Prompt: "a cat surfing"},
				})
				if err != nil {
					t.Fatalf("VideoGeneration returned error: %v", llmtests.GetErrorMessage(err))
				}
				if resp == nil || resp.Status != schemas.VideoStatusQueued {
					t.Fatalf("expected a queued video job, got %#v", resp)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requestedPath string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requestedPath = r.URL.Path
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/v1/chat/completions":
					_, _ = fmt.Fprint(w, `{"id":"chat_1","object":"chat.completion","created":1,"model":"deepseek-ai/DeepSeek-V3.1","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
				case "/v1/completions":
					_, _ = fmt.Fprint(w, `{"id":"cmpl_1","object":"text_completion","created":1,"model":"deepseek-ai/DeepSeek-V3.1","choices":[{"text":" banana","index":0,"finish_reason":"stop"}],"usage":{"prompt_tokens":4,"completion_tokens":1,"total_tokens":5}}`)
				case "/v1/embeddings":
					_, _ = fmt.Fprint(w, `{"object":"list","model":"Qwen/Qwen3-Embedding-8B","data":[{"object":"embedding","index":0,"embedding":[0.1,0.2,0.3]}],"usage":{"prompt_tokens":2,"total_tokens":2}}`)
				case "/v1/audio/speech":
					w.Header().Set("Content-Type", "audio/wav")
					_, _ = w.Write([]byte("RIFFfakeaudio"))
				case "/v1/audio/transcriptions":
					_, _ = fmt.Fprint(w, `{"text":"hello world"}`)
				case "/v1/images/generations":
					_, _ = fmt.Fprint(w, `{"created":1,"data":[{"url":"https://example.invalid/i.png"}]}`)
				case "/v1/inference/Qwen/Qwen3-Reranker-8B":
					_, _ = fmt.Fprint(w, `{"scores":[0.1,0.9,0.2],"input_tokens":12}`)
				case "/v1/videos":
					_, _ = fmt.Fprint(w, `{"id":"vid_1","object":"video.generation.job","created_at":1,"status":"queued","model":"video-model"}`)
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()

			provider := newTestDeepInfraProvider(t, server.URL)
			ctx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
			key := schemas.Key{Value: *schemas.NewSecretVar("test-key")}

			tt.run(t, provider, ctx, key)

			if requestedPath != tt.expectedPath {
				t.Fatalf("expected request path %q, got %q", tt.expectedPath, requestedPath)
			}
		})
	}
}

// TestDeepInfraRerankRanksByScoreNotPosition covers the mapping that the native rerank
// endpoint does not do for us: it answers with a positional score vector, so the provider
// has to rank it, keep Index pointing at the caller's original document position, and honour
// TopN after ranking rather than before.
func TestDeepInfraRerankRanksByScoreNotPosition(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Deliberately out of rank order: the middle document is the most relevant.
		_, _ = fmt.Fprint(w, `{"scores":[0.10,0.90,0.50],"input_tokens":7}`)
	}))
	defer server.Close()

	provider := newTestDeepInfraProvider(t, server.URL)
	ctx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
	key := schemas.Key{Value: *schemas.NewSecretVar("test-key")}

	topN := 2
	returnDocuments := true
	resp, err := provider.Rerank(ctx, key, &schemas.BifrostRerankRequest{
		Provider: schemas.DeepInfra,
		Model:    "Qwen/Qwen3-Reranker-8B",
		Query:    "capital of France",
		Documents: []schemas.RerankDocument{
			{Text: "Berlin is the capital of Germany.", ID: schemas.Ptr("berlin")},
			{Text: "Paris is the capital of France.", ID: schemas.Ptr("paris")},
			{Text: "The Eiffel Tower is in Paris.", ID: schemas.Ptr("tower")},
		},
		Params: &schemas.RerankParameters{TopN: &topN, ReturnDocuments: &returnDocuments},
	})
	if err != nil {
		t.Fatalf("Rerank returned error: %v", llmtests.GetErrorMessage(err))
	}

	if len(resp.Results) != topN {
		t.Fatalf("expected TopN=%d results, got %d", topN, len(resp.Results))
	}
	// Highest score first, and Index addresses the original request position.
	if resp.Results[0].Index != 1 || resp.Results[0].RelevanceScore != 0.90 {
		t.Fatalf("expected document 1 ranked first, got index=%d score=%v", resp.Results[0].Index, resp.Results[0].RelevanceScore)
	}
	if resp.Results[1].Index != 2 || resp.Results[1].RelevanceScore != 0.50 {
		t.Fatalf("expected document 2 ranked second, got index=%d score=%v", resp.Results[1].Index, resp.Results[1].RelevanceScore)
	}
	// TopN must drop the least relevant document, not the last one submitted.
	for _, result := range resp.Results {
		if result.Index == 0 {
			t.Fatal("expected the lowest-scoring document to be truncated by TopN")
		}
	}
	if resp.Results[0].ID == nil || *resp.Results[0].ID != "paris" {
		t.Fatalf("expected the caller's document ID to survive ranking, got %v", resp.Results[0].ID)
	}
	if resp.Results[0].Document == nil || resp.Results[0].Document.Text != "Paris is the capital of France." {
		t.Fatalf("expected ReturnDocuments to attach the original document, got %#v", resp.Results[0].Document)
	}
	if resp.Usage == nil || resp.Usage.PromptTokens != 7 {
		t.Fatalf("expected input_tokens to map onto usage, got %#v", resp.Usage)
	}
}

// TestDeepInfraRerankRejectsMismatchedScores guards the one failure mode that would silently
// mis-rank: a score vector whose length does not line up with the documents we sent cannot be
// mapped back to input positions, so it must be an error rather than a best-effort guess.
func TestDeepInfraRerankRejectsMismatchedScores(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"scores":[0.5]}`)
	}))
	defer server.Close()

	provider := newTestDeepInfraProvider(t, server.URL)
	ctx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
	key := schemas.Key{Value: *schemas.NewSecretVar("test-key")}

	_, err := provider.Rerank(ctx, key, &schemas.BifrostRerankRequest{
		Provider:  schemas.DeepInfra,
		Model:     "Qwen/Qwen3-Reranker-8B",
		Query:     "capital of France",
		Documents: []schemas.RerankDocument{{Text: "Berlin"}, {Text: "Paris"}},
	})
	if err == nil {
		t.Fatal("expected an error when the score vector does not match the document count")
	}
	if !strings.Contains(llmtests.GetErrorMessage(err), "1 scores for 2 documents") {
		t.Fatalf("expected a score/document count mismatch error, got %v", llmtests.GetErrorMessage(err))
	}
}

// TestDeepInfraSpeechStreamUsesNativeEndpoint covers the reason this provider talks to the
// native surface at all: /v1/audio/speech cannot stream, so incremental synthesis has to go
// to /v1/text-to-speech/{voice_id}/stream, which answers with chunked raw audio rather than
// SSE. It asserts the voice reaches the path, the model reaches the body as model_id, and
// every audio byte is delivered before the terminal event.
func TestDeepInfraSpeechStreamUsesNativeEndpoint(t *testing.T) {
	var requestedPath, requestBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		buf := make([]byte, 512)
		n, _ := r.Body.Read(buf)
		requestBody = string(buf[:n])

		w.Header().Set("Content-Type", "audio/wav")
		flusher, _ := w.(http.Flusher)
		for _, part := range []string{"RIFF", "fake", "audio"} {
			_, _ = w.Write([]byte(part))
			if flusher != nil {
				flusher.Flush()
			}
		}
	}))
	defer server.Close()

	provider := newTestDeepInfraProvider(t, server.URL)
	ctx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
	key := schemas.Key{Value: *schemas.NewSecretVar("test-key")}
	postHookRunner := func(_ *schemas.BifrostContext, result *schemas.BifrostResponse, err *schemas.BifrostError) (*schemas.BifrostResponse, *schemas.BifrostError) {
		return result, err
	}

	voice := "af_nicole"
	stream, err := provider.SpeechStream(ctx, postHookRunner, nil, key, &schemas.BifrostSpeechRequest{
		Provider: schemas.DeepInfra,
		Model:    "hexgrad/Kokoro-82M",
		Input:    &schemas.SpeechInput{Input: "hello"},
		Params: &schemas.SpeechParameters{
			VoiceConfig:    &schemas.SpeechVoiceInput{Voice: &voice},
			ResponseFormat: "wav",
		},
	})
	if err != nil {
		t.Fatalf("SpeechStream returned error: %v", llmtests.GetErrorMessage(err))
	}

	var audio []byte
	sawDone := false
	for chunk := range stream {
		if chunk == nil || chunk.BifrostSpeechStreamResponse == nil {
			continue
		}
		switch chunk.BifrostSpeechStreamResponse.Type {
		case schemas.SpeechStreamResponseTypeDelta:
			if sawDone {
				t.Fatal("received an audio delta after the terminal event")
			}
			audio = append(audio, chunk.BifrostSpeechStreamResponse.Audio...)
		case schemas.SpeechStreamResponseTypeDone:
			sawDone = true
		}
	}

	if requestedPath != "/v1/text-to-speech/af_nicole/stream" {
		t.Fatalf("expected the native streaming TTS path with the voice in it, got %q", requestedPath)
	}
	if !strings.Contains(requestBody, `"model_id":"hexgrad/Kokoro-82M"`) {
		t.Fatalf("expected the model to travel as model_id, got body %q", requestBody)
	}
	if !strings.Contains(requestBody, `"output_format":"wav"`) {
		t.Fatalf("expected the response format to map onto output_format, got body %q", requestBody)
	}
	if string(audio) != "RIFFfakeaudio" {
		t.Fatalf("expected the full audio stream to be delivered, got %q", string(audio))
	}
	if !sawDone {
		t.Fatal("expected a terminal speech.audio.done event")
	}
}

// TestDeepInfraErrorEnvelopes covers the three different error shapes DeepInfra emits across
// its two surfaces. Without normalization the native string form and the FastAPI validation
// form both decode to an empty message, which surfaces to callers as a bare status code.
func TestDeepInfraErrorEnvelopes(t *testing.T) {
	tests := []struct {
		name            string
		status          int
		body            string
		expectedMessage string
	}{
		{
			name:            "NativeStringError",
			status:          http.StatusNotFound,
			body:            `{"error":"Model not found"}`,
			expectedMessage: "Model not found",
		},
		{
			name:            "OpenAIObjectError",
			status:          http.StatusUnauthorized,
			body:            `{"error":{"message":"Invalid API token","type":"invalid_request_error","code":"invalid_api_key"}}`,
			expectedMessage: "Invalid API token",
		},
		{
			name:            "ValidationDetailError",
			status:          http.StatusUnprocessableEntity,
			body:            `{"detail":[{"loc":["body","messages"],"msg":"field required","type":"value_error.missing"}]}`,
			expectedMessage: "messages: field required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				_, _ = fmt.Fprint(w, tt.body)
			}))
			defer server.Close()

			provider := newTestDeepInfraProvider(t, server.URL)
			ctx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
			key := schemas.Key{Value: *schemas.NewSecretVar("test-key")}

			_, err := provider.ChatCompletion(ctx, key, &schemas.BifrostChatRequest{
				Provider: schemas.DeepInfra,
				Model:    "deepseek-ai/DeepSeek-V3.1",
				Input:    []schemas.ChatMessage{llmtests.CreateBasicChatMessage("hello")},
			})
			if err == nil {
				t.Fatal("expected an error response")
			}
			if got := llmtests.GetErrorMessage(err); !strings.Contains(got, tt.expectedMessage) {
				t.Fatalf("expected error message to contain %q, got %q", tt.expectedMessage, got)
			}
			if err.StatusCode == nil || *err.StatusCode != tt.status {
				t.Fatalf("expected status %d to be preserved, got %v", tt.status, err.StatusCode)
			}
		})
	}
}

// TestDeepInfraUnsupportedOperations pins the operations DeepInfra genuinely does not offer.
// These must fail fast with an unsupported-operation error so fallbacks can engage, rather
// than issuing a request against a path that does not exist.
func TestDeepInfraUnsupportedOperations(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unsupported operation unexpectedly issued a request to %q", r.URL.Path)
		http.NotFound(w, r)
	}))
	defer server.Close()

	provider := newTestDeepInfraProvider(t, server.URL)
	ctx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
	key := schemas.Key{Value: *schemas.NewSecretVar("test-key")}

	if _, err := provider.TranscriptionStream(ctx, nil, nil, key, &schemas.BifrostTranscriptionRequest{
		Provider: schemas.DeepInfra,
		Model:    "openai/whisper-large-v3",
	}); err == nil {
		t.Fatal("expected TranscriptionStream to be unsupported")
	}

	if _, err := provider.ImageGenerationStream(ctx, nil, nil, key, &schemas.BifrostImageGenerationRequest{
		Provider: schemas.DeepInfra,
		Model:    "stabilityai/sdxl-turbo",
	}); err == nil {
		t.Fatal("expected ImageGenerationStream to be unsupported")
	}

	if _, err := provider.CountTokens(ctx, key, &schemas.BifrostResponsesRequest{
		Provider: schemas.DeepInfra,
		Model:    "deepseek-ai/DeepSeek-V3.1",
	}); err == nil {
		t.Fatal("expected CountTokens to be unsupported")
	}

	if _, err := provider.BatchCreate(ctx, key, &schemas.BifrostBatchCreateRequest{
		Provider: schemas.DeepInfra,
	}); err == nil {
		t.Fatal("expected BatchCreate to be unsupported")
	}
}

// newTestDeepInfraProvider creates a DeepInfra provider pointed at a local test server.
func newTestDeepInfraProvider(t *testing.T, baseURL string) *deepinfraprovider.DeepInfraProvider {
	t.Helper()

	provider, err := deepinfraprovider.NewDeepInfraProvider(&schemas.ProviderConfig{
		NetworkConfig: schemas.NetworkConfig{
			BaseURL:                        baseURL,
			DefaultRequestTimeoutInSeconds: 300,
		},
	}, bifrost.NewNoOpLogger())
	if err != nil {
		t.Fatalf("failed to create DeepInfra provider: %v", err)
	}
	return provider
}
