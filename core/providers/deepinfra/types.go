package deepinfra

// This file holds the wire types for DeepInfra's native surface only. Everything served
// over the OpenAI-compatible surface reuses the shared types in core/providers/openai.

// DeepInfraRerankRequest is the body of a native rerank call to
// POST /v1/inference/{model_name}.
//
// The endpoint takes parallel queries and documents slices; a single query is broadcast
// across every document, which is the shape Bifrost's rerank request always produces.
type DeepInfraRerankRequest struct {
	Queries   []string `json:"queries"`
	Documents []string `json:"documents"`
	// Instruction steers the reranking task. DeepInfra defaults it to
	// "Given a web search query, retrieve relevant passages that answer the query".
	Instruction *string `json:"instruction,omitempty"`

	ExtraParams map[string]interface{} `json:"-"`
}

// GetExtraParams exposes passthrough parameters to the shared request builder.
func (r *DeepInfraRerankRequest) GetExtraParams() map[string]interface{} {
	return r.ExtraParams
}

// DeepInfraRerankResponse is the native rerank response. Scores are relevance
// probabilities in [0, 1], positionally aligned with the request's documents.
type DeepInfraRerankResponse struct {
	Scores      []float64 `json:"scores"`
	InputTokens int       `json:"input_tokens,omitempty"`
}

// DeepInfraTextToSpeechRequest is the body of the native, ElevenLabs-compatible
// POST /v1/text-to-speech/{voice_id}[/stream] call. The voice travels in the path;
// the model travels in the body as model_id.
type DeepInfraTextToSpeechRequest struct {
	Text         string  `json:"text"`
	ModelID      string  `json:"model_id,omitempty"`
	OutputFormat string  `json:"output_format,omitempty"`
	LanguageCode *string `json:"language_code,omitempty"`

	ExtraParams map[string]interface{} `json:"-"`
}

// GetExtraParams exposes passthrough parameters to the shared request builder.
func (r *DeepInfraTextToSpeechRequest) GetExtraParams() map[string]interface{} {
	return r.ExtraParams
}

// DeepInfraVideoGenerationRequest is the body of POST /v1/videos. DeepInfra's video API
// is JSON, unlike OpenAI's multipart video endpoint, so it cannot reuse the shared handler.
type DeepInfraVideoGenerationRequest struct {
	Model          string  `json:"model"`
	Prompt         string  `json:"prompt"`
	NegativePrompt *string `json:"negative_prompt,omitempty"`
	AspectRatio    *string `json:"aspect_ratio,omitempty"`
	Size           *string `json:"size,omitempty"`
	Seconds        *int    `json:"seconds,omitempty"`
	Seed           *int    `json:"seed,omitempty"`
	Style          *string `json:"style,omitempty"`
	// ImageURL drives image-to-video generation.
	ImageURL *string `json:"image_url,omitempty"`

	ExtraParams map[string]interface{} `json:"-"`
}

// GetExtraParams exposes passthrough parameters to the shared request builder.
func (r *DeepInfraVideoGenerationRequest) GetExtraParams() map[string]interface{} {
	return r.ExtraParams
}

// DeepInfraVideoGenerationResponse is the asynchronous job envelope returned by
// POST /v1/videos and GET /v1/videos/{video_id}.
type DeepInfraVideoGenerationResponse struct {
	ID        string `json:"id"`
	Object    string `json:"object"`
	CreatedAt int64  `json:"created_at"`
	Status    string `json:"status"`
	Model     string `json:"model"`
	// Data holds the produced assets once Status reaches a terminal success. DeepInfra
	// documents it only as a list of free-form objects, so the URL is located by probing
	// the conventional keys rather than by a fixed schema.
	Data  []map[string]interface{} `json:"data,omitempty"`
	Error *string                  `json:"error,omitempty"`
}
