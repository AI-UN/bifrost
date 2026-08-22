package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework"
	"github.com/maximhq/bifrost/framework/configstore"
	"github.com/maximhq/bifrost/framework/configstore/tables"
	"github.com/maximhq/bifrost/framework/modelcatalog"
	dynamicplugins "github.com/maximhq/bifrost/framework/plugins"
	"github.com/maximhq/bifrost/transports/bifrost-http/handlers"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
)

// ConfigSnapshotReconciler applies a committed config-store revision to this node's runtime.
type ConfigSnapshotReconciler interface {
	Reconcile(ctx context.Context, revision int64) error
}

type configSnapshotReconciler struct {
	server *BifrostHTTPServer
	mu     sync.Mutex
	last   *runtimeConfigSnapshot
}

type runtimeConfigSnapshot struct {
	Revision       int64
	Hash           string
	Client         *configstore.ClientConfig
	Framework      *tables.TableFrameworkConfig
	Proxy          *tables.GlobalProxyConfig
	Providers      map[schemas.ModelProvider]configstore.ProviderConfig
	MCP            *schemas.MCPConfig
	Governance     *configstore.GovernanceConfig
	Plugins        []*tables.TablePlugin
	FeatureFlags   []tables.TableFeatureFlag
	Prompts        []tables.TablePrompt
	PromptVersions []tables.TablePromptVersion
}

type preparedPlugin struct {
	row    *tables.TablePlugin
	plugin schemas.BasePlugin
	used   bool
}

// NewConfigSnapshotReconciler creates the per-server runtime reconciler.
func NewConfigSnapshotReconciler(server *BifrostHTTPServer) ConfigSnapshotReconciler {
	return &configSnapshotReconciler{server: server}
}

// Reconcile reads one revision-consistent snapshot and applies its definition diff.
func (r *configSnapshotReconciler) Reconcile(ctx context.Context, revision int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.last != nil {
		if revision == r.last.Revision {
			return nil
		}
		if revision < r.last.Revision {
			return fmt.Errorf("config snapshot revision moved backwards: applied=%d target=%d", r.last.Revision, revision)
		}
	}

	candidate, err := r.readSnapshot(ctx, revision)
	if err != nil {
		return err
	}
	if err := r.validateRuntime(candidate); err != nil {
		return err
	}

	if r.last == nil {
		r.last = emptyRuntimeConfigSnapshot()
	}
	if candidate.Hash == r.last.Hash {
		r.last = candidate
		return nil
	}

	prepared, err := r.prepareCandidates(ctx, r.last, candidate)
	if err != nil {
		return err
	}
	defer cleanupPreparedPlugins(prepared)

	applyCtx := context.WithValue(ctx, schemas.BifrostContextKeySkipDBUpdate, true)
	if err := r.applyUpdates(applyCtx, r.last, candidate, prepared); err != nil {
		return err
	}
	if err := r.applyDeletes(applyCtx, r.last, candidate); err != nil {
		return err
	}

	r.last = candidate
	return nil
}

func emptyRuntimeConfigSnapshot() *runtimeConfigSnapshot {
	return &runtimeConfigSnapshot{
		Providers:      map[schemas.ModelProvider]configstore.ProviderConfig{},
		MCP:            &schemas.MCPConfig{ClientConfigs: []*schemas.MCPClientConfig{}},
		Governance:     &configstore.GovernanceConfig{},
		Plugins:        []*tables.TablePlugin{},
		FeatureFlags:   []tables.TableFeatureFlag{},
		Prompts:        []tables.TablePrompt{},
		PromptVersions: []tables.TablePromptVersion{},
	}
}

