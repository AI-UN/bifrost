package guardrails

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/tidwall/gjson"
)

// errUnsupportedShape marks a request whose provider-bound text this build cannot reach: a body
// it cannot decode, or a body that is an opaque stream. An enforcing profile must block such a
// request instead of sending text nothing inspected.
var errUnsupportedShape = errors.New("request shape is not inspectable")

// segmentSet is the provider-bound text of one request. Typed request fields are addressed in
// place, so a rewrite lands on what the provider sends. A serialized body (a passthrough body
// and the raw body a native integration forwards) is held as bytes: only the integration's own
// rewriter knows which native field holds each value, so rawbody.go rewrites and verifies those.
type segmentSet struct {
	texts    []*string                   // string fields of the typed request
	dynamic  []any                       // forwarded JSON subtrees: extra params, inline batch bodies, cached content
	scalars  []*any                      // any-typed fields, which may hold a bare string or a subtree
	verbatim []*schemas.ResponsesMessage // responses items Bifrost re-emits from preserved bytes
	body     []byte                      // passthrough body, a JSON document
	raw      []byte                      // raw request body the provider sends verbatim
}

// collectSegments returns the guardrail-visible text of a request. Field coverage follows the
// fork design: chat messages (any role, so system and developer prompts included) with their
// tool-call arguments and replayed reasoning, responses instructions and items with their text
// blocks, tool arguments/outputs and replayed reasoning, text-completion prompts, embedding text
// parts, rerank queries and documents, decisions, image and video prompts, speech input, cached
// content, inline batch bodies, and the caller's extra parameters. A passthrough body is held
// decoded as well. Media bytes, file ids, encrypted reasoning and provider state are not text.
//
// A text-bearing shape this build cannot traverse returns errUnsupportedShape, which an enforcing
// profile must turn into a block. A request type with no user text returns an empty set.
func collectSegments(ctx *schemas.BifrostContext, req *schemas.BifrostRequest) (*segmentSet, error) {
	if req == nil {
		return nil, nil
	}
	s := &segmentSet{}
	switch {
	case req.ChatRequest != nil:
		chat := req.ChatRequest
		for i := range chat.Input {
			s.appendChatMessage(&chat.Input[i])
		}
		if params := chat.Params; params != nil {
			if params.Metadata != nil {
				s.addDynamic(*params.Metadata)
			}
			if params.Prediction != nil {
				s.addScalar(&params.Prediction.Content)
			}
			s.addDynamic(params.ExtraParams)
		}
	case req.ResponsesRequest != nil:
		s.appendResponsesRequest(req.ResponsesRequest)
	case req.CountTokensRequest != nil:
		s.appendResponsesRequest(req.CountTokensRequest)
	case req.CompactionRequest != nil:
		compaction := req.CompactionRequest
		s.appendResponsesMessages(compaction.Input)
		if compaction.Instructions != nil {
			s.texts = append(s.texts, compaction.Instructions)
		}
		s.addDynamic(compaction.ExtraParams)
	case req.TextCompletionRequest != nil:
		completion := req.TextCompletionRequest
		if input := completion.Input; input != nil {
			if input.PromptStr != nil {
				s.texts = append(s.texts, input.PromptStr)
			}
			for i := range input.PromptArray {
				s.texts = append(s.texts, &input.PromptArray[i])
			}
		}
		if completion.Params != nil {
			s.addDynamic(completion.Params.ExtraParams)
		}
	case req.EmbeddingRequest != nil:
		embedding := req.EmbeddingRequest
		for i := range embedding.Input {
			item := &embedding.Input[i]
			for j := range item.Content {
				if item.Content[j].Text != nil {
					s.texts = append(s.texts, item.Content[j].Text)
				}
			}
			if item.Params != nil {
				s.addDynamic(item.Params.ExtraParams)
			}
		}
		if embedding.Params != nil {
			s.addDynamic(embedding.Params.ExtraParams)
		}
	case req.RerankRequest != nil:
		rerank := req.RerankRequest
		s.texts = append(s.texts, &rerank.Query)
		for i := range rerank.Documents {
			document := &rerank.Documents[i]
			s.texts = append(s.texts, &document.Text)
			s.addDynamic(document.Data)
			s.addDynamic(document.Meta)
		}
		if rerank.Params != nil {
			s.addDynamic(rerank.Params.ExtraParams)
		}
	case req.DecisionRequest != nil:
		decision := req.DecisionRequest
		s.appendDecisionInput(&decision.Input)
		for i := range decision.Questions {
			question := &decision.Questions[i]
			s.appendDecisionText(question.Instructions)
			if criteria := question.Criteria; criteria != nil {
				s.appendDecisionText(criteria.True)
				s.appendDecisionText(criteria.False)
			}
			for j := range question.Choices {
				s.appendDecisionText(question.Choices[j].Description)
			}
			for j := range question.Levels {
				s.appendDecisionText(question.Levels[j].Description)
			}
		}
		s.addDynamic(decision.ExtraParams)
	case req.SpeechRequest != nil:
		speech := req.SpeechRequest
		if speech.Input != nil {
			s.texts = append(s.texts, &speech.Input.Input)
		}
		if speech.Params != nil {
			s.texts = append(s.texts, &speech.Params.Instructions)
			s.addDynamic(speech.Params.ExtraParams)
		}
	case req.TranscriptionRequest != nil:
		transcription := req.TranscriptionRequest
		if transcription.Params != nil {
			if transcription.Params.Prompt != nil {
				s.texts = append(s.texts, transcription.Params.Prompt)
			}
			s.addScalar(&transcription.Params.WebhookMetadata)
			s.addDynamic(transcription.Params.ExtraParams)
		}
	case req.ImageGenerationRequest != nil:
		image := req.ImageGenerationRequest
		if image.Input != nil {
			s.texts = append(s.texts, &image.Input.Prompt)
		}
		if image.Params != nil {
			if image.Params.NegativePrompt != nil {
				s.texts = append(s.texts, image.Params.NegativePrompt)
			}
			s.addDynamic(image.Params.ExtraParams)
		}
	case req.ImageEditRequest != nil:
		image := req.ImageEditRequest
		if image.Input != nil {
			s.texts = append(s.texts, &image.Input.Prompt)
		}
		if image.Params != nil {
			if image.Params.NegativePrompt != nil {
				s.texts = append(s.texts, image.Params.NegativePrompt)
			}
			s.addDynamic(image.Params.ExtraParams)
		}
	case req.ImageVariationRequest != nil:
		if params := req.ImageVariationRequest.Params; params != nil {
			s.addDynamic(params.ExtraParams)
		}
	case req.VideoGenerationRequest != nil:
		video := req.VideoGenerationRequest
		if video.Input != nil {
			s.texts = append(s.texts, &video.Input.Prompt)
		}
		if video.Params != nil {
			if video.Params.NegativePrompt != nil {
				s.texts = append(s.texts, video.Params.NegativePrompt)
			}
			s.addDynamic(video.Params.ExtraParams)
		}
	case req.VideoEditRequest != nil:
		video := req.VideoEditRequest
		if video.Input != nil {
			s.texts = append(s.texts, &video.Input.Prompt)
		}
		if video.Params != nil {
			s.addDynamic(video.Params.ExtraParams)
		}
	case req.VideoRemixRequest != nil:
		video := req.VideoRemixRequest
		if video.Input != nil {
			s.texts = append(s.texts, &video.Input.Prompt)
		}
		s.addDynamic(video.ExtraParams)
	case req.OCRRequest != nil:
		ocr := req.OCRRequest
		if ocr.Params != nil {
			if ocr.Params.DocumentAnnotationPrompt != nil {
				s.texts = append(s.texts, ocr.Params.DocumentAnnotationPrompt)
			}
			s.addDynamic(ocr.Params.ExtraParams)
		}
	case req.CachedContentCreateRequest != nil:
		cached := req.CachedContentCreateRequest
		s.addScalar(&cached.SystemInstruction)
		s.addDynamic(cached.Contents)
		s.addDynamic(cached.Tools)
		s.addScalar(&cached.ToolConfig)
		s.addDynamic(cached.ExtraParams)
	case req.BatchCreateRequest != nil:
		batch := req.BatchCreateRequest
		for i := range batch.Requests {
			item := &batch.Requests[i]
			s.addDynamic(item.Body)
			s.addDynamic(item.Params)
		}
		s.addDynamic(batch.ExtraParams)
	case req.FileUploadRequest != nil:
		// The file is media, or a batch input file named by id, and is not inspected.
		s.addDynamic(req.FileUploadRequest.ExtraParams)
	case req.PassthroughRequest != nil:
		passthrough := req.PassthroughRequest
		switch {
		case passthrough.Form != nil:
			names := make([]string, 0, len(passthrough.Form.Value))
			for name := range passthrough.Form.Value {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				values := passthrough.Form.Value[name]
				for i := range values {
					s.texts = append(s.texts, &values[i])
				}
			}
		case passthrough.BodySource != nil:
			return nil, fmt.Errorf("%w: %s request with an opaque streamed body", errUnsupportedShape, shapeName(req))
		case len(passthrough.Body) > 0:
			if !gjson.ValidBytes(passthrough.Body) {
				return nil, fmt.Errorf("%w: %s request with a body that is not a JSON document", errUnsupportedShape, shapeName(req))
			}
			s.body = passthrough.Body
		}
	}
	if useRawBody(ctx) {
		if raw := rawBody(req); len(raw) > 0 {
			if !gjson.ValidBytes(raw) {
				return nil, fmt.Errorf("%w: %s request with a raw body that is not a JSON document", errUnsupportedShape, shapeName(req))
			}
			s.raw = raw
		}
	}
	return s, nil
}

