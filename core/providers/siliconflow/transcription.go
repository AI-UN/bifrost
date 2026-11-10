package siliconflow

import (
	"github.com/maximhq/bifrost/core/schemas"
)

// prepareSiliconFlowTranscriptionRequest copies the request and clears every
// OpenAI transcription parameter that SiliconFlow does not document, so only
// the multipart `file` and `model` fields reach the upstream. The caller's
// request is never mutated.
func prepareSiliconFlowTranscriptionRequest(request *schemas.BifrostTranscriptionRequest) *schemas.BifrostTranscriptionRequest {
	if request == nil || request.Params == nil {
		return request
	}
	requestCopy := *request
	requestCopy.Params = nil
	return &requestCopy
}
