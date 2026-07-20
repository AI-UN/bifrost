package siliconflow

import (
	"fmt"
	"maps"
	"sort"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/maximhq/bifrost/core/schemas"
)

// siliconFlowRerankReservedKeys are wire fields owned by the typed request;
// extra params must not override them.
var siliconFlowRerankReservedKeys = map[string]struct{}{
	"model":            {},
	"query":            {},
	"documents":        {},
	"top_n":            {},
	"return_documents": {},
}

// ToSiliconFlowRerankRequest converts a Bifrost rerank request to SiliconFlow
// format. SiliconFlow's documents field is a plain string array, so Bifrost
// documents are projected to their text content.
func ToSiliconFlowRerankRequest(request *schemas.BifrostRerankRequest) (*SiliconFlowRerankRequest, error) {
	if request == nil {
		return nil, fmt.Errorf("rerank request is nil")
	}
	if strings.TrimSpace(request.Query) == "" {
		return nil, fmt.Errorf("rerank query is required")
	}
	if len(request.Documents) == 0 {
		return nil, fmt.Errorf("rerank documents are required")
	}

	documents := make([]string, len(request.Documents))
	for i, doc := range request.Documents {
		documents[i] = doc.Text
	}

	converted := &SiliconFlowRerankRequest{
		Model:     request.Model,
		Query:     request.Query,
		Documents: documents,
	}

	if request.Params != nil {
		converted.TopN = request.Params.TopN
		converted.ReturnDocuments = request.Params.ReturnDocuments
		if len(request.Params.ExtraParams) > 0 {
			extraParams := make(map[string]interface{}, len(request.Params.ExtraParams))
			maps.Copy(extraParams, request.Params.ExtraParams)
			for key := range siliconFlowRerankReservedKeys {
				delete(extraParams, key)
			}
			converted.ExtraParams = extraParams
		}
	}

	return converted, nil
}

// ToBifrostRerankResponse converts a SiliconFlow rerank response to Bifrost
// format, accepting the international top-level `tokens` usage and the
// China-region `meta.tokens` usage, and both string and object document
// shapes. Results are stably sorted by score descending, index ascending.
func (response *SiliconFlowRerankResponse) ToBifrostRerankResponse(model string) *schemas.BifrostRerankResponse {
	if response == nil {
		return nil
	}

	bifrostResponse := &schemas.BifrostRerankResponse{
		ID:    response.ID,
		Model: model,
	}

	for _, result := range response.Results {
		rerankResult := schemas.RerankResult{
			Index:          result.Index,
			RelevanceScore: result.RelevanceScore,
		}
		if doc := parseSiliconFlowRerankDocument(result.Document); doc != nil {
			rerankResult.Document = doc
		}
		bifrostResponse.Results = append(bifrostResponse.Results, rerankResult)
	}

	sort.SliceStable(bifrostResponse.Results, func(i, j int) bool {
		if bifrostResponse.Results[i].RelevanceScore == bifrostResponse.Results[j].RelevanceScore {
			return bifrostResponse.Results[i].Index < bifrostResponse.Results[j].Index
		}
		return bifrostResponse.Results[i].RelevanceScore > bifrostResponse.Results[j].RelevanceScore
	})

	tokens := response.Tokens
	if tokens == nil && response.Meta != nil {
		tokens = response.Meta.Tokens
	}
	bifrostResponse.Usage = siliconFlowRerankUsage(tokens)

	return bifrostResponse
}

// parseSiliconFlowRerankDocument parses a rerank document returned by the
// upstream, accepting either a bare string or an object with a text field.
func parseSiliconFlowRerankDocument(raw []byte) *schemas.RerankDocument {
	if len(raw) == 0 {
		return nil
	}

	var text string
	if err := sonic.Unmarshal(raw, &text); err == nil {
		return &schemas.RerankDocument{Text: text}
	}

	var docMap map[string]interface{}
	if err := sonic.Unmarshal(raw, &docMap); err != nil {
		return nil
	}

	doc := &schemas.RerankDocument{}
	populated := false
	if docText, ok := docMap["text"].(string); ok {
		doc.Text = docText
		populated = true
	}
	if id, ok := docMap["id"].(string); ok {
		doc.ID = &id
		populated = true
	}
	meta := make(map[string]interface{})
	for k, v := range docMap {
		if k != "text" && k != "id" {
			meta[k] = v
		}
	}
	if len(meta) > 0 {
		doc.Meta = meta
		populated = true
	}
	if !populated {
		return nil
	}
	return doc
}

// siliconFlowRerankUsage maps rerank token counts onto Bifrost usage.
func siliconFlowRerankUsage(tokens *SiliconFlowRerankTokens) *schemas.BifrostLLMUsage {
	if tokens == nil {
		return nil
	}
	promptTokens := 0
	completionTokens := 0
	hasUsage := false
	if tokens.InputTokens != nil {
		promptTokens = int(*tokens.InputTokens)
		hasUsage = true
	}
	if tokens.OutputTokens != nil {
		completionTokens = int(*tokens.OutputTokens)
		hasUsage = true
	}
	if !hasUsage {
		return nil
	}
	return &schemas.BifrostLLMUsage{
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      promptTokens + completionTokens,
	}
}
