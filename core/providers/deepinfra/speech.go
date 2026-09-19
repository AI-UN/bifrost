package deepinfra

import (
	"context"
	"io"
	"net/http"
	"time"

	providerUtils "github.com/maximhq/bifrost/core/providers/utils"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/valyala/fasthttp"
)

// SpeechStream synthesizes speech incrementally over DeepInfra's native, ElevenLabs-compatible
// POST /v1/text-to-speech/{voice_id}/stream endpoint.
//
// This is the one audio path that cannot go through the OpenAI-compatible surface:
// /v1/audio/speech accepts no stream parameter and answers with the finished clip in a
// single body, so incremental playback is only reachable natively. The response is a
// chunked raw audio octet stream, not SSE, so the bytes are framed into speech deltas here
// rather than by a shared SSE handler.
func (provider *DeepInfraProvider) SpeechStream(ctx *schemas.BifrostContext, postHookRunner schemas.PostHookRunner, postHookSpanFinalizer func(context.Context), key schemas.Key, request *schemas.BifrostSpeechRequest) (chan *schemas.BifrostStreamChunk, *schemas.BifrostError) {
	if request.Input == nil || request.Input.Input == "" {
		return nil, providerUtils.NewBifrostOperationError("speech input is required", nil)
	}

	jsonBody, bifrostErr := providerUtils.CheckContextAndGetRequestBody(
		ctx,
		request,
		func() (providerUtils.RequestBodyWithExtraParams, error) {
			return toDeepInfraTextToSpeechRequest(request), nil
		})
	if bifrostErr != nil {
		return nil, bifrostErr
	}

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)

	providerUtils.SetExtraHeaders(ctx, req, provider.networkConfig.ExtraHeaders, nil)

	req.SetRequestURI(provider.requestURL(ctx, pathTextToSpeechPrefix+resolveSpeechVoice(request)+"/stream"))
	req.Header.SetMethod(http.MethodPost)
	req.Header.SetContentType("application/json")
	if keyValue := key.Value.GetValue(); keyValue != "" {
		req.Header.Set("Authorization", "Bearer "+keyValue)
	}
	req.SetBody(jsonBody)

	// Stream the body instead of buffering the whole clip before the first chunk.
	resp.StreamBody = true

	startTime := time.Now()
	if err := providerUtils.DoStreamingRequest(ctx, provider.streamingClient, req, resp); err != nil {
		providerUtils.ReleaseStreamingResponse(ctx, resp)
		return nil, providerUtils.EnrichError(
			ctx,
			providerUtils.NewBifrostOperationError(schemas.ErrProviderDoRequest, err),
			jsonBody, nil, provider.sendBackRawRequest, provider.sendBackRawResponse, time.Since(startTime),
		)
	}

	if resp.StatusCode() != fasthttp.StatusOK {
		providerUtils.MaterializeStreamErrorBody(ctx, resp)
		bifrostErr := providerUtils.SetErrorLatency(parseDeepInfraError(resp), time.Since(startTime))
		providerUtils.ReleaseStreamingResponse(ctx, resp)
		return nil, providerUtils.EnrichError(ctx, bifrostErr, jsonBody, nil, provider.sendBackRawRequest, provider.sendBackRawResponse, time.Since(startTime))
	}

	providerUtils.SetStreamIdleTimeoutIfEmpty(ctx, provider.networkConfig.StreamIdleTimeoutInSeconds)
	responseChan := make(chan *schemas.BifrostStreamChunk, schemas.DefaultStreamBufferSize)

	go func() {
		defer func() {
			if ctx.Err() == context.Canceled {
				providerUtils.HandleStreamCancellation(ctx, postHookRunner, responseChan, provider.logger, postHookSpanFinalizer, jsonBody)
			} else if ctx.Err() == context.DeadlineExceeded {
				providerUtils.HandleStreamTimeout(ctx, postHookRunner, responseChan, provider.logger, postHookSpanFinalizer, jsonBody)
			}
			providerUtils.CloseStream(ctx, responseChan)
		}()
		// streamFullyRead selects the release strategy below. It is declared before the
		// release defer so that defer runs after the idle-timeout and cancellation
		// cleanups, never while a timer could still touch the stream.
		streamFullyRead := false
		defer func() {
			if streamFullyRead {
				provider.releaseFullyReadStream(ctx, resp)
				return
			}
			providerUtils.ReleaseStreamingResponse(ctx, resp)
		}()

		reader, releaseGzip := providerUtils.DecompressStreamBody(resp)
		defer releaseGzip()

		// The raw body stream is passed separately so a stalled read can be unblocked by
		// closing the socket underneath any decompression layer.
		reader, stopIdleTimeout := providerUtils.NewIdleTimeoutReader(reader, resp.BodyStream(), providerUtils.GetStreamIdleTimeout(ctx), ctx)
		defer stopIdleTimeout()

		stopCancellation := providerUtils.SetupStreamCancellation(ctx, resp.BodyStream(), provider.logger)
		defer stopCancellation()
		defer providerUtils.EnsureStreamFinalizerCalled(ctx, postHookSpanFinalizer)

		buffer := make([]byte, speechStreamChunkSize)
		chunkIndex := -1
		lastChunkTime := time.Now()

		for {
			if ctx.Err() != nil {
				return
			}

			n, err := reader.Read(buffer)

			// Consume the bytes before inspecting the error: io.Reader is allowed to
			// return a non-zero count together with io.EOF, and dropping that read would
			// silently truncate the tail of the clip.
			if n > 0 {
				chunkIndex++
				audioChunk := make([]byte, n)
				copy(audioChunk, buffer[:n])

				response := &schemas.BifrostSpeechStreamResponse{
					Type:  schemas.SpeechStreamResponseTypeDelta,
					Audio: audioChunk,
					ExtraFields: schemas.BifrostResponseExtraFields{
						ChunkIndex: chunkIndex,
						Latency:    time.Since(lastChunkTime).Milliseconds(),
					},
				}
				lastChunkTime = time.Now()

				if providerUtils.ShouldSendBackRawResponse(ctx, provider.sendBackRawResponse) {
					response.ExtraFields.RawResponse = audioChunk
				}

				providerUtils.ProcessAndSendResponse(ctx, postHookRunner, providerUtils.GetBifrostResponseForStreamResponse(nil, nil, nil, response, nil, nil), responseChan, postHookSpanFinalizer)
			}

			if err != nil {
				if ctx.Err() != nil {
					return
				}
				if err == io.EOF {
					streamFullyRead = true
					break
				}
				ctx.SetValue(schemas.BifrostContextKeyStreamEndIndicator, true)
				provider.logger.Warn("Error reading DeepInfra speech stream: %v", err)
				providerUtils.ProcessAndSendError(ctx, postHookRunner, err, responseChan, provider.logger, postHookSpanFinalizer)
				return
			}
		}

		finalResponse := &schemas.BifrostSpeechStreamResponse{
			Type:  schemas.SpeechStreamResponseTypeDone,
			Audio: []byte{},
			ExtraFields: schemas.BifrostResponseExtraFields{
				ChunkIndex: chunkIndex + 1,
				Latency:    time.Since(startTime).Milliseconds(),
			},
		}
		if providerUtils.ShouldSendBackRawRequest(ctx, provider.sendBackRawRequest) {
			providerUtils.ParseAndSetRawRequest(&finalResponse.ExtraFields, jsonBody)
		}

		ctx.SetValue(schemas.BifrostContextKeyStreamEndIndicator, true)
		providerUtils.ProcessAndSendResponse(ctx, postHookRunner, providerUtils.GetBifrostResponseForStreamResponse(nil, nil, nil, finalResponse, nil, nil), responseChan, postHookSpanFinalizer)
	}()

	return responseChan, nil
}

