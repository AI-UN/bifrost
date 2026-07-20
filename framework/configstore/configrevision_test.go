package configstore

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore/tables"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestTriggerMigrations_ConfigRevisionSeed(t *testing.T) {
	store, db := setupFullMigrationDB(t)
	ctx := context.Background()

	revision, err := store.GetConfigRevision(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), revision)

	var rows []tables.TableConfigRevision
	require.NoError(t, db.Order("id").Find(&rows).Error)
	require.Len(t, rows, 1)
	assert.Equal(t, tables.ConfigRevisionSingletonID, rows[0].ID)
	assert.Equal(t, int64(0), rows[0].Revision)
}

func TestExecuteConfigMutation_CommitsMutationAndRevision(t *testing.T) {
	store, _ := setupFullMigrationDB(t)
	ctx := context.Background()
	folder := &tables.TableFolder{ID: "revision-success", Name: "Revision Success"}

	committedRevision, err := store.ExecuteConfigMutation(ctx, 0, func(mutationCtx context.Context) error {
		return store.CreateFolder(mutationCtx, folder)
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), committedRevision)

	storedFolder, err := store.GetFolderByID(ctx, folder.ID)
	require.NoError(t, err)
	assert.Equal(t, folder.Name, storedFolder.Name)

	revision, err := store.GetConfigRevision(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), revision)
}

func TestExecuteConfigMutation_RollsBackMutationAndRevision(t *testing.T) {
	store, _ := setupFullMigrationDB(t)
	ctx := context.Background()
	folder := &tables.TableFolder{ID: "revision-rollback", Name: "Revision Rollback"}
	mutationErr := errors.New("mutation failed")

	committedRevision, err := store.ExecuteConfigMutation(ctx, 0, func(mutationCtx context.Context) error {
		if err := store.CreateFolder(mutationCtx, folder); err != nil {
			return err
		}
		return mutationErr
	})
	assert.Equal(t, int64(0), committedRevision)
	assert.True(t, errors.Is(err, mutationErr))

	_, err = store.GetFolderByID(ctx, folder.ID)
	assert.True(t, errors.Is(err, ErrNotFound))

	revision, err := store.GetConfigRevision(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), revision)
}

func TestExecuteConfigMutation_RollsBackNestedPluginAndRoutingWrites(t *testing.T) {
	store, _ := setupFullMigrationDB(t)
	ctx := context.Background()
	mutationErr := errors.New("mutation failed")

	_, err := store.ExecuteConfigMutation(ctx, 0, func(mutationCtx context.Context) error {
		if err := store.UpdatePlugin(mutationCtx, &tables.TablePlugin{
			Name:    "revision-plugin-rollback",
			Enabled: true,
		}); err != nil {
			return err
		}
		if err := store.CreateRoutingRule(mutationCtx, &tables.TableRoutingRule{
			ID:            "revision-routing-rollback",
			Name:          "Revision Routing Rollback",
			CelExpression: "true",
			Scope:         "global",
		}); err != nil {
			return err
		}
		return mutationErr
	})
	require.ErrorIs(t, err, mutationErr)

	_, err = store.GetPlugin(ctx, "revision-plugin-rollback")
	require.ErrorIs(t, err, ErrNotFound)
	_, err = store.GetRoutingRule(ctx, "revision-routing-rollback")
	require.ErrorIs(t, err, ErrNotFound)
	revision, err := store.GetConfigRevision(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), revision)
}

func TestExecuteConfigMutation_RejectsStaleRevisionWithoutCallingMutator(t *testing.T) {
	store, _ := setupFullMigrationDB(t)
	ctx := context.Background()
	mutatorCalled := false

	committedRevision, err := store.ExecuteConfigMutation(ctx, 7, func(context.Context) error {
		mutatorCalled = true
		return nil
	})
	assert.Equal(t, int64(0), committedRevision)
	assert.False(t, mutatorCalled)
	assert.True(t, errors.Is(err, ErrConfigRevisionConflict))

	var conflict *ConfigRevisionConflictError
	require.True(t, errors.As(err, &conflict))
	assert.Equal(t, int64(7), conflict.Expected)
	assert.Equal(t, int64(0), conflict.Actual)
}

