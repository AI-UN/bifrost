package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/fasthttp/router"
	"github.com/google/uuid"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
	"github.com/maximhq/bifrost/framework/configstore/tables"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
	"github.com/valyala/fasthttp"
)

// PromptCacheReloader is implemented by the prompts plugin to allow the HTTP handler
// to trigger an in-memory cache refresh after any repository mutation.
type PromptCacheReloader interface {
	Reload(ctx context.Context) error
}

// PromptsHandler handles prompt repository endpoints
type PromptsHandler struct {
	store    configstore.ConfigStore
	reloader PromptCacheReloader // optional; nil when the prompts plugin is not loaded
}

// NewPromptsHandler creates a new PromptsHandler.
// reloader may be nil; when set, the in-memory prompt cache is refreshed after mutations.
func NewPromptsHandler(store configstore.ConfigStore, reloader PromptCacheReloader) *PromptsHandler {
	if store == nil {
		return nil
	}
	return &PromptsHandler{store: store, reloader: reloader}
}

// reloadCache triggers a cache refresh if a reloader is configured.
// In synchronized mode, callers surface refresh failures as committed-but-pending.
func (h *PromptsHandler) reloadCache(ctx context.Context) error {
	if h.reloader == nil {
		return nil
	}
	return h.reloader.Reload(ctx)
}

func (h *PromptsHandler) applyPromptMutation(
	ctx *fasthttp.RequestCtx,
	prepared preparedConfigMutation,
	revision int64,
) bool {
	if err := h.reloadCache(ctx); err != nil {
		logger.Error("failed to reload prompt cache: %v", err)
		if prepared.enabled {
			sendConfigMutationPending(ctx, revision)
			return false
		}
	}
	if prepared.enabled {
		setConfigRevisionHeaders(ctx, revision)
	}
	return true
}

// RegisterRoutes registers the routes for the PromptsHandler
func (h *PromptsHandler) RegisterRoutes(r *router.Router, middlewares ...schemas.BifrostHTTPMiddleware) {
	// Folders
	r.GET("/api/prompt-repo/folders", lib.ChainMiddlewares(h.getFolders, middlewares...))
	r.GET("/api/prompt-repo/folders/{id}", lib.ChainMiddlewares(h.getFolderByID, middlewares...))
	r.POST("/api/prompt-repo/folders", lib.ChainMiddlewares(h.createFolder, middlewares...))
	r.PUT("/api/prompt-repo/folders/{id}", lib.ChainMiddlewares(h.updateFolder, middlewares...))
	r.DELETE("/api/prompt-repo/folders/{id}", lib.ChainMiddlewares(h.deleteFolder, middlewares...))

	// Prompts
	r.GET("/api/prompt-repo/prompts", lib.ChainMiddlewares(h.getPrompts, middlewares...))
	r.GET("/api/prompt-repo/prompts/{id}", lib.ChainMiddlewares(h.getPromptByID, middlewares...))
	r.POST("/api/prompt-repo/prompts", lib.ChainMiddlewares(h.createPrompt, middlewares...))
	r.PUT("/api/prompt-repo/prompts/{id}", lib.ChainMiddlewares(h.updatePrompt, middlewares...))
	r.DELETE("/api/prompt-repo/prompts/{id}", lib.ChainMiddlewares(h.deletePrompt, middlewares...))

	// Versions
	r.GET("/api/prompt-repo/prompts/{id}/versions", lib.ChainMiddlewares(h.getPromptVersions, middlewares...))
	r.GET("/api/prompt-repo/versions/{id}", lib.ChainMiddlewares(h.getVersionByID, middlewares...))
	r.POST("/api/prompt-repo/prompts/{id}/versions", lib.ChainMiddlewares(h.createVersion, middlewares...))
	r.DELETE("/api/prompt-repo/versions/{id}", lib.ChainMiddlewares(h.deleteVersion, middlewares...))

	// Sessions
	r.GET("/api/prompt-repo/prompts/{id}/sessions", lib.ChainMiddlewares(h.getPromptSessions, middlewares...))
	r.GET("/api/prompt-repo/sessions/{id}", lib.ChainMiddlewares(h.getSessionByID, middlewares...))
	r.POST("/api/prompt-repo/prompts/{id}/sessions", lib.ChainMiddlewares(h.createSession, middlewares...))
	r.PUT("/api/prompt-repo/sessions/{id}", lib.ChainMiddlewares(h.updateSession, middlewares...))
	r.DELETE("/api/prompt-repo/sessions/{id}", lib.ChainMiddlewares(h.deleteSession, middlewares...))
	r.PUT("/api/prompt-repo/sessions/{id}/rename", lib.ChainMiddlewares(h.renameSession, middlewares...))
	r.POST("/api/prompt-repo/sessions/{id}/commit", lib.ChainMiddlewares(h.commitSession, middlewares...))
}

