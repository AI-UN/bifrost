// Command migratelogsobjectstorage is a one-off migration tool for existing
// Bifrost deployments that enabled logs_store.object_storage after already
// accumulating request logs in the logs database.
//
// Bifrost's HybridLogStore (framework/logstore/hybrid.go) only offloads
// payloads for rows written *after* object storage was configured — Create,
// CreateIfNotExists, and BatchCreateIfNotExists are the only intercepted
// write paths, and Update is a pass-through that never re-offloads. Rows
// written before object storage was enabled keep has_object=false forever
// and are never migrated automatically; this is a deliberate upstream
// design choice (see the identical "no backfill" note for the sibling
// enterprise/audit-logs archival feature), not a bug.
//
// This tool walks every logs row with has_object=false, uploads its payload
// fields to the same bucket/prefix a live Bifrost node would use (read
// straight from config.json's logs_store section, including object_storage
// and object_storage_exclude_fields), then clears those columns from the
// database row and sets has_object=true — i.e. it performs the same "move"
// that Create() does for new writes, just applied retroactively.
//
// Scope: the `logs` table (LLM request logs) only. MCP tool logs
// (mcp_tool_logs) are intentionally out of scope.
//
// Known limitation: HybridLogStore.prepareDBEntry additionally keeps a
// trimmed last-user-message JSON preview in input_history/
// responses_input_history for NEW writes, so the log list can render a
// preview without an object fetch. That trimming logic is unexported and
// tightly coupled to internal helpers (attachment stripping, summary
// truncation), so this tool does not reproduce it — migrated rows get
// input_history/responses_input_history fully cleared like every other
// offloaded column. The full content remains fetchable from object storage
// exactly as it does for any other offloaded row; only that one quick-view
// preview slot is affected until the row is opened in the log detail view.
//
// Safe to interrupt and re-run: it never processes a row twice (has_object
// flips to true only after a successful upload), and a failed upload or DB
// update on one row is logged and skipped rather than aborting the run —
// the next run picks the row back up automatically.
//
// Online safety: only rows older than -min-age (default 1h) are eligible.
// A freshly created row can still be "in flight" even with object_storage
// already configured — Bifrost's own async payload upload for it may not
// have landed yet, or the request may still be completing and a later
// write (e.g. filling in the final output_message once the response
// arrives) could land between this tool's SELECT and its UPDATE. Since
// migrateOne's UPDATE unconditionally blanks every payload column, racing
// that window could silently overwrite a field the live write had just set
// — a real, if narrow, data-loss risk, not merely a wasted duplicate
// upload. -min-age keeps this tool away from any row young enough for that
// race to be possible; LLM requests complete in low single-digit seconds to
// low minutes even when streaming, so the 1h default leaves an enormous
// margin. A row deleted concurrently by the retention cleaner while this
// tool is mid-upload for it is harmless by contrast: the follow-up UPDATE
// simply matches zero rows.
//
// This tool does not shrink the database file or table. Clearing a
// column's value only frees space inside the file for SQLite/Postgres to
// reuse on future writes — it does not return that space to the OS.
// Reclaiming disk space needs a separate, explicit step after migrating
// (VACUUM for SQLite, pg_repack or VACUUM FULL for Postgres); see the hint
// this tool prints on completion.
//
// Schema footprint: the supporting index this tool creates (see
// ensureMigrationIndex) is never recorded in Bifrost's own tracked
// migrations — it's a plain ad-hoc index, invisible to that system either
// way, so it can't desync Bifrost's migration state or block a future
// Bifrost upgrade. By default the index is dropped automatically once a
// run confirms nothing is left to migrate (see -cleanup-index), leaving
// the schema exactly as Bifrost's own migrations define it.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	bifrost "github.com/maximhq/bifrost/core"
	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/logstore"
	"github.com/maximhq/bifrost/framework/objectstore"
	"gorm.io/gorm"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "migratelogsobjectstorage: %v\n", err)
		os.Exit(1)
	}
}

type options struct {
	configPath   string
	concurrency  int
	maxRows      int64
	minAge       time.Duration
	skipIndex    bool
	cleanupIndex bool
	dryRun       bool
}