// shapeName names the request shape for an error, never its content.
func shapeName(req *schemas.BifrostRequest) string {
	if name := string(req.RequestType); name != "" {
		return name
	}
	return "unknown"
}

// addDynamic registers a forwarded JSON subtree so its string values are inspected and rewritten
// in place. A provider merges these into the JSON it sends.
func (s *segmentSet) addDynamic(value any) {
	switch typed := value.(type) {
	case map[string]any:
		if typed != nil {
			s.dynamic = append(s.dynamic, typed)
		}
	case []any:
		if typed != nil {
			s.dynamic = append(s.dynamic, typed)
		}
	}
}

// addScalar registers an any-typed request field, which may hold a bare string or a JSON subtree.
func (s *segmentSet) addScalar(field *any) {
	if field != nil {
		s.scalars = append(s.scalars, field)
	}
}

// appendChatMessage adds a chat message's text: its content string or text blocks, plus every
// tool call's arguments and custom-tool input. A tool message's result is its content string,
// so it is covered by the same branch, and so are the system and developer prompts. Reasoning
// a caller replays is scanned too, since it is forwarded to the provider.
func (s *segmentSet) appendChatMessage(msg *schemas.ChatMessage) {
	if content := msg.Content; content != nil {
		if content.ContentStr != nil {
			s.texts = append(s.texts, content.ContentStr)
		}
		for i := range content.ContentBlocks {
			if block := &content.ContentBlocks[i]; block.Text != nil {
				s.texts = append(s.texts, block.Text)
			}
		}
	}
	assistant := msg.ChatAssistantMessage
	if assistant == nil {
		return
	}
	for i := range assistant.ToolCalls {
		call := &assistant.ToolCalls[i]
		s.texts = append(s.texts, &call.Function.Arguments)
		if call.Custom != nil {
			s.texts = append(s.texts, &call.Custom.Input)
		}
	}
	if assistant.Reasoning != nil {
		s.texts = append(s.texts, assistant.Reasoning)
	}
	for i := range assistant.ReasoningDetails {
		details := &assistant.ReasoningDetails[i]
		if details.Summary != nil {
			s.texts = append(s.texts, details.Summary)
		}
		if details.Text != nil {
			s.texts = append(s.texts, details.Text)
		}
	}
}

