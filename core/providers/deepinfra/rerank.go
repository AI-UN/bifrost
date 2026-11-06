package deepinfra

import (
	"fmt"
	"sort"

	"github.com/bytedance/sonic"
	providerUtils "github.com/maximhq/bifrost/core/providers/utils"
	"github.com/maximhq/bifrost/core/schemas"
)

// Rerank scores documents against a query using DeepInfra's native inference endpoint.
//
// There is no OpenAI-shaped rerank on DeepInfra: POST /v1/inference/{model} takes
// {queries, documents} and answers {scores}, a positional score vector rather than a
// ranked result list. The ordering, index and identity semantics Bifrost's rerank
// contract requires are therefore reconstructed here.
func (provider *DeepInfraProvider) Rerank(ctx *schemas.BifrostContext, key schemas.Key, request *schemas.BifrostRerankRequest) (*schemas.BifrostRerankResponse, *schemas.BifrostError) {
	if len(request.Documents) == 0 {
		return nil, providerUtils.NewBifrostOperationError("rerank requires at least one document", nil)
	}

	jsonBody, bifrostErr := providerUtils.CheckContextAndGetRequestBody(
		ctx,
		request,
		func() (providerUtils.RequestBodyWithExtraParams, error) {
			return toDeepInfraRerankRequest(request), nil
		})
	if bifrostErr != nil {
		return nil, bifrostErr
	}

	url := provider.requestURL(ctx, pathInferencePrefix+request.Model)
	result, bifrostErr := provider.postNativeJSON(ctx, url, key, jsonBody)
	if result.headers != nil {
		ctx.SetValue(schemas.BifrostContextKeyProviderResponseHeaders, result.headers)
	}
	if bifrostErr != nil {
		return nil, providerUtils.EnrichError(ctx, bifrostErr, jsonBody, nil, provider.sendBackRawRequest, provider.sendBackRawResponse, result.latency)
	}

	var providerResponse DeepInfraRerankResponse
	rawRequest, rawResponse, bifrostErr := providerUtils.HandleProviderResponse(
		result.body,
		&providerResponse,
		jsonBody,
		providerUtils.ShouldSendBackRawRequest(ctx, provider.sendBackRawRequest),
		providerUtils.ShouldSendBackRawResponse(ctx, provider.sendBackRawResponse),
	)
	if bifrostErr != nil {
		return nil, providerUtils.EnrichError(ctx, bifrostErr, jsonBody, result.body, provider.sendBackRawRequest, provider.sendBackRawResponse, result.latency)
	}

	// A score vector that does not line up with the documents we sent cannot be mapped back
	// to input positions, and guessing would silently mis-rank. Fail loudly instead.
	if len(providerResponse.Scores) != len(request.Documents) {
		return nil, providerUtils.EnrichError(
			ctx,
			providerUtils.NewBifrostOperationError(
				fmt.Sprintf("rerank returned %d scores for %d documents", len(providerResponse.Scores), len(request.Documents)),
				nil,
			),
			jsonBody, result.body, provider.sendBackRawRequest, provider.sendBackRawResponse, result.latency,
		)
	}

	bifrostResponse := providerResponse.ToBifrostRerankResponse(request)
	bifrostResponse.Model = request.Model
	bifrostResponse.ExtraFields.Latency = result.latency.Milliseconds()
	bifrostResponse.ExtraFields.ProviderResponseHeaders = result.headers

	if providerUtils.ShouldSendBackRawRequest(ctx, provider.sendBackRawRequest) {
		bifrostResponse.ExtraFields.RawRequest = rawRequest
	}
	if providerUtils.ShouldSendBackRawResponse(ctx, provider.sendBackRawResponse) {
		bifrostResponse.ExtraFields.RawResponse = rawResponse
	}

	return bifrostResponse, nil
}

// toDeepInfraRerankRequest converts a Bifrost rerank request into the native body.
// The single query is broadcast across all documents, which is the form DeepInfra
// documents for the common case.
func toDeepInfraRerankRequest(request *schemas.BifrostRerankRequest) *DeepInfraRerankRequest {
	documents := make([]string, len(request.Documents))
	for i, doc := range request.Documents {
		documents[i] = rerankDocumentText(doc)
	}

	body := &DeepInfraRerankRequest{
		Queries:   []string{request.Query},
		Documents: documents,
	}

	if request.Params != nil {
		body.ExtraParams = request.Params.ExtraParams
		// DeepInfra exposes the task steer as a first-class field rather than through the
		// generic passthrough, so lift it out when the caller supplied one.
		if instruction, ok := request.Params.ExtraParams["instruction"].(string); ok && instruction != "" {
			body.Instruction = schemas.Ptr(instruction)
		}
	}

	return body
}

// rerankDocumentText renders a document for an endpoint that ranks plain strings.
// A structured document with no prose is serialized so its content still reaches the model.
func rerankDocumentText(doc schemas.RerankDocument) string {
	if doc.Text != "" || len(doc.Data) == 0 {
		return doc.Text
	}
	encoded, err := sonic.Marshal(doc.Data)
	if err != nil {
		return doc.Text
	}
	return string(encoded)
}

// ToBifrostRerankResponse turns the positional score vector into Bifrost's ranked result
// list. Callers must have already verified that len(Scores) == len(request.Documents).
//
// Index always addresses the caller's original document position, never the rank, and
// results are emitted in descending score order with ties broken by ascending index so
// the output is deterministic for equal scores.
func (response *DeepInfraRerankResponse) ToBifrostRerankResponse(request *schemas.BifrostRerankRequest) *schemas.BifrostRerankResponse {
	returnDocuments := request.Params != nil && request.Params.ReturnDocuments != nil && *request.Params.ReturnDocuments

	results := make([]schemas.RerankResult, len(response.Scores))
	for i, score := range response.Scores {
		results[i] = schemas.RerankResult{
			Index:          i,
			RelevanceScore: score,
			ID:             request.Documents[i].ID,
		}
		if returnDocuments {
			results[i].Document = schemas.Ptr(request.Documents[i])
		}
	}

	sort.SliceStable(results, func(i, j int) bool {
		if results[i].RelevanceScore == results[j].RelevanceScore {
			return results[i].Index < results[j].Index
		}
		return results[i].RelevanceScore > results[j].RelevanceScore
	})

	// TopN truncates after ranking, so it keeps the most relevant documents rather than
	// the first ones the caller happened to pass.
	if request.Params != nil && request.Params.TopN != nil && *request.Params.TopN >= 0 && *request.Params.TopN < len(results) {
		results = results[:*request.Params.TopN]
	}

	bifrostResponse := &schemas.BifrostRerankResponse{Results: results}
	if response.InputTokens > 0 {
		bifrostResponse.Usage = &schemas.BifrostLLMUsage{
			PromptTokens: response.InputTokens,
			TotalTokens:  response.InputTokens,
		}
	}
	return bifrostResponse
}