// resolveSpeechVoice picks the voice id for the native endpoint's path segment.
// Multi-speaker configs have no native equivalent, so only a single voice is honoured.
func resolveSpeechVoice(request *schemas.BifrostSpeechRequest) string {
	if request.Params != nil && request.Params.VoiceConfig != nil && request.Params.VoiceConfig.Voice != nil {
		if voice := *request.Params.VoiceConfig.Voice; voice != "" {
			return voice
		}
	}
	return defaultSpeechStreamVoice
}

// toDeepInfraTextToSpeechRequest converts a Bifrost speech request into the native body.
// The voice is not included: it travels in the URL path.
func toDeepInfraTextToSpeechRequest(request *schemas.BifrostSpeechRequest) *DeepInfraTextToSpeechRequest {
	body := &DeepInfraTextToSpeechRequest{
		Text:    request.Input.Input,
		ModelID: request.Model,
	}

	if request.Params != nil {
		body.ExtraParams = request.Params.ExtraParams
		body.LanguageCode = request.Params.LanguageCode
		// An unrepresentable format is dropped rather than forwarded, so that asking for an
		// OpenAI-only container falls back to DeepInfra's default instead of being rejected.
		if format, ok := normalizeTTSOutputFormat(request.Params.ResponseFormat); ok {
			body.OutputFormat = format
		}
	}

	return body
}

// releaseFullyReadStream returns a response whose chunked body has already been consumed to
// EOF back to the pool.
//
// providerUtils.ReleaseStreamingResponse drains before releasing, which is right for the SSE
// callers it was written for: they stop on a terminal marker with bytes still buffered, and
// the leftovers must be flushed before the connection can be reused. The raw-audio loop is
// the opposite case — it reads the body all the way to EOF, and a fasthttp requestStream
// read past EOF re-enters chunk-header parsing. On a keep-alive connection that header never
// arrives, so the drain blocks forever and the stream channel is never closed.
//
// Claiming the shared connection-closed flag makes the deferred ReleaseStreamingResponse
// return early instead of draining, so the stream is detached and released exactly once here.
func (provider *DeepInfraProvider) releaseFullyReadStream(ctx *schemas.BifrostContext, resp *fasthttp.Response) {
	if resp == nil {
		return
	}
	// Lost the race to a cancellation or idle-timeout closer: it owns the release.
	if prev, _ := ctx.GetAndSetValue(schemas.BifrostContextKeyConnectionClosed, true).(bool); prev {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			provider.logger.Debug("DeepInfra speech stream already closed before release: %v", r)
		}
	}()
	resp.CloseBodyStream()
	fasthttp.ReleaseResponse(resp)
}