// appendResponsesRequest adds a responses-shaped request's text: its top-level instructions
// (the caller's system prompt; /v1/messages system content is converted into a system message
// and is covered by the message walk), every input item, and the caller's extra parameters.
func (s *segmentSet) appendResponsesRequest(request *schemas.BifrostResponsesRequest) {
	if params := request.Params; params != nil {
		if params.Instructions != nil {
			s.texts = append(s.texts, params.Instructions)
		}
		if params.Metadata != nil {
			s.addDynamic(*params.Metadata)
		}
		s.addDynamic(params.ExtraParams)
	}
	s.appendResponsesMessages(request.Input)
}

// appendResponsesMessages adds responses-API items' text: message content and text blocks,
// function-call arguments, function-call output (string or text blocks), custom-tool input,
// compaction summaries, and reasoning summaries the caller replays. An item Bifrost re-emits
// from the bytes it was decoded from is registered so those bytes are rewritten too.
func (s *segmentSet) appendResponsesMessages(messages []schemas.ResponsesMessage) {
	for i := range messages {
		message := &messages[i]
		if message.IsVerbatimItem() {
			s.verbatim = append(s.verbatim, message)
		}
		if content := message.Content; content != nil {
			if content.ContentStr != nil {
				s.texts = append(s.texts, content.ContentStr)
			}
			for j := range content.ContentBlocks {
				block := &content.ContentBlocks[j]
				if block.Text != nil {
					s.texts = append(s.texts, block.Text)
				}
				if block.ResponsesOutputMessageContentCompaction != nil {
					s.texts = append(s.texts, &block.Summary)
				}
			}
		}
		if reasoning := message.ResponsesReasoning; reasoning != nil {
			for j := range reasoning.Summary {
				s.texts = append(s.texts, &reasoning.Summary[j].Text)
			}
		}
		tool := message.ResponsesToolMessage
		if tool == nil {
			continue
		}
		if tool.Arguments != nil {
			s.texts = append(s.texts, tool.Arguments)
		}
		if tool.ResponsesCustomToolCall != nil {
			s.texts = append(s.texts, &tool.ResponsesCustomToolCall.Input)
		}
		if output := tool.Output; output != nil {
			if output.ResponsesToolCallOutputStr != nil {
				s.texts = append(s.texts, output.ResponsesToolCallOutputStr)
			}
			for j := range output.ResponsesFunctionToolCallOutputBlocks {
				if block := &output.ResponsesFunctionToolCallOutputBlocks[j]; block.Text != nil {
					s.texts = append(s.texts, block.Text)
				}
			}
		}
	}
}

