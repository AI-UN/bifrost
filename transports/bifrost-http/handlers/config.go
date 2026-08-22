package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/fasthttp/router"
	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/core/network"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework"
	"github.com/maximhq/bifrost/framework/configstore"
	configstoreTables "github.com/maximhq/bifrost/framework/configstore/tables"
	"github.com/maximhq/bifrost/framework/encrypt"
	"github.com/maximhq/bifrost/framework/modelcatalog"
	"github.com/maximhq/bifrost/plugins/compat"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
	"github.com/valyala/fasthttp"
)

// securityHeaders is the list of headers that cannot be configured in allowlist/denylist
// These headers are always blocked for security reasons regardless of user configuration
var securityHeaders = []string{
	"authorization",
	"proxy-authorization",
	"cookie",
	"host",
	"content-length",
	"connection",
	"transfer-encoding",
	"x-api-key",
	"x-goog-api-key",
	"x-bf-api-key",
	"x-bf-vk",
}

func getPasswordPolicyFailures(password string) []string {
	failures := make([]string, 0, 5)
	hasUppercase := false
	hasLowercase := false
	hasDigit := false
	hasSpecial := false

	for i := 0; i < len(password); i++ {
		char := password[i]
		switch {
		case char >= 'A' && char <= 'Z':
			hasUppercase = true
		case char >= 'a' && char <= 'z':
			hasLowercase = true
		case char >= '0' && char <= '9':
			hasDigit = true
		default:
			hasSpecial = true
		}
	}

	if len(password) < 12 {
		failures = append(failures, "at least 12 characters")
	}
	if !hasUppercase {
		failures = append(failures, "one uppercase letter")
	}
	if !hasLowercase {
		failures = append(failures, "one lowercase letter")
	}
	if !hasDigit {
		failures = append(failures, "one number")
	}
	if !hasSpecial {
		failures = append(failures, "one special character")
	}

	return failures
}

// ConfigManager is the interface for the config manager
type ConfigManager interface {
	ApplyAuthConfig(ctx context.Context, authConfig *configstore.AuthConfig) error
	// ValidateSetupToken checks the one-time bootstrap token required to create the
	// first admin account. Returns true once an admin account already exists.
	ValidateSetupToken(token string) bool
	ReloadClientConfigFromConfigStore(ctx context.Context) error
	UpdateSyncConfig(ctx context.Context) error
	ForceReloadPricing(ctx context.Context) error
	ReloadPlugin(ctx context.Context, name string, path *string, pluginConfig any, placement *schemas.PluginPlacement, order *int) error
	RemovePlugin(ctx context.Context, name string) error
	ReloadProxyConfig(ctx context.Context, config *configstoreTables.GlobalProxyConfig) error
	ReloadHeaderFilterConfig(ctx context.Context, config *configstoreTables.GlobalHeaderFilterConfig) error
}

// ConfigHandler manages runtime configuration updates for Bifrost.
// It provides endpoints to update and retrieve settings persisted via the ConfigStore backed by sql database.
type ConfigHandler struct {
	store         *lib.Config
	configManager ConfigManager
}

// NewConfigHandler creates a new handler for configuration management.
// It requires the Bifrost client, a logger, and the config store.
func NewConfigHandler(configManager ConfigManager, store *lib.Config) *ConfigHandler {
	return &ConfigHandler{
		configManager: configManager,
		store:         store,
	}
}

func (h *ConfigHandler) getConfigRevision(ctx *fasthttp.RequestCtx) {
	sendConfigRevision(ctx, h.store.ConfigStore)
}

// RegisterRoutes registers the configuration-related routes.
// It adds the `PUT /api/config` endpoint.
func (h *ConfigHandler) RegisterRoutes(r *router.Router, middlewares ...schemas.BifrostHTTPMiddleware) {
	r.GET("/api/config", lib.ChainMiddlewares(h.getConfig, middlewares...))
	r.GET("/api/config/revision", lib.ChainMiddlewares(h.getConfigRevision, middlewares...))
	r.PUT("/api/config", lib.ChainMiddlewares(h.updateConfig, middlewares...))
	r.POST("/api/config/metadata", lib.ChainMiddlewares(h.updateMetadata, middlewares...))
	r.GET("/api/version", lib.ChainMiddlewares(h.getVersion, middlewares...))
	r.GET("/api/proxy-config", lib.ChainMiddlewares(h.getProxyConfig, middlewares...))
	r.PUT("/api/proxy-config", lib.ChainMiddlewares(h.updateProxyConfig, middlewares...))
	r.POST("/api/pricing/force-sync", lib.ChainMiddlewares(h.forceSyncPricing, middlewares...))
}

// getVersion handles GET /api/version - Get the current version
func (h *ConfigHandler) getVersion(ctx *fasthttp.RequestCtx) {
	SendJSON(ctx, version)
}

