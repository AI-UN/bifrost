package zai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/maximhq/bifrost/core/providers/openai"
	providerUtils "github.com/maximhq/bifrost/core/providers/utils"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/tidwall/gjson"
)

func chatResponseHandler(responseBody []byte, response *schemas.BifrostChatResponse, requestBody []byte, sendBackRawRequest bool, sendBackRawResponse bool) (rawRequest interface{}, rawResponse interface{}, bifrostErr *schemas.BifrostError) {
	normalized, err := normalizeToolArguments(responseBody)
	if err != nil {
		return nil, nil, providerUtils.NewBifrostOperationError(schemas.ErrProviderResponseUnmarshal, err)
	}
	rawRequest, _, bifrostErr = providerUtils.HandleProviderResponse(normalized, response, requestBody, sendBackRawRequest, false)
	if bifrostErr != nil {
		return nil, nil, bifrostErr
	}
	if sendBackRawResponse {
		rawResponse = compactJSON(responseBody)
	}
	return rawRequest, rawResponse, nil
}

// normalizeToolArguments rewrites only the tool-call argument values that Z.AI may
// emit as JSON objects instead of the JSON string the shared OpenAI decoder expects.
// The rewrite is a targeted sjson patch so every other byte of the payload - notably
// integers wider than float64's exact range - reaches the decoder unchanged.
func normalizeToolArguments(body []byte) ([]byte, error) {
	choices := providerUtils.GetJSONField(body, "choices")
	if !choices.IsArray() {
		return body, nil
	}
	result := body
	for choiceIndex := range choices.Array() {
		for _, container := range [...]string{"message", "delta"} {
			base := fmt.Sprintf("choices.%d.%s.tool_calls", choiceIndex, container)
			toolCalls := providerUtils.GetJSONField(result, base)
			if !toolCalls.IsArray() {
				continue
			}
			for toolIndex := range toolCalls.Array() {
				path := fmt.Sprintf("%s.%d.function.arguments", base, toolIndex)
				arguments := providerUtils.GetJSONField(result, path)
				if arguments.Type != gjson.JSON {
					continue
				}
				patched, err := providerUtils.SetJSONField(result, path, arguments.Raw)
				if err != nil {
					return nil, fmt.Errorf("normalize %s: %w", path, err)
				}
				result = patched
			}
		}
	}
	return result, nil
}

