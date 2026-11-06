package zai

import (
	"encoding/json"
	"fmt"

	providerUtils "github.com/maximhq/bifrost/core/providers/utils"
	"github.com/maximhq/bifrost/core/schemas"
)

type imageGenerationRequest struct {
	Model            string  `json:"model"`
	Prompt           string  `json:"prompt"`
	Quality          *string `json:"quality,omitempty"`
	Size             *string `json:"size,omitempty"`
	UserID           *string `json:"user_id,omitempty"`
	WatermarkEnabled *bool   `json:"watermark_enabled,omitempty"`
}

func (r *imageGenerationRequest) GetExtraParams() map[string]interface{} { return nil }

func (provider *ZAIProvider) imageGeneration(ctx *schemas.BifrostContext, key schemas.Key, request *schemas.BifrostImageGenerationRequest) (*schemas.BifrostImageGenerationResponse, *schemas.BifrostError) {
	if request == nil || request.Input == nil || request.Input.Prompt == "" {
		return nil, providerUtils.NewBifrostBadRequestError("image prompt is required")
	}
	if !provider.profile.supportsImageModel(request.Model) {
		return nil, providerUtils.NewBifrostBadRequestError(fmt.Sprintf("%s image model %q is not documented for this regional API", provider.GetProviderKey(), request.Model))
	}
	if providerUtils.IsLargePayloadPassthroughEnabled(ctx) {
		return nil, providerUtils.NewBifrostBadRequestError("large payload passthrough is not supported for zai image conversion")
	}

	converted := &imageGenerationRequest{
		Model:  request.Model,
		Prompt: request.Input.Prompt,
	}
	if params := request.Params; params != nil {
		converted.Quality = params.Quality
		converted.Size = params.Size
		converted.UserID = params.User
		if params.N != nil || params.Background != nil || params.Moderation != nil || params.PartialImages != nil || params.OutputCompression != nil || params.OutputFormat != nil || params.Style != nil || params.Type != nil || params.ResponseFormat != nil || params.Seed != nil || params.NegativePrompt != nil || params.NumInferenceSteps != nil || len(params.InputImages) > 0 || params.AspectRatio != nil {
			return nil, providerUtils.NewBifrostBadRequestError("zai image generation supports only quality, size, user, and watermark_enabled")
		}
		if params.User != nil && !validZAIStringLength(*params.User, 6, 128) {
			return nil, providerUtils.NewBifrostBadRequestError("zai image user must be between 6 and 128 characters")
		}
		for name, value := range params.ExtraParams {
			if name != "watermark_enabled" {
				return nil, providerUtils.NewBifrostBadRequestError("zai image generation supports only the watermark_enabled extra parameter")
			}
			if !provider.profile.supportsImageWatermark {
				return nil, providerUtils.NewBifrostBadRequestError(fmt.Sprintf("%s image generation does not support watermark_enabled", provider.GetProviderKey()))
			}
			watermarkEnabled, ok := value.(bool)
			if !ok {
				return nil, providerUtils.NewBifrostBadRequestError("zai image watermark_enabled must be a boolean")
			}
			converted.WatermarkEnabled = &watermarkEnabled
		}
	}

	jsonBody, bifrostErr := providerUtils.CheckContextAndGetRequestBody(ctx, request, func() (providerUtils.RequestBodyWithExtraParams, error) {
		return converted, nil
	})
	if bifrostErr != nil {
		return nil, bifrostErr
	}
	responseBody, latency, responseHeaders, bifrostErr := provider.doJSON(ctx, key, pathImageGenerations, jsonBody)
	if responseHeaders != nil {
		ctx.SetValue(schemas.BifrostContextKeyProviderResponseHeaders, responseHeaders)
	}
	if bifrostErr != nil {
		return nil, providerUtils.EnrichError(ctx, bifrostErr, jsonBody, responseBody, provider.sendBackRawRequest, provider.sendBackRawResponse, latency)
	}

	response := &schemas.BifrostImageGenerationResponse{}
	rawRequest, rawResponse, bifrostErr := providerUtils.HandleProviderResponseCtx(ctx, responseBody, response, jsonBody, providerUtils.ShouldSendBackRawRequest(ctx, provider.sendBackRawRequest), providerUtils.ShouldSendBackRawResponse(ctx, provider.sendBackRawResponse))
	if bifrostErr != nil {
		return nil, providerUtils.EnrichError(ctx, bifrostErr, jsonBody, responseBody, provider.sendBackRawRequest, provider.sendBackRawResponse, latency)
	}
	response.ExtraFields.Latency = latency.Milliseconds()
	response.ExtraFields.ProviderResponseHeaders = responseHeaders
	if providerUtils.ShouldSendBackRawRequest(ctx, provider.sendBackRawRequest) {
		response.ExtraFields.RawRequest = rawRequest
	}
	if providerUtils.ShouldSendBackRawResponse(ctx, provider.sendBackRawResponse) {
		response.ExtraFields.RawResponse = rawResponse
	}
	if response.Model == "" {
		response.Model = request.Model
	}
	return response, nil
}

var _ json.Marshaler = (*imageGenerationRequest)(nil)

func (r *imageGenerationRequest) MarshalJSON() ([]byte, error) {
	type alias imageGenerationRequest
	return providerUtils.MarshalSorted((*alias)(r))
}