func run(ctx context.Context, args []string) error {
	opts := options{
		configPath:   "config.json",
		concurrency:  4,
		minAge:       time.Hour,
		cleanupIndex: true,
	}
	fs := flag.NewFlagSet("migratelogsobjectstorage", flag.ContinueOnError)
	fs.StringVar(&opts.configPath, "config", opts.configPath, "path to the Bifrost config.json whose logs_store section (including object_storage) should be used")
	fs.IntVar(&opts.concurrency, "concurrency", opts.concurrency, "rows fetched from the DB and uploaded concurrently at any one time. This is the main memory knob: peak resident memory is roughly 2-3x concurrency full log rows (payload fields plus their JSON/gzip copies while uploading). Lower it (e.g. 1-2) on memory-constrained hosts; raise it for more throughput when RAM allows")
	fs.Int64Var(&opts.maxRows, "max-rows", 0, "stop after dispatching this many rows (0 = unlimited); use to test on a subset first")
	fs.DurationVar(&opts.minAge, "min-age", opts.minAge, "only migrate rows older than this (e.g. 10m, 1h); keeps the tool away from rows a live Bifrost node could still be writing to. Safe to run alongside a live gateway with the default")
	fs.BoolVar(&opts.skipIndex, "skip-index-creation", false, "do not create (or later drop) the supporting index on has_object; only useful if the index already exists or your DB policy forbids ad-hoc DDL from tools")
	fs.BoolVar(&opts.cleanupIndex, "cleanup-index", true, "drop the supporting index once a run confirms there is nothing left to migrate, so the schema returns to exactly what Bifrost's own migrations define. Left in place by a -max-rows-limited run, an aborted run, or when -skip-index-creation is set")
	fs.BoolVar(&opts.dryRun, "dry-run", false, "only report how many rows still need migrating; makes no changes")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if opts.concurrency <= 0 {
		return fmt.Errorf("-concurrency must be positive")
	}
	if opts.minAge < 0 {
		return fmt.Errorf("-min-age must not be negative")
	}
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := loadLogsStoreConfig(opts.configPath)
	if err != nil {
		return err
	}
	if cfg.ObjectStorage == nil {
		return fmt.Errorf("logs_store.object_storage is not configured in %s — nothing to migrate to", opts.configPath)
	}

	logger := bifrost.NewDefaultLogger(schemas.LogLevelInfo)

	// NewLogStore validates and pings the object store exactly as a live
	// Bifrost node would, and is the only public entry point that resolves
	// the sqlite/postgres dialect and applies the right GORM setup — reusing
	// it here avoids re-deriving that connection logic by hand.
	store, err := logstore.NewLogStore(ctx, cfg, logger)
	if err != nil {
		return fmt.Errorf("open logs store: %w", err)
	}
	defer store.Close(ctx)

	hybrid, ok := store.(*logstore.HybridLogStore)
	if !ok {
		return fmt.Errorf("logs_store.object_storage produced %T instead of *logstore.HybridLogStore — is object_storage actually set in %s?", store, opts.configPath)
	}

	db := hybrid.ScopedDB(ctx)
	if db == nil {
		return fmt.Errorf("logs_store type %q has no raw DB handle available — this tool supports sqlite and postgres logs stores only (not clickhouse)", cfg.Type)
	}

	if opts.dryRun {
		// A true no-op: makes no changes, including no supporting index —
		// just one full-table count under whatever indexes already exist.
		cutoff := time.Now().UTC().Add(-opts.minAge)
		var count int64
		if err := db.WithContext(ctx).Model(&logstore.Log{}).Where("has_object = ? AND timestamp <= ?", false, cutoff).Count(&count).Error; err != nil {
			return fmt.Errorf("count unmigrated rows: %w", err)
		}
		fmt.Printf("dry-run: %d log row(s) older than %s have has_object=false and would be migrated; rerun without -dry-run to migrate them\n", count, opts.minAge)
		return nil
	}

	if !opts.skipIndex {
		// has_object has no index in Bifrost's schema (nothing in normal
		// operation filters on it). Without one, every page query below is
		// an unindexed scan that gets more expensive as more rows migrate
		// ahead of it — this index is what keeps the tool's own query fast.
		// CONCURRENTLY on Postgres avoids the lock a plain CREATE INDEX
		// would hold against a live gateway's reads and writes on this
		// table.
		if err := ensureMigrationIndex(ctx, db, cfg.Type); err != nil {
			return err
		}
	}

	// Independent object store client: the one NewLogStore built inside
	// hybrid is unexported, and this tool bypasses HybridLogStore's write
	// path entirely (raw SQL updates instead of hybrid.Update) since the
	// rows already exist and only need has_object flipped, not re-created.
	objStore, err := objectstore.NewObjectStore(ctx, cfg.ObjectStorage, logger)
	if err != nil {
		return fmt.Errorf("open object store: %w", err)
	}
	defer objStore.Close()

	excluded := excludedFieldSet(cfg)
	prefix := cfg.ObjectStorage.GetPrefix()

	migrated, skippedEmpty, failed, complete, err := migrateAll(ctx, db, objStore, prefix, excluded, logger, opts)
	fmt.Printf("done: migrated=%d skipped_empty=%d failed=%d\n", migrated, skippedEmpty, failed)
	if failed > 0 {
		fmt.Printf("%d row(s) failed to migrate and were left with has_object=false; rerun this tool to retry them\n", failed)
	}
	if migrated > 0 {
		switch cfg.Type {
		case logstore.LogStoreTypeSQLite:
			fmt.Println("note: the database file itself has not shrunk — clearing columns only frees space inside the file for reuse. Run `VACUUM;` against it to reclaim disk space (needs free disk space roughly equal to the current file size, and briefly holds a write lock).")
		case logstore.LogStoreTypePostgres:
			fmt.Println("note: the table's on-disk size has not shrunk — clearing columns only lets future inserts reuse the freed space (autovacuum already does this). To physically shrink the file, run `pg_repack` (online, no extended lock) or `VACUUM FULL` (blocks reads/writes for its duration).")
		}
	}
	if !opts.skipIndex {
		if err == nil && complete && opts.cleanupIndex {
			if dropErr := dropMigrationIndex(ctx, db, cfg.Type); dropErr != nil {
				fmt.Printf("warning: migration finished but the supporting index could not be dropped: %v (drop idx_logs_migration_has_object manually if you want the schema back to Bifrost's baseline)\n", dropErr)
			} else {
				fmt.Println("supporting index idx_logs_migration_has_object dropped — schema back to exactly what Bifrost's own migrations define")
			}
		} else if err == nil {
			fmt.Println("supporting index idx_logs_migration_has_object left in place (rows remain, or -cleanup-index=false) — rerun this tool later to finish and it will be dropped automatically once nothing is left")
		}
	}
	return err
}

