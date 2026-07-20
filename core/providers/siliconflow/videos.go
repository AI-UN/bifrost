package siliconflow

import (
	"fmt"
	"maps"
	"strings"

	"github.com/maximhq/bifrost/core/schemas"
)

// siliconFlowVideoReservedKeys are wire fields owned by the typed request;
// extra params must not override them.
var siliconFlowVideoReservedKeys = map[string]struct{}{
	"model":           {},
	"prompt":          {},
	"image_size":      {},
	"negative_prompt": {},
	"image":           {},
	"seed":            {},
}

// ToSiliconFlowVideoSubmitRequest converts a Bifrost video generation request
// to SiliconFlow's /v1/video/submit format.
func ToSiliconFlowVideoSubmitRequest(request *schemas.BifrostVideoGenerationRequest) (*SiliconFlowVideoSubmitRequest, error) {
	if request == nil || request.Input == nil || strings.TrimSpace(request.Input.Prompt) == "" {
		return nil, fmt.Errorf("video generation prompt is required")
	}

	converted := &SiliconFlowVideoSubmitRequest{
		Model:  request.Model,
		Prompt: request.Input.Prompt,
	}
	if request.Input.InputReference != nil && *request.Input.InputReference != "" {
		converted.Image = request.Input.InputReference
	}

	if params := request.Params; params != nil {
		if params.Size != "" {
			converted.ImageSize = &params.Size
		}
		converted.NegativePrompt = params.NegativePrompt
		converted.Seed = params.Seed
		if len(params.ExtraParams) > 0 {
			extraParams := make(map[string]interface{}, len(params.ExtraParams))
			maps.Copy(extraParams, params.ExtraParams)
			for key := range siliconFlowVideoReservedKeys {
				delete(extraParams, key)
			}
			if len(extraParams) > 0 {
				converted.ExtraParams = extraParams
			}
		}
	}

	return converted, nil
}

// toBifrostVideoStatus converts a SiliconFlow video status to a Bifrost video
// status. Unknown statuses return an error rather than being coerced.
func toBifrostVideoStatus(status string) (schemas.VideoStatus, error) {
	switch status {
	case "InQueue":
		return schemas.VideoStatusQueued, nil
	case "InProgress":
		return schemas.VideoStatusInProgress, nil
	case "Succeed":
		return schemas.VideoStatusCompleted, nil
	case "Failed":
		return schemas.VideoStatusFailed, nil
	default:
		return "", fmt.Errorf("unknown SiliconFlow video status %q", status)
	}
}

// ToBifrostVideoGenerationResponse converts a SiliconFlow video status
// response to a Bifrost video generation response. The provider-suffixed ID
// is set by the caller.
func (r *SiliconFlowVideoStatusResponse) ToBifrostVideoGenerationResponse() (*schemas.BifrostVideoGenerationResponse, error) {
	if r == nil {
		return nil, fmt.Errorf("video status response is nil")
	}

	status, err := toBifrostVideoStatus(r.Status)
	if err != nil {
		return nil, err
	}

	response := &schemas.BifrostVideoGenerationResponse{
		Object: "video",
		Status: status,
	}

	if status == schemas.VideoStatusFailed && r.Reason != "" {
		response.Error = &schemas.VideoCreateError{Message: r.Reason}
	}

	if r.Results != nil {
		for _, video := range r.Results.Videos {
			if video.URL == "" {
				continue
			}
			videoURL := video.URL
			response.Videos = append(response.Videos, schemas.VideoOutput{
				Type:        schemas.VideoOutputTypeURL,
				URL:         &videoURL,
				ContentType: "video/mp4",
			})
		}
	}

	return response, nil
}