// ============================================================================
// Request/Response Types
// ============================================================================

// CreateFolderRequest represents the request body for creating a folder
type CreateFolderRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
}

// UpdateFolderRequest represents the request body for updating a folder
type UpdateFolderRequest struct {
	Name              string  `json:"name"`
	Description       *string `json:"description,omitempty"`
	DescriptionExists bool    `json:"-"` // true when description key is present in JSON (even if null)
}

// UnmarshalJSON implements custom unmarshalling to detect presence of description key
func (r *UpdateFolderRequest) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if v, ok := raw["name"]; ok {
		if err := json.Unmarshal(v, &r.Name); err != nil {
			return err
		}
	}
	if v, ok := raw["description"]; ok {
		if err := json.Unmarshal(v, &r.Description); err != nil {
			return err
		}
		r.DescriptionExists = true
	}
	return nil
}

// CreatePromptRequest represents the request body for creating a prompt
type CreatePromptRequest struct {
	Name     string  `json:"name"`
	FolderID *string `json:"folder_id,omitempty"`
}

// UpdatePromptRequest represents the request body for updating a prompt
type UpdatePromptRequest struct {
	Name           string  `json:"name"`
	FolderID       *string `json:"folder_id"`
	FolderIDExists bool    `json:"-"` // true when folder_id key is present in JSON (even if null)
}

// CreateVersionRequest represents the request body for creating a version
type CreateVersionRequest struct {
	CommitMessage string                 `json:"commit_message"`
	Messages      []tables.PromptMessage `json:"messages"`
	ModelParams   tables.ModelParams     `json:"model_params"`
	Provider      string                 `json:"provider"`
	Model         string                 `json:"model"`
	Variables     tables.PromptVariables `json:"variables,omitempty"`
}

// CreateSessionRequest represents the request body for creating a session
type CreateSessionRequest struct {
	Name        string                 `json:"name"`
	VersionID   *uint                  `json:"version_id,omitempty"`
	Messages    []tables.PromptMessage `json:"messages,omitempty"`
	ModelParams tables.ModelParams     `json:"model_params"`
	Provider    string                 `json:"provider"`
	Model       string                 `json:"model"`
	Variables   tables.PromptVariables `json:"variables,omitempty"`
}

// UpdateSessionRequest represents the request body for updating a session
type UpdateSessionRequest struct {
	Name        string                 `json:"name"`
	Messages    []tables.PromptMessage `json:"messages"`
	ModelParams tables.ModelParams     `json:"model_params"`
	Provider    string                 `json:"provider"`
	Model       string                 `json:"model"`
	Variables   tables.PromptVariables `json:"variables,omitempty"`
}

// RenameSessionRequest represents the request body for renaming a session
type RenameSessionRequest struct {
	Name string `json:"name"`
}

// CommitSessionRequest represents the request body for committing a session as a version
type CommitSessionRequest struct {
	CommitMessage  string `json:"commit_message"`
	MessageIndices *[]int `json:"message_indices,omitempty"` // optional: indices of messages to include (0-based). If nil/absent, all messages are included.
}

// ============================================================================
// Folder Handlers
// ============================================================================