func (r *configSnapshotReconciler) readSnapshot(ctx context.Context, target int64) (*runtimeConfigSnapshot, error) {
	if r.server == nil || r.server.Config == nil || r.server.Config.ConfigStore == nil {
		return nil, fmt.Errorf("config snapshot reconciler requires a config store")
	}
	store := r.server.Config.ConfigStore
	revisions, ok := store.(configstore.ConfigRevisionStore)
	if !ok {
		return nil, fmt.Errorf("config store does not support config revisions")
	}

	before, err := revisions.GetConfigRevision(ctx)
	if err != nil {
		return nil, fmt.Errorf("read config revision before snapshot: %w", err)
	}
	if before != target {
		return nil, fmt.Errorf("config snapshot target changed before read: target=%d current=%d", target, before)
	}

	snapshot := &runtimeConfigSnapshot{Revision: target}
	if snapshot.Client, err = store.GetClientConfig(ctx); err != nil {
		return nil, fmt.Errorf("read client config snapshot: %w", err)
	}
	if snapshot.Framework, err = store.GetFrameworkConfig(ctx); err != nil {
		return nil, fmt.Errorf("read framework config snapshot: %w", err)
	}
	if snapshot.Proxy, err = store.GetProxyConfig(ctx); err != nil {
		return nil, fmt.Errorf("read proxy config snapshot: %w", err)
	}
	if snapshot.Providers, err = store.GetProvidersConfig(ctx); err != nil {
		return nil, fmt.Errorf("read provider config snapshot: %w", err)
	}
	if snapshot.MCP, err = store.GetMCPConfig(ctx); err != nil {
		return nil, fmt.Errorf("read MCP config snapshot: %w", err)
	}
	if snapshot.Governance, err = store.GetGovernanceConfig(ctx); err != nil {
		return nil, fmt.Errorf("read governance config snapshot: %w", err)
	}
	if snapshot.Plugins, err = store.GetPlugins(ctx); err != nil {
		return nil, fmt.Errorf("read plugin config snapshot: %w", err)
	}
	if snapshot.FeatureFlags, err = store.ListFeatureFlags(ctx); err != nil {
		return nil, fmt.Errorf("read feature flag snapshot: %w", err)
	}
	if snapshot.Prompts, err = store.GetPrompts(ctx, nil); err != nil {
		return nil, fmt.Errorf("read prompt snapshot: %w", err)
	}
	if snapshot.PromptVersions, err = store.GetAllPromptVersions(ctx); err != nil {
		return nil, fmt.Errorf("read prompt version snapshot: %w", err)
	}

	if snapshot.Providers == nil {
		snapshot.Providers = map[schemas.ModelProvider]configstore.ProviderConfig{}
	}
	if snapshot.MCP == nil {
		snapshot.MCP = &schemas.MCPConfig{ClientConfigs: []*schemas.MCPClientConfig{}}
	}
	if snapshot.Governance == nil {
		snapshot.Governance = &configstore.GovernanceConfig{}
	}
	if snapshot.Plugins == nil {
		snapshot.Plugins = []*tables.TablePlugin{}
	}
	if snapshot.FeatureFlags == nil {
		snapshot.FeatureFlags = []tables.TableFeatureFlag{}
	}
	if snapshot.Prompts == nil {
		snapshot.Prompts = []tables.TablePrompt{}
	}
	if snapshot.PromptVersions == nil {
		snapshot.PromptVersions = []tables.TablePromptVersion{}
	}

	after, err := revisions.GetConfigRevision(ctx)
	if err != nil {
		return nil, fmt.Errorf("read config revision after snapshot: %w", err)
	}
	if after != before {
		return nil, fmt.Errorf("config revision changed during snapshot read: before=%d after=%d", before, after)
	}

	snapshot.Hash, err = canonicalHash("snapshot", struct {
		Client         *configstore.ClientConfig
		Framework      *tables.TableFrameworkConfig
		Proxy          *tables.GlobalProxyConfig
		Providers      map[schemas.ModelProvider]configstore.ProviderConfig
		MCP            *schemas.MCPConfig
		Governance     *configstore.GovernanceConfig
		Plugins        []*tables.TablePlugin
		FeatureFlags   []tables.TableFeatureFlag
		Prompts        []tables.TablePrompt
		PromptVersions []tables.TablePromptVersion
	}{snapshot.Client, snapshot.Framework, snapshot.Proxy, snapshot.Providers, snapshot.MCP, snapshot.Governance, sortedPlugins(snapshot.Plugins), snapshot.FeatureFlags, snapshot.Prompts, snapshot.PromptVersions})
	if err != nil {
		return nil, fmt.Errorf("hash config snapshot: %w", err)
	}
	return snapshot, nil
}

func (r *configSnapshotReconciler) validateRuntime(snapshot *runtimeConfigSnapshot) error {
	s := r.server
	if s == nil || s.Config == nil || s.Config.ConfigStore == nil {
		return fmt.Errorf("config runtime is not initialized")
	}
	if s.Client == nil || s.Config.ClientConfig == nil || snapshot.Client == nil {
		return fmt.Errorf("client config runtime is not available")
	}
	if snapshot.Framework != nil && (s.Config.ModelCatalog == nil || s.Config.FrameworkConfig == nil) {
		return fmt.Errorf("framework/model catalog runtime is not available")
	}
	if len(snapshot.Providers) > 0 && s.Config.ModelCatalog == nil {
		return fmt.Errorf("provider model catalog runtime is not available")
	}
	if len(snapshot.MCP.ClientConfigs) > 0 && (s.Config.MCPConfig == nil || s.MCPServerHandler == nil) {
		return fmt.Errorf("MCP runtime is not available")
	}
	if governanceHasDefinitions(snapshot.Governance) {
		if s.Ctx == nil {
			return fmt.Errorf("governance runtime context is not available")
		}
		if _, err := s.getGovernancePlugin(); err != nil {
			return fmt.Errorf("governance runtime is not available: %w", err)
		}
	}
	if len(snapshot.Plugins) > 0 && s.Config.PluginLoader == nil {
		return fmt.Errorf("plugin runtime loader is not available")
	}
	return nil
}

func (r *configSnapshotReconciler) prepareCandidates(ctx context.Context, old, next *runtimeConfigSnapshot) ([]*preparedPlugin, error) {
	if err := validateMCPChanges(old.MCP, next.MCP); err != nil {
		return nil, err
	}
	if err := r.validateProviderChanges(old.Providers, next.Providers); err != nil {
		return nil, err
	}
	if err := r.validateGovernanceChanges(ctx, old.Governance, next.Governance); err != nil {
		return nil, err
	}

	oldPlugins := pluginMap(old.Plugins)
	prepared := make([]*preparedPlugin, 0)
	for _, row := range sortedPlugins(next.Plugins) {
		if row == nil || !row.Enabled {
			continue
		}
		previous := oldPlugins[row.Name]
		if previous != nil && sameEntity("plugin", previous, row) {
			continue
		}
		plugin, err := InstantiatePlugin(ctx, row.Name, row.Path, row.Config, r.server.Config)
		if err != nil {
			cleanupPreparedPlugins(prepared)
			return nil, fmt.Errorf("prepare plugin %q: %w", row.Name, err)
		}
		if plugin == nil {
			cleanupPreparedPlugins(prepared)
			return nil, fmt.Errorf("prepare plugin %q: plugin factory returned nil", row.Name)
		}
		prepared = append(prepared, &preparedPlugin{row: row, plugin: plugin})
	}
	return prepared, nil
}