// appendDecisionInput adds a decision request's evidence: its text, its message parts, and its
// structured state.
func (s *segmentSet) appendDecisionInput(input *schemas.DecisionInput) {
	if input.Text != nil {
		s.texts = append(s.texts, input.Text)
	}
	s.addDynamic(input.Structured)
	for i := range input.Messages {
		message := &input.Messages[i]
		s.appendDecisionContent(&message.Content)
		if message.Output != nil {
			s.appendDecisionContent(message.Output)
		}
		if message.Arguments != nil {
			s.texts = append(s.texts, message.Arguments)
		}
		if message.Input != nil {
			s.texts = append(s.texts, message.Input)
		}
	}
}

// appendDecisionContent adds one decision message body or tool output.
func (s *segmentSet) appendDecisionContent(content *schemas.DecisionInputContent) {
	if content.Text != nil {
		s.texts = append(s.texts, content.Text)
	}
	for i := range content.Parts {
		part := &content.Parts[i]
		s.appendDecisionText(part.Text)
		if part.Refusal != nil {
			s.texts = append(s.texts, part.Refusal)
		}
		s.addDynamic(part.Data.Structured)
	}
}

// appendDecisionText adds a question's text, criteria side, choice or level description.
func (s *segmentSet) appendDecisionText(text *schemas.DecisionText) {
	if text == nil {
		return
	}
	if text.Text != nil {
		s.texts = append(s.texts, text.Text)
	}
	s.addDynamic(text.Structured)
}

// detectTexts returns the non-empty segments as one slice, in request order, ending with the
// decoded values of a passthrough body and of the raw body. Detectors that talk to a service
// send all of it at once.
func (s *segmentSet) detectTexts() []string {
	if s == nil {
		return nil
	}
	texts := make([]string, 0, len(s.texts)+len(s.dynamic)+len(s.verbatim))
	for _, text := range s.texts {
		if *text != "" {
			texts = append(texts, *text)
		}
	}
	for _, value := range s.dynamic {
		walkDynamicStrings(value, func(text string) bool {
			if text != "" {
				texts = append(texts, text)
			}
			return true
		})
	}
	for _, field := range s.scalars {
		walkDynamicStrings(*field, func(text string) bool {
			if text != "" {
				texts = append(texts, text)
			}
			return true
		})
	}
	for _, message := range s.verbatim {
		walkVerbatimStrings(message, func(text string) bool {
			if text != "" {
				texts = append(texts, text)
			}
			return true
		})
	}
	texts = appendBodyTexts(texts, s.body)
	texts = appendBodyTexts(texts, s.raw)
	return texts
}

// appendBodyTexts appends the decoded string values of a JSON body.
func appendBodyTexts(texts []string, body []byte) []string {
	if len(body) == 0 {
		return texts
	}
	walkJSONStrings(gjson.ParseBytes(body), func(value string) bool {
		if value != "" {
			texts = append(texts, value)
		}
		return true
	})
	return texts
}