// configFile carries only the fields this tool needs to read out of a full
// Bifrost config.json without depending on transports/bifrost-http/lib
// (which would drag in the entire HTTP server config surface).
type configFile struct {
	LogsStore json.RawMessage `json:"logs_store"`
}

// loadLogsStoreConfig reads config.json and decodes its logs_store section
// through logstore.Config's own UnmarshalJSON, so SQLite/Postgres dialect
// config and object_storage (including "env.VAR" secret indirection, via
// schemas.SecretVar) resolve exactly as they would inside a live Bifrost
// process reading the same file.
func loadLogsStoreConfig(path string) (*logstore.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	var cf configFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	if len(cf.LogsStore) == 0 {
		return nil, fmt.Errorf("%s has no logs_store section", path)
	}
	var cfg logstore.Config
	if err := json.Unmarshal(cf.LogsStore, &cfg); err != nil {
		return nil, fmt.Errorf("parse logs_store config: %w", err)
	}
	if !cfg.Enabled {
		return nil, fmt.Errorf("logs_store.enabled is false in %s", path)
	}
	return &cfg, nil
}

func excludedFieldSet(cfg *logstore.Config) map[string]struct{} {
	set := make(map[string]struct{}, len(cfg.ObjectStorageExcludeFields))
	for _, f := range cfg.ObjectStorageExcludeFields {
		set[f] = struct{}{}
	}
	return set
}