func (r *configSnapshotReconciler) validateProviderChanges(old, next map[schemas.ModelProvider]configstore.ProviderConfig) error {
	for provider, config := range next {
		oldConfig, exists := old[provider]
		if exists && sameEntity("provider", oldConfig, config) {
			continue
		}
		var err error
		if exists {
			err = lib.ValidateCustomProviderUpdate(config, oldConfig, provider)
		} else {
			err = lib.ValidateCustomProvider(config, provider)
		}
		if err != nil {
			return fmt.Errorf("prepare provider %q: %w", provider, err)
		}
	}
	return nil
}

func (r *configSnapshotReconciler) validateGovernanceChanges(ctx context.Context, old, next *configstore.GovernanceConfig) error {
	if !governanceHasDefinitions(next) {
		return nil
	}
	plugin, err := r.server.getGovernancePlugin()
	if err != nil {
		return fmt.Errorf("prepare governance snapshot: %w", err)
	}
	store := plugin.GetGovernanceStore()
	oldRules := routingRuleMap(old.RoutingRules)
	for i := range next.RoutingRules {
		rule := &next.RoutingRules[i]
		if previous := oldRules[rule.ID]; previous != nil && sameEntity("governance", previous, rule) {
			continue
		}
		if _, err := store.GetRoutingProgram(ctx, rule); err != nil {
			return fmt.Errorf("prepare routing rule %q: %w", rule.ID, err)
		}
	}
	return nil
}

func (r *configSnapshotReconciler) applyUpdates(ctx context.Context, old, next *runtimeConfigSnapshot, prepared []*preparedPlugin) error {
	if !sameEntity("client", old.Client, next.Client) {
		if err := r.applyClient(next.Client); err != nil {
			return fmt.Errorf("apply client config: %w", err)
		}
	}
	if !sameEntity("framework", old.Framework, next.Framework) {
		if err := r.applyFramework(ctx, next.Framework); err != nil {
			return fmt.Errorf("apply framework/model catalog config: %w", err)
		}
	}
	if !sameEntity("proxy", old.Proxy, next.Proxy) {
		proxy := next.Proxy
		if proxy == nil {
			proxy = &tables.GlobalProxyConfig{}
		}
		if err := r.server.ReloadProxyConfig(ctx, proxy); err != nil {
			return fmt.Errorf("apply proxy config: %w", err)
		}
	}
	if err := r.applyProviderUpdates(ctx, old.Providers, next.Providers); err != nil {
		return err
	}
	if err := r.applyMCPUpdates(ctx, old.MCP, next.MCP); err != nil {
		return err
	}
	if err := r.applyGovernanceUpdates(ctx, old.Governance, next.Governance); err != nil {
		return err
	}
	if err := r.applyPluginUpdates(ctx, old.Plugins, next.Plugins, prepared); err != nil {
		return err
	}
	if !sameEntity("feature_flags", old.FeatureFlags, next.FeatureFlags) {
		if r.server.Config.FeatureFlags == nil {
			return fmt.Errorf("feature flag runtime is not available")
		}
		for _, row := range next.FeatureFlags {
			r.server.Config.FeatureFlags.ApplyRemote(row.ID, row.Enabled, row.UpdatedAt)
		}
	}
	if !sameEntity("prompts", old.Prompts, next.Prompts) || !sameEntity("prompt_versions", old.PromptVersions, next.PromptVersions) {
		if reloader, err := lib.FindPluginAs[handlers.PromptCacheReloader](r.server.Config, r.server.getPromptsPluginName()); err == nil && reloader != nil {
			if err := reloader.Reload(ctx); err != nil {
				return fmt.Errorf("reload prompt cache: %w", err)
			}
		}
	}
	return nil
}

func (r *configSnapshotReconciler) applyDeletes(ctx context.Context, old, next *runtimeConfigSnapshot) error {
	// Reverse of the dependency application order: plugins, governance, MCP, providers.
	if err := r.applyPluginDeletes(ctx, old.Plugins, next.Plugins); err != nil {
		return err
	}
	if err := r.applyGovernanceDeletes(ctx, old.Governance, next.Governance); err != nil {
		return err
	}
	if err := r.applyMCPDeletes(ctx, old.MCP, next.MCP); err != nil {
		return err
	}
	if err := r.applyProviderDeletes(ctx, old.Providers, next.Providers); err != nil {
		return err
	}
	return nil
}