// getFolders handles GET /api/prompt-repo/folders
func (h *PromptsHandler) getFolders(ctx *fasthttp.RequestCtx) {
	folders, err := h.store.GetFolders(ctx)
	if err != nil {
		logger.Error("failed to get folders: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}

	setCurrentConfigRevisionHeaders(ctx, h.store)
	SendJSON(ctx, map[string]any{
		"folders": folders,
	})
}

// getFolderByID handles GET /api/prompt-repo/folders/{id}
func (h *PromptsHandler) getFolderByID(ctx *fasthttp.RequestCtx) {
	idVal := ctx.UserValue("id")
	if idVal == nil {
		SendError(ctx, fasthttp.StatusBadRequest, "folder ID is required")
		return
	}
	id, ok := idVal.(string)
	if !ok {
		SendError(ctx, fasthttp.StatusBadRequest, "invalid folder ID")
		return
	}

	folder, err := h.store.GetFolderByID(ctx, id)
	if err != nil {
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, fasthttp.StatusNotFound, "folder not found")
			return
		}
		logger.Error("failed to get folder: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}

	setCurrentConfigRevisionHeaders(ctx, h.store)
	SendJSON(ctx, map[string]any{
		"folder": folder,
	})
}

// createFolder handles POST /api/prompt-repo/folders
func (h *PromptsHandler) createFolder(ctx *fasthttp.RequestCtx) {
	var req CreateFolderRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		SendError(ctx, fasthttp.StatusBadRequest, "name is required")
		return
	}
	prepared, ok := prepareConfigMutation(ctx, h.store)
	if !ok {
		return
	}

	folder := &tables.TableFolder{ID: uuid.New().String(), Name: req.Name, Description: req.Description}
	revision, err := commitConfigMutation(ctx, h.store, prepared, func(mutationCtx context.Context) error {
		return h.store.CreateFolder(mutationCtx, folder)
	})
	if err != nil {
		if handleConfigMutationError(ctx, err) {
			return
		}
		logger.Error("failed to create folder: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	if !h.applyPromptMutation(ctx, prepared, revision) {
		return
	}
	SendJSON(ctx, map[string]any{"folder": folder})
}

// updateFolder handles PUT /api/prompt-repo/folders/{id}
func (h *PromptsHandler) updateFolder(ctx *fasthttp.RequestCtx) {
	idVal := ctx.UserValue("id")
	if idVal == nil {
		SendError(ctx, fasthttp.StatusBadRequest, "folder ID is required")
		return
	}
	id, ok := idVal.(string)
	if !ok {
		SendError(ctx, fasthttp.StatusBadRequest, "invalid folder ID")
		return
	}
	var req UpdateFolderRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, "invalid request body")
		return
	}
	prepared, ok := prepareConfigMutation(ctx, h.store)
	if !ok {
		return
	}

	folder, err := h.store.GetFolderByID(ctx, id)
	if err != nil {
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, fasthttp.StatusNotFound, "folder not found")
			return
		}
		logger.Error("failed to get folder: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	if req.Name != "" {
		folder.Name = req.Name
	}
	if req.DescriptionExists {
		folder.Description = req.Description
	}

	revision, err := commitConfigMutation(ctx, h.store, prepared, func(mutationCtx context.Context) error {
		return h.store.UpdateFolder(mutationCtx, folder)
	})
	if err != nil {
		if handleConfigMutationError(ctx, err) {
			return
		}
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, fasthttp.StatusNotFound, "folder not found")
			return
		}
		logger.Error("failed to update folder: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	if !h.applyPromptMutation(ctx, prepared, revision) {
		return
	}
	SendJSON(ctx, map[string]any{"folder": folder})
}

// deleteFolder handles DELETE /api/prompt-repo/folders/{id}
func (h *PromptsHandler) deleteFolder(ctx *fasthttp.RequestCtx) {
	idVal := ctx.UserValue("id")
	if idVal == nil {
		SendError(ctx, fasthttp.StatusBadRequest, "folder ID is required")
		return
	}
	id, ok := idVal.(string)
	if !ok {
		SendError(ctx, fasthttp.StatusBadRequest, "invalid folder ID")
		return
	}
	prepared, ok := prepareConfigMutation(ctx, h.store)
	if !ok {
		return
	}

	revision, err := commitConfigMutation(ctx, h.store, prepared, func(mutationCtx context.Context) error {
		return h.store.DeleteFolder(mutationCtx, id)
	})
	if err != nil {
		if handleConfigMutationError(ctx, err) {
			return
		}
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, fasthttp.StatusNotFound, "folder not found")
			return
		}
		logger.Error("failed to delete folder: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	if !h.applyPromptMutation(ctx, prepared, revision) {
		return
	}
	SendJSON(ctx, map[string]any{"message": "folder deleted successfully"})
}

// ============================================================================
// Prompt Handlers
// ============================================================================

