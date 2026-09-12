package handlers

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fasthttp/router"
	"github.com/fasthttp/websocket"
	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
	bfws "github.com/maximhq/bifrost/transports/bifrost-http/websocket"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/valyala/fasthttp"
)

type cpaWSAccount struct {
	config *schemas.ProviderConfig
}

func (a cpaWSAccount) GetConfiguredProviders() ([]schemas.ModelProvider, error) {
	return []schemas.ModelProvider{schemas.CPA}, nil
}

func (a cpaWSAccount) GetConfigForProvider(provider schemas.ModelProvider) (*schemas.ProviderConfig, error) {
	if provider != schemas.CPA {
		return nil, fmt.Errorf("unexpected provider %s", provider)
	}
	return a.config, nil
}

func (a cpaWSAccount) GetKeysForProvider(_ context.Context, provider schemas.ModelProvider) ([]schemas.Key, error) {
	if provider != schemas.CPA {
		return nil, fmt.Errorf("unexpected provider %s", provider)
	}
	return []schemas.Key{{
		ID: "cpa-key", Value: schemas.SecretVar{Val: "cpa-secret"},
		Models: schemas.WhiteList{"*"}, Weight: 1,
	}}, nil
}

type cpaWSTurn struct {
	provider    schemas.ModelProvider
	requestType schemas.RequestType
	totalTokens int
}

type cpaWSUsageCapture struct {
	turns chan cpaWSTurn
}

func (*cpaWSUsageCapture) GetName() string { return "cpa-ws-usage" }
func (*cpaWSUsageCapture) Cleanup() error  { return nil }
func (*cpaWSUsageCapture) PreRequestHook(*schemas.BifrostContext, *schemas.BifrostRequest) error {
	return nil
}
func (*cpaWSUsageCapture) PreLLMHook(_ *schemas.BifrostContext, req *schemas.BifrostRequest) (*schemas.BifrostRequest, *schemas.LLMPluginShortCircuit, error) {
	return req, nil, nil
}
func (p *cpaWSUsageCapture) PostLLMHook(_ *schemas.BifrostContext, resp *schemas.BifrostResponse, err *schemas.BifrostError) (*schemas.BifrostResponse, *schemas.BifrostError, error) {
	if resp != nil && resp.ResponsesStreamResponse != nil {
		chunk := resp.ResponsesStreamResponse
		if chunk.Type == schemas.ResponsesStreamResponseTypeCompleted && chunk.Response != nil && chunk.Response.Usage != nil {
			p.turns <- cpaWSTurn{chunk.ExtraFields.Provider, chunk.ExtraFields.RequestType, chunk.Response.Usage.TotalTokens}
		}
	}
	return resp, err, nil
}