// getConfig handles GET /config - Get the current configuration
func (h *ConfigHandler) getConfig(ctx *fasthttp.RequestCtx) {
	mapConfig := make(map[string]any)

	if query := string(ctx.QueryArgs().Peek("from_db")); query == "true" {
		if h.store.ConfigStore == nil {
			SendError(ctx, fasthttp.StatusServiceUnavailable, "config store not available")
			return
		}
		cc, err := h.store.ConfigStore.GetClientConfig(ctx)
		if err != nil {
			SendError(ctx, fasthttp.StatusInternalServerError,
				fmt.Sprintf("failed to fetch config from db: %v", err))
			return
		}
		if cc != nil {
			mapConfig["client_config"] = cc.Redacted()
		}
		// Fetching framework config
		fc, err := h.store.ConfigStore.GetFrameworkConfig(ctx)
		if err != nil {
			SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("failed to fetch framework config from db: %v", err))
			return
		}
		normalizedFrameworkConfig, _, _ := lib.ResolveFrameworkPricingConfig(fc, nil)
		mapConfig["framework_config"] = *normalizedFrameworkConfig
	} else {
		mapConfig["client_config"] = h.store.ClientConfig.Redacted()
		// Snapshot under the read lock; updateConfig swaps this pointer from
		// another request goroutine.
		h.store.Mu.RLock()
		storedFrameworkConfig := h.store.FrameworkConfig
		h.store.Mu.RUnlock()
		normalizedFrameworkConfig, _, _ := lib.ResolveFrameworkPricingConfig(nil, storedFrameworkConfig)
		mapConfig["framework_config"] = *normalizedFrameworkConfig
	}
	if h.store.ConfigStore != nil {
		// Fetching governance config
		authConfig, err := h.store.ConfigStore.GetAuthConfig(ctx)
		if err != nil {
			logger.Warn("failed to get auth config from store: %v", err)
			SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("failed to get auth config from store: %v", err))
			return
		}
		// Getting username and password from auth config
		// This username password is for the dashboard authentication
		if authConfig != nil {
			// For password, return SecretVar structure with redacted value
			// If from env, preserve env_var reference but clear value
			// If not from env, show <redacted> as the value
			var passwordSecretVar *schemas.SecretVar
			if authConfig.AdminPassword != nil && authConfig.AdminPassword.IsFromSecret() {
				passwordSecretVar = authConfig.AdminPassword.FullyRedacted()
			} else {
				passwordSecretVar = &schemas.SecretVar{
					Val: "<redacted>",
				}
			}
			mapConfig["auth_config"] = map[string]any{
				"admin_username": authConfig.AdminUserName,
				"admin_password": passwordSecretVar,
				"is_enabled":     authConfig.IsEnabled,
			}
		}
		// When authConfig is nil, no admin account has been created yet: leave
		// auth_config unset (rather than a placeholder object) so the UI's
		// isFirstTimeSetup check (!bifrostConfig?.auth_config) can tell "no admin
		// configured yet" apart from "admin configured with empty fields" and show
		// the setup-token field / include setup_token in the create request.
	} else {
		mapConfig["auth_config"] = map[string]any{
			"admin_username": &schemas.SecretVar{},
			"admin_password": &schemas.SecretVar{},
			"is_enabled":     false,
		}
	}
	mapConfig["is_db_connected"] = h.store.ConfigStore != nil
	if h.store.EnvLabel != "" {
		mapConfig["env_label"] = h.store.EnvLabel
	}
	mapConfig["is_git_available"] = CheckGitAvailability()
	mapConfig["is_cache_connected"] = h.store.VectorStore != nil
	mapConfig["is_logs_connected"] = h.store.LogsStore != nil
	mapConfig["is_object_storage_connected"] = h.store.LogsStoreConfig != nil && h.store.LogsStoreConfig.ObjectStorage != nil
	// Fetching proxy config
	if h.store.ConfigStore != nil {
		proxyConfig, err := h.store.ConfigStore.GetProxyConfig(ctx)
		if err != nil {
			logger.Warn("failed to get proxy config from store: %v", err)
		} else if proxyConfig != nil {
			// Redact password if present
			if proxyConfig.Password != "" {
				proxyConfig.Password = "<redacted>"
			}
			mapConfig["proxy_config"] = proxyConfig
		}
		// Fetching restart required config
		restartConfig, err := h.store.ConfigStore.GetRestartRequiredConfig(ctx)
		if err != nil {
			logger.Warn("failed to get restart required config from store: %v", err)
		} else if restartConfig != nil {
			mapConfig["restart_required"] = restartConfig
		}
		// Fetching UI/admin metadata blob (onboarding_dismissed, etc.).
		// This is a free-form key/value store that bypasses config.json sync.
		if metadata, err := h.store.ConfigStore.GetClientMetadata(ctx); err != nil {
			if !errors.Is(err, configstore.ErrNotFound) {
				logger.Warn("failed to get client metadata from store: %v", err)
			}
		} else if len(metadata) > 0 {
			mapConfig["metadata"] = metadata
		}
	}
	setCurrentConfigRevisionHeaders(ctx, h.store.ConfigStore)
	SendJSON(ctx, mapConfig)
}

// updateMetadata handles POST /api/config/metadata - merges a JSON object of
// key/value pairs into the ClientConfig metadata blob. Keys with a nil value
// are removed. Intended for UI/admin preferences (onboarding state, dismissed
// tooltips, etc.) and is auth-gated by the same middleware as the rest of /api/config.
func (h *ConfigHandler) updateMetadata(ctx *fasthttp.RequestCtx) {
	if h.store.ConfigStore == nil {
		SendError(ctx, fasthttp.StatusServiceUnavailable, "config store not available")
		return
	}
	var patch map[string]any
	if err := json.Unmarshal(ctx.PostBody(), &patch); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, "Invalid request payload")
		return
	}
	if len(patch) == 0 {
		SendError(ctx, fasthttp.StatusBadRequest, "patch body must contain at least one key")
		return
	}
	if err := h.store.ConfigStore.UpdateClientMetadata(ctx, patch); err != nil {
		if errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, fasthttp.StatusConflict, fmt.Sprintf("failed to update metadata: %v", err))
			return
		}
		SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("failed to update metadata: %v", err))
		return
	}
	SendJSON(ctx, map[string]any{"success": true})
}

// authConfigWithSetupToken extends the persisted AuthConfig shape with the
// one-time bootstrap setup_token field, for the auth_config object in PUT
// /api/config requests only. setup_token lives here (nested under auth_config
// on the wire) rather than directly on configstore.AuthConfig because that
// type is also what gets persisted to and read back from the DB via
// UpdateAuthConfig/GetAuthConfig; keeping SetupToken off it means there's no
// risk of it ever being written to storage or echoed back by GET /api/config.
type authConfigWithSetupToken struct {
	configstore.AuthConfig
	// SetupToken is the one-time bootstrap token (see AuthMiddleware.bootstrapToken)
	// required only when this request is creating the very first admin account.
	// It is never persisted.
	SetupToken string `json:"setup_token,omitempty"`
}

