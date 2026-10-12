package deepinfra

import (
	"net/url"
	"strconv"
	"strings"

	providerUtils "github.com/maximhq/bifrost/core/providers/utils"
	"github.com/maximhq/bifrost/core/schemas"
)

// VideoGeneration submits a text-to-video (or image-to-video) job to DeepInfra.
//
// DeepInfra's POST /v1/videos takes a JSON body, whereas OpenAI's video endpoint takes
// multipart, so the shared OpenAI video handler cannot serve it. The call is asynchronous:
// it returns a job whose status the caller polls through VideoRetrieve.
func (provider *DeepInfraProvider) VideoGeneration(ctx *schemas.BifrostContext, key schemas.Key, request *schemas.BifrostVideoGenerationRequest) (*schemas.BifrostVideoGenerationResponse, *schemas.BifrostError) {
	if request.Input == nil || request.Input.Prompt == "" {
		return nil, providerUtils.NewBifrostOperationError("video generation requires a prompt", nil)
	}

	jsonBody, bifrostErr := providerUtils.CheckContextAndGetRequestBody(
		ctx,
		request,
		func() (providerUtils.RequestBodyWithExtraParams, error) {
			return toDeepInfraVideoGenerationRequest(request), nil
		})
	if bifrostErr != nil {
		return nil, bifrostErr
	}

	result, bifrostErr := provider.postNativeJSON(ctx, provider.requestURL(ctx, pathVideos), key, jsonBody)
	return provider.buildVideoJobResponse(ctx, result, bifrostErr, jsonBody, request.Model, request.Input.Prompt)
}

// VideoRetrieve polls a previously submitted DeepInfra video job.
func (provider *DeepInfraProvider) VideoRetrieve(ctx *schemas.BifrostContext, key schemas.Key, request *schemas.BifrostVideoRetrieveRequest) (*schemas.BifrostVideoGenerationResponse, *schemas.BifrostError) {
	if request.ID == "" {
		return nil, providerUtils.NewBifrostOperationError("video retrieve requires a video id", nil)
	}

	jobURL := provider.requestURL(ctx, pathVideos) + "/" + url.PathEscape(request.ID)
	result, bifrostErr := provider.getNative(ctx, jobURL, key)
	return provider.buildVideoJobResponse(ctx, result, bifrostErr, nil, "", "")
}

// VideoDownload fetches the rendered asset bytes for a completed DeepInfra video job.
func (provider *DeepInfraProvider) VideoDownload(ctx *schemas.BifrostContext, key schemas.Key, request *schemas.BifrostVideoDownloadRequest) (*schemas.BifrostVideoDownloadResponse, *schemas.BifrostError) {
	if request.ID == "" {
		return nil, providerUtils.NewBifrostOperationError("video download requires a video id", nil)
	}

	contentURL := provider.requestURL(ctx, pathVideos) + "/" + url.PathEscape(request.ID) + "/content"
	if request.Variant != nil && *request.Variant != "" {
		contentURL += "?variant=" + url.QueryEscape(string(*request.Variant))
	}

	result, bifrostErr := provider.getNative(ctx, contentURL, key)
	if result.headers != nil {
		ctx.SetValue(schemas.BifrostContextKeyProviderResponseHeaders, result.headers)
	}
	if bifrostErr != nil {
		return nil, providerUtils.EnrichError(ctx, bifrostErr, nil, nil, provider.sendBackRawRequest, provider.sendBackRawResponse, result.latency)
	}

	return &schemas.BifrostVideoDownloadResponse{
		VideoID:     request.ID,
		Content:     result.body,
		ContentType: result.headers["Content-Type"],
		ExtraFields: schemas.BifrostResponseExtraFields{
			Latency:                 result.latency.Milliseconds(),
			ProviderResponseHeaders: result.headers,
		},
	}, nil
}

// buildVideoJobResponse decodes a DeepInfra job envelope into Bifrost's video response.
// model and prompt backfill the fields DeepInfra echoes only on some responses.
func (provider *DeepInfraProvider) buildVideoJobResponse(ctx *schemas.BifrostContext, result nativeResponse, bifrostErr *schemas.BifrostError, jsonBody []byte, model, prompt string) (*schemas.BifrostVideoGenerationResponse, *schemas.BifrostError) {
	if result.headers != nil {
		ctx.SetValue(schemas.BifrostContextKeyProviderResponseHeaders, result.headers)
	}
	if bifrostErr != nil {
		return nil, providerUtils.EnrichError(ctx, bifrostErr, jsonBody, nil, provider.sendBackRawRequest, provider.sendBackRawResponse, result.latency)
	}

	var job DeepInfraVideoGenerationResponse
	rawRequest, rawResponse, bifrostErr := providerUtils.HandleProviderResponse(
		result.body,
		&job,
		jsonBody,
		providerUtils.ShouldSendBackRawRequest(ctx, provider.sendBackRawRequest),
		providerUtils.ShouldSendBackRawResponse(ctx, provider.sendBackRawResponse),
	)
	if bifrostErr != nil {
		return nil, providerUtils.EnrichError(ctx, bifrostErr, jsonBody, result.body, provider.sendBackRawRequest, provider.sendBackRawResponse, result.latency)
	}

	response := job.ToBifrostVideoGenerationResponse()
	if response.Model == "" {
		response.Model = model
	}
	if response.Prompt == "" {
		response.Prompt = prompt
	}
	response.ExtraFields.Latency = result.latency.Milliseconds()
	response.ExtraFields.ProviderResponseHeaders = result.headers

	if providerUtils.ShouldSendBackRawRequest(ctx, provider.sendBackRawRequest) {
		response.ExtraFields.RawRequest = rawRequest
	}
	if providerUtils.ShouldSendBackRawResponse(ctx, provider.sendBackRawResponse) {
		response.ExtraFields.RawResponse = rawResponse
	}

	return response, nil
}