// getPrompts handles GET /api/prompt-repo/prompts
func (h *PromptsHandler) getPrompts(ctx *fasthttp.RequestCtx) {
	var folderID *string
	if folderIDParam := string(ctx.QueryArgs().Peek("folder_id")); folderIDParam != "" {
		folderID = &folderIDParam
	}

	prompts, err := h.store.GetPrompts(ctx, folderID)
	if err != nil {
		logger.Error("failed to get prompts: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}

	setCurrentConfigRevisionHeaders(ctx, h.store)
	SendJSON(ctx, map[string]any{
		"prompts": prompts,
	})
}

// getPromptByID handles GET /api/prompt-repo/prompts/{id}
func (h *PromptsHandler) getPromptByID(ctx *fasthttp.RequestCtx) {
	idVal := ctx.UserValue("id")
	if idVal == nil {
		SendError(ctx, fasthttp.StatusBadRequest, "prompt ID is required")
		return
	}
	id, ok := idVal.(string)
	if !ok {
		SendError(ctx, fasthttp.StatusBadRequest, "invalid prompt ID")
		return
	}

	prompt, err := h.store.GetPromptByID(ctx, id)
	if err != nil {
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, fasthttp.StatusNotFound, "prompt not found")
			return
		}
		logger.Error("failed to get prompt: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}

	setCurrentConfigRevisionHeaders(ctx, h.store)
	SendJSON(ctx, map[string]any{
		"prompt": prompt,
	})
}

// createPrompt handles POST /api/prompt-repo/prompts
func (h *PromptsHandler) createPrompt(ctx *fasthttp.RequestCtx) {
	var req CreatePromptRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		SendError(ctx, fasthttp.StatusBadRequest, "name is required")
		return
	}
	prepared, ok := prepareConfigMutation(ctx, h.store)
	if !ok {
		return
	}
	if req.FolderID != nil && *req.FolderID == "" {
		req.FolderID = nil
	}
	if req.FolderID != nil {
		if _, err := h.store.GetFolderByID(ctx, *req.FolderID); err != nil {
			if errors.Is(err, configstore.ErrNotFound) {
				SendError(ctx, fasthttp.StatusBadRequest, "folder not found")
				return
			}
			logger.Error("failed to get folder: %v", err)
			SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
			return
		}
	}
	prompt := &tables.TablePrompt{ID: uuid.New().String(), Name: req.Name, FolderID: req.FolderID}
	revision, err := commitConfigMutation(ctx, h.store, prepared, func(mutationCtx context.Context) error {
		return h.store.CreatePrompt(mutationCtx, prompt)
	})
	if err != nil {
		if handleConfigMutationError(ctx, err) {
			return
		}
		logger.Error("failed to create prompt: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	if !h.applyPromptMutation(ctx, prepared, revision) {
		return
	}
	SendJSON(ctx, map[string]any{"prompt": prompt})
}

// updatePrompt handles PUT /api/prompt-repo/prompts/{id}
func (h *PromptsHandler) updatePrompt(ctx *fasthttp.RequestCtx) {
	idVal := ctx.UserValue("id")
	if idVal == nil {
		SendError(ctx, fasthttp.StatusBadRequest, "prompt ID is required")
		return
	}
	id, ok := idVal.(string)
	if !ok {
		SendError(ctx, fasthttp.StatusBadRequest, "invalid prompt ID")
		return
	}
	var req UpdatePromptRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, "invalid request body")
		return
	}
	var rawFields map[string]json.RawMessage
	if err := json.Unmarshal(ctx.PostBody(), &rawFields); err == nil {
		_, req.FolderIDExists = rawFields["folder_id"]
	}
	prepared, ok := prepareConfigMutation(ctx, h.store)
	if !ok {
		return
	}

	prompt, err := h.store.GetPromptByID(ctx, id)
	if err != nil {
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, fasthttp.StatusNotFound, "prompt not found")
			return
		}
		logger.Error("failed to get prompt: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	if req.Name != "" {
		prompt.Name = req.Name
	}
	if req.FolderIDExists {
		if req.FolderID == nil || *req.FolderID == "" {
			prompt.FolderID = nil
		} else {
			if _, err := h.store.GetFolderByID(ctx, *req.FolderID); err != nil {
				if errors.Is(err, configstore.ErrNotFound) {
					SendError(ctx, fasthttp.StatusBadRequest, "folder not found")
					return
				}
				logger.Error("failed to get folder: %v", err)
				SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
				return
			}
			prompt.FolderID = req.FolderID
		}
	}
	revision, err := commitConfigMutation(ctx, h.store, prepared, func(mutationCtx context.Context) error {
		return h.store.UpdatePrompt(mutationCtx, prompt)
	})
	if err != nil {
		if handleConfigMutationError(ctx, err) {
			return
		}
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, fasthttp.StatusNotFound, "prompt not found")
			return
		}
		logger.Error("failed to update prompt: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	if !h.applyPromptMutation(ctx, prepared, revision) {
		return
	}
	SendJSON(ctx, map[string]any{"prompt": prompt})
}

