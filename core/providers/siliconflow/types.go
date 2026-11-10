package siliconflow

import (
	"encoding/json"
)

// SiliconFlowError models SiliconFlow's native error envelope
// (`{"code": ..., "message": ..., "data": ...}`).
type SiliconFlowError struct {
	Code    *json.Number    `json:"code,omitempty"`
	Message string          `json:"message,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// SiliconFlowRerankRequest is the request body for SiliconFlow's /v1/rerank endpoint.
// SiliconFlow accepts documents as an array of strings only.
type SiliconFlowRerankRequest struct {
	Model           string   `json:"model"`
	Query           string   `json:"query"`
	Documents       []string `json:"documents"`
	TopN            *int     `json:"top_n,omitempty"`
	ReturnDocuments *bool    `json:"return_documents,omitempty"`

	ExtraParams map[string]interface{} `json:"-"`
}

// GetExtraParams satisfies providerUtils.RequestBodyWithExtraParams.
func (r *SiliconFlowRerankRequest) GetExtraParams() map[string]interface{} {
	return r.ExtraParams
}

// SiliconFlowRerankTokens represents rerank token usage counts.
type SiliconFlowRerankTokens struct {
	InputTokens  *int64 `json:"input_tokens,omitempty"`
	OutputTokens *int64 `json:"output_tokens,omitempty"`
}

// SiliconFlowRerankMeta wraps the China-region rerank usage envelope (`meta.tokens`).
type SiliconFlowRerankMeta struct {
	Tokens *SiliconFlowRerankTokens `json:"tokens,omitempty"`
}

// SiliconFlowRerankResult is a single reranked document entry. Document is a
// string/object union depending on region and return_documents.
type SiliconFlowRerankResult struct {
	Index          int             `json:"index"`
	RelevanceScore float64         `json:"relevance_score"`
	Document       json.RawMessage `json:"document,omitempty"`
}

// SiliconFlowRerankResponse accepts both the international top-level `tokens`
// usage and the China-region `meta.tokens` usage.
type SiliconFlowRerankResponse struct {
	ID      string                    `json:"id,omitempty"`
	Results []SiliconFlowRerankResult `json:"results"`
	Tokens  *SiliconFlowRerankTokens  `json:"tokens,omitempty"`
	Meta    *SiliconFlowRerankMeta    `json:"meta,omitempty"`
}

// SiliconFlowImageRequest is the request body for /v1/images/generations
// (shared by generation and edit; edit populates exactly one image field).
type SiliconFlowImageRequest struct {
	Model             string  `json:"model"`
	Prompt            string  `json:"prompt"`
	NegativePrompt    *string `json:"negative_prompt,omitempty"`
	ImageSize         *string `json:"image_size,omitempty"`
	BatchSize         *int    `json:"batch_size,omitempty"`
	Seed              *int    `json:"seed,omitempty"`
	NumInferenceSteps *int    `json:"num_inference_steps,omitempty"`
	OutputFormat      *string `json:"output_format,omitempty"`
	AspectRatio       *string `json:"aspect_ratio,omitempty"`
	Image             *string `json:"image,omitempty"`        // Qwen-Image-Edit, FLUX.1-Kontext-dev
	InputImage        *string `json:"input_image,omitempty"`  // FLUX.1-Kontext-pro/max
	ImagePrompt       *string `json:"image_prompt,omitempty"` // FLUX-1.1-pro / FLUX-1.1-pro-Ultra

	ExtraParams map[string]interface{} `json:"-"`
}

// GetExtraParams satisfies providerUtils.RequestBodyWithExtraParams.
func (r *SiliconFlowImageRequest) GetExtraParams() map[string]interface{} {
	return r.ExtraParams
}

// SiliconFlowImageData is a single generated image entry.
type SiliconFlowImageData struct {
	URL string `json:"url"`
}

// SiliconFlowImageTimings carries generation timing metadata.
type SiliconFlowImageTimings struct {
	Inference *float64 `json:"inference,omitempty"`
}

// SiliconFlowImageResponse is the response body for /v1/images/generations.
type SiliconFlowImageResponse struct {
	Images  []SiliconFlowImageData   `json:"images"`
	Timings *SiliconFlowImageTimings `json:"timings,omitempty"`
	Seed    *int                     `json:"seed,omitempty"`
}

// SiliconFlowSpeechRequest is the request body for /v1/audio/speech.
// Only officially documented fields are typed; sample_rate, gain, and
// references pass through ExtraParams.
type SiliconFlowSpeechRequest struct {
	Model          string   `json:"model"`
	Input          string   `json:"input"`
	Voice          *string  `json:"voice,omitempty"`
	ResponseFormat *string  `json:"response_format,omitempty"`
	Speed          *float64 `json:"speed,omitempty"`
	Stream         bool     `json:"stream"`

	ExtraParams map[string]interface{} `json:"-"`
}

// GetExtraParams satisfies providerUtils.RequestBodyWithExtraParams.
func (r *SiliconFlowSpeechRequest) GetExtraParams() map[string]interface{} {
	return r.ExtraParams
}

// SiliconFlowFileObject is a file entry returned by /v1/files. The upload
// response uses camelCase `createdAt` while the list response uses
// snake_case `created_at`; both are accepted.
type SiliconFlowFileObject struct {
	ID             string `json:"id"`
	Object         string `json:"object,omitempty"`
	Bytes          int64  `json:"bytes,omitempty"`
	CreatedAt      int64  `json:"created_at,omitempty"`
	CreatedAtCamel int64  `json:"createdAt,omitempty"`
	Filename       string `json:"filename,omitempty"`
	Purpose        string `json:"purpose,omitempty"`
	Status         string `json:"status,omitempty"`
}

// createdAtUnix returns whichever timestamp variant the upstream populated.
func (f *SiliconFlowFileObject) createdAtUnix() int64 {
	if f.CreatedAt != 0 {
		return f.CreatedAt
	}
	return f.CreatedAtCamel
}

// SiliconFlowFileListResponse is the response body for GET /v1/files.
type SiliconFlowFileListResponse struct {
	Object  string                  `json:"object,omitempty"`
	Data    []SiliconFlowFileObject `json:"data"`
	HasMore bool                    `json:"has_more,omitempty"`
}

// SiliconFlowBatchReplace carries SiliconFlow's batch-level model replacement.
type SiliconFlowBatchReplace struct {
	Model string `json:"model"`
}

// SiliconFlowBatchRequest is the request body for POST /v1/batches.
type SiliconFlowBatchRequest struct {
	InputFileID      string                   `json:"input_file_id"`
	Endpoint         string                   `json:"endpoint"`
	CompletionWindow string                   `json:"completion_window"`
	Metadata         map[string]string        `json:"metadata,omitempty"`
	Replace          *SiliconFlowBatchReplace `json:"replace,omitempty"`

	ExtraParams map[string]interface{} `json:"-"`
}

// GetExtraParams satisfies providerUtils.RequestBodyWithExtraParams.
func (r *SiliconFlowBatchRequest) GetExtraParams() map[string]interface{} {
	return r.ExtraParams
}

// SiliconFlowBatchRequestCounts tracks the counts of requests in a batch.
type SiliconFlowBatchRequestCounts struct {
	Total     int `json:"total"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
}