// toDeepInfraVideoGenerationRequest converts a Bifrost video request into the native body.
func toDeepInfraVideoGenerationRequest(request *schemas.BifrostVideoGenerationRequest) *DeepInfraVideoGenerationRequest {
	body := &DeepInfraVideoGenerationRequest{
		Model:    request.Model,
		Prompt:   request.Input.Prompt,
		ImageURL: request.Input.InputReference,
	}

	if request.Params == nil {
		return body
	}

	body.ExtraParams = request.Params.ExtraParams
	body.NegativePrompt = request.Params.NegativePrompt
	body.Seed = request.Params.Seed
	if request.Params.Size != "" {
		body.Size = schemas.Ptr(request.Params.Size)
	}
	// Bifrost carries duration as a string for providers that accept fractional or
	// unit-suffixed values; DeepInfra takes whole seconds, so a non-integer is dropped
	// rather than sent in a form the API would reject.
	if request.Params.Seconds != nil {
		if seconds, err := strconv.Atoi(*request.Params.Seconds); err == nil {
			body.Seconds = &seconds
		}
	}
	if aspectRatio, ok := request.Params.ExtraParams["aspect_ratio"].(string); ok && aspectRatio != "" {
		body.AspectRatio = schemas.Ptr(aspectRatio)
	}
	if style, ok := request.Params.ExtraParams["style"].(string); ok && style != "" {
		body.Style = schemas.Ptr(style)
	}

	return body
}

// ToBifrostVideoGenerationResponse maps DeepInfra's job envelope onto Bifrost's shape.
func (job *DeepInfraVideoGenerationResponse) ToBifrostVideoGenerationResponse() *schemas.BifrostVideoGenerationResponse {
	response := &schemas.BifrostVideoGenerationResponse{
		ID:        job.ID,
		Object:    "video",
		CreatedAt: job.CreatedAt,
		Model:     job.Model,
		Status:    toBifrostVideoStatus(job.Status),
	}

	// DeepInfra reports failures as a bare string rather than a structured error object.
	if job.Error != nil && *job.Error != "" {
		response.Error = &schemas.VideoCreateError{Message: *job.Error}
		response.Status = schemas.VideoStatusFailed
	}

	for _, item := range job.Data {
		if output, ok := toVideoOutput(item); ok {
			response.Videos = append(response.Videos, output)
		}
	}

	return response
}

// toBifrostVideoStatus normalizes DeepInfra's job status vocabulary. DeepInfra types the
// field as a free-form string, so unrecognized in-flight values fall back to in_progress
// rather than being reported as a terminal state the caller would stop polling on.
func toBifrostVideoStatus(status string) schemas.VideoStatus {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "queued", "pending", "not_started":
		return schemas.VideoStatusQueued
	case "completed", "succeeded", "success", "finished":
		return schemas.VideoStatusCompleted
	case "failed", "error", "cancelled", "canceled":
		return schemas.VideoStatusFailed
	default:
		return schemas.VideoStatusInProgress
	}
}

// toVideoOutput extracts one rendered asset from a job data entry. DeepInfra documents the
// entries only as free-form objects, so the conventional URL and base64 keys are probed in
// turn; an entry carrying neither is skipped rather than emitted as an empty output.
func toVideoOutput(item map[string]interface{}) (schemas.VideoOutput, bool) {
	contentType, _ := item["content_type"].(string)
	if contentType == "" {
		contentType = "video/mp4"
	}
	id, _ := item["id"].(string)

	for _, key := range []string{"url", "video_url", "video"} {
		if value, ok := item[key].(string); ok && value != "" {
			return schemas.VideoOutput{
				ID:          id,
				Type:        schemas.VideoOutputTypeURL,
				URL:         schemas.Ptr(value),
				ContentType: contentType,
			}, true
		}
	}

	for _, key := range []string{"b64_json", "base64", "video_base64"} {
		if value, ok := item[key].(string); ok && value != "" {
			return schemas.VideoOutput{
				ID:          id,
				Type:        schemas.VideoOutputTypeBase64,
				Base64Data:  schemas.Ptr(value),
				ContentType: contentType,
			}, true
		}
	}

	return schemas.VideoOutput{}, false
}