// fingerprint returns a cheap identity for the inspected text: how many segments it has, how many
// bytes it holds, and an FNV-1a hash over their bytes. A later phase uses it to tell input that
// changed since the last inspection (a continuation turn, an injected prompt, a same-size swap)
// from one merely retried. It walks the segments instead of collecting them.
func (s *segmentSet) fingerprint() (count int, bytes int, sum uint64) {
	if s == nil {
		return 0, 0, fnvOffset64
	}
	state := &fingerprintState{sum: fnvOffset64}
	add := state.add
	for _, text := range s.texts {
		add(*text)
	}
	for _, value := range s.dynamic {
		walkDynamicStrings(value, add)
	}
	for _, field := range s.scalars {
		walkDynamicStrings(*field, add)
	}
	for _, message := range s.verbatim {
		walkVerbatimStrings(message, add)
	}
	if len(s.body) > 0 {
		walkJSONStrings(gjson.ParseBytes(s.body), add)
	}
	if len(s.raw) > 0 {
		walkJSONStrings(gjson.ParseBytes(s.raw), add)
	}
	return state.count, state.bytes, state.sum
}

// FNV-1a 64 parameters, for the fingerprint hash.
const (
	fnvOffset64 = 14695981039346656037
	fnvPrime64  = 1099511628211
)

// fingerprintState accumulates a segment identity while the request's text is walked.
type fingerprintState struct {
	count int
	bytes int
	sum   uint64
}

// add hashes one segment. A zero byte separates segments, so text split differently across two
// segments does not hash the same.
func (f *fingerprintState) add(text string) bool {
	if text == "" {
		return true
	}
	if f.count > 0 {
		f.sum *= fnvPrime64 // hash a zero separator byte
	}
	for i := 0; i < len(text); i++ {
		f.sum ^= uint64(text[i])
		f.sum *= fnvPrime64
	}
	f.count++
	f.bytes += len(text)
	return true
}

// rewrite applies the replacements to every text segment in place and returns the replacements it
// actually made. Where matches overlap, the whole union is replaced, so no part of a matched value
// survives: a merged span keeps the placeholder of the earliest-starting match. A literal found
// only in a body that rawbody.go rewrites is reported as applied too, so the caller publishes it
// and the outbound body is verified.
func (s *segmentSet) rewrite(replacements map[string]string) map[string]string {
	applied := make(map[string]string, len(replacements))
	if s == nil || len(replacements) == 0 {
		return applied
	}
	for _, text := range s.texts {
		if replaced := applyReplacements(*text, replacements, applied); replaced != *text {
			*text = replaced
		}
	}
	for _, value := range s.dynamic {
		rewriteDynamic(value, replacements, applied)
	}
	for _, field := range s.scalars {
		*field, _ = rewriteDynamic(*field, replacements, applied)
	}
	for _, message := range s.verbatim {
		rewriteVerbatim(message, replacements, applied)
	}
	recordBodyMatches(s.body, replacements, applied)
	recordBodyMatches(s.raw, replacements, applied)
	return applied
}

// applyReplacements replaces the union of every literal match in text and records what it
// replaced. Overlapping matches merge: replacing them one at a time leaves the part of a longer
// value no shorter literal covered.
func applyReplacements(text string, replacements map[string]string, applied map[string]string) string {
	if text == "" || len(replacements) == 0 {
		return text
	}
	var matches []textMatch
	for literal := range replacements {
		if literal == "" {
			continue
		}
		for offset := 0; offset < len(text); {
			index := strings.Index(text[offset:], literal)
			if index < 0 {
				break
			}
			start := offset + index
			matches = append(matches, textMatch{start: start, end: start + len(literal), literal: literal})
			offset = start + 1
		}
	}
	if len(matches) == 0 {
		return text
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].start != matches[j].start {
			return matches[i].start < matches[j].start
		}
		return matches[i].end > matches[j].end
	})

	var builder strings.Builder
	builder.Grow(len(text))
	cursor := 0
	for i := 0; i < len(matches); {
		start, end := matches[i].start, matches[i].end
		next := i + 1
		for next < len(matches) && matches[next].start < end {
			if matches[next].end > end {
				end = matches[next].end
			}
			next++
		}
		literal := text[start:end]
		replacement, ok := replacements[literal]
		if !ok {
			replacement = replacements[matches[i].literal]
		}
		if _, seen := applied[literal]; !seen {
			applied[literal] = replacement
		}
		builder.WriteString(text[cursor:start])
		builder.WriteString(replacement)
		cursor = end
		i = next
	}
	builder.WriteString(text[cursor:])
	return builder.String()
}