func (r *configSnapshotReconciler) applyClient(config *configstore.ClientConfig) error {
	if config == nil {
		return fmt.Errorf("client config is missing")
	}
	s := r.server
	if err := s.Client.UpdateToolManagerConfig(
		config.MCPAgentDepth,
		config.MCPToolExecutionTimeout,
		config.MCPCodeModeBindingLevel,
		config.MCPDisableAutoToolInject,
	); err != nil {
		return err
	}

	candidate, err := cloneJSON(config)
	if err != nil {
		return err
	}
	s.Config.Mu.Lock()
	*s.Config.ClientConfig = *candidate
	s.Config.Mu.Unlock()
	s.Config.SetHeaderMatcher(lib.NewHeaderMatcher(candidate.HeaderFilterConfig))
	if s.AuthMiddleware != nil {
		s.AuthMiddleware.UpdateWhitelistedRoutes(candidate.WhitelistedRoutes)
		s.AuthMiddleware.UpdateTempTokenAuthEnabled(candidate.MCPEnableTempTokenAuth)
	}
	if s.CORSMiddleware != nil {
		s.CORSMiddleware.UpdateConfig(s.Config)
	}
	s.Client.ReloadConfig(schemas.BifrostConfig{
		Account:            lib.NewBaseAccount(s.Config),
		InitialPoolSize:    candidate.InitialPoolSize,
		DropExcessRequests: candidate.DropExcessRequests,
		LLMPlugins:         s.Config.GetLoadedLLMPlugins(),
		MCPPlugins:         s.Config.GetLoadedMCPPlugins(),
		MCPConfig:          s.Config.MCPConfig,
		Logger:             logger,
	})
	return nil
}

func (r *configSnapshotReconciler) applyFramework(ctx context.Context, config *tables.TableFrameworkConfig) error {
	if config == nil {
		return fmt.Errorf("framework config is missing")
	}
	candidate := &modelcatalog.Config{
		PricingURL:             config.PricingURL,
		PricingSyncInterval:    config.PricingSyncInterval,
		ModelParametersURL:     config.ModelParametersURL,
		MCPLibraryURL:          config.MCPLibraryURL,
		MCPLibrarySyncInterval: config.MCPLibrarySyncInterval,
	}
	old := r.server.Config.FrameworkConfig.Pricing
	if err := r.server.Config.ModelCatalog.UpdateSyncConfig(ctx, candidate); err != nil {
		return err
	}
	if err := r.server.Config.ModelCatalog.ReloadFromDB(ctx); err != nil {
		if old != nil {
			_ = r.server.Config.ModelCatalog.UpdateSyncConfig(ctx, old)
		}
		return err
	}
	r.server.Config.Mu.Lock()
	if r.server.Config.FrameworkConfig == nil {
		r.server.Config.FrameworkConfig = &framework.FrameworkConfig{Pricing: candidate}
	} else if r.server.Config.FrameworkConfig.Pricing == nil {
		r.server.Config.FrameworkConfig.Pricing = candidate
	} else {
		*r.server.Config.FrameworkConfig.Pricing = *candidate
	}
	r.server.Config.Mu.Unlock()
	return nil
}

func (r *configSnapshotReconciler) applyProviderUpdates(ctx context.Context, old, next map[schemas.ModelProvider]configstore.ProviderConfig) error {
	for _, provider := range sortedProviderNames(next) {
		config := next[provider]
		previous, exists := old[provider]
		if exists && sameEntity("provider", previous, config) {
			continue
		}
		candidate, err := cloneJSON(config)
		if err != nil {
			return fmt.Errorf("clone provider %q: %w", provider, err)
		}
		if exists {
			if err := r.server.Config.UpdateProviderConfig(ctx, provider, candidate); err != nil {
				return fmt.Errorf("apply provider %q update: %w", provider, err)
			}
		} else if _, runtimeErr := r.server.Config.GetProviderConfigRaw(provider); runtimeErr == nil {
			if err := r.server.Config.UpdateProviderConfig(ctx, provider, candidate); err != nil {
				return fmt.Errorf("reconcile locally-applied provider %q: %w", provider, err)
			}
		} else {
			if err := r.server.Config.AddProvider(ctx, provider, candidate); err != nil {
				return fmt.Errorf("apply provider %q create: %w", provider, err)
			}
			if err := r.server.Client.UpdateProvider(provider); err != nil {
				_ = r.server.Config.RemoveProvider(ctx, provider)
				return fmt.Errorf("activate provider %q: %w", provider, err)
			}
		}
		r.server.Config.ModelCatalog.SetKeyConfigForProvider(provider, candidate.Keys)
		r.server.Config.ModelCatalog.InvalidateLiveProvider(provider)
		r.server.RefreshLiveModelsForProvider(ctx, provider, candidate.Keys)
	}
	return nil
}

func (r *configSnapshotReconciler) applyMCPUpdates(ctx context.Context, old, next *schemas.MCPConfig) error {
	oldClients := mcpClientMap(old)
	for _, client := range sortedMCPClients(next) {
		previous := oldClients[client.ID]
		if previous != nil && sameEntity("mcp", previous, client) {
			continue
		}
		candidate, err := cloneJSON(client)
		if err != nil {
			return fmt.Errorf("clone MCP client %q: %w", client.ID, err)
		}
		if previous == nil {
			if _, runtimeErr := r.server.Config.GetMCPClient(client.ID); runtimeErr == nil {
				if err := r.server.UpdateMCPClient(ctx, client.ID, candidate); err != nil {
					return fmt.Errorf("reconcile locally-applied MCP client %q: %w", client.ID, err)
				}
			} else if err := r.server.AddMCPClient(ctx, candidate); err != nil {
				return fmt.Errorf("apply MCP client %q create: %w", client.ID, err)
			}
			continue
		}
		if mcpCredentialsChanged(previous, candidate) && !isPerUserMCPAuth(candidate.AuthType) {
			if err := r.server.UpdateMCPClientCredentials(ctx, client.ID, candidate); err != nil {
				return fmt.Errorf("apply MCP client %q connection: %w", client.ID, err)
			}
		}
		if err := r.server.UpdateMCPClient(ctx, client.ID, candidate); err != nil {
			return fmt.Errorf("apply MCP client %q update: %w", client.ID, err)
		}
	}
	return nil
}

