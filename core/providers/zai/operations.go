package zai

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/maximhq/bifrost/core/providers/openai"
	providerUtils "github.com/maximhq/bifrost/core/providers/utils"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/valyala/fasthttp"
)

const (
	pathEmbeddings = "/embeddings"
	pathRerank     = "/rerank"
	pathTokenizer  = "/tokenizer"
)

type zhipuRerankRequest struct {
	Model           string                 `json:"model"`
	Query           string                 `json:"query"`
	Documents       []string               `json:"documents"`
	TopN            *int                   `json:"top_n,omitempty"`
	ReturnDocuments *bool                  `json:"return_documents,omitempty"`
	ExtraParams     map[string]interface{} `json:"-"`
}

func (r *zhipuRerankRequest) GetExtraParams() map[string]interface{} {
	if r == nil {
		return nil
	}
	return r.ExtraParams
}

func (provider *ZAIProvider) Embedding(ctx *schemas.BifrostContext, key schemas.Key, request *schemas.BifrostEmbeddingRequest) (*schemas.BifrostEmbeddingResponse, *schemas.BifrostError) {
	if provider.GetProviderKey() != schemas.Zhipu {
		return nil, providerUtils.NewUnsupportedOperationError(schemas.EmbeddingRequest, provider.GetProviderKey())
	}
	if err := provider.checkOperation(schemas.EmbeddingRequest); err != nil {
		return nil, err
	}
	ctx.SetValue(schemas.BifrostContextKeyPassthroughExtraParams, true)
	return openai.HandleOpenAIEmbeddingRequest(
		ctx,
		provider.client,
		provider.requestURL(ctx, pathEmbeddings),
		request,
		openai.BearerAuthHeader(key),
		provider.networkConfig.ExtraHeaders,
		provider.GetProviderKey(),
		providerUtils.ShouldSendBackRawRequest(ctx, provider.sendBackRawRequest),
		providerUtils.ShouldSendBackRawResponse(ctx, provider.sendBackRawResponse),
		nil,
		openai.ParseOpenAIError,
		provider.logger,
	)
}

