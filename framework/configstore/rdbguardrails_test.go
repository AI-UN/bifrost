package configstore

import (
	"context"
	"errors"
	"testing"

	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore/tables"
	"github.com/maximhq/bifrost/framework/encrypt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func testGuardrailProvider(id uint, name string) *tables.TableGuardrailProvider {
	return &tables.TableGuardrailProvider{
		ID:           id,
		ProviderName: "presidio",
		PolicyName:   name,
		Enabled:      true,
		Timeout:      15,
		ConfigJSON:   `{"analyzer_url":"https://presidio.internal","language":"en","score_threshold":0.5}`,
	}
}

func testGuardrailRule(id uint, name string, profileIDs []uint) *tables.TableGuardrailRule {
	rate := 50
	return &tables.TableGuardrailRule{
		ID:                id,
		Name:              name,
		Description:       "blocks PII on chat requests",
		Enabled:           true,
		Target:            "llm",
		ApplyTo:           "input",
		CELExpression:     `request.model == "gpt-4o"`,
		QueryJSON:         `{"provider":"openai","model":"gpt-4o"}`,
		SamplingRate:      &rate,
		Timeout:           30,
		ProviderConfigIDs: profileIDs,
	}
}

func TestGuardrailProviderCRUD(t *testing.T) {
	ctx := context.Background()
	store := setupRDBTestStore(t)

	// An explicit ID (config.json seeds them) is kept as given.
	require.NoError(t, store.CreateGuardrailProvider(ctx, testGuardrailProvider(5, "support-pii")))
	// A create without an ID takes max(id)+1, not the dialect's sequence.
	generated := testGuardrailProvider(0, "billing-pii")
	require.NoError(t, store.CreateGuardrailProvider(ctx, generated))
	assert.Equal(t, uint(6), generated.ID, "the next explicit ID follows the highest existing one")

	got, err := store.GetGuardrailProvider(ctx, 5)
	require.NoError(t, err)
	assert.Equal(t, "support-pii", got.PolicyName)
	assert.True(t, got.Enabled)
	assert.Equal(t, 15, got.Timeout)
	assert.JSONEq(t, `{"analyzer_url":"https://presidio.internal","language":"en","score_threshold":0.5}`, got.ConfigJSON)

	_, err = store.GetGuardrailProvider(ctx, 99)
	require.ErrorIs(t, err, ErrNotFound)

	list, err := store.GetGuardrailProviders(ctx)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, []uint{5, 6}, []uint{list[0].ID, list[1].ID}, "ordered by id")

	// Update replaces settings and keeps created_at.
	createdAt := got.CreatedAt
	got.PolicyName = "support-pii-v2"
	got.Enabled = false
	got.ConfigJSON = `{"analyzer_url":"https://presidio.internal","language":"fr"}`
	require.NoError(t, store.UpdateGuardrailProvider(ctx, got))
	reloaded, err := store.GetGuardrailProvider(ctx, 5)
	require.NoError(t, err)
	assert.Equal(t, "support-pii-v2", reloaded.PolicyName)
	assert.False(t, reloaded.Enabled)
	assert.Equal(t, createdAt.UTC(), reloaded.CreatedAt.UTC(), "created_at is immutable")
	assert.JSONEq(t, `{"analyzer_url":"https://presidio.internal","language":"fr"}`, reloaded.ConfigJSON)

	require.NoError(t, store.DeleteGuardrailProvider(ctx, 5))
	_, err = store.GetGuardrailProvider(ctx, 5)
	require.ErrorIs(t, err, ErrNotFound)
	require.ErrorIs(t, store.DeleteGuardrailProvider(ctx, 5), ErrNotFound)
}

func TestGuardrailProviderConfigEncryptedAtRest(t *testing.T) {
	ctx := context.Background()
	store := setupRDBTestStore(t)

	// The configstore test binary initializes a key in init(), so writes are encrypted.
	provider := testGuardrailProvider(1, "support-pii")
	plainConfig := provider.ConfigJSON
	require.NoError(t, store.CreateGuardrailProvider(ctx, provider))
	assert.Equal(t, plainConfig, provider.ConfigJSON, "the caller's struct stays plaintext in memory")

	var raw tables.TableGuardrailProvider
	require.NoError(t, store.DB().Session(&gorm.Session{SkipHooks: true}).First(&raw, "id = ?", 1).Error)
	assert.Equal(t, tables.EncryptionStatusEncrypted, raw.EncryptionStatus)
	assert.NotContains(t, raw.ConfigJSON, "analyzer_url", "the provider config is encrypted at rest")

	reloaded, err := store.GetGuardrailProvider(ctx, 1)
	require.NoError(t, err)
	assert.JSONEq(t, provider.ConfigJSON, reloaded.ConfigJSON, "and decrypted on read")
}