// deletePrompt handles DELETE /api/prompt-repo/prompts/{id}
func (h *PromptsHandler) deletePrompt(ctx *fasthttp.RequestCtx) {
	idVal := ctx.UserValue("id")
	if idVal == nil {
		SendError(ctx, fasthttp.StatusBadRequest, "prompt ID is required")
		return
	}
	id, ok := idVal.(string)
	if !ok {
		SendError(ctx, fasthttp.StatusBadRequest, "invalid prompt ID")
		return
	}
	prepared, ok := prepareConfigMutation(ctx, h.store)
	if !ok {
		return
	}
	revision, err := commitConfigMutation(ctx, h.store, prepared, func(mutationCtx context.Context) error {
		return h.store.DeletePrompt(mutationCtx, id)
	})
	if err != nil {
		if handleConfigMutationError(ctx, err) {
			return
		}
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, fasthttp.StatusNotFound, "prompt not found")
			return
		}
		logger.Error("failed to delete prompt: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	if !h.applyPromptMutation(ctx, prepared, revision) {
		return
	}
	SendJSON(ctx, map[string]any{"message": "prompt deleted successfully"})
}

// ============================================================================
// Version Handlers
// ============================================================================

// getPromptVersions handles GET /api/prompt-repo/prompts/{id}/versions
func (h *PromptsHandler) getPromptVersions(ctx *fasthttp.RequestCtx) {
	idVal := ctx.UserValue("id")
	if idVal == nil {
		SendError(ctx, fasthttp.StatusBadRequest, "prompt ID is required")
		return
	}
	promptID, ok := idVal.(string)
	if !ok {
		SendError(ctx, fasthttp.StatusBadRequest, "invalid prompt ID")
		return
	}

	versions, err := h.store.GetPromptVersions(ctx, promptID)
	if err != nil {
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, fasthttp.StatusNotFound, "prompt not found")
			return
		}
		logger.Error("failed to get versions: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}

	setCurrentConfigRevisionHeaders(ctx, h.store)
	SendJSON(ctx, map[string]any{
		"versions": versions,
	})
}

// getVersionByID handles GET /api/prompt-repo/versions/{id}
func (h *PromptsHandler) getVersionByID(ctx *fasthttp.RequestCtx) {
	idVal := ctx.UserValue("id")
	if idVal == nil {
		SendError(ctx, fasthttp.StatusBadRequest, "version ID is required")
		return
	}
	idStr, ok := idVal.(string)
	if !ok {
		SendError(ctx, fasthttp.StatusBadRequest, "invalid version ID")
		return
	}
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, "invalid version ID")
		return
	}

	version, err := h.store.GetPromptVersionByID(ctx, uint(id))
	if err != nil {
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, fasthttp.StatusNotFound, "version not found")
			return
		}
		logger.Error("failed to get version: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}

	setCurrentConfigRevisionHeaders(ctx, h.store)
	SendJSON(ctx, map[string]any{
		"version": version,
	})
}

