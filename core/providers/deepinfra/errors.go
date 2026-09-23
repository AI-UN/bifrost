package deepinfra

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bytedance/sonic"
	providerUtils "github.com/maximhq/bifrost/core/providers/utils"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/valyala/fasthttp"
)

// DeepInfraErrorResponse captures the three error envelopes DeepInfra emits:
//
//   - {"error": "Model not found"} from the native inference surface (DeepError).
//   - {"error": {"message": ..., "type": ..., "code": ...}} from the OpenAI-compatible surface.
//   - {"detail": [{"loc": [...], "msg": ..., "type": ...}]} from request validation, which is
//     also sometimes a bare string.
//
// Error is kept raw because the same key carries both a string and an object depending on
// which surface rejected the request.
type DeepInfraErrorResponse struct {
	Error  json.RawMessage `json:"error,omitempty"`
	Detail json.RawMessage `json:"detail,omitempty"`
}

// deepInfraValidationError mirrors FastAPI's ValidationError entry.
type deepInfraValidationError struct {
	Loc  []any  `json:"loc,omitempty"`
	Msg  string `json:"msg,omitempty"`
	Type string `json:"type,omitempty"`
}

// parseDeepInfraError normalizes a non-2xx DeepInfra response into a BifrostError.
// It satisfies openai.ErrorConverter so it can be injected into the shared OpenAI handlers.
func parseDeepInfraError(resp *fasthttp.Response) *schemas.BifrostError {
	var errorResp DeepInfraErrorResponse
	bifrostErr := providerUtils.HandleProviderAPIError(resp, &errorResp)
	if bifrostErr == nil {
		return nil
	}

	if bifrostErr.Error == nil {
		bifrostErr.Error = &schemas.ErrorField{}
	}

	applyDeepInfraErrorField(bifrostErr, errorResp.Error)
	if strings.TrimSpace(bifrostErr.Error.Message) == "" {
		if msg := parseDeepInfraDetail(errorResp.Detail); msg != "" {
			bifrostErr.Error.Message = msg
			if bifrostErr.Error.Type == nil {
				errorType := schemas.Ptr("invalid_request_error")
				bifrostErr.Error.Type = errorType
				bifrostErr.Type = errorType
			}
		}
	}

	if strings.TrimSpace(bifrostErr.Error.Message) == "" {
		if bifrostErr.StatusCode != nil {
			bifrostErr.Error.Message = fmt.Sprintf("provider API error (status %d)", *bifrostErr.StatusCode)
		} else {
			bifrostErr.Error.Message = "provider API error"
		}
	}

	return bifrostErr
}

// applyDeepInfraErrorField copies the "error" member onto the BifrostError, accepting
// either the native string form or the OpenAI-style object form.
func applyDeepInfraErrorField(bifrostErr *schemas.BifrostError, raw json.RawMessage) {
	if len(raw) == 0 {
		return
	}

	var message string
	if err := sonic.Unmarshal(raw, &message); err == nil {
		if strings.TrimSpace(message) != "" {
			bifrostErr.Error.Message = message
		}
		return
	}

	var nested schemas.ErrorField
	if err := sonic.Unmarshal(raw, &nested); err != nil {
		return
	}
	if strings.TrimSpace(nested.Message) != "" {
		bifrostErr.Error.Message = nested.Message
	}
	if nested.Type != nil && strings.TrimSpace(*nested.Type) != "" {
		bifrostErr.Error.Type = nested.Type
		bifrostErr.Type = nested.Type
	}
	if nested.Code != nil && strings.TrimSpace(*nested.Code) != "" {
		bifrostErr.Error.Code = nested.Code
	}
	if nested.Param != nil {
		bifrostErr.Error.Param = nested.Param
	}
	if nested.EventID != nil {
		bifrostErr.Error.EventID = nested.EventID
	}
}

// parseDeepInfraDetail renders a FastAPI validation payload into a single message.
// The payload is either a bare string or an array of per-field errors.
func parseDeepInfraDetail(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	var message string
	if err := sonic.Unmarshal(raw, &message); err == nil {
		return strings.TrimSpace(message)
	}

	var entries []deepInfraValidationError
	if err := sonic.Unmarshal(raw, &entries); err != nil {
		return ""
	}

	parts := make([]string, 0, len(entries))
	for _, entry := range entries {
		if strings.TrimSpace(entry.Msg) == "" {
			continue
		}
		if field := validationErrorField(entry.Loc); field != "" {
			parts = append(parts, field+": "+entry.Msg)
			continue
		}
		parts = append(parts, entry.Msg)
	}
	return strings.Join(parts, "; ")
}

// validationErrorField renders FastAPI's "loc" tuple as a dotted field path.
func validationErrorField(loc []any) string {
	parts := make([]string, 0, len(loc))
	for _, item := range loc {
		switch value := item.(type) {
		case string:
			if value != "body" && value != "query" && value != "path" {
				parts = append(parts, value)
			}
		case float64:
			parts = append(parts, fmt.Sprintf("%d", int(value)))
		}
	}
	return strings.Join(parts, ".")
}
