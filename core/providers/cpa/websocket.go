package cpa

import "github.com/maximhq/bifrost/core/schemas"

var _ schemas.WebSocketCapableProvider = (*CPAProvider)(nil)

// SupportsWebSocketMode enables native Responses WebSocket transport to CPA.
// CPA resolves model aliases and decides which upstream supports WebSocket mode.
func (provider *CPAProvider) SupportsWebSocketMode() bool {
	return provider.openai.SupportsWebSocketMode()
}

// WebSocketResponsesURL targets CPA's /v1/responses using the configured base URL.
func (provider *CPAProvider) WebSocketResponsesURL(key schemas.Key) string {
	return provider.openai.WebSocketResponsesURL(key)
}

// WebSocketHeaders authenticates with the selected CPA key and includes configured
// extra headers, using the same handshake as an OpenAI-compatible custom provider.
func (provider *CPAProvider) WebSocketHeaders(key schemas.Key) map[string]string {
	return provider.openai.WebSocketHeaders(key)
}