func TestGuardrailProviderConfigPlaintextWhenEncryptionDisabled(t *testing.T) {
	encrypt.Init("", bifrost.NewDefaultLogger(schemas.LogLevelInfo))
	t.Cleanup(func() { encrypt.Init(testEncryptionKey, bifrost.NewDefaultLogger(schemas.LogLevelInfo)) })

	ctx := context.Background()
	// This store migrates every table, so the startup encryption pass can walk all of them.
	store, db := setupEncryptionTestStore(t)
	provider := testGuardrailProvider(1, "support-pii")
	require.NoError(t, store.CreateGuardrailProvider(ctx, provider))

	var raw tables.TableGuardrailProvider
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).First(&raw, "id = ?", 1).Error)
	assert.Equal(t, tables.EncryptionStatusPlainText, raw.EncryptionStatus)
	assert.JSONEq(t, provider.ConfigJSON, raw.ConfigJSON)

	// Enabling a key later encrypts the rows the startup pass finds.
	encrypt.Init(testEncryptionKey, bifrost.NewDefaultLogger(schemas.LogLevelInfo))
	require.NoError(t, store.EncryptPlaintextRows(ctx))
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).First(&raw, "id = ?", 1).Error)
	assert.Equal(t, tables.EncryptionStatusEncrypted, raw.EncryptionStatus)
	assert.NotContains(t, raw.ConfigJSON, "analyzer_url")
	reloaded, err := store.GetGuardrailProvider(ctx, 1)
	require.NoError(t, err)
	assert.JSONEq(t, provider.ConfigJSON, reloaded.ConfigJSON)
}

func TestGuardrailRuleCRUD(t *testing.T) {
	ctx := context.Background()
	store := setupRDBTestStore(t)

	// Profile order is the execution order and must survive the round-trip.
	require.NoError(t, store.CreateGuardrailRule(ctx, testGuardrailRule(1, "chat-pii", []uint{3, 1, 2})))
	generated := testGuardrailRule(0, "completion-pii", []uint{1})
	require.NoError(t, store.CreateGuardrailRule(ctx, generated))
	assert.Equal(t, uint(2), generated.ID, "the next explicit ID follows the highest existing one")

	got, err := store.GetGuardrailRule(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, "chat-pii", got.Name)
	assert.Equal(t, "llm", got.Target)
	assert.Equal(t, "input", got.ApplyTo)
	assert.Equal(t, `request.model == "gpt-4o"`, got.CELExpression)
	require.NotNil(t, got.SamplingRate)
	assert.Equal(t, 50, *got.SamplingRate)
	assert.Equal(t, []uint{3, 1, 2}, got.ProviderConfigIDs, "profile order is preserved")

	var raw tables.TableGuardrailRule
	require.NoError(t, store.DB().Session(&gorm.Session{SkipHooks: true}).First(&raw, "id = ?", 1).Error)
	assert.Equal(t, "[3,1,2]", raw.ProviderConfigIDsJSON)

	_, err = store.GetGuardrailRule(ctx, 99)
	require.ErrorIs(t, err, ErrNotFound)

	// Limit 0 returns everything; a limit/offset pages and still reports the total.
	all, total, err := store.GetGuardrailRules(ctx, GuardrailRulesQueryParams{})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	require.Len(t, all, 2)
	assert.Equal(t, []uint{1, 2}, []uint{all[0].ID, all[1].ID}, "ordered by id")
	page, total, err := store.GetGuardrailRules(ctx, GuardrailRulesQueryParams{Limit: 1, Offset: 1})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	require.Len(t, page, 1)
	assert.Equal(t, uint(2), page[0].ID)

	// Update replaces settings, keeps created_at and rewrites the ordered profile list.
	createdAt := got.CreatedAt
	got.Name = "chat-pii-v2"
	got.Enabled = false
	got.SamplingRate = nil
	got.ProviderConfigIDs = []uint{2, 3}
	require.NoError(t, store.UpdateGuardrailRule(ctx, got))
	reloaded, err := store.GetGuardrailRule(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, "chat-pii-v2", reloaded.Name)
	assert.False(t, reloaded.Enabled)
	assert.Nil(t, reloaded.SamplingRate)
	assert.Equal(t, []uint{2, 3}, reloaded.ProviderConfigIDs)
	assert.Equal(t, createdAt.UTC(), reloaded.CreatedAt.UTC(), "created_at is immutable")

	require.NoError(t, store.DeleteGuardrailRule(ctx, 1))
	_, err = store.GetGuardrailRule(ctx, 1)
	require.ErrorIs(t, err, ErrNotFound)
	require.ErrorIs(t, store.DeleteGuardrailRule(ctx, 1), ErrNotFound)
}

// TestGuardrailReadsJoinTheCallersTransaction covers the read side of the reference checks: a
// check taken inside a transaction sees that transaction's own writes, which is what keeps a
// profile delete and the rule reference it must reject in one window.
func TestGuardrailReadsJoinTheCallersTransaction(t *testing.T) {
	ctx := context.Background()
	store := setupRDBTestStore(t)
	require.NoError(t, store.CreateGuardrailProvider(ctx, testGuardrailProvider(5, "support-pii")))

	rollback := errors.New("roll back")
	err := store.ExecuteTransaction(ctx, func(tx *gorm.DB) error {
		require.NoError(t, store.DeleteGuardrailProvider(ctx, 5, tx))

		_, err := store.GetGuardrailProvider(ctx, 5, tx)
		assert.ErrorIs(t, err, ErrNotFound, "the transaction sees its own uncommitted delete")
		providers, err := store.GetGuardrailProviders(ctx, tx)
		require.NoError(t, err)
		assert.Empty(t, providers)

		return rollback
	})
	require.ErrorIs(t, err, rollback)

	// The rollback leaves the row in place for the next reader.
	got, err := store.GetGuardrailProvider(ctx, 5)
	require.NoError(t, err)
	assert.Equal(t, "support-pii", got.PolicyName)
}