func (r *configSnapshotReconciler) applyGovernanceUpdates(ctx context.Context, old, next *configstore.GovernanceConfig) error {
	if !governanceHasDefinitions(old) && !governanceHasDefinitions(next) {
		return nil
	}
	plugin, err := r.server.getGovernancePlugin()
	if err != nil {
		return fmt.Errorf("apply governance config: %w", err)
	}
	store := plugin.GetGovernanceStore()

	applyChangedValues(old.Budgets, next.Budgets, func(v tables.TableBudget) string { return v.ID }, func(v *tables.TableBudget) {
		store.UpsertBudgetConfig(ctx, v.ID, v)
	})
	applyChangedValues(old.RateLimits, next.RateLimits, func(v tables.TableRateLimit) string { return v.ID }, func(v *tables.TableRateLimit) {
		store.UpsertRateLimitConfig(ctx, v.ID, v)
	})
	applyChangedValues(old.Customers, next.Customers, func(v tables.TableCustomer) string { return v.ID }, func(v *tables.TableCustomer) {
		store.UpdateCustomerInMemory(ctx, v, nil)
	})
	applyChangedValues(old.Teams, next.Teams, func(v tables.TableTeam) string { return v.ID }, func(v *tables.TableTeam) {
		store.UpdateTeamInMemory(ctx, v, nil)
	})
	applyChangedValues(old.Providers, next.Providers, func(v tables.TableProvider) string { return v.Name }, func(v *tables.TableProvider) {
		store.UpdateProviderInMemory(ctx, v)
	})
	applyChangedValues(old.ModelConfigs, next.ModelConfigs, func(v tables.TableModelConfig) string { return v.ID }, func(v *tables.TableModelConfig) {
		store.UpdateModelConfigInMemory(ctx, v)
	})

	oldVKs := virtualKeyMap(old.VirtualKeys)
	applyChangedValues(old.VirtualKeys, next.VirtualKeys, func(v tables.TableVirtualKey) string { return v.ID }, func(v *tables.TableVirtualKey) {
		if previous := oldVKs[v.ID]; previous != nil && previous.Value.IsSet() && v.Value.IsSet() && previous.Value.GetValue() != v.Value.GetValue() && r.server.MCPServerHandler != nil {
			r.server.MCPServerHandler.DeleteVKMCPServer(previous.Value.GetValue())
		}
		store.UpdateVirtualKeyInMemory(ctx, v, nil, nil, nil)
		if r.server.MCPServerHandler != nil {
			r.server.MCPServerHandler.SyncVKMCPServer(v)
		}
	})

	oldRules := routingRuleMap(old.RoutingRules)
	for i := range next.RoutingRules {
		rule := &next.RoutingRules[i]
		if previous := oldRules[rule.ID]; previous != nil && sameEntity("governance", previous, rule) {
			continue
		}
		if err := store.UpdateRoutingRuleInMemory(ctx, rule); err != nil {
			return fmt.Errorf("apply routing rule %q: %w", rule.ID, err)
		}
	}
	oldOverrides := pricingOverrideMap(old.PricingOverrides)
	for i := range next.PricingOverrides {
		override := &next.PricingOverrides[i]
		if previous := oldOverrides[override.ID]; previous != nil && sameEntity("governance", previous, override) {
			continue
		}
		if err := r.server.UpsertPricingOverride(ctx, override); err != nil {
			return fmt.Errorf("apply pricing override %q: %w", override.ID, err)
		}
	}
	if !sameEntity("governance", old.AuthConfig, next.AuthConfig) {
		auth := next.AuthConfig
		if auth == nil {
			auth = &configstore.AuthConfig{}
		}
		if err := r.server.ApplyAuthConfig(ctx, auth); err != nil {
			return fmt.Errorf("apply auth config: %w", err)
		}
	}
	if !sameEntity("governance", old.ComplexityAnalyzerConfig, next.ComplexityAnalyzerConfig) {
		if err := r.server.ReloadComplexityAnalyzerConfig(ctx, next.ComplexityAnalyzerConfig); err != nil {
			return fmt.Errorf("apply complexity analyzer config: %w", err)
		}
	}
	return nil
}

func (r *configSnapshotReconciler) applyPluginUpdates(ctx context.Context, oldRows, nextRows []*tables.TablePlugin, prepared []*preparedPlugin) error {
	old := pluginMap(oldRows)
	candidates := make(map[string]*preparedPlugin, len(prepared))
	for _, candidate := range prepared {
		candidates[candidate.row.Name] = candidate
	}
	for _, row := range sortedPlugins(nextRows) {
		previous := old[row.Name]
		if previous != nil && sameEntity("plugin", previous, row) {
			continue
		}
		if row.Enabled {
			candidate := candidates[row.Name]
			if candidate == nil {
				return fmt.Errorf("prepared plugin %q is missing", row.Name)
			}
			if err := r.server.SyncLoadedPlugin(ctx, row.Name, candidate.plugin, row.Placement, row.Order); err != nil {
				return fmt.Errorf("apply plugin %q: %w", row.Name, err)
			}
			candidate.used = true
			continue
		}
		disabledCtx := context.WithValue(ctx, handlers.PluginDisabledKey, true)
		if err := r.server.RemovePlugin(disabledCtx, row.Name); err != nil && !errors.Is(err, dynamicplugins.ErrPluginNotFound) {
			return fmt.Errorf("disable plugin %q: %w", row.Name, err)
		}
		if err := r.server.markPluginDisabled(row.Name); err != nil {
			return fmt.Errorf("mark plugin %q disabled: %w", row.Name, err)
		}
	}
	r.server.Config.Mu.Lock()
	r.server.Config.PluginConfigs = pluginConfigs(nextRows)
	r.server.Config.Mu.Unlock()
	return nil
}

