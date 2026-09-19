package gemini

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/valyala/fasthttp"
)

const geminiStreamFixture = `data: {"candidates":[{"content":{"parts":[{"text":"Hello"}],"role":"model"},"index":0}],"modelVersion":"gemini-2.5-pro","responseId":"resp_1"}

data: {"candidates":[{"content":{"parts":[{"text":" world"}],"role":"model"},"index":0,"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":9,"candidatesTokenCount":4,"totalTokenCount":13},"modelVersion":"gemini-2.5-pro","responseId":"resp_1"}

`

type quietLogger struct{}

func (quietLogger) Debug(string, ...any)                   {}
func (quietLogger) Info(string, ...any)                    {}
func (quietLogger) Warn(string, ...any)                    {}
func (quietLogger) Error(string, ...any)                   {}
func (quietLogger) Fatal(string, ...any)                   {}
func (quietLogger) SetLevel(schemas.LogLevel)              {}
func (quietLogger) SetOutputType(schemas.LoggerOutputType) {}
func (quietLogger) LogHTTPRequest(schemas.LogLevel, string) schemas.LogEventBuilder {
	return nil
}

func runGeminiStream(t *testing.T, fallback bool) []*schemas.BifrostStreamChunk {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(geminiStreamFixture)); err != nil {
			t.Errorf("write stream: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	ctx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
	if fallback {
		ctx.SetValue(schemas.BifrostContextKeyIsResponsesToChatCompletionFallback, true)
	}

	stream, bifrostErr := HandleGeminiChatCompletionStream(
		ctx,
		&fasthttp.Client{Dial: func(addr string) (net.Conn, error) { return net.Dial("tcp", addr) }},
		server.URL,
		[]byte(`{"contents":[]}`),
		map[string]string{}, nil, 30, false, false,
		schemas.Gemini,
		"gemini-2.5-pro",
		func(_ *schemas.BifrostContext, resp *schemas.BifrostResponse, err *schemas.BifrostError) (*schemas.BifrostResponse, *schemas.BifrostError) {
			return resp, err
		},
		nil,
		quietLogger{},
		func(context.Context) {},
	)
	if bifrostErr != nil {
		t.Fatalf("stream setup failed: %+v", bifrostErr)
	}

	var chunks []*schemas.BifrostStreamChunk
	timeout := time.After(15 * time.Second)
	for {
		select {
		case chunk, ok := <-stream:
			if !ok {
				return chunks
			}
			chunks = append(chunks, chunk)
		case <-timeout:
			t.Fatal("timed out waiting for the stream to close")
		}
	}
}

// The GenAI route has no chat-chunk converter, so before this the transport dropped every
// chunk of a downgraded stream after the 200 had already been committed.
func TestGeminiChatStreamReAssemblesResponsesOnFallback(t *testing.T) {
	chunks := runGeminiStream(t, true)
	if len(chunks) == 0 {
		t.Fatal("no chunks received")
	}

	var text string
	var sawCompleted bool
	for _, chunk := range chunks {
		if chunk.BifrostChatResponse != nil {
			t.Fatalf("received a raw chat chunk on the fallback path: %+v", chunk.BifrostChatResponse)
		}
		event := chunk.BifrostResponsesStreamResponse
		if event == nil {
			continue
		}
		if event.Type == schemas.ResponsesStreamResponseTypeOutputTextDelta && event.Delta != nil {
			text += *event.Delta
		}
		if event.Type == schemas.ResponsesStreamResponseTypeCompleted {
			sawCompleted = true
			if event.Response == nil || event.Response.Usage == nil {
				t.Fatal("terminal event carries no usage, so the request cannot be costed")
			}
		}
	}

	if text != "Hello world" {
		t.Errorf("re-assembled text = %q, want %q", text, "Hello world")
	}
	if !sawCompleted {
		t.Error("stream ended without a terminal response.completed event")
	}
}

func TestGeminiChatStreamKeepsChatChunksWithoutFallback(t *testing.T) {
	chunks := runGeminiStream(t, false)
	if len(chunks) == 0 {
		t.Fatal("no chunks received")
	}

	var text string
	for _, chunk := range chunks {
		if chunk.BifrostResponsesStreamResponse != nil {
			t.Fatalf("received a Responses event without the fallback flag: %+v", chunk.BifrostResponsesStreamResponse)
		}
		resp := chunk.BifrostChatResponse
		if resp == nil || len(resp.Choices) == 0 {
			continue
		}
		if delta := resp.Choices[0].ChatStreamResponseChoice; delta != nil && delta.Delta != nil && delta.Delta.Content != nil {
			text += *delta.Delta.Content
		}
	}

	if text != "Hello world" {
		t.Errorf("chat text = %q, want %q", text, "Hello world")
	}
}
