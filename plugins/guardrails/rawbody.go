package guardrails

import (
	"bytes"
	"errors"
	"mime"
	"strings"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/tidwall/gjson"
)

// syncRawBody carries the replacements into the bytes a request sends verbatim, then verifies
// that no replaced value survives in them. Two kinds of bytes are involved:
//
//   - a passthrough body, or the raw body a native integration forwards, which only the
//     integration's own rewriter can edit field by field;
//   - a multipart passthrough form, whose fields were already encoded into the streamed body
//     before any plugin ran, so the stream is re-encoded from the rewritten form.
//
// An error means the caller must block: forwarding the original would send the value the caller
// asked to redact.
func (p *Plugin) syncRawBody(ctx *schemas.BifrostContext, req *schemas.BifrostRequest, applied map[string]string) error {
	if req == nil || len(applied) == 0 {
		return nil
	}
	if passthrough := req.PassthroughRequest; passthrough != nil {
		return syncPassthroughBody(ctx, passthrough, applied)
	}
	if useRawBody(ctx) {
		if body := rawBody(req); len(body) > 0 {
			rewritten, err := applyRewriter(ctx, body, applied)
			if err != nil {
				return err
			}
			req.SetRawRequestBody(rewritten)
			if bodyHoldsLiteral(rewritten, applied) {
				return errors.New("a redacted value is still present in the request body sent verbatim")
			}
		}
	}
	return verifyVerbatimItems(req, applied)
}

// syncPassthroughBody rewrites and verifies a passthrough call's outbound body or form.
func syncPassthroughBody(ctx *schemas.BifrostContext, passthrough *schemas.BifrostPassthroughRequest, applied map[string]string) error {
	if passthrough.Form != nil {
		if err := reencodeFormBody(passthrough); err != nil {
			return err
		}
		for _, values := range passthrough.Form.Value {
			for _, value := range values {
				if textHoldsLiteral(value, applied) {
					return errors.New("a redacted value is still present in the multipart passthrough fields")
				}
			}
		}
		return nil
	}
	if passthrough.BodySource != nil {
		return errors.New("the passthrough body is an opaque stream that cannot be rewritten")
	}
	if len(passthrough.Body) == 0 {
		return nil
	}
	rewritten, err := applyRewriter(ctx, passthrough.Body, applied)
	if err != nil {
		return err
	}
	passthrough.Body = rewritten
	if bodyHoldsLiteral(rewritten, applied) {
		return errors.New("a redacted value is still present in the passthrough body")
	}
	return nil
}

// reencodeFormBody rebuilds a rewritten multipart form's streamed body. The transport encoded the
// form before the plugin ran, so the stream it left still holds the original field values; the
// boundary is the one the client sent, which the router keeps in the request's safe headers.
func reencodeFormBody(passthrough *schemas.BifrostPassthroughRequest) error {
	if passthrough.BodySource == nil {
		return nil // sent from the form itself
	}
	mediaType, params, err := mime.ParseMediaType(passthrough.SafeHeaders["content-type"])
	if err != nil || mediaType != "multipart/form-data" || params["boundary"] == "" {
		return errors.New("the multipart passthrough body cannot be re-encoded without its boundary")
	}
	source, err := schemas.NewMultipartFormBodySource(passthrough.Form, params["boundary"], passthrough.FormTypes)
	if err != nil {
		return err
	}
	passthrough.BodySource = source
	return nil
}

// verifyVerbatimItems reports an error when a responses item Bifrost re-emits from preserved
// bytes still holds a replaced value, so a preserved copy that could not be rewritten blocks the
// request instead of carrying the value to the provider.
func verifyVerbatimItems(req *schemas.BifrostRequest, applied map[string]string) error {
	var input []schemas.ResponsesMessage
	switch {
	case req.ResponsesRequest != nil:
		input = req.ResponsesRequest.Input
	case req.CountTokensRequest != nil:
		input = req.CountTokensRequest.Input
	case req.CompactionRequest != nil:
		input = req.CompactionRequest.Input
	}
	for i := range input {
		message := &input[i]
		if !message.IsVerbatimItem() {
			continue
		}
		raw, err := schemas.Marshal(*message)
		if err != nil {
			return err
		}
		if bodyHoldsLiteral(raw, applied) {
			return errors.New("a redacted value is still present in a preserved responses item")
		}
	}
	return nil
}