// ensureMigrationIndex creates a supporting index on (has_object, id) if one
// isn't already present. has_object carries no index in Bifrost's own
// schema — nothing in normal operation ever filters on it — so without
// this, every page query in migrateAll is an unindexed scan whose cost
// grows with how many rows have already migrated ahead of it.
//
// Postgres uses CONCURRENTLY so building the index does not take a lock
// that would block a live Bifrost node's reads and writes against the same
// table; that variant cannot run inside a transaction, so this goes
// through the raw *sql.DB rather than gorm's Exec, which may otherwise
// wrap it in one.
func ensureMigrationIndex(ctx context.Context, db *gorm.DB, dialect logstore.LogStoreType) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get raw db handle for index creation: %w", err)
	}
	switch dialect {
	case logstore.LogStoreTypePostgres:
		_, err = sqlDB.ExecContext(ctx, `CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_logs_migration_has_object ON logs (has_object, timestamp, id)`)
	case logstore.LogStoreTypeSQLite:
		_, err = sqlDB.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_logs_migration_has_object ON logs (has_object, timestamp, id)`)
	}
	if err != nil {
		return fmt.Errorf("create supporting index on has_object: %w", err)
	}
	return nil
}

// dropMigrationIndex removes the index created by ensureMigrationIndex. Only
// called once migrateAll reports true completion (see its doc comment), so
// the schema is left exactly as Bifrost's own tracked migrations define it
// — this tool's index is deliberately never recorded in that migration
// history, so nothing there would ever notice or reconcile it either way.
func dropMigrationIndex(ctx context.Context, db *gorm.DB, dialect logstore.LogStoreType) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get raw db handle for index removal: %w", err)
	}
	switch dialect {
	case logstore.LogStoreTypePostgres:
		_, err = sqlDB.ExecContext(ctx, `DROP INDEX CONCURRENTLY IF EXISTS idx_logs_migration_has_object`)
	case logstore.LogStoreTypeSQLite:
		_, err = sqlDB.ExecContext(ctx, `DROP INDEX IF EXISTS idx_logs_migration_has_object`)
	}
	if err != nil {
		return fmt.Errorf("drop supporting index on has_object: %w", err)
	}
	return nil
}

type outcome int

const (
	outcomeMigrated outcome = iota
	outcomeSkippedEmpty
	outcomeFailed
)

// migrateAll walks rows still marked has_object=false in small pages sized
// to opts.concurrency (not a separate large batch size), migrating each
// page's rows concurrently and waiting for the whole page to reach a final
// outcome before fetching the next one.
//
// That wait is required for correctness, not just style: has_object only
// flips to true once migrateOne's DB update commits, so if the next page's
// query ran while rows from the current page were still mid-upload, it
// would see the same has_object=false rows again and dispatch them a
// second time — duplicate uploads and duplicate outcome counts. Waiting for
// wg.Wait() before the next query closes that window.
//
// This bounds peak resident rows to exactly opts.concurrency regardless of
// table size or individual payload size, instead of the old behaviour of
// loading a large fixed batch (independent of concurrency) up front and
// holding all of it in memory while only a handful of workers drained it.
//
// Pagination advances by a (timestamp, id) cursor, not purely by
// has_object=false shrinking. That matters for a row whose payload is
// genuinely and permanently empty (e.g. a request that errored before
// anything was captured): migrateOne leaves such a row at has_object=false
// forever by design (see isPayloadEmpty in hybrid.go — the live gateway
// does the exact same thing for it), so relying only on the has_object
// filter to shrink would make this tool re-fetch and re-skip that same row
// on every iteration forever, never reaching len(page) == 0. Advancing the
// cursor past every row we've seen — regardless of its outcome —
// guarantees each page makes forward progress and the run terminates.
// Ordering by timestamp rather than id also means a row that ages past
// -min-age partway through a long run is never stranded behind the
// cursor — it always sits chronologically ahead of whatever's already been
// processed.
//
// The returned complete flag is true only when a page query genuinely
// finds nothing left, as opposed to stopping early because of -max-rows, a
// context cancellation, or the circuit breaker below. The caller uses it
// to decide whether it's safe to drop the supporting index — dropping it
// after a -max-rows-limited or aborted run would just force the next
// invocation to rebuild it immediately.
//
// A page that migrates and skips nothing (every row in it failed) trips a
// circuit breaker after a few consecutive occurrences, so a persistent
// failure (bad bucket, revoked credentials) aborts the run instead of
// spinning forever re-fetching and re-failing the same rows.
func migrateAll(ctx context.Context, db *gorm.DB, objStore objectstore.ObjectStore, prefix string, excluded map[string]struct{}, logger schemas.Logger, opts options) (migrated, skippedEmpty, failed int64, complete bool, err error) {
	payloadFields := logstore.PayloadFieldNames()
	var dispatched int64
	var lastTimestamp time.Time
	var lastID string
	consecutiveDeadPages := 0
	lastPrint := time.Now()

	for {
		if cErr := ctx.Err(); cErr != nil {
			return migrated, skippedEmpty, failed, false, cErr
		}

		pageSize := opts.concurrency
		if opts.maxRows > 0 {
			remaining := opts.maxRows - dispatched
			if remaining <= 0 {
				return migrated, skippedEmpty, failed, false, nil
			}
			if int64(pageSize) > remaining {
				pageSize = int(remaining)
			}
		}

		cutoff := time.Now().UTC().Add(-opts.minAge)
		var page []*logstore.Log
		qErr := db.WithContext(ctx).
			Where("has_object = ? AND timestamp <= ? AND (timestamp, id) > (?, ?)", false, cutoff, lastTimestamp, lastID).
			Order("timestamp, id").Limit(pageSize).Find(&page).Error
		if qErr != nil {
			return migrated, skippedEmpty, failed, false, fmt.Errorf("query page: %w", qErr)
		}
		if len(page) == 0 {
			return migrated, skippedEmpty, failed, true, nil
		}
		dispatched += int64(len(page))
		last := page[len(page)-1]
		lastTimestamp, lastID = last.Timestamp, last.ID

		var pageMigrated, pageSkipped, pageFailed atomic.Int64
		var wg sync.WaitGroup
		for _, entry := range page {
			wg.Add(1)
			go func(entry *logstore.Log) {
				defer wg.Done()
				switch migrateOne(ctx, db, objStore, prefix, excluded, payloadFields, entry, logger) {
				case outcomeMigrated:
					pageMigrated.Add(1)
				case outcomeSkippedEmpty:
					pageSkipped.Add(1)
				case outcomeFailed:
					pageFailed.Add(1)
				}
			}(entry)
		}
		wg.Wait()

		migrated += pageMigrated.Load()
		skippedEmpty += pageSkipped.Load()
		failed += pageFailed.Load()
		if time.Since(lastPrint) >= 5*time.Second {
			fmt.Printf("progress: migrated=%d skipped_empty=%d failed=%d\n", migrated, skippedEmpty, failed)
			lastPrint = time.Now()
		}

		if pageMigrated.Load() == 0 && pageSkipped.Load() == 0 {
			consecutiveDeadPages++
			const maxConsecutiveDeadPages = 3
			if consecutiveDeadPages >= maxConsecutiveDeadPages {
				return migrated, skippedEmpty, failed, false, fmt.Errorf("%d consecutive page(s) failed entirely — aborting to avoid retrying the same rows forever; check object storage connectivity/credentials in -config", consecutiveDeadPages)
			}
		} else {
			consecutiveDeadPages = 0
		}
	}
}

// migrateOne uploads entry's payload and, on success, clears the offloaded
// columns and sets has_object=true. Any failure is logged and reported as
// outcomeFailed rather than returned: one bad row must never abort a run
// that could otherwise migrate millions of others.
func migrateOne(ctx context.Context, db *gorm.DB, objStore objectstore.ObjectStore, prefix string, excluded map[string]struct{}, payloadFields []string, entry *logstore.Log, logger schemas.Logger) outcome {
	payload := logstore.ExtractPayloadFiltered(entry, excluded)

	empty := true
	for _, f := range payloadFields {
		if payload[f] != "" {
			empty = false
			break
		}
	}
	if empty {
		// Nothing offloadable (e.g. an in-flight "processing" row with no
		// content yet). Leave has_object=false — there is nothing to fetch
		// back later, and uploading an empty object would be pure waste.
		return outcomeSkippedEmpty
	}

	data, err := logstore.MarshalPayload(payload)
	if err != nil {
		logger.Warn("migratelogsobjectstorage: marshal payload for log %s: %v", entry.ID, err)
		return outcomeFailed
	}

	key := logstore.ObjectKey(prefix, entry.Timestamp, entry.ID)
	if err := objStore.Put(ctx, key, data, logstore.BuildTags(entry)); err != nil {
		logger.Warn("migratelogsobjectstorage: upload payload for log %s: %v", entry.ID, err)
		return outcomeFailed
	}

	updates := make(map[string]any, len(payloadFields)+1)
	updates["has_object"] = true
	for _, f := range payloadFields {
		if _, skip := excluded[f]; skip {
			continue
		}
		updates[f] = ""
	}
	// The object is already written under a deterministic key at this point.
	// If this update fails, has_object stays false and a re-run simply
	// re-uploads (overwrites) the same key before retrying the update — safe
	// either way.
	if err := db.WithContext(ctx).Model(&logstore.Log{}).Where("id = ?", entry.ID).Updates(updates).Error; err != nil {
		logger.Warn("migratelogsobjectstorage: mark has_object for log %s: %v", entry.ID, err)
		return outcomeFailed
	}
	return outcomeMigrated
}
