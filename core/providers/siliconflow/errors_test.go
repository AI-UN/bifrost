package siliconflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func TestParseSiliconFlowErrorEnvelopes(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		message    string
		code       *string
		errorType  *string
	}{
		{
			name:       "OpenAI envelope",
			statusCode: fasthttp.StatusBadRequest,
			body:       `{"error":{"message":"model not found","type":"invalid_request_error","param":"model","code":"model_not_found"}}`,
			message:    "model not found",
			code:       stringPointerForSFErrorTest("model_not_found"),
			errorType:  stringPointerForSFErrorTest("invalid_request_error"),
		},
		{
			name:       "SiliconFlow native envelope",
			statusCode: fasthttp.StatusTooManyRequests,
			body:       `{"code":30002,"message":"rate limit exceeded","data":null}`,
			message:    "rate limit exceeded",
			code:       stringPointerForSFErrorTest("30002"),
		},
		{
			name:       "invalid JSON fallback",
			statusCode: fasthttp.StatusBadGateway,
			body:       `upstream unavailable`,
			message:    "provider API error: upstream unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var response fasthttp.Response
			response.SetStatusCode(tt.statusCode)
			response.SetBodyString(tt.body)

			got := parseSiliconFlowError(&response)

			require.NotNil(t, got)
			require.NotNil(t, got.StatusCode)
			assert.Equal(t, tt.statusCode, *got.StatusCode)
			require.NotNil(t, got.Error)
			assert.Equal(t, tt.message, got.Error.Message)
			assert.Equal(t, tt.code, got.Error.Code)
			assert.Equal(t, tt.errorType, got.Error.Type)
		})
	}
}

func stringPointerForSFErrorTest(value string) *string {
	return &value
}
