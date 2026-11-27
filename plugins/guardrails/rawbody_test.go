package guardrails

import (
	"mime/multipart"
	"strings"
	"testing"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/stretchr/testify/require"
)

// plainRewriter rewrites every literal in a body, the way an integration rewriter does for the
// fields it knows.
func plainRewriter(body []byte, replacements map[string]string) ([]byte, error) {
	rewritten := string(body)
	for literal, replacement := range replacements {
		rewritten = strings.ReplaceAll(rewritten, literal, replacement)
	}
	return []byte(rewritten), nil
}

// unchangedRewriter claims success without rewriting anything, standing in for a rewriter whose
// field rules miss a value: verification must still refuse the body.
func unchangedRewriter(body []byte, _ map[string]string) ([]byte, error) { return body, nil }

func rewriterContext(t *testing.T, rewriter schemas.RawRequestBodyTextRewriter) *schemas.BifrostContext {
	t.Helper()
	ctx := newTestContext()
	ctx.SetValue(schemas.BifrostContextKeyRawRequestBodyTextRewriter, rewriter)
	return ctx
}

func TestSyncRawBodyNeverTouchesABodylessRequest(t *testing.T) {
	plugin := &Plugin{}
	apply := map[string]string{"alex@example.com": "[EMAIL_ADDRESS]"}

	require.NoError(t, plugin.syncRawBody(newTestContext(), chatRequest("mail alex@example.com"), nil))
	require.NoError(t, plugin.syncRawBody(newTestContext(), chatRequest("x"), map[string]string{}))

	ctx := newTestContext()
	ctx.SetValue(schemas.BifrostContextKeyUseRawRequestBody, true)
	require.NoError(t, plugin.syncRawBody(ctx, chatRequest("x"), apply), "no raw body to rewrite")

	// A body this build cannot rewrite is only a problem when a value had to be replaced.
	opaque := &schemas.BifrostRequest{
		RequestType:        schemas.PassthroughRequest,
		PassthroughRequest: &schemas.BifrostPassthroughRequest{BodySource: schemas.NewBytesBodySource([]byte("x"))},
	}
	require.NoError(t, plugin.syncRawBody(newTestContext(), opaque, nil))
	require.Error(t, plugin.syncRawBody(newTestContext(), opaque, apply))
}

func TestSyncRawBodyRewrite(t *testing.T) {
	apply := map[string]string{"alex@example.com": "[EMAIL_ADDRESS]"}

	t.Run("passthrough body", func(t *testing.T) {
		request := &schemas.BifrostRequest{
			RequestType:        schemas.PassthroughRequest,
			PassthroughRequest: &schemas.BifrostPassthroughRequest{Body: []byte(`{"input":"mail alex@example.com"}`)},
		}
		ctx := rewriterContext(t, plainRewriter)
		require.NoError(t, (&Plugin{}).syncRawBody(ctx, request, apply))
		require.Equal(t, `{"input":"mail [EMAIL_ADDRESS]"}`, string(request.PassthroughRequest.Body))
		require.NotContains(t, string(request.PassthroughRequest.Body), "alex@example.com")
	})

	t.Run("raw request body", func(t *testing.T) {
		request := chatRequest("clean")
		request.ChatRequest.RawRequestBody = []byte(`{"metadata":{"user_id":"alex@example.com"}}`)
		ctx := rewriterContext(t, plainRewriter)
		ctx.SetValue(schemas.BifrostContextKeyUseRawRequestBody, true)

		require.NoError(t, (&Plugin{}).syncRawBody(ctx, request, apply))
		require.JSONEq(t, `{"metadata":{"user_id":"[EMAIL_ADDRESS]"}}`, string(request.ChatRequest.RawRequestBody))
	})

	t.Run("raw body of a type the typed walk does not cover", func(t *testing.T) {
		request := &schemas.BifrostRequest{
			RequestType: schemas.RerankRequest,
			RerankRequest: &schemas.BifrostRerankRequest{
				Query:          "q",
				RawRequestBody: []byte(`{"query":"alex@example.com"}`),
			},
		}
		ctx := rewriterContext(t, plainRewriter)
		ctx.SetValue(schemas.BifrostContextKeyUseRawRequestBody, true)

		require.NoError(t, (&Plugin{}).syncRawBody(ctx, request, apply))
		require.Equal(t, `{"query":"[EMAIL_ADDRESS]"}`, string(request.RerankRequest.RawRequestBody))
	})
}

// A rewriter that reports success but leaves the value behind must not be trusted.
func TestSyncRawBodyRefusesABodyThatStillHoldsTheValue(t *testing.T) {
	apply := map[string]string{"alex@example.com": "[EMAIL_ADDRESS]"}

	passthrough := &schemas.BifrostRequest{
		RequestType:        schemas.PassthroughRequest,
		PassthroughRequest: &schemas.BifrostPassthroughRequest{Body: []byte(`{"input":"mail alex@example.com"}`)},
	}
	require.Error(t, (&Plugin{}).syncRawBody(rewriterContext(t, unchangedRewriter), passthrough, apply))

	// The value is escaped in the body, which is how a provider would read it.
	escaped := &schemas.BifrostRequest{
		RequestType:        schemas.PassthroughRequest,
		PassthroughRequest: &schemas.BifrostPassthroughRequest{Body: []byte(`{"input":"mail alex\u0040example.com"}`)},
	}
	require.Error(t, (&Plugin{}).syncRawBody(rewriterContext(t, unchangedRewriter), escaped, apply),
		"verification reads the decoded value, so an escape cannot hide it")

	raw := chatRequest("clean")
	raw.ChatRequest.RawRequestBody = []byte(`{"messages":[{"content":"alex@example.com"}]}`)
	ctx := rewriterContext(t, unchangedRewriter)
	ctx.SetValue(schemas.BifrostContextKeyUseRawRequestBody, true)
	require.Error(t, (&Plugin{}).syncRawBody(ctx, raw, apply))

	// A missing rewriter cannot rewrite either.
	rawMissing := chatRequest("clean")
	rawMissing.ChatRequest.RawRequestBody = []byte(`{"messages":[]}`)
	missing := newTestContext()
	missing.SetValue(schemas.BifrostContextKeyUseRawRequestBody, true)
	require.Error(t, (&Plugin{}).syncRawBody(missing, rawMissing, apply))
}