func (r *configSnapshotReconciler) applyPluginDeletes(ctx context.Context, oldRows, nextRows []*tables.TablePlugin) error {
	next := pluginMap(nextRows)
	old := sortedPlugins(oldRows)
	for i := len(old) - 1; i >= 0; i-- {
		row := old[i]
		if _, exists := next[row.Name]; exists {
			continue
		}
		if err := r.server.RemovePlugin(ctx, row.Name); err != nil && !errors.Is(err, dynamicplugins.ErrPluginNotFound) {
			return fmt.Errorf("delete plugin %q from runtime: %w", row.Name, err)
		}
	}
	return nil
}

func (r *configSnapshotReconciler) applyGovernanceDeletes(ctx context.Context, old, next *configstore.GovernanceConfig) error {
	if !governanceHasDefinitions(old) {
		return nil
	}
	plugin, err := r.server.getGovernancePlugin()
	if err != nil {
		return fmt.Errorf("delete governance runtime definitions: %w", err)
	}
	store := plugin.GetGovernanceStore()

	deleteMissing(old.PricingOverrides, next.PricingOverrides, func(v tables.TablePricingOverride) string { return v.ID }, func(id string) {
		r.server.Config.ModelCatalog.DeletePricingOverride(id)
	})
	deleteMissing(old.RoutingRules, next.RoutingRules, func(v tables.TableRoutingRule) string { return v.ID }, func(id string) {
		_ = store.DeleteRoutingRuleInMemory(ctx, id)
	})
	oldVKs := virtualKeyMap(old.VirtualKeys)
	deleteMissing(old.VirtualKeys, next.VirtualKeys, func(v tables.TableVirtualKey) string { return v.ID }, func(id string) {
		store.DeleteVirtualKeyInMemory(ctx, id)
		if previous := oldVKs[id]; previous != nil && previous.Value.IsSet() && r.server.MCPServerHandler != nil {
			r.server.MCPServerHandler.DeleteVKMCPServer(previous.Value.GetValue())
		}
	})
	deleteMissing(old.ModelConfigs, next.ModelConfigs, func(v tables.TableModelConfig) string { return v.ID }, func(id string) {
		store.DeleteModelConfigInMemory(ctx, id)
	})
	deleteMissing(old.Providers, next.Providers, func(v tables.TableProvider) string { return v.Name }, func(id string) {
		store.DeleteProviderInMemory(ctx, id)
	})
	deleteMissing(old.Teams, next.Teams, func(v tables.TableTeam) string { return v.ID }, func(id string) {
		store.DeleteTeamInMemory(ctx, id)
	})
	deleteMissing(old.Customers, next.Customers, func(v tables.TableCustomer) string { return v.ID }, func(id string) {
		store.DeleteCustomerInMemory(ctx, id)
	})
	deleteMissing(old.RateLimits, next.RateLimits, func(v tables.TableRateLimit) string { return v.ID }, func(id string) {
		store.DeleteRateLimit(ctx, id)
	})
	deleteMissing(old.Budgets, next.Budgets, func(v tables.TableBudget) string { return v.ID }, func(id string) {
		store.DeleteBudget(ctx, id)
	})

	cloned, err := cloneJSON(next)
	if err != nil {
		return fmt.Errorf("clone governance config: %w", err)
	}
	r.server.Config.Mu.Lock()
	if r.server.Config.GovernanceConfig == nil {
		r.server.Config.GovernanceConfig = cloned
	} else {
		*r.server.Config.GovernanceConfig = *cloned
	}
	r.server.Config.Mu.Unlock()
	return nil
}

func (r *configSnapshotReconciler) applyMCPDeletes(ctx context.Context, old, next *schemas.MCPConfig) error {
	nextClients := mcpClientMap(next)
	oldClients := sortedMCPClients(old)
	for i := len(oldClients) - 1; i >= 0; i-- {
		client := oldClients[i]
		if _, exists := nextClients[client.ID]; exists {
			continue
		}
		if _, runtimeErr := r.server.Config.GetMCPClient(client.ID); runtimeErr == nil {
			if err := r.server.RemoveMCPClient(ctx, client.ID); err != nil {
				return fmt.Errorf("delete MCP client %q from runtime: %w", client.ID, err)
			}
		}
	}
	return nil
}

func (r *configSnapshotReconciler) applyProviderDeletes(ctx context.Context, old, next map[schemas.ModelProvider]configstore.ProviderConfig) error {
	providers := sortedProviderNames(old)
	for i := len(providers) - 1; i >= 0; i-- {
		provider := providers[i]
		if _, exists := next[provider]; exists {
			continue
		}
		if _, runtimeErr := r.server.Config.GetProviderConfigRaw(provider); runtimeErr == nil {
			if err := r.server.Client.RemoveProvider(provider); err != nil && !strings.Contains(err.Error(), "not found") {
				return fmt.Errorf("delete provider %q from core: %w", provider, err)
			}
			if err := r.server.Config.RemoveProvider(ctx, provider); err != nil {
				return fmt.Errorf("delete provider %q from config: %w", provider, err)
			}
		}
		r.server.Config.ModelCatalog.InvalidateLiveProvider(provider)
		r.server.Config.ModelCatalog.RemoveKeyConfigForProvider(provider)
	}
	return nil
}

