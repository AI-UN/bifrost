package handlers

import (
	"testing"

	"github.com/valyala/fasthttp"
)

func TestHealthHandlerReadinessGate(t *testing.T) {
	t.Run("not ready", func(t *testing.T) {
		handler := NewHealthHandler(nil, func() bool { return false })
		ctx := &fasthttp.RequestCtx{}

		handler.getReady(ctx)

		if got := ctx.Response.StatusCode(); got != fasthttp.StatusServiceUnavailable {
			t.Fatalf("status = %d, want %d", got, fasthttp.StatusServiceUnavailable)
		}
	})

	t.Run("ready", func(t *testing.T) {
		handler := NewHealthHandler(nil, func() bool { return true })
		ctx := &fasthttp.RequestCtx{}

		handler.getReady(ctx)

		if got := ctx.Response.StatusCode(); got != fasthttp.StatusOK {
			t.Fatalf("status = %d, want %d", got, fasthttp.StatusOK)
		}
	})
}