// createVersion handles POST /api/prompt-repo/prompts/{id}/versions
func (h *PromptsHandler) createVersion(ctx *fasthttp.RequestCtx) {
	idVal := ctx.UserValue("id")
	if idVal == nil {
		SendError(ctx, fasthttp.StatusBadRequest, "prompt ID is required")
		return
	}
	promptID, ok := idVal.(string)
	if !ok {
		SendError(ctx, fasthttp.StatusBadRequest, "invalid prompt ID")
		return
	}
	var req CreateVersionRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, "invalid request body")
		return
	}
	prepared, ok := prepareConfigMutation(ctx, h.store)
	if !ok {
		return
	}
	if _, err := h.store.GetPromptByID(ctx, promptID); err != nil {
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, fasthttp.StatusNotFound, "prompt not found")
			return
		}
		logger.Error("failed to get prompt: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	messages := make([]tables.TablePromptVersionMessage, 0, len(req.Messages))
	for _, msg := range req.Messages {
		messages = append(messages, tables.TablePromptVersionMessage{PromptID: promptID, Message: msg})
	}
	var versionVars tables.PromptVariables
	if len(req.Variables) > 0 {
		versionVars = make(tables.PromptVariables, len(req.Variables))
		for key := range req.Variables {
			versionVars[key] = ""
		}
	}
	version := &tables.TablePromptVersion{
		PromptID: promptID, CommitMessage: req.CommitMessage, ModelParams: req.ModelParams,
		Provider: req.Provider, Model: req.Model, Variables: versionVars, Messages: messages,
	}
	revision, err := commitConfigMutation(ctx, h.store, prepared, func(mutationCtx context.Context) error {
		return h.store.CreatePromptVersion(mutationCtx, version)
	})
	if err != nil {
		if handleConfigMutationError(ctx, err) {
			return
		}
		logger.Error("failed to create version: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	if !h.applyPromptMutation(ctx, prepared, revision) {
		return
	}
	SendJSON(ctx, map[string]any{"version": version})
}

// deleteVersion handles DELETE /api/prompt-repo/versions/{id}
func (h *PromptsHandler) deleteVersion(ctx *fasthttp.RequestCtx) {
	idVal := ctx.UserValue("id")
	if idVal == nil {
		SendError(ctx, fasthttp.StatusBadRequest, "version ID is required")
		return
	}
	idStr, ok := idVal.(string)
	if !ok {
		SendError(ctx, fasthttp.StatusBadRequest, "invalid version ID")
		return
	}
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, "invalid version ID")
		return
	}
	prepared, ok := prepareConfigMutation(ctx, h.store)
	if !ok {
		return
	}
	revision, err := commitConfigMutation(ctx, h.store, prepared, func(mutationCtx context.Context) error {
		return h.store.DeletePromptVersion(mutationCtx, uint(id))
	})
	if err != nil {
		if handleConfigMutationError(ctx, err) {
			return
		}
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, fasthttp.StatusNotFound, "version not found")
			return
		}
		logger.Error("failed to delete version: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	if !h.applyPromptMutation(ctx, prepared, revision) {
		return
	}
	SendJSON(ctx, map[string]any{"message": "version deleted successfully"})
}

// ============================================================================
// Session Handlers
// ============================================================================

// getPromptSessions handles GET /api/prompt-repo/prompts/{id}/sessions
func (h *PromptsHandler) getPromptSessions(ctx *fasthttp.RequestCtx) {
	idVal := ctx.UserValue("id")
	if idVal == nil {
		SendError(ctx, fasthttp.StatusBadRequest, "prompt ID is required")
		return
	}
	promptID, ok := idVal.(string)
	if !ok {
		SendError(ctx, fasthttp.StatusBadRequest, "invalid prompt ID")
		return
	}

	sessions, err := h.store.GetPromptSessions(ctx, promptID)
	if err != nil {
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, fasthttp.StatusNotFound, "prompt not found")
			return
		}
		logger.Error("failed to get sessions: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}

	SendJSON(ctx, map[string]any{
		"sessions": sessions,
	})
}

// getSessionByID handles GET /api/prompt-repo/sessions/{id}
func (h *PromptsHandler) getSessionByID(ctx *fasthttp.RequestCtx) {
	idVal := ctx.UserValue("id")
	if idVal == nil {
		SendError(ctx, fasthttp.StatusBadRequest, "session ID is required")
		return
	}
	idStr, ok := idVal.(string)
	if !ok {
		SendError(ctx, fasthttp.StatusBadRequest, "invalid session ID")
		return
	}
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, "invalid session ID")
		return
	}

	session, err := h.store.GetPromptSessionByID(ctx, uint(id))
	if err != nil {
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, fasthttp.StatusNotFound, "session not found")
			return
		}
		logger.Error("failed to get session: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}

	SendJSON(ctx, map[string]any{
		"session": session,
	})
}