// ApplyAuthConfig applies dashboard authentication config without persisting it.
func (s *BifrostHTTPServer) ApplyAuthConfig(ctx context.Context, config *configstore.AuthConfig) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.Config == nil {
		return fmt.Errorf("config runtime is not available")
	}
	if config == nil {
		return fmt.Errorf("auth config is nil")
	}
	if config.IsEnabled && (config.AdminUserName == nil || config.AdminUserName.GetValue() == "" || config.AdminPassword == nil || config.AdminPassword.GetValue() == "") {
		return fmt.Errorf("username and password are required when auth is enabled")
	}
	candidate, err := cloneJSON(config)
	if err != nil {
		return fmt.Errorf("clone auth config: %w", err)
	}
	s.Config.Mu.Lock()
	if s.Config.GovernanceConfig == nil {
		s.Config.GovernanceConfig = &configstore.GovernanceConfig{}
	}
	s.Config.GovernanceConfig.AuthConfig = candidate
	s.Config.Mu.Unlock()
	if s.AuthMiddleware != nil {
		s.AuthMiddleware.UpdateAuthConfig(candidate)
	}
	return nil
}

func validateMCPChanges(old, next *schemas.MCPConfig) error {
	seen := make(map[string]struct{}, len(next.ClientConfigs))
	oldClients := mcpClientMap(old)
	for _, client := range next.ClientConfigs {
		if client == nil || strings.TrimSpace(client.ID) == "" {
			return fmt.Errorf("prepare MCP snapshot: every client requires an ID")
		}
		if _, exists := seen[client.ID]; exists {
			return fmt.Errorf("prepare MCP snapshot: duplicate client ID %q", client.ID)
		}
		seen[client.ID] = struct{}{}
		if strings.TrimSpace(client.Name) == "" {
			return fmt.Errorf("prepare MCP client %q: name is required", client.ID)
		}
		if previous := oldClients[client.ID]; previous != nil && mcpImmutableChanged(previous, client) {
			return fmt.Errorf("MCP client %q connection_type, connection_string, stdio, TLS, or auth_type changed; safe hot update is unavailable and a restart is required", client.ID)
		}
	}
	return nil
}

func mcpImmutableChanged(old, next *schemas.MCPClientConfig) bool {
	oldAuth := old.AuthType
	if oldAuth == "" {
		oldAuth = schemas.MCPAuthTypeHeaders
	}
	nextAuth := next.AuthType
	if nextAuth == "" {
		nextAuth = schemas.MCPAuthTypeHeaders
	}
	return !sameEntity("mcp", struct {
		ConnectionType   schemas.MCPConnectionType
		ConnectionString *schemas.SecretVar
		StdioConfig      *schemas.MCPStdioConfig
		TLSConfig        *schemas.MCPTLSConfig
		AuthType         schemas.MCPAuthType
	}{old.ConnectionType, old.ConnectionString, old.StdioConfig, old.TLSConfig, oldAuth}, struct {
		ConnectionType   schemas.MCPConnectionType
		ConnectionString *schemas.SecretVar
		StdioConfig      *schemas.MCPStdioConfig
		TLSConfig        *schemas.MCPTLSConfig
		AuthType         schemas.MCPAuthType
	}{next.ConnectionType, next.ConnectionString, next.StdioConfig, next.TLSConfig, nextAuth})
}

func mcpCredentialsChanged(old, next *schemas.MCPClientConfig) bool {
	return !sameEntity("mcp", struct {
		Headers       map[string]schemas.SecretVar
		OauthConfigID *string
	}{old.Headers, old.OauthConfigID}, struct {
		Headers       map[string]schemas.SecretVar
		OauthConfigID *string
	}{next.Headers, next.OauthConfigID})
}

func isPerUserMCPAuth(auth schemas.MCPAuthType) bool {
	return auth == schemas.MCPAuthTypePerUserOauth || auth == schemas.MCPAuthTypePerUserHeaders
}

func governanceHasDefinitions(config *configstore.GovernanceConfig) bool {
	if config == nil {
		return false
	}
	return len(config.VirtualKeys) > 0 || len(config.Teams) > 0 || len(config.Customers) > 0 ||
		len(config.Budgets) > 0 || len(config.RateLimits) > 0 || len(config.ModelConfigs) > 0 ||
		len(config.Providers) > 0 || len(config.RoutingRules) > 0 || len(config.PricingOverrides) > 0 ||
		config.AuthConfig != nil || config.ComplexityAnalyzerConfig != nil
}

func cleanupPreparedPlugins(prepared []*preparedPlugin) {
	for _, candidate := range prepared {
		if candidate != nil && !candidate.used && candidate.plugin != nil {
			_ = candidate.plugin.Cleanup()
		}
	}
}

