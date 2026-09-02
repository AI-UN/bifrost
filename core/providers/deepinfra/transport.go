package deepinfra

import (
	"net/http"
	"time"

	providerUtils "github.com/maximhq/bifrost/core/providers/utils"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/valyala/fasthttp"
)

// nativeResponse carries everything a native (non-OpenAI-shaped) call needs to build a
// Bifrost response: the decoded body, the measured latency and the upstream headers.
type nativeResponse struct {
	body    []byte
	latency time.Duration
	headers map[string]string
}

// doNativeRequest performs a unary request against one of DeepInfra's native endpoints and
// returns the decoded body. jsonBody is nil for GET requests.
//
// Errors are normalized through parseDeepInfraError so the native surface reports failures
// in the same shape as the OpenAI-compatible surface. The returned headers are populated
// even on failure so the caller can forward them.
func (provider *DeepInfraProvider) doNativeRequest(ctx *schemas.BifrostContext, method, url string, key schemas.Key, jsonBody []byte) (nativeResponse, *schemas.BifrostError) {
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	providerUtils.SetExtraHeaders(ctx, req, provider.networkConfig.ExtraHeaders, nil)

	req.SetRequestURI(url)
	req.Header.SetMethod(method)
	if keyValue := key.Value.GetValue(); keyValue != "" {
		req.Header.Set("Authorization", "Bearer "+keyValue)
	}
	if jsonBody != nil {
		req.Header.SetContentType("application/json")
		req.SetBody(jsonBody)
	}

	latency, bifrostErr, wait := providerUtils.MakeRequestWithContext(ctx, provider.client, req, resp)
	defer wait()
	if bifrostErr != nil {
		return nativeResponse{latency: latency}, bifrostErr
	}

	// Extract headers before the status check so error responses forward them too.
	headers := providerUtils.ExtractProviderResponseHeaders(resp)

	if resp.StatusCode() != fasthttp.StatusOK {
		return nativeResponse{latency: latency, headers: headers}, providerUtils.SetErrorLatency(parseDeepInfraError(resp), latency)
	}

	body, err := providerUtils.CheckAndDecodeBody(resp)
	if err != nil {
		return nativeResponse{latency: latency, headers: headers}, providerUtils.NewBifrostOperationError(schemas.ErrProviderResponseDecode, err)
	}

	return nativeResponse{body: body, latency: latency, headers: headers}, nil
}

// postNativeJSON is the JSON POST shorthand for doNativeRequest.
func (provider *DeepInfraProvider) postNativeJSON(ctx *schemas.BifrostContext, url string, key schemas.Key, jsonBody []byte) (nativeResponse, *schemas.BifrostError) {
	return provider.doNativeRequest(ctx, http.MethodPost, url, key, jsonBody)
}

// getNative is the GET shorthand for doNativeRequest.
func (provider *DeepInfraProvider) getNative(ctx *schemas.BifrostContext, url string, key schemas.Key) (nativeResponse, *schemas.BifrostError) {
	return provider.doNativeRequest(ctx, http.MethodGet, url, key, nil)
}