// applyRewriter hands an owned copy of a raw body to the integration's rewriter, which is the
// only code that knows the native field holding each literal. A missing or failing rewriter is
// an error: the caller must block instead of forwarding the original body.
func applyRewriter(ctx *schemas.BifrostContext, body []byte, applied map[string]string) ([]byte, error) {
	var rewriter schemas.RawRequestBodyTextRewriter
	if ctx != nil {
		rewriter, _ = ctx.Value(schemas.BifrostContextKeyRawRequestBodyTextRewriter).(schemas.RawRequestBodyTextRewriter)
	}
	if rewriter == nil {
		return nil, errors.New("no raw request body rewriter is registered for this integration")
	}
	owned := make([]byte, len(body))
	copy(owned, body)
	return rewriter(owned, applied)
}

// bodyHoldsLiteral reports whether a body still carries a replaced value. A JSON body is read as
// the provider reads it, decoded, so an escaped value cannot hide; any other body is checked whole.
func bodyHoldsLiteral(body []byte, applied map[string]string) bool {
	if len(body) == 0 || len(applied) == 0 {
		return false
	}
	if !gjson.ValidBytes(body) {
		for literal := range applied {
			if literal != "" && bytes.Contains(body, []byte(literal)) {
				return true
			}
		}
		return false
	}
	found := false
	walkJSONStrings(gjson.ParseBytes(body), func(value string) bool {
		if textHoldsLiteral(value, applied) {
			found = true
			return false
		}
		return true
	})
	return found
}

// textHoldsLiteral reports whether one text value still carries a replaced value.
func textHoldsLiteral(text string, applied map[string]string) bool {
	if text == "" {
		return false
	}
	for literal := range applied {
		if literal != "" && strings.Contains(text, literal) {
			return true
		}
	}
	return false
}

// rawBody returns the body a native integration forwards verbatim when the raw-request-body
// context flag is set. Request types that carry no user text (list, retrieve, delete, cancel)
// have none to inspect.
func rawBody(req *schemas.BifrostRequest) []byte {
	switch {
	case req.ChatRequest != nil:
		return req.ChatRequest.RawRequestBody
	case req.TextCompletionRequest != nil:
		return req.TextCompletionRequest.RawRequestBody
	case req.ResponsesRequest != nil:
		return req.ResponsesRequest.RawRequestBody
	case req.CountTokensRequest != nil:
		return req.CountTokensRequest.RawRequestBody
	case req.CompactionRequest != nil:
		return req.CompactionRequest.RawRequestBody
	case req.EmbeddingRequest != nil:
		return req.EmbeddingRequest.RawRequestBody
	case req.RerankRequest != nil:
		return req.RerankRequest.RawRequestBody
	case req.DecisionRequest != nil:
		return req.DecisionRequest.RawRequestBody
	case req.OCRRequest != nil:
		return req.OCRRequest.RawRequestBody
	case req.SpeechRequest != nil:
		return req.SpeechRequest.RawRequestBody
	case req.TranscriptionRequest != nil:
		return req.TranscriptionRequest.RawRequestBody
	case req.ImageGenerationRequest != nil:
		return req.ImageGenerationRequest.RawRequestBody
	case req.ImageEditRequest != nil:
		return req.ImageEditRequest.RawRequestBody
	case req.VideoGenerationRequest != nil:
		return req.VideoGenerationRequest.RawRequestBody
	case req.VideoEditRequest != nil:
		return req.VideoEditRequest.RawRequestBody
	case req.VideoRemixRequest != nil:
		return req.VideoRemixRequest.RawRequestBody
	case req.CachedContentCreateRequest != nil:
		return req.CachedContentCreateRequest.RawRequestBody
	}
	return nil
}
