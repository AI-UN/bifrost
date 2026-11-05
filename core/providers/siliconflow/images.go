package siliconflow

import (
	"fmt"
	"maps"
	"strings"

	providerUtils "github.com/maximhq/bifrost/core/providers/utils"
	"github.com/maximhq/bifrost/core/schemas"
)

// siliconFlowImageReservedKeys are wire fields owned by the typed request;
// extra params must not override them.
var siliconFlowImageReservedKeys = map[string]struct{}{
	"model":               {},
	"prompt":              {},
	"negative_prompt":     {},
	"image_size":          {},
	"batch_size":          {},
	"seed":                {},
	"num_inference_steps": {},
	"output_format":       {},
	"aspect_ratio":        {},
	"image":               {},
	"input_image":         {},
	"image_prompt":        {},
}

// siliconFlowImageEditField resolves which wire field carries the input image
// for a given SiliconFlow image-edit model. Unknown models return an error;
// there is no catch-all.
func siliconFlowImageEditField(model string) (string, error) {
	switch {
	case strings.HasPrefix(model, "Qwen/Qwen-Image-Edit"),
		model == "black-forest-labs/FLUX.1-Kontext-dev":
		return "image", nil
	case model == "black-forest-labs/FLUX.1-Kontext-pro",
		model == "black-forest-labs/FLUX.1-Kontext-max":
		return "input_image", nil
	case model == "black-forest-labs/FLUX-1.1-pro",
		model == "black-forest-labs/FLUX-1.1-pro-Ultra":
		return "image_prompt", nil
	default:
		return "", fmt.Errorf("model %s is not a supported SiliconFlow image edit model", model)
	}
}

// ToSiliconFlowImageGenerationRequest converts a Bifrost image generation
// request to SiliconFlow format.
func ToSiliconFlowImageGenerationRequest(request *schemas.BifrostImageGenerationRequest) (*SiliconFlowImageRequest, error) {
	if request == nil || request.Input == nil || strings.TrimSpace(request.Input.Prompt) == "" {
		return nil, fmt.Errorf("image generation prompt is required")
	}

	converted := &SiliconFlowImageRequest{
		Model:  request.Model,
		Prompt: request.Input.Prompt,
	}

	if params := request.Params; params != nil {
		converted.NegativePrompt = params.NegativePrompt
		converted.Seed = params.Seed
		converted.NumInferenceSteps = params.NumInferenceSteps
		converted.OutputFormat = params.OutputFormat
		converted.AspectRatio = params.AspectRatio
		if params.Size != nil && strings.TrimSpace(strings.ToLower(*params.Size)) != "auto" {
			converted.ImageSize = params.Size
		}
		if params.N != nil {
			converted.BatchSize = params.N
		}
		converted.ExtraParams = copySiliconFlowImageExtraParams(params.ExtraParams)
	}

	return converted, nil
}

// ToSiliconFlowImageEditRequest converts a Bifrost image edit request to
// SiliconFlow format. Exactly one non-empty input image is required; it is
// encoded as a base64 data URL and placed in the model-specific wire field.
func ToSiliconFlowImageEditRequest(request *schemas.BifrostImageEditRequest) (*SiliconFlowImageRequest, error) {
	if request == nil || request.Input == nil || strings.TrimSpace(request.Input.Prompt) == "" {
		return nil, fmt.Errorf("image edit prompt is required")
	}
	if len(request.Input.Images) == 0 || len(request.Input.Images[0].Image) == 0 {
		return nil, fmt.Errorf("image edit requires exactly one input image")
	}
	if len(request.Input.Images) > 1 {
		return nil, fmt.Errorf("SiliconFlow image edit supports exactly one input image, got %d", len(request.Input.Images))
	}

	field, err := siliconFlowImageEditField(request.Model)
	if err != nil {
		return nil, err
	}

	converted := &SiliconFlowImageRequest{
		Model:  request.Model,
		Prompt: request.Input.Prompt,
	}

	imageDataURL := providerUtils.FileBytesToBase64DataURL(request.Input.Images[0].Image)
	switch field {
	case "image":
		converted.Image = &imageDataURL
	case "input_image":
		converted.InputImage = &imageDataURL
	case "image_prompt":
		converted.ImagePrompt = &imageDataURL
	}

	if params := request.Params; params != nil {
		converted.NegativePrompt = params.NegativePrompt
		converted.Seed = params.Seed
		converted.NumInferenceSteps = params.NumInferenceSteps
		converted.OutputFormat = params.OutputFormat
		if params.Size != nil && strings.TrimSpace(strings.ToLower(*params.Size)) != "auto" {
			converted.ImageSize = params.Size
		}
		if params.N != nil {
			converted.BatchSize = params.N
		}
		converted.ExtraParams = copySiliconFlowImageExtraParams(params.ExtraParams)
	}

	return converted, nil
}

// copySiliconFlowImageExtraParams defensively copies extra params, dropping
// reserved wire keys.
func copySiliconFlowImageExtraParams(extraParams map[string]interface{}) map[string]interface{} {
	if len(extraParams) == 0 {
		return nil
	}
	copied := make(map[string]interface{}, len(extraParams))
	maps.Copy(copied, extraParams)
	for key := range siliconFlowImageReservedKeys {
		delete(copied, key)
	}
	if len(copied) == 0 {
		return nil
	}
	return copied
}

// ToBifrostImageGenerationResponse converts a SiliconFlow image response to
// Bifrost format, backfilling parameters from the originating request.
func (response *SiliconFlowImageResponse) ToBifrostImageGenerationResponse(sfReq *SiliconFlowImageRequest) (*schemas.BifrostImageGenerationResponse, error) {
	if response == nil {
		return nil, fmt.Errorf("image response is nil")
	}

	data := make([]schemas.ImageData, 0, len(response.Images))
	for i, img := range response.Images {
		if img.URL == "" {
			continue
		}
		data = append(data, schemas.ImageData{
			URL:   img.URL,
			Index: i,
		})
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("SiliconFlow image response contained no image URLs")
	}

	bifrostResponse := &schemas.BifrostImageGenerationResponse{
		Model: sfReq.Model,
		Data:  data,
	}

	responseParams := &schemas.ImageGenerationResponseParameters{}
	hasParams := false
	if sfReq.ImageSize != nil {
		responseParams.Size = *sfReq.ImageSize
		hasParams = true
	}
	if sfReq.AspectRatio != nil {
		responseParams.AspectRatio = *sfReq.AspectRatio
		hasParams = true
	}
	if sfReq.OutputFormat != nil {
		responseParams.OutputFormat = *sfReq.OutputFormat
		hasParams = true
	}
	if response.Seed != nil {
		responseParams.Seeds = []int{*response.Seed}
		hasParams = true
	}
	if hasParams {
		bifrostResponse.ImageGenerationResponseParameters = responseParams
	}

	return bifrostResponse, nil
}
