package siliconflow

import (
	"fmt"
	"maps"
	"strings"

	"github.com/maximhq/bifrost/core/schemas"
)

// siliconFlowSpeechReservedKeys are wire fields owned by the typed request;
// extra params must not override them.
var siliconFlowSpeechReservedKeys = map[string]struct{}{
	"model":           {},
	"input":           {},
	"voice":           {},
	"response_format": {},
	"speed":           {},
	"stream":          {},
}

// ToSiliconFlowSpeechRequest converts a Bifrost speech request to SiliconFlow
// format, enforcing per-model-family voice/reference rules:
//   - fishaudio/fish-speech-*: a non-empty string voice is required and
//     `references` extra params are rejected.
//   - FunAudioLLM/CosyVoice*: exactly one of a non-empty string voice or a
//     non-empty `references` extra param must be provided.
//
// Unknown TTS models are rejected. Only officially documented fields are
// sent; extras like sample_rate, gain, and references pass through
// ExtraParams.
func ToSiliconFlowSpeechRequest(request *schemas.BifrostSpeechRequest, stream bool) (*SiliconFlowSpeechRequest, error) {
	if request == nil || request.Input == nil || strings.TrimSpace(request.Input.Input) == "" {
		return nil, fmt.Errorf("speech input text is required")
	}
	if strings.TrimSpace(request.Model) == "" {
		return nil, fmt.Errorf("speech model is required")
	}

	var voice *string
	var extraParams map[string]interface{}
	if params := request.Params; params != nil {
		if params.VoiceConfig != nil {
			if len(params.VoiceConfig.MultiVoiceConfig) > 0 {
				return nil, fmt.Errorf("SiliconFlow speech does not support multi-voice configuration")
			}
			if params.VoiceConfig.Voice != nil && strings.TrimSpace(*params.VoiceConfig.Voice) != "" {
				voice = params.VoiceConfig.Voice
			}
		}
		if len(params.ExtraParams) > 0 {
			extraParams = make(map[string]interface{}, len(params.ExtraParams))
			maps.Copy(extraParams, params.ExtraParams)
			for key := range siliconFlowSpeechReservedKeys {
				delete(extraParams, key)
			}
		}
	}

	hasReferences := siliconFlowSpeechHasReferences(extraParams)

	switch {
	case strings.HasPrefix(request.Model, "fishaudio/fish-speech"):
		if voice == nil {
			return nil, fmt.Errorf("model %s requires a string voice", request.Model)
		}
		if hasReferences {
			return nil, fmt.Errorf("model %s does not support voice references", request.Model)
		}
	case strings.HasPrefix(request.Model, "FunAudioLLM/CosyVoice"):
		if voice != nil && hasReferences {
			return nil, fmt.Errorf("model %s accepts either a string voice or references, not both", request.Model)
		}
		if voice == nil && !hasReferences {
			return nil, fmt.Errorf("model %s requires either a string voice or references", request.Model)
		}
	default:
		return nil, fmt.Errorf("model %s is not a supported SiliconFlow speech model", request.Model)
	}

	converted := &SiliconFlowSpeechRequest{
		Model:  request.Model,
		Input:  request.Input.Input,
		Voice:  voice,
		Stream: stream,
	}
	if params := request.Params; params != nil {
		if params.ResponseFormat != "" {
			converted.ResponseFormat = &params.ResponseFormat
		}
		converted.Speed = params.Speed
	}
	if len(extraParams) > 0 {
		converted.ExtraParams = extraParams
	}

	return converted, nil
}

// siliconFlowSpeechHasReferences reports whether extra params carry a
// non-empty `references` payload.
func siliconFlowSpeechHasReferences(extraParams map[string]interface{}) bool {
	raw, ok := extraParams["references"]
	if !ok || raw == nil {
		return false
	}
	if list, ok := raw.([]interface{}); ok {
		return len(list) > 0
	}
	return true
}