// The transport encoded the multipart form into the streamed body before any plugin ran, so the
// stream has to be rebuilt from the rewritten fields.
func TestSyncRawBodyReencodesMultipartForm(t *testing.T) {
	form := multipartForm(t, []string{
		"--b\r\nContent-Disposition: form-data; name=\"prompt\"\r\n\r\nmail alex@example.com\r\n",
		"--b\r\nContent-Disposition: form-data; name=\"file\"; filename=\"a.txt\"\r\nContent-Type: text/plain\r\n\r\nfile bytes\r\n",
		"--b--\r\n",
	})
	original, err := schemas.NewMultipartFormBodySource(form, "b", nil)
	require.NoError(t, err)

	request := &schemas.BifrostRequest{
		RequestType: schemas.PassthroughRequest,
		PassthroughRequest: &schemas.BifrostPassthroughRequest{
			Form:        form,
			BodySource:  original,
			SafeHeaders: map[string]string{"content-type": "multipart/form-data; boundary=b"},
		},
	}
	plugin := &Plugin{}
	apply := map[string]string{"alex@example.com": "[EMAIL_ADDRESS]"}

	// The field is rewritten in place by the inspection pass.
	form.Value["prompt"][0] = "mail [EMAIL_ADDRESS]"
	require.NoError(t, plugin.syncRawBody(newTestContext(), request, apply))
	require.NotSame(t, original, request.PassthroughRequest.BodySource)

	body, err := schemas.ReadBodySource(request.PassthroughRequest.BodySource)
	require.NoError(t, err)
	require.Contains(t, string(body), "mail [EMAIL_ADDRESS]")
	require.NotContains(t, string(body), "alex@example.com")
}

func TestSyncRawBodyRefusesMultipartFormThatStillHoldsTheValue(t *testing.T) {
	form := multipartForm(t, []string{
		"--b\r\nContent-Disposition: form-data; name=\"prompt\"\r\n\r\nmail alex@example.com\r\n",
		"--b--\r\n",
	})
	request := &schemas.BifrostRequest{
		RequestType: schemas.PassthroughRequest,
		PassthroughRequest: &schemas.BifrostPassthroughRequest{
			Form:        form,
			SafeHeaders: map[string]string{"content-type": "multipart/form-data; boundary=b"},
		},
	}
	require.Error(t, (&Plugin{}).syncRawBody(newTestContext(), request, map[string]string{"alex@example.com": "[EMAIL_ADDRESS]"}))
}

// Without the boundary the streamed form cannot be rebuilt, so the call must be refused rather
// than sent with the original fields.
func TestSyncRawBodyRefusesMultipartFormWithoutBoundary(t *testing.T) {
	form := multipartForm(t, []string{
		"--b\r\nContent-Disposition: form-data; name=\"prompt\"\r\n\r\nmail alex@example.com\r\n",
		"--b--\r\n",
	})
	source, err := schemas.NewMultipartFormBodySource(form, "b", nil)
	require.NoError(t, err)
	request := &schemas.BifrostRequest{
		RequestType: schemas.PassthroughRequest,
		PassthroughRequest: &schemas.BifrostPassthroughRequest{
			Form:        form,
			BodySource:  source,
			SafeHeaders: map[string]string{"content-type": "multipart/form-data"},
		},
	}
	require.Error(t, (&Plugin{}).syncRawBody(newTestContext(), request, map[string]string{"alex@example.com": "[EMAIL_ADDRESS]"}))
}

// An item Bifrost re-emits from preserved bytes is checked too, so a preserved copy that could not
// be rewritten blocks the request instead of leaking the value.
func TestSyncRawBodyRefusesAPreservedItemHoldingTheValue(t *testing.T) {
	var message schemas.ResponsesMessage
	require.NoError(t, schemas.Unmarshal([]byte(
		`{"type":"tool_search_call","call_id":"c1","arguments":{"query":"alex@example.com"}}`,
	), &message))
	request := &schemas.BifrostRequest{
		RequestType:      schemas.ResponsesRequest,
		ResponsesRequest: &schemas.BifrostResponsesRequest{Input: []schemas.ResponsesMessage{message}},
	}

	plugin := &Plugin{}
	ctx := newTestContext()
	apply := map[string]string{"alex@example.com": "[EMAIL_ADDRESS]"}
	require.Error(t, plugin.syncRawBody(ctx, request, apply))

	// Once the inspection pass has rewritten the preserved bytes, verification passes.
	set, err := collectSegments(ctx, request)
	require.NoError(t, err)
	set.rewrite(apply)
	require.NoError(t, plugin.syncRawBody(ctx, request, apply))
}

func multipartForm(t *testing.T, parts []string) *multipart.Form {
	t.Helper()
	form, err := multipart.NewReader(strings.NewReader(strings.Join(parts, "")), "b").ReadForm(1 << 20)
	require.NoError(t, err)
	t.Cleanup(func() { _ = form.RemoveAll() })
	return form
}