// updateConfig updates the core configuration settings.
// Currently, it supports hot-reloading of the `drop_excess_requests` setting.
// Note that settings like `prometheus_labels` cannot be changed at runtime.
func (h *ConfigHandler) updateConfig(ctx *fasthttp.RequestCtx) {
	if h.store.ConfigStore == nil {
		SendError(ctx, fasthttp.StatusInternalServerError, "Config store not initialized")
		return
	}

	payload := struct {
		ClientConfig    configstore.ClientConfig               `json:"client_config"`
		FrameworkConfig configstoreTables.TableFrameworkConfig `json:"framework_config"`
		AuthConfig      *authConfigWithSetupToken              `json:"auth_config"`
	}{}

	if err := json.Unmarshal(ctx.PostBody(), &payload); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, "Invalid request payload")
		return
	}

	prepared, ok := prepareConfigMutation(ctx, h.store.ConfigStore)
	if !ok {
		return
	}

	if err := lib.ValidateBaseURL(payload.ClientConfig.MCPExternalClientURL.GetValue()); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("mcp_external_client_url %v", err))
		return
	}

	if payload.FrameworkConfig.PricingURL != nil && *payload.FrameworkConfig.PricingURL != modelcatalog.DefaultPricingURL {
		if err := checkURLAccessibility(*payload.FrameworkConfig.PricingURL); err != nil {
			logger.Warn("failed to check the accessibility of the pricing URL: %v", err)
			SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("failed to check the accessibility of the pricing URL: %v", err))
			return
		}
	}
	if payload.FrameworkConfig.ModelParametersURL != nil &&
		*payload.FrameworkConfig.ModelParametersURL != "" &&
		*payload.FrameworkConfig.ModelParametersURL != modelcatalog.DefaultModelParametersURL {
		if err := checkURLAccessibility(*payload.FrameworkConfig.ModelParametersURL); err != nil {
			logger.Warn("failed to check the accessibility of the model parameters URL: %v", err)
			SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("failed to check the accessibility of the model parameters URL: %v", err))
			return
		}
	}
	if payload.FrameworkConfig.PricingSyncInterval != nil && *payload.FrameworkConfig.PricingSyncInterval <= 0 {
		logger.Warn("pricing sync interval must be greater than 0")
		SendError(ctx, fasthttp.StatusBadRequest, "pricing sync interval must be greater than 0")
		return
	}
	if payload.FrameworkConfig.MCPLibraryURL != nil &&
		*payload.FrameworkConfig.MCPLibraryURL != "" &&
		*payload.FrameworkConfig.MCPLibraryURL != modelcatalog.DefaultMCPLibraryURL {
		if err := checkURLAccessibility(*payload.FrameworkConfig.MCPLibraryURL); err != nil {
			logger.Warn("failed to check the accessibility of the MCP library URL: %v", err)
			SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("failed to check the accessibility of the MCP library URL: %v", err))
			return
		}
	}
	if payload.FrameworkConfig.MCPLibrarySyncInterval != nil && *payload.FrameworkConfig.MCPLibrarySyncInterval <= 0 {
		logger.Warn("MCP library sync interval must be greater than 0")
		SendError(ctx, fasthttp.StatusBadRequest, "MCP library sync interval must be greater than 0")
		return
	}
	// Checking the live models sync interval. Unlike the intervals above, 0 is
	// accepted: it is the documented way to turn the background refresher off.
	if payload.FrameworkConfig.LiveModelsSyncInterval != nil {
		interval := *payload.FrameworkConfig.LiveModelsSyncInterval
		if interval < 0 {
			logger.Warn("live models sync interval cannot be negative")
			SendError(ctx, fasthttp.StatusBadRequest, "live models sync interval cannot be negative (use 0 to disable background refresh)")
			return
		}
		if interval > 0 && interval < modelcatalog.MinimumLiveModelsSyncIntervalSec {
			logger.Warn("live models sync interval is below the minimum of %d seconds", modelcatalog.MinimumLiveModelsSyncIntervalSec)
			SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("live models sync interval must be 0 (disabled) or at least %d seconds", modelcatalog.MinimumLiveModelsSyncIntervalSec))
			return
		}
	}

	if h.store.ClientConfig == nil {
		SendError(ctx, fasthttp.StatusInternalServerError, "Client config not initialized")
		return
	}
	currentConfig := *h.store.ClientConfig
	if prepared.enabled {
		dbConfig, err := h.store.ConfigStore.GetClientConfig(ctx)
		if err != nil {
			SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("failed to get client config from store: %v", err))
			return
		}
		if dbConfig != nil {
			currentConfig = *dbConfig
		}
	}
	updatedConfig := currentConfig

	switch payload.ClientConfig.MCPServerAuthMode {
	case "", configstoreTables.MCPServerAuthModeHeaders, configstoreTables.MCPServerAuthModeBoth, configstoreTables.MCPServerAuthModeOAuth:
	default:
		SendError(ctx, fasthttp.StatusBadRequest, "mcp_server_auth_mode must be one of: headers, both, oauth")
		return
	}
	effectiveAuthMode := payload.ClientConfig.MCPServerAuthMode
	if effectiveAuthMode == "" {
		effectiveAuthMode = currentConfig.MCPServerAuthMode
	}
	effectiveOAuth2Config := currentConfig.OAuth2ServerConfig
	if payload.ClientConfig.OAuth2ServerConfig != nil {
		effectiveOAuth2Config = payload.ClientConfig.OAuth2ServerConfig
	}
	if effectiveOAuth2Config != nil &&
		effectiveOAuth2Config.DisableVKIdentity &&
		effectiveAuthMode != configstoreTables.MCPServerAuthModeOAuth {
		SendError(ctx, fasthttp.StatusBadRequest, "disable_vk_identity is only valid when mcp_server_auth_mode is oauth")
		return
	}
	if effectiveOAuth2Config != nil && effectiveOAuth2Config.AuthCodeTTL > configstoreTables.MaxAuthCodeTTL {
		SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("auth_code_ttl must not exceed %d seconds (15 minutes)", configstoreTables.MaxAuthCodeTTL))
		return
	}

	// The 'error' behavior rejects exactly the requests token_exchange
	// clients rely on (an identity token alongside a virtual key), so the
	// two settings are mutually exclusive. Validate this before the atomic
	// configuration write so an invalid request cannot commit a revision.
	if payload.ClientConfig.DualCredentialConflictBehavior == configstoreTables.DualCredentialConflictBehaviorError && h.store.MCPConfig != nil {
		for _, mcpClient := range h.store.MCPConfig.ClientConfigs {
			if mcpClient.AuthType == schemas.MCPAuthTypeTokenExchange {
				SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("dual_credential_conflict_behavior cannot be set to 'error' while MCP client %q uses auth_type 'token_exchange'; delete that client first or choose 'prefer_idp'/'prefer_vk'", mcpClient.Name))
				return
			}
		}
	}

	var restartReasons []string
	if payload.ClientConfig.DropExcessRequests != currentConfig.DropExcessRequests {
		updatedConfig.DropExcessRequests = payload.ClientConfig.DropExcessRequests
	}

	if payload.ClientConfig.MCPCodeModeBindingLevel != "" &&
		payload.ClientConfig.MCPCodeModeBindingLevel != string(schemas.CodeModeBindingLevelServer) &&
		payload.ClientConfig.MCPCodeModeBindingLevel != string(schemas.CodeModeBindingLevelTool) {
		logger.Warn("mcp_code_mode_binding_level must be 'server' or 'tool'")
		SendError(ctx, fasthttp.StatusBadRequest, "mcp_code_mode_binding_level must be 'server' or 'tool'")
		return
	}

	if payload.ClientConfig.MCPAgentDepth > 0 {
		updatedConfig.MCPAgentDepth = payload.ClientConfig.MCPAgentDepth
	}
	if payload.ClientConfig.MCPToolExecutionTimeout > 0 {
		updatedConfig.MCPToolExecutionTimeout = payload.ClientConfig.MCPToolExecutionTimeout
	}
	if payload.ClientConfig.MCPCodeModeBindingLevel != "" {
		updatedConfig.MCPCodeModeBindingLevel = payload.ClientConfig.MCPCodeModeBindingLevel
	}
	updatedConfig.MCPDisableAutoToolInject = payload.ClientConfig.MCPDisableAutoToolInject
	updatedConfig.MCPToolSyncInterval = payload.ClientConfig.MCPToolSyncInterval
	updatedConfig.MCPEnableTempTokenAuth = payload.ClientConfig.MCPEnableTempTokenAuth

	if !slices.Equal(payload.ClientConfig.PrometheusLabels, currentConfig.PrometheusLabels) {
		updatedConfig.PrometheusLabels = payload.ClientConfig.PrometheusLabels
		restartReasons = append(restartReasons, "Prometheus labels")
	}
	if !slices.Equal(payload.ClientConfig.AllowedOrigins, currentConfig.AllowedOrigins) {
		updatedConfig.AllowedOrigins = payload.ClientConfig.AllowedOrigins
		restartReasons = append(restartReasons, "Allowed origins")
	}
	if !slices.Equal(payload.ClientConfig.AllowedHeaders, currentConfig.AllowedHeaders) {
		updatedConfig.AllowedHeaders = payload.ClientConfig.AllowedHeaders
		restartReasons = append(restartReasons, "Allowed headers")
	}
	if payload.ClientConfig.InitialPoolSize > 0 {
		if payload.ClientConfig.InitialPoolSize != currentConfig.InitialPoolSize {
			restartReasons = append(restartReasons, "Initial pool size")
		}
		updatedConfig.InitialPoolSize = payload.ClientConfig.InitialPoolSize
	}
	if payload.ClientConfig.EnableLogging != nil {
		payloadLogging := *payload.ClientConfig.EnableLogging
		currentLogging := currentConfig.EnableLogging == nil || *currentConfig.EnableLogging
		if payloadLogging != currentLogging {
			restartReasons = append(restartReasons, "Logging changed")
		}
		updatedConfig.EnableLogging = payload.ClientConfig.EnableLogging
	}

	updatedConfig.DisableContentLogging = payload.ClientConfig.DisableContentLogging
	// No restart needed - logging plugin holds a live pointer to ClientConfig.RetainContentInObjectStorage.
	updatedConfig.RetainContentInObjectStorage = payload.ClientConfig.RetainContentInObjectStorage
	updatedConfig.DisableDBPingsInHealth = payload.ClientConfig.DisableDBPingsInHealth
	updatedConfig.DumpErrorsInConsoleLogs = payload.ClientConfig.DumpErrorsInConsoleLogs
	updatedConfig.EnforceAuthOnInference = payload.ClientConfig.EnforceAuthOnInference
	updatedConfig.EnforceGovernanceHeader = payload.ClientConfig.EnforceAuthOnInference
	updatedConfig.EnforceSCIMAuth = payload.ClientConfig.EnforceAuthOnInference

	// Only update when explicitly provided to avoid clearing the stored default (prefer_idp).
	// The conflict-vs-token_exchange validation already ran up front, before
	// any live mutation.
	if payload.ClientConfig.DualCredentialConflictBehavior != "" {
		updatedConfig.DualCredentialConflictBehavior = payload.ClientConfig.DualCredentialConflictBehavior
	}
	if payload.ClientConfig.MaxRequestBodySizeMB > 0 {
		if payload.ClientConfig.MaxRequestBodySizeMB != currentConfig.MaxRequestBodySizeMB {
			restartReasons = append(restartReasons, "Max request body size")
		}
		updatedConfig.MaxRequestBodySizeMB = payload.ClientConfig.MaxRequestBodySizeMB
	}

	compatChanged := payload.ClientConfig.Compat != currentConfig.Compat
	updatedConfig.Compat = payload.ClientConfig.Compat
	if payload.ClientConfig.AsyncJobResultTTL > 0 {
		updatedConfig.AsyncJobResultTTL = payload.ClientConfig.AsyncJobResultTTL
	}
	updatedConfig.RequiredHeaders = payload.ClientConfig.RequiredHeaders
	updatedConfig.LoggingHeaders = payload.ClientConfig.LoggingHeaders
	updatedConfig.WhitelistedRoutes = payload.ClientConfig.WhitelistedRoutes
	updatedConfig.HideDeletedVirtualKeysInFilters = payload.ClientConfig.HideDeletedVirtualKeysInFilters
	updatedConfig.AllowPerRequestContentStorageOverride = payload.ClientConfig.AllowPerRequestContentStorageOverride
	updatedConfig.AllowPerRequestRawOverride = payload.ClientConfig.AllowPerRequestRawOverride
	updatedConfig.AllowDirectKeys = payload.ClientConfig.AllowDirectKeys
	if payload.ClientConfig.RoutingChainMaxDepth > 0 {
		updatedConfig.RoutingChainMaxDepth = payload.ClientConfig.RoutingChainMaxDepth
	}
	updatedConfig.MCPExternalClientURL = payload.ClientConfig.MCPExternalClientURL
	if payload.ClientConfig.MCPServerAuthMode != "" {
		updatedConfig.MCPServerAuthMode = payload.ClientConfig.MCPServerAuthMode
	}
	if payload.ClientConfig.OAuth2ServerConfig != nil {
		updatedConfig.OAuth2ServerConfig = payload.ClientConfig.OAuth2ServerConfig
	}

	headerFilterChanged := !headerFilterConfigEqual(payload.ClientConfig.HeaderFilterConfig, currentConfig.HeaderFilterConfig)
	if headerFilterChanged {
		if err := validateHeaderFilterConfig(payload.ClientConfig.HeaderFilterConfig); err != nil {
			logger.Warn("invalid header filter config: %v", err)
			SendError(ctx, fasthttp.StatusBadRequest, err.Error())
			return
		}
		updatedConfig.HeaderFilterConfig = payload.ClientConfig.HeaderFilterConfig
	}
	if payload.ClientConfig.LogRetentionDays < 1 {
		logger.Warn("log_retention_days must be at least 1")
		SendError(ctx, fasthttp.StatusBadRequest, "log_retention_days must be at least 1")
		return
	}
	updatedConfig.LogRetentionDays = payload.ClientConfig.LogRetentionDays

	frameworkConfig, err := h.store.ConfigStore.GetFrameworkConfig(ctx)
	if err != nil {
		logger.Warn("failed to get framework config from store: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("failed to get framework config from store: %v", err))
		return
	}
	if frameworkConfig == nil {
		frameworkConfig = &configstoreTables.TableFrameworkConfig{
			ID:                     0,
			PricingURL:             bifrost.Ptr(modelcatalog.DefaultPricingURL),
			PricingSyncInterval:    bifrost.Ptr(int64(modelcatalog.DefaultSyncInterval.Seconds())),
			ModelParametersURL:     bifrost.Ptr(modelcatalog.DefaultModelParametersURL),
			MCPLibraryURL:          bifrost.Ptr(modelcatalog.DefaultMCPLibraryURL),
			MCPLibrarySyncInterval: bifrost.Ptr(int64(modelcatalog.DefaultSyncInterval.Seconds())),
			LiveModelsSyncInterval: bifrost.Ptr(int64(modelcatalog.DefaultLiveModelsSyncInterval.Seconds())),
		}
	}
	if frameworkConfig.PricingURL == nil {
		frameworkConfig.PricingURL = bifrost.Ptr(modelcatalog.DefaultPricingURL)
	}
	if frameworkConfig.PricingSyncInterval == nil {
		frameworkConfig.PricingSyncInterval = bifrost.Ptr(int64(modelcatalog.DefaultSyncInterval.Seconds()))
	}
	if frameworkConfig.ModelParametersURL == nil {
		frameworkConfig.ModelParametersURL = bifrost.Ptr(modelcatalog.DefaultModelParametersURL)
	}
	if frameworkConfig.MCPLibraryURL == nil {
		frameworkConfig.MCPLibraryURL = bifrost.Ptr(modelcatalog.DefaultMCPLibraryURL)
	}
	if frameworkConfig.MCPLibrarySyncInterval == nil {
		frameworkConfig.MCPLibrarySyncInterval = bifrost.Ptr(int64(modelcatalog.DefaultSyncInterval.Seconds()))
	}
	if frameworkConfig.LiveModelsSyncInterval == nil {
		frameworkConfig.LiveModelsSyncInterval = bifrost.Ptr(int64(modelcatalog.DefaultLiveModelsSyncInterval.Seconds()))
	}

	frameworkChanged := false
	if payload.FrameworkConfig.PricingURL != nil && *payload.FrameworkConfig.PricingURL != *frameworkConfig.PricingURL {
		frameworkConfig.PricingURL = payload.FrameworkConfig.PricingURL
		frameworkChanged = true
	}
	if payload.FrameworkConfig.PricingSyncInterval != nil {
		syncInterval := int64(*payload.FrameworkConfig.PricingSyncInterval)
		if syncInterval != *frameworkConfig.PricingSyncInterval {
			frameworkConfig.PricingSyncInterval = &syncInterval
			frameworkChanged = true
		}
	}
	if payload.FrameworkConfig.ModelParametersURL != nil {
		effectiveURL := *payload.FrameworkConfig.ModelParametersURL
		if effectiveURL == "" {
			effectiveURL = modelcatalog.DefaultModelParametersURL
		}
		if effectiveURL != *frameworkConfig.ModelParametersURL {
			frameworkConfig.ModelParametersURL = &effectiveURL
			frameworkChanged = true
		}
	}
	if payload.FrameworkConfig.MCPLibraryURL != nil {
		effectiveURL := *payload.FrameworkConfig.MCPLibraryURL
		if effectiveURL == "" {
			effectiveURL = modelcatalog.DefaultMCPLibraryURL
		}
		if effectiveURL != *frameworkConfig.MCPLibraryURL {
			frameworkConfig.MCPLibraryURL = &effectiveURL
			frameworkChanged = true
		}
	}
	if payload.FrameworkConfig.MCPLibrarySyncInterval != nil &&
		*payload.FrameworkConfig.MCPLibrarySyncInterval != *frameworkConfig.MCPLibrarySyncInterval {
		frameworkConfig.MCPLibrarySyncInterval = payload.FrameworkConfig.MCPLibrarySyncInterval
		frameworkChanged = true
	}

	var persistedAuthConfig *configstore.AuthConfig
	authChanged := false
	if payload.AuthConfig != nil {
		currentAuthConfig, authErr := h.store.ConfigStore.GetAuthConfig(ctx)
		if authErr != nil && !errors.Is(authErr, configstore.ErrNotFound) {
			logger.Warn("failed to get auth config from store: %v", authErr)
			SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("failed to get auth config from store: %v", authErr))
			return
		}

		requestedAuthConfig := payload.AuthConfig
		if requestedAuthConfig.IsEnabled {
			if requestedAuthConfig.AdminUserName == nil {
				requestedAuthConfig.AdminUserName = &schemas.SecretVar{}
			}
			if requestedAuthConfig.AdminPassword == nil {
				requestedAuthConfig.AdminPassword = &schemas.SecretVar{}
			}
			if requestedAuthConfig.AdminUserName.IsFromSecret() && requestedAuthConfig.AdminUserName.GetValue() == "" {
				SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("external reference %s for admin_username resolved to an empty value", requestedAuthConfig.AdminUserName.GetRawRef()))
				return
			}
			if requestedAuthConfig.AdminPassword.IsFromSecret() && requestedAuthConfig.AdminPassword.GetValue() == "" {
				SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("external reference %s for admin_password resolved to an empty value", requestedAuthConfig.AdminPassword.GetRawRef()))
				return
			}
			if currentAuthConfig == nil &&
				(requestedAuthConfig.AdminUserName.GetValue() == "" || requestedAuthConfig.AdminPassword.GetValue() == "") {
				SendError(ctx, fasthttp.StatusBadRequest, "auth username and password must be provided")
				return
			}
			if currentAuthConfig == nil && !h.configManager.ValidateSetupToken(payload.AuthConfig.SetupToken) {
				SendError(ctx, fasthttp.StatusForbidden, "a valid setup token is required to create the initial admin account; configure setup_token in config.json (or the BIFROST_SETUP_TOKEN env var) and pass it in this request")
				return
			}
			if requestedAuthConfig.AdminUserName.GetValue() != "" {
				if requestedAuthConfig.AdminPassword.ShouldPreserveStored() {
					if currentAuthConfig == nil || currentAuthConfig.AdminPassword.GetValue() == "" {
						SendError(ctx, fasthttp.StatusBadRequest, "auth password must be provided")
						return
					}
					requestedAuthConfig.AdminPassword = currentAuthConfig.AdminPassword
				} else {
					passwordPolicyFailures := getPasswordPolicyFailures(requestedAuthConfig.AdminPassword.GetValue())
					if len(passwordPolicyFailures) > 0 {
						SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("auth password must include %s", strings.Join(passwordPolicyFailures, ", ")))
						return
					}
					hashedPassword, hashErr := encrypt.Hash(requestedAuthConfig.AdminPassword.GetValue())
					if hashErr != nil {
						logger.Warn("failed to hash password: %v", hashErr)
						SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("failed to hash password: %v", hashErr))
						return
					}
					if requestedAuthConfig.AdminPassword.IsFromSecret() {
						secret := *requestedAuthConfig.AdminPassword
						secret.Val = hashedPassword
						requestedAuthConfig.AdminPassword = &secret
					} else {
						requestedAuthConfig.AdminPassword = &schemas.SecretVar{Val: hashedPassword}
					}
				}
			}
			if requestedAuthConfig.AdminUserName.GetValue() == "" || requestedAuthConfig.AdminPassword.GetValue() == "" {
				SendError(ctx, fasthttp.StatusBadRequest, "auth username and password must be provided")
				return
			}
			persistedAuthConfig = &requestedAuthConfig.AuthConfig
		} else if currentAuthConfig != nil {
			if requestedAuthConfig.AdminPassword == nil || requestedAuthConfig.AdminPassword.ShouldPreserveStored() {
				requestedAuthConfig.AdminPassword = currentAuthConfig.AdminPassword
			}
			if requestedAuthConfig.AdminUserName == nil || requestedAuthConfig.AdminUserName.GetValue() == "" {
				requestedAuthConfig.AdminUserName = currentAuthConfig.AdminUserName
			}
			persistedAuthConfig = &requestedAuthConfig.AuthConfig
		}

		if persistedAuthConfig != nil {
			if currentAuthConfig == nil {
				authChanged = persistedAuthConfig.IsEnabled
			} else {
				usernameChanged := !persistedAuthConfig.AdminUserName.Equals(currentAuthConfig.AdminUserName)
				passwordChanged := persistedAuthConfig.AdminPassword.GetValue() != currentAuthConfig.AdminPassword.GetValue()
				authChanged = persistedAuthConfig.IsEnabled != currentAuthConfig.IsEnabled || usernameChanged || passwordChanged
			}
		}
	}
	if payload.FrameworkConfig.LiveModelsSyncInterval != nil {
		syncInterval := *payload.FrameworkConfig.LiveModelsSyncInterval
		if frameworkConfig.LiveModelsSyncInterval == nil || syncInterval != *frameworkConfig.LiveModelsSyncInterval {
			frameworkConfig.LiveModelsSyncInterval = &syncInterval
			frameworkChanged = true
		}
	}

	var restartConfig *configstoreTables.RestartRequiredConfig
	if len(restartReasons) > 0 {
		restartConfig = &configstoreTables.RestartRequiredConfig{
			Required: true,
			Reason: fmt.Sprintf(
				"%s settings have been updated. A restart is required for changes to take full effect.",
				strings.Join(restartReasons, ", "),
			),
		}
	}

	revision, err := commitConfigMutation(ctx, h.store.ConfigStore, prepared, func(mutationCtx context.Context) error {
		if err := h.store.ConfigStore.UpdateClientConfig(mutationCtx, &updatedConfig); err != nil {
			return fmt.Errorf("saving client configuration: %w", err)
		}
		if frameworkChanged {
			if err := h.store.ConfigStore.UpdateFrameworkConfig(mutationCtx, frameworkConfig); err != nil {
				return fmt.Errorf("saving framework configuration: %w", err)
			}
		}
		if persistedAuthConfig != nil {
			if err := h.store.ConfigStore.UpdateAuthConfig(mutationCtx, persistedAuthConfig); err != nil {
				return fmt.Errorf("saving auth configuration: %w", err)
			}
		}
		if restartConfig != nil {
			if err := h.store.ConfigStore.SetRestartRequiredConfig(mutationCtx, restartConfig); err != nil {
				return fmt.Errorf("saving restart-required marker: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		if handleConfigMutationError(ctx, err) {
			return
		}
		logger.Warn("failed to save configuration: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("failed to save configuration: %v", err))
		return
	}

	applyFailed := false
	if h.store.MCPConfig != nil {
		if h.store.MCPConfig.ToolManagerConfig == nil {
			h.store.MCPConfig.ToolManagerConfig = &schemas.MCPToolManagerConfig{}
		}
		h.store.MCPConfig.ToolSyncInterval = time.Duration(updatedConfig.MCPToolSyncInterval) * time.Second
		h.store.MCPConfig.ToolManagerConfig.MaxAgentDepth = updatedConfig.MCPAgentDepth
		h.store.MCPConfig.ToolManagerConfig.ToolExecutionTimeout = schemas.Duration(time.Duration(updatedConfig.MCPToolExecutionTimeout) * time.Second)
		h.store.MCPConfig.ToolManagerConfig.CodeModeBindingLevel = schemas.CodeModeBindingLevel(updatedConfig.MCPCodeModeBindingLevel)
		h.store.MCPConfig.ToolManagerConfig.DisableAutoToolInject = updatedConfig.MCPDisableAutoToolInject
	}
	if err := h.configManager.ReloadClientConfigFromConfigStore(ctx); err != nil {
		logger.Warn("configuration committed but client runtime apply failed: %v", err)
		applyFailed = true
	}
	if headerFilterChanged {
		if err := h.configManager.ReloadHeaderFilterConfig(ctx, updatedConfig.HeaderFilterConfig); err != nil {
			logger.Warn("configuration committed but header filter apply failed: %v", err)
			applyFailed = true
		}
	}
	if compatChanged {
		newCompat := updatedConfig.Compat
		compatEnabled := newCompat.ConvertTextToChat ||
			newCompat.ConvertChatToResponses ||
			newCompat.ConvertResponsesToChat ||
			newCompat.ShouldDropParams ||
			newCompat.ShouldConvertParams
		if compatEnabled {
			compatConfig := &compat.Config{
				ConvertTextToChat:      newCompat.ConvertTextToChat,
				ConvertChatToResponses: newCompat.ConvertChatToResponses,
				ConvertResponsesToChat: newCompat.ConvertResponsesToChat,
				ShouldDropParams:       newCompat.ShouldDropParams,
				ShouldConvertParams:    newCompat.ShouldConvertParams,
			}
			if err := h.configManager.ReloadPlugin(ctx, compat.PluginName, nil, compatConfig, nil, nil); err != nil {
				logger.Warn("configuration committed but compat plugin apply failed: %v", err)
				applyFailed = true
			}
		} else {
			disabledCtx := context.WithValue(ctx, PluginDisabledKey, true)
			if err := h.configManager.RemovePlugin(disabledCtx, compat.PluginName); err != nil {
				logger.Warn("configuration committed but compat plugin removal failed: %v", err)
				applyFailed = true
			}
		}
	}
	if frameworkChanged {
		var syncSeconds int64
		if frameworkConfig.PricingSyncInterval != nil {
			syncSeconds = *frameworkConfig.PricingSyncInterval
		} else {
			syncSeconds = int64(modelcatalog.DefaultSyncInterval.Seconds())
		}
		updatedFrameworkConfig := &framework.FrameworkConfig{
			Pricing: &modelcatalog.Config{
				PricingURL:             frameworkConfig.PricingURL,
				PricingSyncInterval:    &syncSeconds,
				ModelParametersURL:     frameworkConfig.ModelParametersURL,
				MCPLibraryURL:          frameworkConfig.MCPLibraryURL,
				MCPLibrarySyncInterval: frameworkConfig.MCPLibrarySyncInterval,
				LiveModelsSyncInterval: frameworkConfig.LiveModelsSyncInterval,
			},
		}
		// Publish the new config under the write lock: other request goroutines
		// read this pointer through LiveModelsSyncInterval and UpdateSyncConfig.
		// A whole new struct is swapped in rather than mutated in place, which is
		// what lets readers use the pointer after releasing the lock. Scoped to
		// the assignment alone — UpdateSyncConfig takes the read lock itself, and
		// sync.RWMutex is not reentrant.
		h.store.Mu.Lock()
		h.store.FrameworkConfig = updatedFrameworkConfig
		h.store.Mu.Unlock()
		if err := h.configManager.UpdateSyncConfig(ctx); err != nil {
			logger.Warn("configuration committed but framework runtime apply failed: %v", err)
			applyFailed = true
		}
	}
	if persistedAuthConfig != nil {
		if err := h.configManager.ApplyAuthConfig(ctx, persistedAuthConfig); err != nil {
			logger.Warn("configuration committed but auth runtime apply failed: %v", err)
			applyFailed = true
		}

	}
	if authChanged {
		if err := h.store.ConfigStore.FlushSessions(ctx); err != nil {
			logger.Warn("configuration committed but existing sessions could not be flushed: %v", err)
			applyFailed = true
		}
	}

	if applyFailed {
		sendConfigMutationPending(ctx, revision)
		return
	}
	if prepared.enabled {
		setConfigRevisionHeaders(ctx, revision)
	}
	ctx.SetStatusCode(fasthttp.StatusOK)
	SendJSON(ctx, map[string]any{
		"status":  "success",
		"message": "configuration updated successfully",
	})
}

// forceSyncPricing triggers an immediate pricing sync and resets the pricing sync timer
func (h *ConfigHandler) forceSyncPricing(ctx *fasthttp.RequestCtx) {
	if h.store.ConfigStore == nil {
		SendError(ctx, fasthttp.StatusServiceUnavailable, "config store not available")
		return
	}

	if err := h.configManager.ForceReloadPricing(ctx); err != nil {
		logger.Warn("failed to force pricing sync: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("failed to force pricing sync: %v", err))
		return
	}

	ctx.SetStatusCode(fasthttp.StatusOK)
	SendJSON(ctx, map[string]any{
		"status":  "success",
		"message": "pricing synced successfully",
	})
}

// getProxyConfig handles GET /api/proxy-config - Get the current proxy configuration
func (h *ConfigHandler) getProxyConfig(ctx *fasthttp.RequestCtx) {
	if h.store.ConfigStore == nil {
		SendError(ctx, fasthttp.StatusServiceUnavailable, "config store not available")
		return
	}
	proxyConfig, err := h.store.ConfigStore.GetProxyConfig(ctx)
	if err != nil {
		SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("failed to get proxy config: %v", err))
		return
	}
	setCurrentConfigRevisionHeaders(ctx, h.store.ConfigStore)
	if proxyConfig == nil {
		// Return default empty config
		SendJSON(ctx, configstoreTables.GlobalProxyConfig{
			Enabled: false,
			Type:    network.GlobalProxyTypeHTTP,
		})
		return
	}
	// Redact password if present
	if proxyConfig.Password != "" {
		proxyConfig.Password = "<redacted>"
	}
	SendJSON(ctx, proxyConfig)
}

// updateProxyConfig handles PUT /api/proxy-config - Update the proxy configuration
func (h *ConfigHandler) updateProxyConfig(ctx *fasthttp.RequestCtx) {
	if h.store.ConfigStore == nil {
		SendError(ctx, fasthttp.StatusInternalServerError, "config store not initialized")
		return
	}

	var payload configstoreTables.GlobalProxyConfig
	if err := json.Unmarshal(ctx.PostBody(), &payload); err != nil {
		SendError(ctx, fasthttp.StatusBadRequest, "Invalid request payload")
		return
	}

	prepared, ok := prepareConfigMutation(ctx, h.store.ConfigStore)
	if !ok {
		return
	}

	if payload.Enabled {
		switch payload.Type {
		case network.GlobalProxyTypeHTTP:
		case network.GlobalProxyTypeSOCKS5, network.GlobalProxyTypeTCP:
			SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("proxy type %s is not yet supported", payload.Type))
			return
		default:
			SendError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("invalid proxy type: %s", payload.Type))
			return
		}
		if payload.URL == "" {
			SendError(ctx, fasthttp.StatusBadRequest, "proxy URL is required when proxy is enabled")
			return
		}
		if payload.Timeout < 0 {
			SendError(ctx, fasthttp.StatusBadRequest, "proxy timeout must be non-negative")
			return
		}
	}

	if payload.Password == "<redacted>" {
		existingConfig, err := h.store.ConfigStore.GetProxyConfig(ctx)
		if err != nil && !errors.Is(err, configstore.ErrNotFound) {
			SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("failed to get existing proxy config: %v", err))
			return
		}
		if existingConfig != nil {
			payload.Password = existingConfig.Password
		} else {
			payload.Password = ""
		}
	}

	restartConfig := &configstoreTables.RestartRequiredConfig{
		Required: true,
		Reason:   "Proxy configuration has been updated. A restart is required for all changes to take full effect.",
	}
	revision, err := commitConfigMutation(ctx, h.store.ConfigStore, prepared, func(mutationCtx context.Context) error {
		if err := h.store.ConfigStore.UpdateProxyConfig(mutationCtx, &payload); err != nil {
			return fmt.Errorf("saving proxy configuration: %w", err)
		}
		if err := h.store.ConfigStore.SetRestartRequiredConfig(mutationCtx, restartConfig); err != nil {
			return fmt.Errorf("saving restart-required marker: %w", err)
		}
		return nil
	})
	if err != nil {
		if handleConfigMutationError(ctx, err) {
			return
		}
		logger.Warn("failed to save proxy configuration: %v", err)
		SendError(ctx, fasthttp.StatusInternalServerError, fmt.Sprintf("failed to save proxy configuration: %v", err))
		return
	}

	if err := h.configManager.ReloadProxyConfig(ctx, &payload); err != nil {
		logger.Warn("proxy configuration committed but runtime apply failed: %v", err)
		sendConfigMutationPending(ctx, revision)
		return
	}

	if prepared.enabled {
		setConfigRevisionHeaders(ctx, revision)
	}
	ctx.SetStatusCode(fasthttp.StatusOK)
	SendJSON(ctx, map[string]any{
		"status":  "success",
		"message": "proxy configuration updated successfully",
	})
}

