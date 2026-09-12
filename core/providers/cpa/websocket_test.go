package cpa

import (
	"strings"
	"testing"

	"github.com/maximhq/bifrost/core/schemas"
)

func TestCPAWebSocketResponsesURL(t *testing.T) {
	for _, tc := range []struct {
		baseURL string
		wantURL string
	}{
		{"http://cpa.internal:8317", "ws://cpa.internal:8317/v1/responses"},
		{"https://cpa.example/", "wss://cpa.example/v1/responses"},
		{" https://gateway.example/proxy/cpa/// ", "wss://gateway.example/proxy/cpa/v1/responses"},
	} {
		t.Run(tc.baseURL, func(t *testing.T) {
			provider := newTestProvider(t, tc.baseURL)
			wsProvider, ok := any(provider).(schemas.WebSocketCapableProvider)
			if !ok {
				t.Fatal("CPA must expose WebSocketCapableProvider so the gateway can select native WebSocket transport")
			}
			if !wsProvider.SupportsWebSocketMode() {
				t.Fatal("CPA must support Responses WebSocket mode")
			}
			if got := wsProvider.WebSocketResponsesURL(testKey()); got != tc.wantURL {
				t.Errorf("WebSocketResponsesURL = %q, want %q", got, tc.wantURL)
			}
		})
	}
}

func TestCPAWebSocketHeadersUseSelectedKey(t *testing.T) {
	config := &schemas.ProviderConfig{
		NetworkConfig: schemas.NetworkConfig{
			BaseURL: "https://cpa.example",
			ExtraHeaders: map[string]string{
				"authorization": "Bearer stale-key",
				"Originator":    "codex_cli_rs",
				"X-CPA-Test":    "configured",
			},
		},
	}
	provider, err := NewCPAProvider(config, testLogger{})
	if err != nil {
		t.Fatal(err)
	}
	wsProvider, ok := any(provider).(schemas.WebSocketCapableProvider)
	if !ok {
		t.Fatal("CPA must expose WebSocketCapableProvider")
	}
	for _, value := range []string{"first-cpa-key", "second-cpa-key"} {
		key := schemas.Key{Value: schemas.SecretVar{Val: value}}
		headers := wsProvider.WebSocketHeaders(key)
		if headers["Authorization"] != "Bearer "+value {
			t.Error("WebSocket handshake must authenticate with the selected CPA key")
		}
		for name := range headers {
			if strings.EqualFold(name, "Authorization") && name != "Authorization" {
				t.Error("extra headers introduced a duplicate authorization header")
			}
		}
		if headers["Originator"] != "codex_cli_rs" || headers["X-CPA-Test"] != "configured" {
			t.Error("configured extra headers were not preserved")
		}
		headers["X-CPA-Test"] = "request-local"
	}
	if config.NetworkConfig.ExtraHeaders["X-CPA-Test"] != "configured" {
		t.Error("WebSocket headers must not mutate the provider configuration")
	}
}