func (provider *ZAIProvider) Rerank(ctx *schemas.BifrostContext, key schemas.Key, request *schemas.BifrostRerankRequest) (*schemas.BifrostRerankResponse, *schemas.BifrostError) {
	if provider.GetProviderKey() != schemas.Zhipu {
		return nil, providerUtils.NewUnsupportedOperationError(schemas.RerankRequest, provider.GetProviderKey())
	}
	if err := provider.checkOperation(schemas.RerankRequest); err != nil {
		return nil, err
	}
	if request == nil {
		return nil, providerUtils.NewBifrostBadRequestError("rerank request is required")
	}
	if providerUtils.IsLargePayloadPassthroughEnabled(ctx) {
		return nil, providerUtils.NewBifrostBadRequestError("large payload passthrough is not supported for zhipu rerank conversion")
	}
	ctx.SetValue(schemas.BifrostContextKeyPassthroughExtraParams, true)
	jsonBody, bifrostErr := providerUtils.CheckContextAndGetRequestBody(ctx, request, func() (providerUtils.RequestBodyWithExtraParams, error) {
		documents := make([]string, len(request.Documents))
		for i, document := range request.Documents {
			if document.Data != nil {
				if document.Text != "" {
					return nil, providerUtils.InvalidRequestErrorf("zhipu rerank document %d cannot set both text and data", i)
				}
				serialized, err := providerUtils.MarshalSorted(document.Data)
				if err != nil {
					return nil, providerUtils.InvalidRequestErrorf("zhipu rerank document %d data must be JSON-serializable", i)
				}
				documents[i] = string(serialized)
				continue
			}
			documents[i] = document.Text
		}
		result := &zhipuRerankRequest{
			Model:     request.Model,
			Query:     request.Query,
			Documents: documents,
		}
		if request.Params != nil {
			result.TopN = request.Params.TopN
			result.ReturnDocuments = request.Params.ReturnDocuments
			result.ExtraParams = request.Params.ExtraParams
		}
		return result, nil
	})
	if bifrostErr != nil {
		return nil, bifrostErr
	}

	responseBody, latency, responseHeaders, bifrostErr := provider.doJSON(ctx, key, pathRerank, jsonBody)
	if responseHeaders != nil {
		ctx.SetValue(schemas.BifrostContextKeyProviderResponseHeaders, responseHeaders)
	}
	if bifrostErr != nil {
		return nil, providerUtils.EnrichError(ctx, bifrostErr, jsonBody, responseBody, provider.sendBackRawRequest, provider.sendBackRawResponse, latency)
	}

	var upstream struct {
		ID      string `json:"id"`
		Results []struct {
			Index          int             `json:"index"`
			RelevanceScore float64         `json:"relevance_score"`
			Document       json.RawMessage `json:"document"`
		} `json:"results"`
		Usage *schemas.BifrostLLMUsage `json:"usage,omitempty"`
	}
	rawRequest, rawResponse, bifrostErr := providerUtils.HandleProviderResponseCtx(ctx, responseBody, &upstream, jsonBody, providerUtils.ShouldSendBackRawRequest(ctx, provider.sendBackRawRequest), providerUtils.ShouldSendBackRawResponse(ctx, provider.sendBackRawResponse))
	if bifrostErr != nil {
		return nil, providerUtils.EnrichError(ctx, bifrostErr, jsonBody, responseBody, provider.sendBackRawRequest, provider.sendBackRawResponse, latency)
	}
	response := &schemas.BifrostRerankResponse{ID: upstream.ID, Model: request.Model, Usage: upstream.Usage}
	returnDocuments := request.Params != nil && request.Params.ReturnDocuments != nil && *request.Params.ReturnDocuments
	response.Results = make([]schemas.RerankResult, 0, len(upstream.Results))
	for _, result := range upstream.Results {
		mapped := schemas.RerankResult{Index: result.Index, RelevanceScore: result.RelevanceScore}
		if result.Index >= 0 && result.Index < len(request.Documents) {
			mapped.ID = request.Documents[result.Index].ID
			if returnDocuments {
				document := request.Documents[result.Index]
				mapped.Document = &document
			}
		}
		response.Results = append(response.Results, mapped)
	}
	response.ExtraFields.Latency = latency.Milliseconds()
	response.ExtraFields.ProviderResponseHeaders = responseHeaders
	if providerUtils.ShouldSendBackRawRequest(ctx, provider.sendBackRawRequest) {
		response.ExtraFields.RawRequest = rawRequest
	}
	if providerUtils.ShouldSendBackRawResponse(ctx, provider.sendBackRawResponse) {
		response.ExtraFields.RawResponse = rawResponse
	}
	return response, nil
}

type tokenizerRequest struct {
	Model    string                 `json:"model"`
	Messages []openai.OpenAIMessage `json:"messages"`
}