func compactJSON(body []byte) []byte {
	var compact bytes.Buffer
	if err := json.Compact(&compact, body); err != nil {
		return append([]byte(nil), body...)
	}
	return compact.Bytes()
}
func toChatRequest(ctx *schemas.BifrostContext, request *schemas.BifrostChatRequest, providerName schemas.ModelProvider, stream bool) (*chatRequest, error) {
	if request == nil || request.Input == nil {
		return nil, providerUtils.InvalidRequestErrorf("chat messages are required")
	}

	if request.Params != nil {
		if toolChoice := request.Params.ToolChoice; toolChoice != nil {
			if toolChoice.ChatToolChoiceStruct != nil {
				return nil, providerUtils.InvalidRequestErrorf("zai providers support only tool_choice=auto")
			}
			if toolChoice.ChatToolChoiceStr != nil && *toolChoice.ChatToolChoiceStr != string(schemas.ChatToolChoiceTypeAuto) {
				return nil, providerUtils.InvalidRequestErrorf("zai providers support only tool_choice=auto")
			}
		}
		if request.Params.User != nil && !validZAIStringLength(*request.Params.User, 6, 128) {
			return nil, providerUtils.InvalidRequestErrorf("zai user must be between 6 and 128 characters")
		}
		if responseFormat, ok := schemas.ParseChatResponseFormat(request.Params.ResponseFormat); ok {
			switch responseFormat.Type {
			case "text", "json_object":
			default:
				return nil, providerUtils.InvalidRequestErrorf("zai providers support only text and json_object response formats")
			}
		}
		if requestID, exists := request.Params.ExtraParams["request_id"]; exists {
			value, ok := requestID.(string)
			if !ok || !validZAIStringLength(value, 6, 64) {
				return nil, providerUtils.InvalidRequestErrorf("zai request_id must be a string between 6 and 64 characters")
			}
		}
		if request.Params.Reasoning != nil {
			if request.Params.Reasoning.MaxTokens != nil {
				return nil, providerUtils.InvalidRequestErrorf("zai providers do not support reasoning.max_tokens")
			}
			if request.Params.Reasoning.Effort != nil {
				switch *request.Params.Reasoning.Effort {
				case "max", "xhigh", "high", "medium", "low", "minimal", "none":
				default:
					return nil, providerUtils.InvalidRequestErrorf("zai reasoning.effort must be one of max, xhigh, high, medium, low, minimal, or none")
				}
			}
		}
	}
	var nativeEffort *string
	if request.Params != nil && request.Params.Reasoning != nil && request.Params.Reasoning.Effort != nil {
		nativeEffort = new(*request.Params.Reasoning.Effort)
	}

	requestCopy := *request
	requestCopy.Provider = providerName
	openAIRequest := openai.ToOpenAIChatRequest(ctx, &requestCopy)
	if openAIRequest == nil {
		return nil, providerUtils.InvalidRequestErrorf("chat messages are required")
	}
	if nativeEffort != nil {
		if openAIRequest.Reasoning == nil {
			openAIRequest.Reasoning = &schemas.ChatReasoning{}
		}
		openAIRequest.Reasoning.Effort = nativeEffort
	}

	// Z.AI retains the legacy max_tokens name rather than OpenAI's
	// max_completion_tokens replacement.
	openAIRequest.MaxTokens = openAIRequest.MaxCompletionTokens
	openAIRequest.MaxCompletionTokens = nil
	openAIRequest.Stream = schemas.Ptr(stream)
	openAIRequest.StreamOptions = nil

	// Z.AI accepts a 6-128 character user_id, so the original value is carried over
	// instead of openAIRequest.User, which the shared converter drops beyond 64.
	var userID *string
	if request.Params != nil {
		userID = request.Params.User
	}
	openAIRequest.User = nil

	var thinking *thinkingConfig
	if openAIRequest.Reasoning != nil {
		reasoning := *openAIRequest.Reasoning
		openAIRequest.Reasoning = &reasoning

		if reasoning.Enabled != nil {
			if *reasoning.Enabled {
				thinking = &thinkingConfig{Type: "enabled"}
			} else {
				thinking = &thinkingConfig{Type: "disabled"}
				openAIRequest.Reasoning.Effort = nil
			}
		}
		if reasoning.Effort != nil && *reasoning.Effort == "none" {
			if reasoning.Enabled != nil && *reasoning.Enabled {
				return nil, providerUtils.InvalidRequestErrorf("reasoning.enabled=true conflicts with reasoning.effort=none")
			}
			thinking = &thinkingConfig{Type: "disabled"}
			openAIRequest.Reasoning.Effort = nil
		}
	}

	if thinking != nil && thinking.Type == "disabled" && requiresThinking(providerName, request.Model) {
		return nil, providerUtils.InvalidRequestErrorf("zai model %s requires thinking and cannot disable reasoning", request.Model)
	}

	return &chatRequest{
		openAI:   openAIRequest,
		thinking: thinking,
		userID:   userID,
	}, nil
}

func validZAIStringLength(value string, min, max int) bool {
	length := len([]rune(value))
	return length >= min && length <= max
}

// requiredThinkingModels cannot accept thinking.type=disabled; both regions
// document the combination as an error for these models.
var requiredThinkingModels = map[string]struct{}{
	"glm-5.3":       {},
	"glm-5.3-flash": {},
}

func requiresThinking(providerName schemas.ModelProvider, model string) bool {
	_, bare := schemas.ParseModelString(model, providerName)
	_, found := requiredThinkingModels[strings.ToLower(schemas.NormalizeModelName(bare))]
	return found
}
