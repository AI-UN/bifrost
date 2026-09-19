package cpa

import (
	"context"
	"fmt"
	"strings"

	"github.com/maximhq/bifrost/core/providers/anthropic"
	"github.com/maximhq/bifrost/core/providers/gemini"
	"github.com/maximhq/bifrost/core/providers/openai"
	providerUtils "github.com/maximhq/bifrost/core/providers/utils"
	schemas "github.com/maximhq/bifrost/core/schemas"
)

// CPAProvider implements schemas.Provider for a CLI Proxy API instance by delegating
// to the built-in providers for the three dialects CPA accepts. See the package
// comment for why this is one provider rather than three.
type CPAProvider struct {
	logger    schemas.Logger
	openai    *openai.OpenAIProvider
	anthropic schemas.Provider
	gemini    schemas.Provider
}

var _ schemas.Provider = (*CPAProvider)(nil)

// NewCPAProvider creates a new CPA provider instance.
//
// base_url is required and must point at the CPA server root (the host serving
// /v1/chat/completions, /v1/messages and /v1beta/models). There is no public
// default: CPA is always self-hosted.
//
// Each sub-provider gets its own config clone, and the Gemini one is anchored at
// /v1beta because that provider appends "/models/{model}:<action>" to its base URL
// without inserting a version segment.
func NewCPAProvider(config *schemas.ProviderConfig, logger schemas.Logger) (*CPAProvider, error) {
	config.CheckAndSetDefaults()

	baseURL := strings.TrimRight(strings.TrimSpace(config.NetworkConfig.BaseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("network_config.base_url is required for cpa and must point at the CLI Proxy API server")
	}

	return &CPAProvider{
		logger:    logger,
		openai:    openai.NewOpenAIProvider(dialectConfig(config, baseURL, schemas.OpenAI), logger),
		anthropic: anthropic.NewAnthropicProvider(dialectConfig(config, baseURL, schemas.Anthropic), logger),
		gemini:    gemini.NewGeminiProvider(dialectConfig(config, baseURL+geminiAPIVersionPath, schemas.Gemini), logger),
	}, nil
}

// GetProviderKey returns the provider identifier for CPA.
func (provider *CPAProvider) GetProviderKey() schemas.ModelProvider {
	return schemas.CPA
}

// dialectProvider returns the sub-provider for d, and records on the context which
// built-in provider is actually serving this attempt.
//
// That context value is load-bearing: converters resolve several wire decisions
// through schemas.ResolveBaseProvider (reasoning item-id embedding, prompt-cache
// breakpoint placement), and core stamps it from the provider key — which is "cpa"
// for every dialect here, so without this write the family gating would see an
// unknown provider. The key is not reserved and provider calls run outside the
// plugin phases that block restricted writes, so a plain SetValue lands.
func (provider *CPAProvider) dialectProvider(ctx *schemas.BifrostContext, d dialect) schemas.Provider {
	if ctx != nil {
		ctx.SetValue(schemas.BifrostContextKeyBaseProviderType, d.baseProvider())
	}
	switch d {
	case dialectAnthropic:
		return provider.anthropic
	case dialectGemini:
		return provider.gemini
	default:
		return provider.openai
	}
}

// unsupported builds the error for an operation CPA exposes no route for.
func (provider *CPAProvider) unsupported(requestType schemas.RequestType) *schemas.BifrostError {
	return providerUtils.NewUnsupportedOperationError(requestType, provider.GetProviderKey())
}

// ListModels lists the models CPA currently has an available upstream for.
//
// Only the OpenAI dialect is queried. CPA builds every dialect's catalog from one
// global registry and only reshapes the output per dialect, so listing all three
// would return the same models three times.
func (provider *CPAProvider) ListModels(ctx *schemas.BifrostContext, keys []schemas.Key, request *schemas.BifrostListModelsRequest) (*schemas.BifrostListModelsResponse, *schemas.BifrostError) {
	return provider.dialectProvider(ctx, dialectOpenAI).ListModels(ctx, keys, request)
}

// TextCompletion performs a legacy text completion request via CPA's
// /v1/completions. Pinned to the OpenAI dialect: CPA exposes no Anthropic
// /v1/complete route.
func (provider *CPAProvider) TextCompletion(ctx *schemas.BifrostContext, key schemas.Key, request *schemas.BifrostTextCompletionRequest) (*schemas.BifrostTextCompletionResponse, *schemas.BifrostError) {
	return provider.dialectProvider(ctx, dialectOpenAI).TextCompletion(ctx, key, request)
}

// TextCompletionStream streams a legacy text completion via CPA's /v1/completions.
func (provider *CPAProvider) TextCompletionStream(ctx *schemas.BifrostContext, postHookRunner schemas.PostHookRunner, postHookSpanFinalizer func(context.Context), key schemas.Key, request *schemas.BifrostTextCompletionRequest) (chan *schemas.BifrostStreamChunk, *schemas.BifrostError) {
	return provider.dialectProvider(ctx, dialectOpenAI).TextCompletionStream(ctx, postHookRunner, postHookSpanFinalizer, key, request)
}

// ChatCompletion performs a chat completion request in the caller's own dialect.
func (provider *CPAProvider) ChatCompletion(ctx *schemas.BifrostContext, key schemas.Key, request *schemas.BifrostChatRequest) (*schemas.BifrostChatResponse, *schemas.BifrostError) {
	return provider.dialectProvider(ctx, ingressDialect(ctx)).ChatCompletion(ctx, key, request)
}

// ChatCompletionStream streams a chat completion in the caller's own dialect.
func (provider *CPAProvider) ChatCompletionStream(ctx *schemas.BifrostContext, postHookRunner schemas.PostHookRunner, postHookSpanFinalizer func(context.Context), key schemas.Key, request *schemas.BifrostChatRequest) (chan *schemas.BifrostStreamChunk, *schemas.BifrostError) {
	return provider.dialectProvider(ctx, ingressDialect(ctx)).ChatCompletionStream(ctx, postHookRunner, postHookSpanFinalizer, key, request)
}

// Responses performs a Responses-API request in the caller's own dialect: OpenAI
// clients reach CPA's /v1/responses, Anthropic clients reach /v1/messages, and
// Gemini clients reach :generateContent.
func (provider *CPAProvider) Responses(ctx *schemas.BifrostContext, key schemas.Key, request *schemas.BifrostResponsesRequest) (*schemas.BifrostResponsesResponse, *schemas.BifrostError) {
	return provider.dialectProvider(ctx, ingressDialect(ctx)).Responses(ctx, key, request)
}

// ResponsesStream streams a Responses-API request in the caller's own dialect.
func (provider *CPAProvider) ResponsesStream(ctx *schemas.BifrostContext, postHookRunner schemas.PostHookRunner, postHookSpanFinalizer func(context.Context), key schemas.Key, request *schemas.BifrostResponsesRequest) (chan *schemas.BifrostStreamChunk, *schemas.BifrostError) {
	return provider.dialectProvider(ctx, ingressDialect(ctx)).ResponsesStream(ctx, postHookRunner, postHookSpanFinalizer, key, request)
}

// CountTokens counts tokens for a request.
//
// Gemini callers get CPA's :countTokens; everyone else gets /v1/messages/count_tokens,
// because CPA publishes no OpenAI-dialect token-count route. The Anthropic route
// accepts any registered model — CPA translates — so an OpenAI-ingress count still
// succeeds there rather than failing as unsupported.
//
// Accuracy varies by upstream and is CPA's to decide: Gemini and first-party Claude
// call a real upstream count endpoint, while Codex, OpenAI-compatible and
// third-party Claude gateways return a local tokenizer estimate.
func (provider *CPAProvider) CountTokens(ctx *schemas.BifrostContext, key schemas.Key, request *schemas.BifrostResponsesRequest) (*schemas.BifrostCountTokensResponse, *schemas.BifrostError) {
	d := dialectAnthropic
	if ingressDialect(ctx) == dialectGemini {
		d = dialectGemini
	}
	return provider.dialectProvider(ctx, d).CountTokens(ctx, key, request)
}

// Compaction runs Codex remote compaction through CPA's /v1/responses/compact, which
// forwards to the upstream compaction endpoint rather than compacting locally.
// Pinned to the OpenAI dialect — it is the only dialect with the route — and
// non-streaming, which the request shape already enforces and CPA rejects otherwise.
func (provider *CPAProvider) Compaction(ctx *schemas.BifrostContext, key schemas.Key, request *schemas.BifrostCompactionRequest) (*schemas.BifrostCompactionResponse, *schemas.BifrostError) {
	return provider.dialectProvider(ctx, dialectOpenAI).Compaction(ctx, key, request)
}

// ImageGeneration generates images. Dialect follows the model family, not the
// caller's protocol — see imageDialect.
func (provider *CPAProvider) ImageGeneration(ctx *schemas.BifrostContext, key schemas.Key, request *schemas.BifrostImageGenerationRequest) (*schemas.BifrostImageGenerationResponse, *schemas.BifrostError) {
	d, bifrostErr := imageDialect(ctx, request.Model, schemas.ImageGenerationRequest)
	if bifrostErr != nil {
		return nil, bifrostErr
	}
	return provider.dialectProvider(ctx, d).ImageGeneration(ctx, key, request)
}

// ImageGenerationStream streams image generation. Only the OpenAI dialect supports
// it; Gemini image models resolve to the Gemini sub-provider, which reports the
// operation unsupported (under the cpa provider name).
func (provider *CPAProvider) ImageGenerationStream(ctx *schemas.BifrostContext, postHookRunner schemas.PostHookRunner, postHookSpanFinalizer func(context.Context), key schemas.Key, request *schemas.BifrostImageGenerationRequest) (chan *schemas.BifrostStreamChunk, *schemas.BifrostError) {
	d, bifrostErr := imageDialect(ctx, request.Model, schemas.ImageGenerationStreamRequest)
	if bifrostErr != nil {
		return nil, bifrostErr
	}
	return provider.dialectProvider(ctx, d).ImageGenerationStream(ctx, postHookRunner, postHookSpanFinalizer, key, request)
}

// ImageEdit edits images. CPA's /v1/images/edits accepts both multipart and JSON
// bodies; Gemini image models are edited through :generateContent instead.
func (provider *CPAProvider) ImageEdit(ctx *schemas.BifrostContext, key schemas.Key, request *schemas.BifrostImageEditRequest) (*schemas.BifrostImageGenerationResponse, *schemas.BifrostError) {
	d, bifrostErr := imageDialect(ctx, request.Model, schemas.ImageEditRequest)
	if bifrostErr != nil {
		return nil, bifrostErr
	}
	return provider.dialectProvider(ctx, d).ImageEdit(ctx, key, request)
}

// ImageEditStream streams image edits. OpenAI dialect only, same as
// ImageGenerationStream.
func (provider *CPAProvider) ImageEditStream(ctx *schemas.BifrostContext, postHookRunner schemas.PostHookRunner, postHookSpanFinalizer func(context.Context), key schemas.Key, request *schemas.BifrostImageEditRequest) (chan *schemas.BifrostStreamChunk, *schemas.BifrostError) {
	d, bifrostErr := imageDialect(ctx, request.Model, schemas.ImageEditStreamRequest)
	if bifrostErr != nil {
		return nil, bifrostErr
	}
	return provider.dialectProvider(ctx, d).ImageEditStream(ctx, postHookRunner, postHookSpanFinalizer, key, request)
}

// Passthrough forwards a raw request, choosing the dialect from its path.
func (provider *CPAProvider) Passthrough(ctx *schemas.BifrostContext, key schemas.Key, req *schemas.BifrostPassthroughRequest) (*schemas.BifrostPassthroughResponse, *schemas.BifrostError) {
	return provider.dialectProvider(ctx, passthroughDialect(req.Path)).Passthrough(ctx, key, req)
}

// PassthroughStream forwards a raw streaming request, choosing the dialect from its
// path.
func (provider *CPAProvider) PassthroughStream(ctx *schemas.BifrostContext, postHookRunner schemas.PostHookRunner, postHookSpanFinalizer func(context.Context), key schemas.Key, req *schemas.BifrostPassthroughRequest) (chan *schemas.BifrostStreamChunk, *schemas.BifrostError) {
	return provider.dialectProvider(ctx, passthroughDialect(req.Path)).PassthroughStream(ctx, postHookRunner, postHookSpanFinalizer, key, req)
}

// ---------------------------------------------------------------------------
// Operations CPA exposes no route for.
//
// /v1/images/variations, embeddings, rerank, OCR, audio, batches, files,
// containers and cached content are all absent from CPA's router. Video
// generation is present but only in an xAI-shaped form that does not line up with
// the Bifrost video schema, so it stays unsupported until that mapping is
// verified against a live subscription.
// ---------------------------------------------------------------------------

// ImageVariation is not supported by the CPA provider.
func (provider *CPAProvider) ImageVariation(_ *schemas.BifrostContext, _ schemas.Key, _ *schemas.BifrostImageVariationRequest) (*schemas.BifrostImageGenerationResponse, *schemas.BifrostError) {
	return nil, provider.unsupported(schemas.ImageVariationRequest)
}

// Embedding is not supported by the CPA provider.
func (provider *CPAProvider) Embedding(_ *schemas.BifrostContext, _ schemas.Key, _ *schemas.BifrostEmbeddingRequest) (*schemas.BifrostEmbeddingResponse, *schemas.BifrostError) {
	return nil, provider.unsupported(schemas.EmbeddingRequest)
}

// Rerank is not supported by the CPA provider.
func (provider *CPAProvider) Rerank(_ *schemas.BifrostContext, _ schemas.Key, _ *schemas.BifrostRerankRequest) (*schemas.BifrostRerankResponse, *schemas.BifrostError) {
	return nil, provider.unsupported(schemas.RerankRequest)
}

// OCR is not supported by the CPA provider.
func (provider *CPAProvider) OCR(_ *schemas.BifrostContext, _ schemas.Key, _ *schemas.BifrostOCRRequest) (*schemas.BifrostOCRResponse, *schemas.BifrostError) {
	return nil, provider.unsupported(schemas.OCRRequest)
}

// Speech is not supported by the CPA provider.
func (provider *CPAProvider) Speech(_ *schemas.BifrostContext, _ schemas.Key, _ *schemas.BifrostSpeechRequest) (*schemas.BifrostSpeechResponse, *schemas.BifrostError) {
	return nil, provider.unsupported(schemas.SpeechRequest)
}

// SpeechStream is not supported by the CPA provider.
func (provider *CPAProvider) SpeechStream(_ *schemas.BifrostContext, _ schemas.PostHookRunner, _ func(context.Context), _ schemas.Key, _ *schemas.BifrostSpeechRequest) (chan *schemas.BifrostStreamChunk, *schemas.BifrostError) {
	return nil, provider.unsupported(schemas.SpeechStreamRequest)
}

// Transcription is not supported by the CPA provider.
func (provider *CPAProvider) Transcription(_ *schemas.BifrostContext, _ schemas.Key, _ *schemas.BifrostTranscriptionRequest) (*schemas.BifrostTranscriptionResponse, *schemas.BifrostError) {
	return nil, provider.unsupported(schemas.TranscriptionRequest)
}

// TranscriptionStream is not supported by the CPA provider.
func (provider *CPAProvider) TranscriptionStream(_ *schemas.BifrostContext, _ schemas.PostHookRunner, _ func(context.Context), _ schemas.Key, _ *schemas.BifrostTranscriptionRequest) (chan *schemas.BifrostStreamChunk, *schemas.BifrostError) {
	return nil, provider.unsupported(schemas.TranscriptionStreamRequest)
}

// VideoGeneration is not supported by the CPA provider.
func (provider *CPAProvider) VideoGeneration(_ *schemas.BifrostContext, _ schemas.Key, _ *schemas.BifrostVideoGenerationRequest) (*schemas.BifrostVideoGenerationResponse, *schemas.BifrostError) {
	return nil, provider.unsupported(schemas.VideoGenerationRequest)
}

// VideoEdit is not supported by the CPA provider.
func (provider *CPAProvider) VideoEdit(_ *schemas.BifrostContext, _ schemas.Key, _ *schemas.BifrostVideoEditRequest) (*schemas.BifrostVideoEditResponse, *schemas.BifrostError) {
	return nil, provider.unsupported(schemas.VideoEditRequest)
}

// VideoRetrieve is not supported by the CPA provider.
func (provider *CPAProvider) VideoRetrieve(_ *schemas.BifrostContext, _ schemas.Key, _ *schemas.BifrostVideoRetrieveRequest) (*schemas.BifrostVideoGenerationResponse, *schemas.BifrostError) {
	return nil, provider.unsupported(schemas.VideoRetrieveRequest)
}

// VideoDownload is not supported by the CPA provider.
func (provider *CPAProvider) VideoDownload(_ *schemas.BifrostContext, _ schemas.Key, _ *schemas.BifrostVideoDownloadRequest) (*schemas.BifrostVideoDownloadResponse, *schemas.BifrostError) {
	return nil, provider.unsupported(schemas.VideoDownloadRequest)
}

// VideoDelete is not supported by the CPA provider.
func (provider *CPAProvider) VideoDelete(_ *schemas.BifrostContext, _ schemas.Key, _ *schemas.BifrostVideoDeleteRequest) (*schemas.BifrostVideoDeleteResponse, *schemas.BifrostError) {
	return nil, provider.unsupported(schemas.VideoDeleteRequest)
}

// VideoList is not supported by the CPA provider.
func (provider *CPAProvider) VideoList(_ *schemas.BifrostContext, _ schemas.Key, _ *schemas.BifrostVideoListRequest) (*schemas.BifrostVideoListResponse, *schemas.BifrostError) {
	return nil, provider.unsupported(schemas.VideoListRequest)
}

// VideoRemix is not supported by the CPA provider.
func (provider *CPAProvider) VideoRemix(_ *schemas.BifrostContext, _ schemas.Key, _ *schemas.BifrostVideoRemixRequest) (*schemas.BifrostVideoGenerationResponse, *schemas.BifrostError) {
	return nil, provider.unsupported(schemas.VideoRemixRequest)
}

// BatchCreate is not supported by the CPA provider.
func (provider *CPAProvider) BatchCreate(_ *schemas.BifrostContext, _ schemas.Key, _ *schemas.BifrostBatchCreateRequest) (*schemas.BifrostBatchCreateResponse, *schemas.BifrostError) {
	return nil, provider.unsupported(schemas.BatchCreateRequest)
}

// BatchList is not supported by the CPA provider.
func (provider *CPAProvider) BatchList(_ *schemas.BifrostContext, _ []schemas.Key, _ *schemas.BifrostBatchListRequest) (*schemas.BifrostBatchListResponse, *schemas.BifrostError) {
	return nil, provider.unsupported(schemas.BatchListRequest)
}

// BatchRetrieve is not supported by the CPA provider.
func (provider *CPAProvider) BatchRetrieve(_ *schemas.BifrostContext, _ []schemas.Key, _ *schemas.BifrostBatchRetrieveRequest) (*schemas.BifrostBatchRetrieveResponse, *schemas.BifrostError) {
	return nil, provider.unsupported(schemas.BatchRetrieveRequest)
}

// BatchCancel is not supported by the CPA provider.
func (provider *CPAProvider) BatchCancel(_ *schemas.BifrostContext, _ []schemas.Key, _ *schemas.BifrostBatchCancelRequest) (*schemas.BifrostBatchCancelResponse, *schemas.BifrostError) {
	return nil, provider.unsupported(schemas.BatchCancelRequest)
}

// BatchDelete is not supported by the CPA provider.
func (provider *CPAProvider) BatchDelete(_ *schemas.BifrostContext, _ []schemas.Key, _ *schemas.BifrostBatchDeleteRequest) (*schemas.BifrostBatchDeleteResponse, *schemas.BifrostError) {
	return nil, provider.unsupported(schemas.BatchDeleteRequest)
}

// BatchResults is not supported by the CPA provider.
func (provider *CPAProvider) BatchResults(_ *schemas.BifrostContext, _ []schemas.Key, _ *schemas.BifrostBatchResultsRequest) (*schemas.BifrostBatchResultsResponse, *schemas.BifrostError) {
	return nil, provider.unsupported(schemas.BatchResultsRequest)
}

// FileUpload is not supported by the CPA provider.
func (provider *CPAProvider) FileUpload(_ *schemas.BifrostContext, _ schemas.Key, _ *schemas.BifrostFileUploadRequest) (*schemas.BifrostFileUploadResponse, *schemas.BifrostError) {
	return nil, provider.unsupported(schemas.FileUploadRequest)
}

// FileList is not supported by the CPA provider.
func (provider *CPAProvider) FileList(_ *schemas.BifrostContext, _ []schemas.Key, _ *schemas.BifrostFileListRequest) (*schemas.BifrostFileListResponse, *schemas.BifrostError) {
	return nil, provider.unsupported(schemas.FileListRequest)
}

// FileRetrieve is not supported by the CPA provider.
func (provider *CPAProvider) FileRetrieve(_ *schemas.BifrostContext, _ []schemas.Key, _ *schemas.BifrostFileRetrieveRequest) (*schemas.BifrostFileRetrieveResponse, *schemas.BifrostError) {
	return nil, provider.unsupported(schemas.FileRetrieveRequest)
}

// FileDelete is not supported by the CPA provider.
func (provider *CPAProvider) FileDelete(_ *schemas.BifrostContext, _ []schemas.Key, _ *schemas.BifrostFileDeleteRequest) (*schemas.BifrostFileDeleteResponse, *schemas.BifrostError) {
	return nil, provider.unsupported(schemas.FileDeleteRequest)
}

// FileContent is not supported by the CPA provider.
func (provider *CPAProvider) FileContent(_ *schemas.BifrostContext, _ []schemas.Key, _ *schemas.BifrostFileContentRequest) (*schemas.BifrostFileContentResponse, *schemas.BifrostError) {
	return nil, provider.unsupported(schemas.FileContentRequest)
}

// CachedContentCreate is not supported by the CPA provider.
func (provider *CPAProvider) CachedContentCreate(_ *schemas.BifrostContext, _ schemas.Key, _ *schemas.BifrostCachedContentCreateRequest) (*schemas.BifrostCachedContentCreateResponse, *schemas.BifrostError) {
	return nil, provider.unsupported(schemas.CachedContentCreateRequest)
}

// CachedContentList is not supported by the CPA provider.
func (provider *CPAProvider) CachedContentList(_ *schemas.BifrostContext, _ []schemas.Key, _ *schemas.BifrostCachedContentListRequest) (*schemas.BifrostCachedContentListResponse, *schemas.BifrostError) {
	return nil, provider.unsupported(schemas.CachedContentListRequest)
}

// CachedContentRetrieve is not supported by the CPA provider.
func (provider *CPAProvider) CachedContentRetrieve(_ *schemas.BifrostContext, _ []schemas.Key, _ *schemas.BifrostCachedContentRetrieveRequest) (*schemas.BifrostCachedContentRetrieveResponse, *schemas.BifrostError) {
	return nil, provider.unsupported(schemas.CachedContentRetrieveRequest)
}

// CachedContentUpdate is not supported by the CPA provider.
func (provider *CPAProvider) CachedContentUpdate(_ *schemas.BifrostContext, _ []schemas.Key, _ *schemas.BifrostCachedContentUpdateRequest) (*schemas.BifrostCachedContentUpdateResponse, *schemas.BifrostError) {
	return nil, provider.unsupported(schemas.CachedContentUpdateRequest)
}

// CachedContentDelete is not supported by the CPA provider.
func (provider *CPAProvider) CachedContentDelete(_ *schemas.BifrostContext, _ []schemas.Key, _ *schemas.BifrostCachedContentDeleteRequest) (*schemas.BifrostCachedContentDeleteResponse, *schemas.BifrostError) {
	return nil, provider.unsupported(schemas.CachedContentDeleteRequest)
}

// ContainerCreate is not supported by the CPA provider.
func (provider *CPAProvider) ContainerCreate(_ *schemas.BifrostContext, _ schemas.Key, _ *schemas.BifrostContainerCreateRequest) (*schemas.BifrostContainerCreateResponse, *schemas.BifrostError) {
	return nil, provider.unsupported(schemas.ContainerCreateRequest)
}

// ContainerList is not supported by the CPA provider.
func (provider *CPAProvider) ContainerList(_ *schemas.BifrostContext, _ []schemas.Key, _ *schemas.BifrostContainerListRequest) (*schemas.BifrostContainerListResponse, *schemas.BifrostError) {
	return nil, provider.unsupported(schemas.ContainerListRequest)
}

// ContainerRetrieve is not supported by the CPA provider.
func (provider *CPAProvider) ContainerRetrieve(_ *schemas.BifrostContext, _ []schemas.Key, _ *schemas.BifrostContainerRetrieveRequest) (*schemas.BifrostContainerRetrieveResponse, *schemas.BifrostError) {
	return nil, provider.unsupported(schemas.ContainerRetrieveRequest)
}

// ContainerDelete is not supported by the CPA provider.
func (provider *CPAProvider) ContainerDelete(_ *schemas.BifrostContext, _ []schemas.Key, _ *schemas.BifrostContainerDeleteRequest) (*schemas.BifrostContainerDeleteResponse, *schemas.BifrostError) {
	return nil, provider.unsupported(schemas.ContainerDeleteRequest)
}

// ContainerFileCreate is not supported by the CPA provider.
func (provider *CPAProvider) ContainerFileCreate(_ *schemas.BifrostContext, _ schemas.Key, _ *schemas.BifrostContainerFileCreateRequest) (*schemas.BifrostContainerFileCreateResponse, *schemas.BifrostError) {
	return nil, provider.unsupported(schemas.ContainerFileCreateRequest)
}

// ContainerFileList is not supported by the CPA provider.
func (provider *CPAProvider) ContainerFileList(_ *schemas.BifrostContext, _ []schemas.Key, _ *schemas.BifrostContainerFileListRequest) (*schemas.BifrostContainerFileListResponse, *schemas.BifrostError) {
	return nil, provider.unsupported(schemas.ContainerFileListRequest)
}

// ContainerFileRetrieve is not supported by the CPA provider.
func (provider *CPAProvider) ContainerFileRetrieve(_ *schemas.BifrostContext, _ []schemas.Key, _ *schemas.BifrostContainerFileRetrieveRequest) (*schemas.BifrostContainerFileRetrieveResponse, *schemas.BifrostError) {
	return nil, provider.unsupported(schemas.ContainerFileRetrieveRequest)
}

// ContainerFileContent is not supported by the CPA provider.
func (provider *CPAProvider) ContainerFileContent(_ *schemas.BifrostContext, _ []schemas.Key, _ *schemas.BifrostContainerFileContentRequest) (*schemas.BifrostContainerFileContentResponse, *schemas.BifrostError) {
	return nil, provider.unsupported(schemas.ContainerFileContentRequest)
}

// ContainerFileDelete is not supported by the CPA provider.
func (provider *CPAProvider) ContainerFileDelete(_ *schemas.BifrostContext, _ []schemas.Key, _ *schemas.BifrostContainerFileDeleteRequest) (*schemas.BifrostContainerFileDeleteResponse, *schemas.BifrostError) {
	return nil, provider.unsupported(schemas.ContainerFileDeleteRequest)
}