// createSession handles POST /api/prompt-repo/prompts/{id}/sessions
func (h *PromptsHandler) createSession(ctx *fasthttp.RequestCtx) {
	idVal := ctx.UserValue("id")
	if idVal == nil {
		SendError(ctx, fasthttp.StatusBadRequest, "prompt ID is required")
		return
	}
	promptID, ok := idVal.(string)
	if !ok {
		SendError(ctx, fasthttp.StatusBadRequest, "invalid prompt ID")
		return
	}

	var req CreateSessionRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, "invalid request body")
		return
	}

	// Verify prompt exists
	if _, err := h.store.GetPromptByID(ctx, promptID); err != nil {
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, fasthttp.StatusNotFound, "prompt not found")
			return
		}
		logger.Error("failed to get prompt: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}

	// If version_id is provided, copy messages from that version
	var messages []tables.TablePromptSessionMessage
	if req.VersionID != nil {
		version, err := h.store.GetPromptVersionByID(ctx, *req.VersionID)
		if err != nil {
			if errors.Is(err, configstore.ErrNotFound) {
				SendError(ctx, fasthttp.StatusBadRequest, "version not found")
				return
			}
			logger.Error("failed to get version: %v", err)
			SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
			return
		}
		// Verify version belongs to this prompt
		if version.PromptID != promptID {
			SendError(ctx, fasthttp.StatusBadRequest, "version does not belong to this prompt")
			return
		}
		// Copy messages from version
		for _, msg := range version.Messages {
			messages = append(messages, tables.TablePromptSessionMessage{
				PromptID: promptID,
				Message:  msg.Message,
			})
		}
		// Use version's model params, provider, model if not provided
		if req.Provider == "" {
			req.Provider = version.Provider
		}
		if req.Model == "" {
			req.Model = version.Model
		}
		if len(req.ModelParams) == 0 {
			req.ModelParams = version.ModelParams
		}
		if len(req.Variables) == 0 && len(version.Variables) > 0 {
			req.Variables = make(tables.PromptVariables, len(version.Variables))
			for key := range version.Variables {
				req.Variables[key] = ""
			}
		}
	} else {
		// Use provided messages
		for _, msg := range req.Messages {
			messages = append(messages, tables.TablePromptSessionMessage{
				PromptID: promptID,
				Message:  msg,
			})
		}
	}

	session := &tables.TablePromptSession{
		PromptID:    promptID,
		VersionID:   req.VersionID,
		Name:        req.Name,
		ModelParams: req.ModelParams,
		Provider:    req.Provider,
		Model:       req.Model,
		Variables:   req.Variables,
		Messages:    messages,
	}

	if err := h.store.CreatePromptSession(ctx, session); err != nil {
		logger.Error("failed to create session: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}

	SendJSON(ctx, map[string]any{
		"session": session,
	})
}

// updateSession handles PUT /api/prompt-repo/sessions/{id}
func (h *PromptsHandler) updateSession(ctx *fasthttp.RequestCtx) {
	idVal := ctx.UserValue("id")
	if idVal == nil {
		SendError(ctx, fasthttp.StatusBadRequest, "session ID is required")
		return
	}
	idStr, ok := idVal.(string)
	if !ok {
		SendError(ctx, fasthttp.StatusBadRequest, "invalid session ID")
		return
	}
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, "invalid session ID")
		return
	}

	var req UpdateSessionRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, "invalid request body")
		return
	}

	session, err := h.store.GetPromptSessionByID(ctx, uint(id))
	if err != nil {
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, fasthttp.StatusNotFound, "session not found")
			return
		}
		logger.Error("failed to get session: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}

	if req.Name != "" {
		session.Name = req.Name
	}
	session.ModelParams = req.ModelParams
	session.Provider = req.Provider
	session.Model = req.Model
	if req.Variables != nil {
		session.Variables = req.Variables
	}

	// Update messages
	var messages []tables.TablePromptSessionMessage
	for _, msg := range req.Messages {
		messages = append(messages, tables.TablePromptSessionMessage{
			PromptID: session.PromptID,
			Message:  msg,
		})
	}
	session.Messages = messages

	if err := h.store.UpdatePromptSession(ctx, session); err != nil {
		logger.Error("failed to update session: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}

	SendJSON(ctx, map[string]any{
		"session": session,
	})
}

// deleteSession handles DELETE /api/prompt-repo/sessions/{id}
func (h *PromptsHandler) deleteSession(ctx *fasthttp.RequestCtx) {
	idVal := ctx.UserValue("id")
	if idVal == nil {
		SendError(ctx, fasthttp.StatusBadRequest, "session ID is required")
		return
	}
	idStr, ok := idVal.(string)
	if !ok {
		SendError(ctx, fasthttp.StatusBadRequest, "invalid session ID")
		return
	}
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, "invalid session ID")
		return
	}

	if err := h.store.DeletePromptSession(ctx, uint(id)); err != nil {
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, fasthttp.StatusNotFound, "session not found")
			return
		}
		logger.Error("failed to delete session: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}

	SendJSON(ctx, map[string]any{
		"message": "session deleted successfully",
	})
}