// headerFilterConfigEqual compares two GlobalHeaderFilterConfig for equality
func headerFilterConfigEqual(a, b *configstoreTables.GlobalHeaderFilterConfig) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return slices.Equal(a.Allowlist, b.Allowlist) && slices.Equal(a.Denylist, b.Denylist)
}

// validateHeaderFilterConfig validates that no exact security header names are in the allowlist or denylist
// and that wildcard patterns use valid syntax (only trailing * is supported).
// Wildcard patterns that would match security headers are allowed because security headers
// are unconditionally stripped at runtime regardless of configuration.
// Returns an error if any exact security headers are found or patterns are invalid.
func validateHeaderFilterConfig(config *configstoreTables.GlobalHeaderFilterConfig) error {
	if config == nil {
		return nil
	}

	// Validate pattern syntax and normalize entries (trim, lowercase, drop empties)
	filteredAllow := config.Allowlist[:0]
	for _, header := range config.Allowlist {
		h := strings.ToLower(strings.TrimSpace(header))
		if h == "" {
			continue
		}
		if idx := strings.Index(h, "*"); idx != -1 && idx != len(h)-1 {
			return fmt.Errorf("invalid pattern %q: wildcard (*) is only supported at the end of a pattern", h)
		}
		filteredAllow = append(filteredAllow, h)
	}
	config.Allowlist = filteredAllow
	filteredDeny := config.Denylist[:0]
	for _, header := range config.Denylist {
		h := strings.ToLower(strings.TrimSpace(header))
		if h == "" {
			continue
		}
		if idx := strings.Index(h, "*"); idx != -1 && idx != len(h)-1 {
			return fmt.Errorf("invalid pattern %q: wildcard (*) is only supported at the end of a pattern", h)
		}
		filteredDeny = append(filteredDeny, h)
	}
	config.Denylist = filteredDeny

	var foundSecurityHeaders []string

	// Check allowlist for exact security header names.
	// Wildcard patterns are allowed — security headers are always stripped at runtime
	// unconditionally in ctx.go, regardless of allowlist/denylist configuration.
	for _, header := range config.Allowlist {
		headerLower := strings.ToLower(strings.TrimSpace(header))
		if strings.Contains(headerLower, "*") {
			continue
		}
		if slices.Contains(securityHeaders, headerLower) {
			foundSecurityHeaders = append(foundSecurityHeaders, headerLower)
		}
	}

	// Check denylist for exact security header names.
	for _, header := range config.Denylist {
		headerLower := strings.ToLower(strings.TrimSpace(header))
		if strings.Contains(headerLower, "*") {
			continue
		}
		if slices.Contains(securityHeaders, headerLower) && !slices.Contains(foundSecurityHeaders, headerLower) {
			foundSecurityHeaders = append(foundSecurityHeaders, headerLower)
		}
	}

	if len(foundSecurityHeaders) > 0 {
		return fmt.Errorf("the following headers are not allowed to be configured: %s. These headers are security headers and are always blocked", strings.Join(foundSecurityHeaders, ", "))
	}

	return nil
}

// checkURLAccessibility verifies that the given URL is reachable.
// For file:// URLs it checks that the path exists on disk.
// For http(s):// URLs it performs a GET and expects a 200 OK.
func checkURLAccessibility(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if parsed.Scheme == "file" {
		info, err := os.Stat(parsed.Path)
		if err != nil {
			return fmt.Errorf("file not accessible: %w", err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("path is not a regular file")
		}
		return nil
	}
	if err := bifrost.ValidateExternalURL(rawURL, true); err != nil {
		return fmt.Errorf("URL validation failed: %w", err)
	}
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(rawURL)
	if err != nil {
		return err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}
