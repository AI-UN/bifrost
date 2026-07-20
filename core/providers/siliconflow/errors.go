package siliconflow

import (
	"strings"

	"github.com/bytedance/sonic"
	providerUtils "github.com/maximhq/bifrost/core/providers/utils"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/valyala/fasthttp"
)

// parseSiliconFlowError parses SiliconFlow error responses. It first tries the
// OpenAI-compatible envelope (`{"error": {...}}`), then SiliconFlow's native
// envelope (`{"code", "message", "data"}`), and finally falls back to the
// generic provider API error handling.
func parseSiliconFlowError(resp *fasthttp.Response) *schemas.BifrostError {
	var openAIStyle schemas.BifrostError
	bifrostErr := providerUtils.HandleProviderAPIError(resp, &openAIStyle)

	if bifrostErr.Error == nil {
		bifrostErr.Error = &schemas.ErrorField{}
	}

	if openAIStyle.Error != nil && strings.TrimSpace(openAIStyle.Error.Message) != "" {
		bifrostErr.Error.Type = openAIStyle.Error.Type
		bifrostErr.Error.Code = openAIStyle.Error.Code
		bifrostErr.Error.Message = openAIStyle.Error.Message
		bifrostErr.Error.Param = openAIStyle.Error.Param
		return bifrostErr
	}

	// Try SiliconFlow's native error envelope on the decoded body.
	if decodedBody, decodeErr := providerUtils.CheckAndDecodeBody(resp); decodeErr == nil {
		var sfErr SiliconFlowError
		if err := sonic.Unmarshal(decodedBody, &sfErr); err == nil && strings.TrimSpace(sfErr.Message) != "" {
			bifrostErr.Error.Message = sfErr.Message
			if sfErr.Code != nil {
				code := sfErr.Code.String()
				bifrostErr.Error.Code = &code
			}
			return bifrostErr
		}
	}

	if strings.TrimSpace(bifrostErr.Error.Message) == "" {
		bifrostErr.Error.Message = "provider API error"
	}

	return bifrostErr
}
