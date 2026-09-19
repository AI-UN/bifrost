package anthropic

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

// anthropicStreamFixture is a healthy Anthropic message stream: message_start through
// message_stop, with usage on message_delta.
const anthropicStreamFixture = `event: message_start
data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-sonnet-4","content":[],"usage":{"input_tokens":9,"output_tokens":0}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" world"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":4}}

event: message_stop
data: {"type":"message_stop"}

`

func serveSSE(t *testing.T, body string) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(body)); err != nil {
			t.Errorf("write stream: %v", err)
		}
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}))
	t.Cleanup(server.Close)
	return server.URL
}

func collectChunks(t *testing.T, stream chan *schemas.BifrostStreamChunk) []*schemas.BifrostStreamChunk {
	t.Helper()
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

func passthroughPostHook(_ *schemas.BifrostContext, resp *schemas.BifrostResponse, err *schemas.BifrostError) (*schemas.BifrostResponse, *schemas.BifrostError) {
	return resp, err
}

// quietLogger keeps handler warnings out of the test output without pulling in the core
// package, which would import-cycle back into this one.
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

func runAnthropicStream(t *testing.T, fallback bool) []*schemas.BifrostStreamChunk {
	t.Helper()
	ctx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
	if fallback {
		ctx.SetValue(schemas.BifrostContextKeyIsResponsesToChatCompletionFallback, true)
	}

	stream, bifrostErr := HandleAnthropicChatCompletionStreaming(
		ctx,
		&fasthttp.Client{Dial: func(addr string) (net.Conn, error) { return net.Dial("tcp", addr) }},
		serveSSE(t, anthropicStreamFixture),
		[]byte(`{"model":"claude-sonnet-4","stream":true}`),
		map[string]string{}, nil, 30, nil, false, false,
		schemas.Anthropic,
		passthroughPostHook,
		nil, nil,
		quietLogger{},
		func(context.Context) {},
	)
	if bifrostErr != nil {
		t.Fatalf("stream setup failed: %+v", bifrostErr)
	}
	return collectChunks(t, stream)
}

// Core downgrades a ResponsesStream request to ChatCompletionStream and expects the
// provider to re-assemble Responses events. Anthropic's native chat stream used to ignore
// that flag and emit raw chat chunks, which no Responses-speaking caller can render.
func TestAnthropicChatStreamReAssemblesResponsesOnFallback(t *testing.T) {
	chunks := runAnthropicStream(t, true)
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
			if event.Response.Usage.InputTokens != 9 {
				t.Errorf("input tokens = %d, want 9", event.Response.Usage.InputTokens)
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

// Without the flag the same handler must keep emitting chat chunks: every OpenAI-shaped
// caller and every non-downgraded request depends on it.
func TestAnthropicChatStreamKeepsChatChunksWithoutFallback(t *testing.T) {
	chunks := runAnthropicStream(t, false)
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
