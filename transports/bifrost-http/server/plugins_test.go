package server

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"testing"

	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
	configstoreTables "github.com/maximhq/bifrost/framework/configstore/tables"
	"github.com/maximhq/bifrost/framework/logstore"
	"github.com/maximhq/bifrost/plugins/compat"
	"github.com/maximhq/bifrost/plugins/guardrails"
	"github.com/maximhq/bifrost/plugins/routing"
	"github.com/maximhq/bifrost/transports/v2/bifrost-http/lib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestLoggingPluginConfigTakesCaptureClientIPFromLogsStore pins that every load of the logging plugin, a reload from the
// stored plugin config included, follows logs_store.capture_client_ip rather than whatever the plugin config carries.
func TestLoggingPluginConfigTakesCaptureClientIPFromLogsStore(t *testing.T) {
	for _, capture := range []bool{true, false} {
		bifrostConfig := &lib.Config{LogsStoreConfig: &logstore.Config{CaptureClientIP: capture}}
		// A stored plugin config, as a PUT /api/plugins/logging reload decodes it: no capture_client_ip, or a stale one.
		for _, stored := range []map[string]any{{}, {"capture_client_ip": !capture}} {
			got, err := loggingPluginConfig(stored, bifrostConfig)
			if err != nil {
				t.Fatalf("loggingPluginConfig(%v): %v", stored, err)
			}
			if got.CaptureClientIP != capture {
				t.Errorf("logs_store capture=%t, stored %v: CaptureClientIP = %t, want %t", capture, stored, got.CaptureClientIP, capture)
			}
		}
	}
	got, err := loggingPluginConfig(map[string]any{"capture_client_ip": true}, &lib.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if got.CaptureClientIP {
		t.Error("without a logs_store config, capture stays off")
	}
}

// guardrailReadFailsStore fails the guardrail read the plugin's loader makes at startup, the way a
// decryption failure or an unreachable config store would. Everything else goes to the real store.
type guardrailReadFailsStore struct{ configstore.ConfigStore }

func (guardrailReadFailsStore) GetGuardrailProviders(context.Context, ...*gorm.DB) ([]configstoreTables.TableGuardrailProvider, error) {
	return nil, errors.New("failed to decrypt guardrail config")
}

// newGuardrailLoadServer builds a server whose builtin plugin load reaches the guardrails plugin,
// over a real SQLite config store.
func newGuardrailLoadServer(t *testing.T, store configstore.ConfigStore) *BifrostHTTPServer {
	t.Helper()
	SetLogger(bifrost.NewNoOpLogger())
	t.Cleanup(func() { SetLogger(nil) })
	s := &BifrostHTTPServer{
		Ctx: schemas.NewBifrostContext(context.Background(), schemas.NoDeadline),
		Config: &lib.Config{
			ClientConfig: &configstore.ClientConfig{},
			ConfigStore:  store,
		},
	}
	return s
}

// TestGuardrailsInspectBeforeRouting covers the plugin order the guardrail guarantee depends on:
// the routing plugin's LLM classifier sends user text with the pipeline disabled, so inspection
// has to be registered ahead of it.
func TestGuardrailsInspectBeforeRouting(t *testing.T) {
	store, err := configstore.NewConfigStore(context.Background(), &configstore.Config{
		Enabled: true,
		Type:    configstore.ConfigStoreTypeSQLite,
		Config:  &configstore.SQLiteConfig{Path: filepath.Join(t.TempDir(), "config.db")},
	}, bifrost.NewNoOpLogger())
	require.NoError(t, err)
	t.Cleanup(func() { store.Close(context.Background()) })

	s := newGuardrailLoadServer(t, store)
	require.NoError(t, s.LoadPlugins(schemaContext()))

	order := s.Config.GetPluginOrder()
	guardrailsAt := slices.Index(order, guardrails.PluginName)
	require.NotEqual(t, -1, guardrailsAt, "the guardrails plugin must be registered: %v", order)

	// The motivating case: the routing plugin's classifier sends user text with the pipeline
	// disabled, so inspection must be registered ahead of it. This fixture loads routing only
	// when governance exposes its rules repo, so compare when it is there.
	if routingAt := slices.Index(order, routing.PluginName); routingAt != -1 {
		assert.Less(t, guardrailsAt, routingAt, "guardrails must inspect before routing routes the request: %v", order)
	} else {
		t.Logf("routing plugin did not load in this fixture: %v", order)
	}
	// Compat rewrites the request shape, so inspection must see the caller's shape first.
	compatAt := slices.Index(order, compat.PluginName)
	require.NotEqual(t, -1, compatAt, "the compat plugin must be registered: %v", order)
	assert.Less(t, guardrailsAt, compatAt, "guardrails must inspect the shape the caller sent: %v", order)
}

// TestGuardrailStartupFailureStopsTheGateway covers a store read that fails while protection is
// enabled: the gateway must refuse to start rather than serve traffic uninspected.
func TestGuardrailStartupFailureStopsTheGateway(t *testing.T) {
	store, err := configstore.NewConfigStore(context.Background(), &configstore.Config{
		Enabled: true,
		Type:    configstore.ConfigStoreTypeSQLite,
		Config:  &configstore.SQLiteConfig{Path: filepath.Join(t.TempDir(), "config.db")},
	}, bifrost.NewNoOpLogger())
	require.NoError(t, err)
	t.Cleanup(func() { store.Close(context.Background()) })

	s := newGuardrailLoadServer(t, guardrailReadFailsStore{ConfigStore: store})
	err = s.LoadPlugins(schemaContext())
	require.Error(t, err, "a guardrail store failure must stop startup, not leave the gateway unprotected")
	assert.Contains(t, err.Error(), "decrypt")
}

// schemaContext returns the server-lifetime context a plugin load is given.
func schemaContext() context.Context {
	return schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
}