func TestExecuteConfigMutation_SQLiteSequentialExpectedRevision(t *testing.T) {
	store, _ := setupFullMigrationDB(t)
	ctx := context.Background()
	firstFolder := &tables.TableFolder{ID: "revision-first", Name: "Revision First"}
	secondFolder := &tables.TableFolder{ID: "revision-second", Name: "Revision Second"}

	firstRevision, err := store.ExecuteConfigMutation(ctx, 0, func(mutationCtx context.Context) error {
		return store.CreateFolder(mutationCtx, firstFolder)
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), firstRevision)

	secondMutatorCalled := false
	secondRevision, err := store.ExecuteConfigMutation(ctx, 0, func(mutationCtx context.Context) error {
		secondMutatorCalled = true
		return store.CreateFolder(mutationCtx, secondFolder)
	})
	assert.Equal(t, int64(0), secondRevision)
	assert.False(t, secondMutatorCalled)
	assert.True(t, errors.Is(err, ErrConfigRevisionConflict))

	var conflict *ConfigRevisionConflictError
	require.True(t, errors.As(err, &conflict))
	assert.Equal(t, int64(0), conflict.Expected)
	assert.Equal(t, int64(1), conflict.Actual)

	_, err = store.GetFolderByID(ctx, firstFolder.ID)
	require.NoError(t, err)
	_, err = store.GetFolderByID(ctx, secondFolder.ID)
	assert.True(t, errors.Is(err, ErrNotFound))

	revision, err := store.GetConfigRevision(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), revision)
}

func TestExecuteConfigMutation_PostgresConcurrentExpectedRevision(t *testing.T) {
	store := setupPostgresConfigRevisionStore(t)
	ctx := context.Background()
	start := make(chan struct{})
	results := make(chan configRevisionMutationResult, 2)
	var mutatorCalls atomic.Int32
	var wg sync.WaitGroup

	for i := range 2 {
		folder := tables.TableFolder{
			ID:   fmt.Sprintf("revision-postgres-%d", i),
			Name: fmt.Sprintf("Revision Postgres %d", i),
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			revision, err := store.ExecuteConfigMutation(ctx, 0, func(mutationCtx context.Context) error {
				mutatorCalls.Add(1)
				return store.CreateFolder(mutationCtx, &folder)
			})
			results <- configRevisionMutationResult{folderID: folder.ID, revision: revision, err: err}
		}()
	}

	close(start)
	wg.Wait()
	close(results)

	var allResults []configRevisionMutationResult
	for result := range results {
		allResults = append(allResults, result)
	}
	require.Len(t, allResults, 2)

	successes := 0
	conflicts := 0
	for _, result := range allResults {
		if result.err == nil {
			successes++
			assert.Equal(t, int64(1), result.revision)
			_, err := store.GetFolderByID(ctx, result.folderID)
			require.NoError(t, err)
			continue
		}

		assert.True(t, errors.Is(result.err, ErrConfigRevisionConflict))
		var conflict *ConfigRevisionConflictError
		require.True(t, errors.As(result.err, &conflict))
		assert.Equal(t, int64(0), conflict.Expected)
		assert.Equal(t, int64(1), conflict.Actual)
		assert.Equal(t, int64(0), result.revision)
		_, err := store.GetFolderByID(ctx, result.folderID)
		assert.True(t, errors.Is(err, ErrNotFound))
		conflicts++
	}

	assert.Equal(t, 1, successes)
	assert.Equal(t, 1, conflicts)
	assert.Equal(t, int32(1), mutatorCalls.Load())

	revision, err := store.GetConfigRevision(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), revision)
}

type configRevisionMutationResult struct {
	folderID string
	revision int64
	err      error
}

func setupPostgresConfigRevisionStore(t *testing.T) *RDBConfigStore {
	t.Helper()

	adminDB, err := gorm.Open(postgres.Open(postgresDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Skipf("postgres not available: %v", err)
	}
	adminSQLDB, err := adminDB.DB()
	if err != nil {
		t.Skipf("postgres not available: %v", err)
	}
	if err := adminSQLDB.Ping(); err != nil {
		_ = adminSQLDB.Close()
		t.Skipf("postgres not available: %v", err)
	}

	schema := fmt.Sprintf("configrevision_%d", time.Now().UnixNano())
	require.NoError(t, adminDB.Exec("CREATE SCHEMA "+schema).Error)
	t.Cleanup(func() {
		adminDB.Exec("DROP SCHEMA IF EXISTS " + schema + " CASCADE")
		_ = adminSQLDB.Close()
	})

	db, err := gorm.Open(postgres.Open(postgresDSN+" search_path="+schema), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Ping())
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	ctx := context.Background()
	require.NoError(t, db.WithContext(ctx).AutoMigrate(&tables.TableFolder{}))
	require.NoError(t, migrationAddConfigRevisionTable(ctx, db, testMigrationLogger))

	store := &RDBConfigStore{logger: bifrost.NewDefaultLogger(schemas.LogLevelInfo)}
	store.db.Store(db)
	store.migrateOnFreshFn = func(ctx context.Context, fn func(context.Context, *gorm.DB) error) error {
		return fn(ctx, store.DB())
	}
	store.refreshPoolFn = func(context.Context) error { return nil }
	return store
}
