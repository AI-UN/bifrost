// Package cpa implements the CLI Proxy API (CPA) provider.
//
// CPA is a self-hosted gateway that puts Claude Code, ChatGPT and Google AI
// subscriptions behind three client dialects at once: Anthropic Messages
// (/v1/messages), OpenAI Chat/Responses (/v1/chat/completions, /v1/responses) and
// Gemini generateContent (/v1beta/models/{model}:...). All three dialects accept the
// same API key, serve the same model set, and CPA translates between the dialect a
// client speaks and whatever protocol the model's upstream account actually needs.
//
// This provider is therefore a facade over the three built-in providers that already
// speak those dialects, all pointed at one CPA instance. Two things follow from
// keeping it a single provider:
//
//   - The model catalog holds one cpa/<model> row per model instead of one per
//     dialect. Catalog identity is (provider, model), so three provider entries
//     against one backend necessarily triple every model.
//   - The model-to-upstream decision stays in CPA, the only component that knows
//     it. Bifrost picks which dialect to *speak*, never which subscription runs.
package cpa

import (
	"strings"

	providerUtils "github.com/maximhq/bifrost/core/providers/utils"
	schemas "github.com/maximhq/bifrost/core/schemas"
)

// geminiAPIVersionPath is the version segment the Gemini provider expects to already
// be part of its base URL: it builds "<base>/models/{model}:<action>" and never
// inserts a version itself.
const geminiAPIVersionPath = "/v1beta"

// Integration identifiers recorded on the context by the HTTP transport for every
// SDK-compatibility route. Values mirror integrations.RouteConfigType; they are
// duplicated as plain strings because core must not import transports.
const (
	integrationAnthropic = "anthropic"
	integrationGenAI     = "genai"
)

// dialect is one of the three request shapes CPA accepts.
type dialect uint8

const (
	dialectOpenAI dialect = iota
	dialectAnthropic
	dialectGemini
)

// baseProvider is the built-in provider whose wire format this dialect is.
func (d dialect) baseProvider() schemas.ModelProvider {
	switch d {
	case dialectAnthropic:
		return schemas.Anthropic
	case dialectGemini:
		return schemas.Gemini
	default:
		return schemas.OpenAI
	}
}

// ingressDialect picks the CPA dialect from the protocol the client actually used.
// The HTTP transport records it on the context for every integration route, so
// Claude Code keeps speaking Anthropic and Codex keeps speaking OpenAI Responses all
// the way through to CPA, and CPA performs exactly one translation into the
// upstream's protocol.
//
// Model names are deliberately not consulted. Which upstream serves a model is a
// function of CPA's registry, its per-account aliases and its executor selection —
// one alias can even round-robin across several upstream models. Re-deriving that
// from a "gpt-"/"claude-" prefix here would be a second, weaker copy of a decision
// CPA already makes correctly, and it would silently disagree whenever an operator
// aliases a model across families.
//
// Requests that never passed through an integration route (Go SDK callers) carry no
// integration type and fall back to the OpenAI dialect.
func ingressDialect(ctx *schemas.BifrostContext) dialect {
	if ctx == nil {
		return dialectOpenAI
	}
	integration, ok := ctx.Value(schemas.BifrostContextKeyIntegrationType).(string)
	if !ok {
		return dialectOpenAI
	}
	switch strings.ToLower(strings.TrimSpace(integration)) {
	case integrationAnthropic:
		return dialectAnthropic
	case integrationGenAI:
		return dialectGemini
	default:
		return dialectOpenAI
	}
}

// imageDialect picks the dialect for an image operation.
//
// Unlike the conversational endpoints, CPA's two image surfaces accept disjoint
// model sets, so the inbound protocol cannot decide this one:
//
//   - /v1/images/generations and /v1/images/edits are gated to a fixed allowlist —
//     gpt-image-* (Codex), grok-imagine-* (xAI) and openai-compatibility models
//     configured with image: true. Anything else is rejected with a 400 naming the
//     supported models, which is CPA's allowlist to own, not ours to mirror.
//   - Google AI image models (gemini-*-image) are not on that allowlist. They are
//     ordinary Gemini models with an image output modality, reachable only through
//     /v1beta/models/{model}:generateContent.
//   - Imagen publishes only :predict, and CPA's Gemini route dispatches just
//     generateContent, streamGenerateContent and countTokens with no default case.
//     A :predict request falls through that switch and returns an empty 200 with no
//     body, so it has to fail here as an explicit unsupported operation rather than
//     as a silent success.
func imageDialect(ctx *schemas.BifrostContext, model string, requestType schemas.RequestType) (dialect, *schemas.BifrostError) {
	if schemas.IsImagenModelFamily(ctx, model) {
		return dialectOpenAI, providerUtils.NewUnsupportedOperationError(requestType, schemas.CPA)
	}
	if schemas.IsGeminiModelFamily(ctx, model) {
		return dialectGemini, nil
	}
	return dialectOpenAI, nil
}

// passthroughDialect picks the dialect from the raw path the caller asked for, since
// a passthrough request carries no converted body to infer from.
func passthroughDialect(path string) dialect {
	trimmed := strings.TrimSpace(path)
	switch {
	case trimmed == "/v1beta" || strings.HasPrefix(trimmed, "/v1beta/"):
		return dialectGemini
	case trimmed == "/v1/messages" || strings.HasPrefix(trimmed, "/v1/messages/"):
		return dialectAnthropic
	default:
		return dialectOpenAI
	}
}

// dialectConfig clones config for one sub-provider, retargeting its base URL and
// forcing its reported identity to cpa.
//
// The clone is mandatory: every sub-provider constructor mutates the config it is
// given (it fills in a default base URL), and the Gemini sub-provider needs a
// different base URL from the other two.
//
// CustomProviderKey is what makes a sub-provider report "cpa" instead of its own
// name — GetProviderKey on all three routes through providerUtils.GetProviderName,
// which returns the custom key when set. Without it, every error, ExtraFields entry
// and log line would attribute the request to openai/anthropic/gemini. Any gating
// the operator configured on the cpa provider is carried across unchanged.
func dialectConfig(config *schemas.ProviderConfig, baseURL string, base schemas.ModelProvider) *schemas.ProviderConfig {
	clone := *config
	clone.NetworkConfig.BaseURL = baseURL

	custom := schemas.CustomProviderConfig{
		CustomProviderKey: string(schemas.CPA),
		BaseProviderType:  base,
	}
	if config.CustomProviderConfig != nil {
		custom.IsKeyLess = config.CustomProviderConfig.IsKeyLess
		custom.AllowedRequests = config.CustomProviderConfig.AllowedRequests
		custom.RequestPathOverrides = config.CustomProviderConfig.RequestPathOverrides
		custom.DoesNotSendDoneMarker = config.CustomProviderConfig.DoesNotSendDoneMarker
	}
	clone.CustomProviderConfig = &custom

	return &clone
}
