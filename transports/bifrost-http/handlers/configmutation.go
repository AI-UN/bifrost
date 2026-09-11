package handlers

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/maximhq/bifrost/framework/configstore"
	"github.com/valyala/fasthttp"
)

const (
	configRevisionHeader = "X-Bifrost-Config-Revision"
	ifMatchHeader        = "If-Match"
)

type preparedConfigMutation struct {
	expectedRevision int64
	enabled          bool
}

type configMutationResponse struct {
	Revision int64  `json:"revision"`
	Pending  bool   `json:"pending,omitempty"`
	Status   string `json:"status,omitempty"`
}

func prepareConfigMutation(ctx *fasthttp.RequestCtx, store configstore.ConfigStore) (preparedConfigMutation, bool) {
	syncMode, ok := store.(configstore.ConfigSyncMode)
	if !ok || !syncMode.IsConfigSyncEnabled() {
		return preparedConfigMutation{}, true
	}

	raw := strings.TrimSpace(string(ctx.Request.Header.Peek(ifMatchHeader)))
	if raw == "" {
		SendJSONWithStatus(ctx, map[string]any{
			"error": "missing If-Match configuration revision",
		}, fasthttp.StatusPreconditionRequired)
		return preparedConfigMutation{}, false
	}
	if strings.HasPrefix(raw, "W/") {
		raw = strings.TrimSpace(strings.TrimPrefix(raw, "W/"))
	}
	raw = strings.Trim(raw, `"`)
	revision, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || revision < 0 {
		SendJSONWithStatus(ctx, map[string]any{
			"error": "invalid If-Match configuration revision",
		}, fasthttp.StatusBadRequest)
		return preparedConfigMutation{}, false
	}
	return preparedConfigMutation{
		expectedRevision: revision,
		enabled:          true,
	}, true
}

func commitConfigMutation(
	ctx context.Context,
	store configstore.ConfigStore,
	prepared preparedConfigMutation,
	mutate func(context.Context) error,
) (int64, error) {
	if mutate == nil {
		return 0, fmt.Errorf("config mutation callback cannot be nil")
	}
	if !prepared.enabled {
		return 0, mutate(ctx)
	}
	revisionStore, ok := store.(configstore.ConfigRevisionStore)
	if !ok {
		return 0, fmt.Errorf("config sync is enabled but the config store has no revision support")
	}
	return revisionStore.ExecuteConfigMutation(ctx, prepared.expectedRevision, mutate)
}

func handleConfigMutationError(ctx *fasthttp.RequestCtx, err error) bool {
	var conflict *configstore.ConfigRevisionConflictError
	if !errors.As(err, &conflict) {
		return false
	}
	setConfigRevisionHeaders(ctx, conflict.Actual)
	SendJSONWithStatus(ctx, map[string]any{
		"error":            "configuration revision conflict",
		"current_revision": conflict.Actual,
	}, fasthttp.StatusConflict)
	return true
}

func setConfigRevisionHeaders(ctx *fasthttp.RequestCtx, revision int64) {
	value := strconv.FormatInt(revision, 10)
	ctx.Response.Header.Set(configRevisionHeader, value)
	ctx.Response.Header.Set("ETag", `"`+value+`"`)
}

func setCurrentConfigRevisionHeaders(ctx *fasthttp.RequestCtx, store configstore.ConfigStore) {
	syncMode, ok := store.(configstore.ConfigSyncMode)
	if !ok || !syncMode.IsConfigSyncEnabled() {
		return
	}
	revisionStore, ok := store.(configstore.ConfigRevisionStore)
	if !ok {
		return
	}
	revision, err := revisionStore.GetConfigRevision(ctx)
	if err != nil {
		return
	}
	setConfigRevisionHeaders(ctx, revision)
}

func sendConfigMutationPending(ctx *fasthttp.RequestCtx, revision int64) {
	if revision <= 0 {
		SendError(ctx, fasthttp.StatusInternalServerError, "failed to reload in-memory state after configuration update")
		return
	}
	setConfigRevisionHeaders(ctx, revision)
	SendJSONWithStatus(ctx, configMutationResponse{
		Revision: revision,
		Pending:  true,
		Status:   "committed_pending_apply",
	}, fasthttp.StatusAccepted)
}

func sendConfigRevision(ctx *fasthttp.RequestCtx, store configstore.ConfigStore) {
	revisionStore, ok := store.(configstore.ConfigRevisionStore)
	if !ok {
		SendJSON(ctx, configMutationResponse{})
		return
	}
	revision, err := revisionStore.GetConfigRevision(ctx)
	if err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, "failed to read configuration revision")
		return
	}
	setConfigRevisionHeaders(ctx, revision)
	SendJSON(ctx, configMutationResponse{Revision: revision})
}