// Exercise the actual upgrade handler and upstream dial. A successful HTTP/SSE
// bridge must not pass this test: both turns must share one CPA WebSocket.
func TestCPAWebSocketNativeTransport(t *testing.T) {
	SetLogger(bifrost.NewNoOpLogger())
	var upgrades, httpRequests atomic.Int32
	requests := make(chan []byte, 2)
	events := [][]string{
		{
			`{"type":"response.created","sequence_number":0,"response":{"id":"resp_1","model":"gpt-5.6-sol","status":"in_progress"}}`,
			`{"type":"response.output_item.added","sequence_number":1,"output_index":0,"item":{"type":"function_call","id":"fc_1","call_id":"call_1","name":"lookup","arguments":""}}`,
			`{"type":"response.function_call_arguments.delta","sequence_number":2,"output_index":0,"item_id":"fc_1","delta":"{}"}`,
			`{"type":"response.completed","sequence_number":3,"response":{"id":"resp_1","model":"gpt-5.6-sol","status":"completed","output":[{"type":"function_call","id":"fc_1","call_id":"call_1","name":"lookup","arguments":"{}"}],"usage":{"input_tokens":11,"output_tokens":7,"total_tokens":18}}}`,
		},
		{
			`{"type":"response.created","sequence_number":0,"response":{"id":"resp_2","model":"gpt-5.6-sol","status":"in_progress"}}`,
			`{"type":"response.output_text.delta","sequence_number":1,"output_index":0,"content_index":0,"item_id":"msg_2","delta":"OK"}`,
			`{"type":"response.completed","sequence_number":2,"response":{"id":"resp_2","model":"gpt-5.6-sol","status":"completed","output":[],"usage":{"input_tokens":19,"output_tokens":2,"total_tokens":21}}}`,
		},
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !websocket.IsWebSocketUpgrade(r) {
			httpRequests.Add(1)
			http.Error(w, "expected native WebSocket transport", http.StatusBadRequest)
			return
		}
		if r.URL.Path != "/proxy/cpa/v1/responses" || r.Header.Get("Authorization") != "Bearer cpa-secret" || r.Header.Get("Originator") != "codex_cli_rs" {
			t.Error("CPA handshake used an incorrect path, credential or configured header")
			http.Error(w, "invalid handshake", http.StatusBadRequest)
			return
		}
		upgrader := newWSTestUpgrader()
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("CPA upgrade: %v", err)
			return
		}
		defer conn.Close()
		upgrades.Add(1)
		_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
		_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		for _, turn := range events {
			_, message, err := conn.ReadMessage()
			if err != nil {
				t.Errorf("CPA read: %v", err)
				return
			}
			requests <- message
			for _, event := range turn {
				if err := conn.WriteMessage(websocket.TextMessage, []byte(event)); err != nil {
					t.Errorf("CPA write: %v", err)
					return
				}
			}
		}
		// Let the gateway finish reading the last event before closing upstream.
		_, _, _ = conn.ReadMessage()
	}))
	defer upstream.Close()

	account := cpaWSAccount{config: &schemas.ProviderConfig{
		NetworkConfig: schemas.NetworkConfig{
			BaseURL: upstream.URL + "/proxy/cpa", AllowPrivateNetwork: true,
			DefaultRequestTimeoutInSeconds: 5, StreamIdleTimeoutInSeconds: 5,
			ExtraHeaders: map[string]string{"Originator": "codex_cli_rs"},
		},
		ConcurrencyAndBufferSize: schemas.ConcurrencyAndBufferSize{Concurrency: 1, BufferSize: 1},
		OpenAIConfig:             &schemas.OpenAIConfig{DisableStore: true},
	}}
	capture := &cpaWSUsageCapture{turns: make(chan cpaWSTurn, 2)}
	client, err := bifrost.Init(context.Background(), schemas.BifrostConfig{
		Account: account, Logger: bifrost.NewNoOpLogger(), LLMPlugins: []schemas.LLMPlugin{capture},
	})
	require.NoError(t, err)
	defer client.Shutdown()

	cfg := &lib.Config{
		ClientConfig:    &configstore.ClientConfig{},
		WebSocketConfig: &schemas.WebSocketConfig{},
		Providers: map[schemas.ModelProvider]configstore.ProviderConfig{schemas.CPA: {
			NetworkConfig: &account.config.NetworkConfig, OpenAIConfig: account.config.OpenAIConfig,
		}},
	}
	cfg.WebSocketConfig.CheckAndSetDefaults()
	pool := bfws.NewPool(cfg.WebSocketConfig.Pool)
	defer pool.Close()
	handler := NewWSResponsesHandler(client, cfg, pool)
	routes := router.New()
	handler.RegisterRoutes(routes)
	server := &fasthttp.Server{Handler: routes.Handler}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	serverDone := make(chan error, 1)
	go func() { serverDone <- server.Serve(listener) }()
	defer func() {
		handler.Close()
		require.NoError(t, server.ShutdownWithContext(context.Background()))
		require.NoError(t, <-serverDone)
	}()

	conn, response, err := websocket.DefaultDialer.Dial("ws://"+listener.Addr().String()+"/v1/responses", http.Header{
		"Authorization": {"Bearer sk-bf-client-credential"},
	})
	require.NoError(t, err)
	defer conn.Close()
	require.Equal(t, http.StatusSwitchingProtocols, response.StatusCode)
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(10*time.Second)))
	require.NoError(t, conn.SetWriteDeadline(time.Now().Add(10*time.Second)))

	for i, event := range []string{
		`{"type":"response.create","model":"cpa/gpt-5.6-sol","store":false,"input":[{"role":"user","content":"Use lookup"}],"tools":[{"type":"function","name":"lookup","parameters":{"type":"object","properties":{}}}]}`,
		`{"type":"response.create","model":"cpa/gpt-5.6-sol","store":false,"previous_response_id":"resp_1","input":[{"type":"function_call_output","call_id":"call_1","output":"OK"}]}`,
	} {
		require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(event)))
		for _, expected := range events[i] {
			kind, actual, err := conn.ReadMessage()
			require.NoError(t, err)
			require.Equal(t, websocket.TextMessage, kind)
			require.Equal(t, expected, string(actual), "native response frames must be relayed unchanged")
		}
		select {
		case sent := <-requests:
			require.Equal(t, "gpt-5.6-sol", gjson.GetBytes(sent, "model").String())
			require.Equal(t, "false", gjson.GetBytes(sent, "store").Raw)
			require.Equal(t, gjson.Get(event, "input").Raw, gjson.GetBytes(sent, "input").Raw)
			require.Equal(t, gjson.Get(event, "previous_response_id").String(), gjson.GetBytes(sent, "previous_response_id").String())
			require.False(t, strings.Contains(string(sent), "sk-bf-client-credential"))
		case <-time.After(time.Second):
			t.Fatal("CPA did not receive the response.create event")
		}
		select {
		case turn := <-capture.turns:
			require.Equal(t, schemas.CPA, turn.provider)
			require.Equal(t, schemas.WebSocketResponsesRequest, turn.requestType)
			require.Equal(t, []int{18, 21}[i], turn.totalTokens)
		case <-time.After(time.Second):
			t.Fatal("terminal usage was not delivered to post hooks")
		}
	}
	require.EqualValues(t, 1, upgrades.Load(), "continuations must reuse the same upstream socket")
	require.Zero(t, httpRequests.Load(), "CPA must not fall back to HTTP/SSE")
}