// textMatch is one literal occurrence in a segment.
type textMatch struct {
	start   int
	end     int
	literal string
}

// rewriteDynamic rewrites the string values of a forwarded JSON value in place, through maps and
// slices, and reports whether anything changed. A bare string is returned, since it cannot be
// assigned in place.
func rewriteDynamic(value any, replacements map[string]string, applied map[string]string) (any, bool) {
	switch typed := value.(type) {
	case string:
		replaced := applyReplacements(typed, replacements, applied)
		return replaced, replaced != typed
	case map[string]any:
		changed := false
		for key, item := range typed {
			if next, rewritten := rewriteDynamic(item, replacements, applied); rewritten {
				typed[key] = next
				changed = true
			}
		}
		return typed, changed
	case []any:
		changed := false
		for i, item := range typed {
			if next, rewritten := rewriteDynamic(item, replacements, applied); rewritten {
				typed[i] = next
				changed = true
			}
		}
		return typed, changed
	}
	return value, false
}

// walkDynamicStrings calls visit for every string value of a forwarded JSON value, in sorted key
// order, and stops when visit returns false.
func walkDynamicStrings(value any, visit func(string) bool) bool {
	switch typed := value.(type) {
	case string:
		return visit(typed)
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if !walkDynamicStrings(typed[key], visit) {
				return false
			}
		}
	case []any:
		for _, item := range typed {
			if !walkDynamicStrings(item, visit) {
				return false
			}
		}
	}
	return true
}

// walkJSONStrings calls visit for every string value of a JSON document, decoded, so an escaped
// value the provider reads as text is inspected as that text.
func walkJSONStrings(value gjson.Result, visit func(string) bool) bool {
	if value.IsObject() || value.IsArray() {
		stopped := false
		value.ForEach(func(_, item gjson.Result) bool {
			if !walkJSONStrings(item, visit) {
				stopped = true
				return false
			}
			return true
		})
		return !stopped
	}
	if value.Type != gjson.String {
		return true
	}
	return visit(value.String())
}

// recordBodyMatches records the literals a body carries, without touching its bytes: the
// integration rewriter owns that, and rawbody.go verifies what it produced.
func recordBodyMatches(body []byte, replacements map[string]string, applied map[string]string) {
	if len(body) == 0 || len(replacements) == 0 {
		return
	}
	walkJSONStrings(gjson.ParseBytes(body), func(value string) bool {
		for literal, placeholder := range replacements {
			if literal == "" || !strings.Contains(value, literal) {
				continue
			}
			if _, seen := applied[literal]; !seen {
				applied[literal] = placeholder
			}
		}
		return true
	})
}

// walkVerbatimStrings calls visit for every string value of an item Bifrost re-emits from the
// bytes it was decoded from.
func walkVerbatimStrings(message *schemas.ResponsesMessage, visit func(string) bool) bool {
	raw, err := schemas.Marshal(*message)
	if err != nil {
		return true
	}
	return walkJSONStrings(gjson.ParseBytes(raw), visit)
}

// rewriteVerbatim rewrites an item that keeps its original bytes: the item is re-encoded from a
// copy of those bytes with its string values replaced, so the preserved copy cannot carry a value
// its typed fields no longer hold. A rewrite that fails leaves the literal in place, which
// rawbody.go's verification turns into a block.
func rewriteVerbatim(message *schemas.ResponsesMessage, replacements map[string]string, applied map[string]string) {
	raw, err := schemas.Marshal(*message)
	if err != nil {
		return
	}
	var tree any
	if err := schemas.Unmarshal(raw, &tree); err != nil {
		return
	}
	if _, changed := rewriteDynamic(tree, replacements, applied); !changed {
		return
	}
	rewritten, err := schemas.Marshal(tree)
	if err != nil {
		return
	}
	var item schemas.ResponsesMessage
	if err := schemas.Unmarshal(rewritten, &item); err != nil {
		return
	}
	*message = item
}

// useRawBody reports whether the caller asked for the raw request body to be forwarded.
func useRawBody(ctx *schemas.BifrostContext) bool {
	if ctx == nil {
		return false
	}
	use, _ := ctx.Value(schemas.BifrostContextKeyUseRawRequestBody).(bool)
	return use
}
