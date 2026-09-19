package handlers

import (
	"context"
	"errors"
	"time"

	"github.com/bytedance/sonic"
	"github.com/fasthttp/router"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
	"github.com/maximhq/bifrost/framework/featureflags"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
	"github.com/valyala/fasthttp"
)

// FeatureFlagsHandler serves the toggle UI/API. Persisted overrides are the
// source of truth; the in-memory store is updated only after the DB commit.
type FeatureFlagsHandler struct {
	store       *featureflags.Store
	configStore configstore.ConfigStore
}

// NewFeatureFlagsHandler wires the handler to its dependencies. Both must
// be non-nil at server boot; the handler intentionally does not lazily
// resolve them because feature flag state is needed during request
// dispatch and a missing store would cause silent off-by-default behavior.
func NewFeatureFlagsHandler(store *featureflags.Store, configStore configstore.ConfigStore) *FeatureFlagsHandler {
	return &FeatureFlagsHandler{store: store, configStore: configStore}
}

// RegisterRoutes mounts the feature flag endpoints. Only GET and PUT are
// exposed: flags are code-declared via featureflags.Register, so there is
// nothing to "create" or "delete" via the API. Stale DB rows for
// unregistered flags surface in the list with registered=false so
// operators can see them, but they cannot be toggled or removed.
func (h *FeatureFlagsHandler) RegisterRoutes(r *router.Router, middlewares ...schemas.BifrostHTTPMiddleware) {
	r.GET("/api/feature-flags", lib.ChainMiddlewares(h.listFlags, middlewares...))
	r.PUT("/api/feature-flags/{id}", lib.ChainMiddlewares(h.updateFlag, middlewares...))
}

// featureFlagsListResponse keeps the wire format flexible: wrapping the
// array in an object lets us add pagination / counts later without breaking
// existing UI clients.
type featureFlagsListResponse struct {
	Flags []featureflags.FlagStatus `json:"flags"`
}

func (h *FeatureFlagsHandler) listFlags(ctx *fasthttp.RequestCtx) {
	flags := h.store.List()
	setCurrentConfigRevisionHeaders(ctx, h.configStore)
	SendJSON(ctx, featureFlagsListResponse{Flags: flags})
}

type updateFlagRequest struct {
	// Pointer so that a missing field decodes as nil rather than false;
	// otherwise a PUT with an empty body silently disables the flag.
	Enabled *bool `json:"enabled"`
}

func (h *FeatureFlagsHandler) updateFlag(ctx *fasthttp.RequestCtx) {
	if h.configStore == nil {
		SendError(ctx, fasthttp.StatusServiceUnavailable, "Config store not available")
		return
	}

	id, ok := ctx.UserValue("id").(string)
	if !ok || id == "" {
		SendError(ctx, fasthttp.StatusBadRequest, "Invalid flag id")
		return
	}

	var req updateFlagRequest
	if err := sonic.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, "Invalid request payload")
		return
	}
	if req.Enabled == nil {
		SendError(ctx, fasthttp.StatusBadRequest, "Missing required field: enabled")
		return
	}

	prepared, ok := prepareConfigMutation(ctx, h.configStore)
	if !ok {
		return
	}

	status, err := h.store.Status(id)
	switch {
	case errors.Is(err, featureflags.ErrFlagNotFound):
		SendError(ctx, fasthttp.StatusNotFound, "Feature flag is not registered")
		return
	case err != nil:
		SendError(ctx, fasthttp.StatusInternalServerError, "Failed to read feature flag: "+err.Error())
		return
	case !status.Registered:
		SendError(ctx, fasthttp.StatusNotFound, "Feature flag is not registered")
		return
	case status.EnterpriseOnly && status.Locked:
		SendError(ctx, fasthttp.StatusForbidden, "Feature flag is enterprise-only")
		return
	case status.Locked:
		SendError(ctx, fasthttp.StatusConflict, "Feature flag is locked by config.json / Helm")
		return
	}

	enabled := *req.Enabled
	writtenAt := time.Now().UnixNano()
	revision, err := commitConfigMutation(ctx, h.configStore, prepared, func(mutationCtx context.Context) error {
		return h.configStore.UpsertFeatureFlag(mutationCtx, id, enabled, writtenAt)
	})
	if err != nil {
		if handleConfigMutationError(ctx, err) {
			return
		}
		SendError(ctx, fasthttp.StatusInternalServerError, "Failed to persist feature flag: "+err.Error())
		return
	}

	h.store.ApplyRemote(id, enabled, writtenAt)
	appliedStatus, err := h.store.Status(id)
	if err != nil || appliedStatus.Enabled != enabled || appliedStatus.UpdatedAt != writtenAt {
		sendConfigMutationPending(ctx, revision)
		return
	}

	if prepared.enabled {
		setConfigRevisionHeaders(ctx, revision)
	}
	SendJSON(ctx, appliedStatus)
}
