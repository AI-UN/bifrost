package handlers

import (
	"context"
	"sync"
	"time"

	"github.com/fasthttp/router"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
	"github.com/valyala/fasthttp"
)

// HealthHandler manages HTTP requests for liveness and readiness checks.
type HealthHandler struct {
	config  *lib.Config
	isReady func() bool
}

// NewHealthHandler creates a new health handler instance.
func NewHealthHandler(config *lib.Config, isReady func() bool) *HealthHandler {
	return &HealthHandler{
		config:  config,
		isReady: isReady,
	}
}

// RegisterRoutes registers the health-related routes.
func (h *HealthHandler) RegisterRoutes(r *router.Router, middlewares ...schemas.BifrostHTTPMiddleware) {
	r.GET("/health", lib.ChainMiddlewares(h.getHealth, middlewares...))
	r.GET("/ready", lib.ChainMiddlewares(h.getReady, middlewares...))
}

// getReady handles GET /ready and gates traffic until the initial config snapshot is applied.
func (h *HealthHandler) getReady(ctx *fasthttp.RequestCtx) {
	if h.isReady != nil && !h.isReady() {
		SendError(ctx, fasthttp.StatusServiceUnavailable, "server is not ready")
		return
	}
	SendJSON(ctx, map[string]string{"status": "ok"})
}

// getHealth handles GET /health and reports liveness dependencies.
func (h *HealthHandler) getHealth(ctx *fasthttp.RequestCtx) {
	// If DB pings are disabled, just return OK
	if h.config.ClientConfig.DisableDBPingsInHealth {
		SendJSON(ctx, map[string]any{"status": "ok", "components": map[string]any{"db_pings": "disabled"}})
		return
	}
	// Pinging config store
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var errors []string
	var mu sync.Mutex
	var wg sync.WaitGroup

	if h.config.ConfigStore != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := h.config.ConfigStore.Ping(reqCtx); err != nil {
				mu.Lock()
				errors = append(errors, "config store not available")
				mu.Unlock()
			}
		}()
	}

	// Pinging log store
	if h.config.LogsStore != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := h.config.LogsStore.Ping(reqCtx); err != nil {
				mu.Lock()
				errors = append(errors, "log store not available")
				mu.Unlock()
			}
		}()
	}

	// Pinging vector store
	if h.config.VectorStore != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := h.config.VectorStore.Ping(reqCtx); err != nil {
				mu.Lock()
				errors = append(errors, "vector store not available")
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	if len(errors) > 0 {
		SendError(ctx, fasthttp.StatusServiceUnavailable, errors[0])
		return
	}
	SendJSON(ctx, map[string]any{"status": "ok", "components": map[string]any{"db_pings": "ok"}})
}