func (r *tokenizerRequest) GetExtraParams() map[string]interface{} { return nil }
func (r *tokenizerRequest) GetRawRequestBody() []byte              { return nil }
func (provider *ZAIProvider) CountTokens(ctx *schemas.BifrostContext, key schemas.Key, request *schemas.BifrostResponsesRequest) (*schemas.BifrostCountTokensResponse, *schemas.BifrostError) {
	if err := provider.checkOperation(schemas.CountTokensRequest); err != nil {
		return nil, err
	}
	if request == nil {
		return nil, providerUtils.NewBifrostBadRequestError("tokenizer request is required")
	}
	if providerUtils.IsLargePayloadPassthroughEnabled(ctx) {
		return nil, providerUtils.NewBifrostBadRequestError("large payload passthrough is not supported for zai tokenizer conversion")
	}
	chatRequest := request.ToChatRequest()
	chatRequest.Provider = provider.GetProviderKey()
	converted := openai.ToOpenAIChatRequest(ctx, chatRequest)
	if converted == nil {
		return nil, providerUtils.NewBifrostBadRequestError("tokenizer messages are required")
	}
	jsonBody, bifrostErr := providerUtils.CheckContextAndGetRequestBody(ctx, &tokenizerRequest{Model: request.Model, Messages: converted.Messages}, func() (providerUtils.RequestBodyWithExtraParams, error) {
		return &tokenizerRequest{Model: request.Model, Messages: converted.Messages}, nil
	})
	if bifrostErr != nil {
		return nil, bifrostErr
	}
	responseBody, latency, responseHeaders, bifrostErr := provider.doJSON(ctx, key, pathTokenizer, jsonBody)
	if responseHeaders != nil {
		ctx.SetValue(schemas.BifrostContextKeyProviderResponseHeaders, responseHeaders)
	}
	if bifrostErr != nil {
		return nil, providerUtils.EnrichError(ctx, bifrostErr, jsonBody, responseBody, provider.sendBackRawRequest, provider.sendBackRawResponse, latency)
	}
	var upstream struct {
		Model string `json:"model"`
		Usage struct {
			PromptTokens int `json:"prompt_tokens"`
			TotalTokens  int `json:"total_tokens"`
		} `json:"usage"`
	}
	rawRequest, rawResponse, bifrostErr := providerUtils.HandleProviderResponseCtx(ctx, responseBody, &upstream, jsonBody, providerUtils.ShouldSendBackRawRequest(ctx, provider.sendBackRawRequest), providerUtils.ShouldSendBackRawResponse(ctx, provider.sendBackRawResponse))
	if bifrostErr != nil {
		return nil, providerUtils.EnrichError(ctx, bifrostErr, jsonBody, responseBody, provider.sendBackRawRequest, provider.sendBackRawResponse, latency)
	}
	total := upstream.Usage.TotalTokens
	if total == 0 {
		total = upstream.Usage.PromptTokens
	}
	response := &schemas.BifrostCountTokensResponse{
		Object:      "tokenizer",
		Model:       firstNonEmpty(upstream.Model, request.Model),
		InputTokens: upstream.Usage.PromptTokens,
		TotalTokens: &total,
		ExtraFields: schemas.BifrostResponseExtraFields{Latency: latency.Milliseconds(), ProviderResponseHeaders: responseHeaders},
	}
	if providerUtils.ShouldSendBackRawRequest(ctx, provider.sendBackRawRequest) {
		response.ExtraFields.RawRequest = rawRequest
	}
	if providerUtils.ShouldSendBackRawResponse(ctx, provider.sendBackRawResponse) {
		response.ExtraFields.RawResponse = rawResponse
	}
	return response, nil
}

func (provider *ZAIProvider) doJSON(ctx *schemas.BifrostContext, key schemas.Key, path string, body []byte) ([]byte, time.Duration, map[string]string, *schemas.BifrostError) {
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)
	providerUtils.SetExtraHeaders(ctx, req, provider.networkConfig.ExtraHeaders, nil)
	req.SetRequestURI(provider.requestURL(ctx, path))
	req.Header.SetMethod(http.MethodPost)
	req.Header.SetContentType("application/json")
	for header, value := range openai.BearerAuthHeader(key) {
		req.Header.Set(header, value)
	}
	req.SetBody(body)
	latency, bifrostErr, wait := providerUtils.MakeRequestWithContext(ctx, provider.client, req, resp)
	defer wait()
	headers := providerUtils.ExtractProviderResponseHeaders(resp)
	if bifrostErr != nil {
		return nil, latency, headers, bifrostErr
	}
	if resp.StatusCode() < 200 || resp.StatusCode() >= 300 {
		providerUtils.MaterializeStreamErrorBody(ctx, resp)
		bodyCopy := append([]byte(nil), resp.Body()...)
		return bodyCopy, latency, headers, openai.ParseOpenAIError(resp)
	}
	decoded, err := providerUtils.CheckAndDecodeBody(resp)
	if err != nil {
		return nil, latency, headers, providerUtils.NewBifrostOperationError(schemas.ErrProviderResponseDecode, err)
	}
	return decoded, latency, headers, bifrostErr
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
