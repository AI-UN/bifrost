package configstore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/maximhq/bifrost/framework/configstore/tables"
	"gorm.io/gorm"
)

// ConfigRevisionStore is the optional capability used by multinode runtime-config synchronization.
type ConfigRevisionStore interface {
	GetConfigRevision(ctx context.Context) (int64, error)
	ExecuteConfigMutation(ctx context.Context, expectedRevision int64, mutate func(context.Context) error) (int64, error)
}

// ConfigSyncMode exposes whether runtime configuration is database-authoritative.
type ConfigSyncMode interface {
	SetConfigSyncEnabled(enabled bool)
	IsConfigSyncEnabled() bool
}

// SetConfigSyncEnabled enables revision-CAS enforcement and peer synchronization.
func (s *RDBConfigStore) SetConfigSyncEnabled(enabled bool) {
	s.configSyncEnabled.Store(enabled)
}

// IsConfigSyncEnabled reports whether runtime configuration synchronization is enabled.
func (s *RDBConfigStore) IsConfigSyncEnabled() bool {
	return s.configSyncEnabled.Load()
}

// GetConfigRevision returns the committed global runtime-configuration revision.
func (s *RDBConfigStore) GetConfigRevision(ctx context.Context) (int64, error) {
	var row tables.TableConfigRevision
	if err := s.dbForContext(ctx).First(&row, tables.ConfigRevisionSingletonID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, fmt.Errorf("config revision row: %w", ErrNotFound)
		}
		return 0, fmt.Errorf("getting config revision: %w", err)
	}
	return row.Revision, nil
}

// ExecuteConfigMutation serializes a configuration write through the singleton revision row.
// mutate and the revision increment commit atomically. A stale expectedRevision aborts without
// invoking mutate.
func (s *RDBConfigStore) ExecuteConfigMutation(
	ctx context.Context,
	expectedRevision int64,
	mutate func(context.Context) error,
) (int64, error) {
	if mutate == nil {
		return 0, fmt.Errorf("config mutation callback cannot be nil")
	}
	if expectedRevision < 0 {
		return 0, fmt.Errorf("expected config revision cannot be negative")
	}

	committedRevision := expectedRevision + 1
	err := s.dbForContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&tables.TableConfigRevision{}).
			Where("id = ? AND revision = ?", tables.ConfigRevisionSingletonID, expectedRevision).
			Updates(map[string]any{
				"revision":   committedRevision,
				"updated_at": time.Now().UTC(),
			})
		if result.Error != nil {
			return fmt.Errorf("claiming config revision: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			var current tables.TableConfigRevision
			if err := tx.First(&current, tables.ConfigRevisionSingletonID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return fmt.Errorf("config revision row: %w", ErrNotFound)
				}
				return fmt.Errorf("getting current config revision: %w", err)
			}
			return &ConfigRevisionConflictError{
				Expected: expectedRevision,
				Actual:   current.Revision,
			}
		}
		return mutate(withTransaction(ctx, tx))
	})
	if err != nil {
		return 0, err
	}
	return committedRevision, nil
}
