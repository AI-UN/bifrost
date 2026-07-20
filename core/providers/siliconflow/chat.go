package siliconflow

import (
	"maps"
	"strings"

	"github.com/maximhq/bifrost/core/schemas"
)

// prepareSiliconFlowChatRequest maps Bifrost's typed reasoning parameters onto
// SiliconFlow's `enable_thinking` / `thinking_budget` extra params and clears
// the copied Reasoning so the generic OpenAI converter never emits
// `reasoning_effort`, which SiliconFlow does not define. The caller's request
// is never mutated: the request, params, and extra params are all
// defensively copied before modification.
//
// DeepSeek-V3.1 models reject thinking together with tool calls, so
// `enable_thinking` is forced to false for those models whenever tools are
// present, overriding any earlier mapping.
func prepareSiliconFlowChatRequest(request *schemas.BifrostChatRequest) *schemas.BifrostChatRequest {
	if request == nil || request.Params == nil {
		return request
	}

	params := request.Params
	needsThinkingMapping := params.Reasoning != nil
	forceDisableThinking := strings.HasPrefix(request.Model, "deepseek-ai/DeepSeek-V3.1") && len(params.Tools) > 0
	if !needsThinkingMapping && !forceDisableThinking {
		return request
	}

	requestCopy := *request
	paramsCopy := *params
	extraParams := make(map[string]interface{}, len(params.ExtraParams)+2)
	maps.Copy(extraParams, params.ExtraParams)

	if reasoning := paramsCopy.Reasoning; reasoning != nil {
		if reasoning.Enabled != nil {
			extraParams["enable_thinking"] = *reasoning.Enabled
		} else if reasoning.Effort != nil {
			extraParams["enable_thinking"] = *reasoning.Effort != "none"
		}
		if reasoning.MaxTokens != nil {
			extraParams["thinking_budget"] = *reasoning.MaxTokens
		}
		paramsCopy.Reasoning = nil
	}

	if forceDisableThinking {
		extraParams["enable_thinking"] = false
	}

	paramsCopy.ExtraParams = extraParams
	requestCopy.Params = &paramsCopy
	return &requestCopy
}