// SiliconFlowBatchResponse is a batch object returned by /v1/batches.
// Errors is `null` or a plain string array on the wire; a nil slice
// represents `null` directly.
type SiliconFlowBatchResponse struct {
	ID               string                         `json:"id"`
	Object           string                         `json:"object,omitempty"`
	Endpoint         string                         `json:"endpoint,omitempty"`
	Errors           []string                       `json:"errors,omitempty"`
	InputFileID      string                         `json:"input_file_id,omitempty"`
	CompletionWindow string                         `json:"completion_window,omitempty"`
	Status           string                         `json:"status,omitempty"`
	OutputFileID     *string                        `json:"output_file_id,omitempty"`
	ErrorFileID      *string                        `json:"error_file_id,omitempty"`
	CreatedAt        int64                          `json:"created_at,omitempty"`
	InProgressAt     *int64                         `json:"in_progress_at,omitempty"`
	ExpiresAt        *int64                         `json:"expires_at,omitempty"`
	FinalizingAt     *int64                         `json:"finalizing_at,omitempty"`
	CompletedAt      *int64                         `json:"completed_at,omitempty"`
	FailedAt         *int64                         `json:"failed_at,omitempty"`
	ExpiredAt        *int64                         `json:"expired_at,omitempty"`
	CancellingAt     *int64                         `json:"cancelling_at,omitempty"`
	CancelledAt      *int64                         `json:"cancelled_at,omitempty"`
	RequestCounts    *SiliconFlowBatchRequestCounts `json:"request_counts,omitempty"`
	Metadata         map[string]string              `json:"metadata,omitempty"`
}

// SiliconFlowBatchListResponse is the response body for GET /v1/batches.
type SiliconFlowBatchListResponse struct {
	Object  string                     `json:"object,omitempty"`
	Data    []SiliconFlowBatchResponse `json:"data"`
	FirstID *string                    `json:"first_id,omitempty"`
	LastID  *string                    `json:"last_id,omitempty"`
	HasMore bool                       `json:"has_more,omitempty"`
}

// SiliconFlowVideoSubmitRequest is the request body for POST /v1/video/submit.
type SiliconFlowVideoSubmitRequest struct {
	Model          string  `json:"model"`
	Prompt         string  `json:"prompt"`
	ImageSize      *string `json:"image_size,omitempty"`
	NegativePrompt *string `json:"negative_prompt,omitempty"`
	Image          *string `json:"image,omitempty"`
	Seed           *int    `json:"seed,omitempty"`

	ExtraParams map[string]interface{} `json:"-"`
}

// GetExtraParams satisfies providerUtils.RequestBodyWithExtraParams.
func (r *SiliconFlowVideoSubmitRequest) GetExtraParams() map[string]interface{} {
	return r.ExtraParams
}

// SiliconFlowVideoSubmitResponse is the response body for POST /v1/video/submit.
type SiliconFlowVideoSubmitResponse struct {
	RequestID string `json:"requestId"`
}

// SiliconFlowVideoStatusRequest is the request body for POST /v1/video/status.
type SiliconFlowVideoStatusRequest struct {
	RequestID string `json:"requestId"`
}

// SiliconFlowVideoOutput is a single generated video entry.
type SiliconFlowVideoOutput struct {
	URL string `json:"url"`
}

// SiliconFlowVideoResults carries generated videos and metadata.
type SiliconFlowVideoResults struct {
	Videos []SiliconFlowVideoOutput `json:"videos,omitempty"`
	Seed   *int                     `json:"seed,omitempty"`
}

// SiliconFlowVideoStatusResponse is the response body for POST /v1/video/status.
type SiliconFlowVideoStatusResponse struct {
	Status  string                   `json:"status"`
	Reason  string                   `json:"reason,omitempty"`
	Results *SiliconFlowVideoResults `json:"results,omitempty"`
}
