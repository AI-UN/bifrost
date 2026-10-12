package utils

import (
	"context"

	"github.com/maximhq/bifrost/core/schemas"
)

// ResponsesStreamFallback re-assembles a Responses stream from the chat chunks a provider
// emits after core downgraded a ResponsesStream request to ChatCompletionStream.
//
// Core leaves that re-assembly to the provider: OpenAI-shaped chat streams get it inside
// HandleOpenAIChatCompletionStreaming, providers with a native chat stream use this type.
//
// A nil *ResponsesStreamFallback means the request was not downgraded; every method is
// nil-safe so call sites need a single branch.
type ResponsesStreamFallback struct {
	state *schemas.ChatToResponsesStreamState
}

// NewResponsesStreamFallback returns a converter when ctx carries the downgrade flag, and
// nil otherwise. Callers must Release the result.
func NewResponsesStreamFallback(ctx *schemas.BifrostContext) *ResponsesStreamFallback {
	if ctx == nil {
		return nil
	}
	enabled, ok := ctx.Value(schemas.BifrostContextKeyIsResponsesToChatCompletionFallback).(bool)
	if !ok || !enabled {
		return nil
	}
	return &ResponsesStreamFallback{state: schemas.AcquireChatToResponsesStreamState()}
}

// Active reports whether chunks must be converted before being sent.
func (f *ResponsesStreamFallback) Active() bool {
	return f != nil
}

// Release returns the pooled conversion state. Safe to call on a nil receiver and more
// than once.
func (f *ResponsesStreamFallback) Release() {
	if f == nil || f.state == nil {
		return
	}
	schemas.ReleaseChatToResponsesStreamState(f.state)
	f.state = nil
}

// Send converts one chat chunk into its Responses events and forwards each of them.
// Conversion happens before the post-hooks run, matching the OpenAI path, so accumulation,
// logging, caching and cost see the same events the client does. It owns the send because
// one chat chunk spreads into several events that each need their own index.
//
// Returns false when the stream produced a terminal error and must stop; the error has
// already been reported on responseChan.
func (f *ResponsesStreamFallback) Send(
	ctx *schemas.BifrostContext,
	postHookRunner schemas.PostHookRunner,
	chunk *schemas.BifrostChatResponse,
	responseChan chan *schemas.BifrostStreamChunk,
	logger schemas.Logger,
	postHookSpanFinalizer func(context.Context),
	isLastChunk bool,
) bool {
	if f == nil || chunk == nil {
		return true
	}

	events := chunk.ToBifrostResponsesStreamResponse(f.state)
	for i, event := range events {
		if event == nil {
			continue
		}
		if event.Type == schemas.ResponsesStreamResponseTypeError {
			bifrostErr := &schemas.BifrostError{
				Type:           schemas.Ptr(string(schemas.ResponsesStreamResponseTypeError)),
				IsBifrostError: false,
				Error:          &schemas.ErrorField{},
			}
			if event.Message != nil {
				bifrostErr.Error.Message = *event.Message
			}
			if event.Param != nil {
				bifrostErr.Error.Param = *event.Param
			}
			if event.Code != nil {
				bifrostErr.Error.Code = event.Code
			}
			ctx.SetValue(schemas.BifrostContextKeyStreamEndIndicator, true)
			ProcessAndSendBifrostError(ctx, postHookRunner, bifrostErr, responseChan, logger, postHookSpanFinalizer)
			return false
		}

		event.ExtraFields = chunk.ExtraFields
		event.ExtraFields.ChunkIndex = event.SequenceNumber
		// These events are synthesized, not decoded from an upstream Responses frame.
		// Carrying the chat chunk's raw body over would make the Anthropic route's
		// Claude Code passthrough forward it verbatim, emitting content block indices the
		// client never opened.
		event.ExtraFields.RawResponse = nil

		// Only the last event may carry the end indicator: setting it earlier finalizes the
		// deferred span while events are still being written.
		if isLastChunk && i == len(events)-1 {
			ctx.SetValue(schemas.BifrostContextKeyStreamEndIndicator, true)
		}

		ProcessAndSendResponse(ctx, postHookRunner, GetBifrostResponseForStreamResponse(nil, nil, event, nil, nil, nil), responseChan, postHookSpanFinalizer)
	}

	return true
}