func pluginConfigs(rows []*tables.TablePlugin) []*schemas.PluginConfig {
	configs := make([]*schemas.PluginConfig, 0, len(rows))
	for _, row := range sortedPlugins(rows) {
		version := row.Version
		configs = append(configs, &schemas.PluginConfig{
			Enabled: row.Enabled, Name: row.Name, Path: row.Path, Version: &version,
			Config: row.Config, Placement: row.Placement, Order: row.Order,
		})
	}
	return configs
}

func canonicalHash(domain string, value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	var decoded any
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.UseNumber()
	if err := decoder.Decode(&decoded); err != nil {
		return "", err
	}
	normalized := normalizeCanonical(domain, decoded)
	data, err = json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func normalizeCanonical(domain string, value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, child := range typed {
			if omitCanonicalField(domain, key) {
				continue
			}
			out[key] = normalizeCanonical(domain, child)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i := range typed {
			out[i] = normalizeCanonical(domain, typed[i])
		}
		return out
	default:
		return value
	}
}

func omitCanonicalField(domain, key string) bool {
	key = strings.ToLower(key)
	if key == "config_hash" || key == "created_at" || key == "updated_at" || key == "deleted_at" ||
		key == "last_reset" || key == "last_used_at" || key == "last_checked_at" {
		return true
	}
	if key == "current_usage" || key == "token_current_usage" || key == "request_current_usage" {
		return true
	}
	if key == "discovered_tools" || key == "discovered_tool_name_mapping" || key == "is_ping_available" || key == "state" {
		return true
	}
	if (domain == "provider" || domain == "snapshot") && (key == "status" || key == "description") {
		return true
	}
	if (domain == "framework" || domain == "plugin") && key == "id" {
		return true
	}
	return false
}

func sameEntity(domain string, left, right any) bool {
	leftHash, leftErr := canonicalHash(domain, left)
	rightHash, rightErr := canonicalHash(domain, right)
	return leftErr == nil && rightErr == nil && leftHash == rightHash
}

func cloneJSON[T any](value T) (T, error) {
	var out T
	data, err := json.Marshal(value)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return out, err
	}
	return out, nil
}

func sortedProviderNames(providers map[schemas.ModelProvider]configstore.ProviderConfig) []schemas.ModelProvider {
	names := make([]schemas.ModelProvider, 0, len(providers))
	for name := range providers {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool { return names[i] < names[j] })
	return names
}

func sortedMCPClients(config *schemas.MCPConfig) []*schemas.MCPClientConfig {
	if config == nil {
		return []*schemas.MCPClientConfig{}
	}
	clients := append([]*schemas.MCPClientConfig{}, config.ClientConfigs...)
	sort.Slice(clients, func(i, j int) bool {
		if clients[i] == nil {
			return false
		}
		if clients[j] == nil {
			return true
		}
		return clients[i].ID < clients[j].ID
	})
	return clients
}

func mcpClientMap(config *schemas.MCPConfig) map[string]*schemas.MCPClientConfig {
	out := map[string]*schemas.MCPClientConfig{}
	if config == nil {
		return out
	}
	for _, client := range config.ClientConfigs {
		if client != nil {
			out[client.ID] = client
		}
	}
	return out
}

func sortedPlugins(rows []*tables.TablePlugin) []*tables.TablePlugin {
	out := make([]*tables.TablePlugin, 0, len(rows))
	for _, row := range rows {
		if row != nil {
			out = append(out, row)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func pluginMap(rows []*tables.TablePlugin) map[string]*tables.TablePlugin {
	out := make(map[string]*tables.TablePlugin, len(rows))
	for _, row := range rows {
		if row != nil {
			out[row.Name] = row
		}
	}
	return out
}

func virtualKeyMap(rows []tables.TableVirtualKey) map[string]*tables.TableVirtualKey {
	out := make(map[string]*tables.TableVirtualKey, len(rows))
	for i := range rows {
		out[rows[i].ID] = &rows[i]
	}
	return out
}

func routingRuleMap(rows []tables.TableRoutingRule) map[string]*tables.TableRoutingRule {
	out := make(map[string]*tables.TableRoutingRule, len(rows))
	for i := range rows {
		out[rows[i].ID] = &rows[i]
	}
	return out
}

func pricingOverrideMap(rows []tables.TablePricingOverride) map[string]*tables.TablePricingOverride {
	out := make(map[string]*tables.TablePricingOverride, len(rows))
	for i := range rows {
		out[rows[i].ID] = &rows[i]
	}
	return out
}

func applyChangedValues[T any](old, next []T, key func(T) string, apply func(*T)) {
	oldMap := make(map[string]T, len(old))
	for _, value := range old {
		oldMap[key(value)] = value
	}
	ordered := append([]T{}, next...)
	sort.Slice(ordered, func(i, j int) bool { return key(ordered[i]) < key(ordered[j]) })
	for i := range ordered {
		previous, exists := oldMap[key(ordered[i])]
		if exists && sameEntity("governance", previous, ordered[i]) {
			continue
		}
		apply(&ordered[i])
	}
}

func deleteMissing[T any](old, next []T, key func(T) string, remove func(string)) {
	nextSet := make(map[string]struct{}, len(next))
	for _, value := range next {
		nextSet[key(value)] = struct{}{}
	}
	keys := make([]string, 0)
	for _, value := range old {
		id := key(value)
		if _, exists := nextSet[id]; !exists {
			keys = append(keys, id)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(keys)))
	for _, id := range keys {
		remove(id)
	}
}

var _ ConfigSnapshotReconciler = (*configSnapshotReconciler)(nil)