// renameSession handles PUT /api/prompt-repo/sessions/{id}/rename
func (h *PromptsHandler) renameSession(ctx *fasthttp.RequestCtx) {
	idVal := ctx.UserValue("id")
	if idVal == nil {
		SendError(ctx, fasthttp.StatusBadRequest, "session ID is required")
		return
	}
	idStr, ok := idVal.(string)
	if !ok {
		SendError(ctx, fasthttp.StatusBadRequest, "invalid session ID")
		return
	}
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, "invalid session ID")
		return
	}

	var req RenameSessionRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, "invalid request body")
		return
	}

	session, err := h.store.GetPromptSessionByID(ctx, uint(id))
	if err != nil {
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, fasthttp.StatusNotFound, "session not found")
			return
		}
		logger.Error("failed to get session: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}

	if err := h.store.RenamePromptSession(ctx, session.ID, req.Name); err != nil {
		logger.Error("failed to rename session: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}

	session.Name = req.Name
	SendJSON(ctx, map[string]any{
		"session": session,
	})
}

// commitSession handles POST /api/prompt-repo/sessions/{id}/commit
func (h *PromptsHandler) commitSession(ctx *fasthttp.RequestCtx) {
	idVal := ctx.UserValue("id")
	if idVal == nil {
		SendError(ctx, fasthttp.StatusBadRequest, "session ID is required")
		return
	}
	idStr, ok := idVal.(string)
	if !ok {
		SendError(ctx, fasthttp.StatusBadRequest, "invalid session ID")
		return
	}
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, "invalid session ID")
		return
	}
	var req CommitSessionRequest
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, "invalid request body")
		return
	}
	if req.CommitMessage == "" {
		SendError(ctx, fasthttp.StatusBadRequest, "commit_message is required")
		return
	}
	prepared, ok := prepareConfigMutation(ctx, h.store)
	if !ok {
		return
	}
	session, err := h.store.GetPromptSessionByID(ctx, uint(id))
	if err != nil {
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, fasthttp.StatusNotFound, "session not found")
			return
		}
		logger.Error("failed to get session: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	messages := make([]tables.TablePromptVersionMessage, 0, len(session.Messages))
	if req.MessageIndices != nil {
		seen := make(map[int]struct{}, len(*req.MessageIndices))
		for _, idx := range *req.MessageIndices {
			if _, exists := seen[idx]; exists {
				continue
			}
			seen[idx] = struct{}{}
			if idx < 0 || idx >= len(session.Messages) {
				SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("message index %d out of range (0-%d)", idx, len(session.Messages)-1))
				return
			}
			messages = append(messages, tables.TablePromptVersionMessage{PromptID: session.PromptID, Message: session.Messages[idx].Message})
		}
	} else {
		for _, msg := range session.Messages {
			messages = append(messages, tables.TablePromptVersionMessage{PromptID: session.PromptID, Message: msg.Message})
		}
	}
	if len(messages) == 0 {
		SendError(ctx, fasthttp.StatusBadRequest, "at least one message must be included in the version")
		return
	}
	var versionVars tables.PromptVariables
	if len(session.Variables) > 0 {
		versionVars = make(tables.PromptVariables, len(session.Variables))
		for key := range session.Variables {
			versionVars[key] = ""
		}
	}
	version := &tables.TablePromptVersion{
		PromptID: session.PromptID, CommitMessage: req.CommitMessage, ModelParams: session.ModelParams,
		Provider: session.Provider, Model: session.Model, Variables: versionVars, Messages: messages,
	}
	revision, err := commitConfigMutation(ctx, h.store, prepared, func(mutationCtx context.Context) error {
		return h.store.CreatePromptVersion(mutationCtx, version)
	})
	if err != nil {
		if handleConfigMutationError(ctx, err) {
			return
		}
		logger.Error("failed to create version: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, err.Error())
		return
	}
	if !h.applyPromptMutation(ctx, prepared, revision) {
		return
	}
	SendJSON(ctx, map[string]any{"version": version})
}
