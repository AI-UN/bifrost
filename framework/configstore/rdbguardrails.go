package configstore

import (
	"context"

	"github.com/maximhq/bifrost/framework/configstore/tables"
	"gorm.io/gorm"
)

// guardrailDB returns the transaction to write through: the caller's when given one, so a
// guardrail write joins an enclosing transaction, otherwise the pool.
func (s *RDBConfigStore) guardrailDB(ctx context.Context, tx ...*gorm.DB) *gorm.DB {
	db := s.DB()
	if len(tx) > 0 && tx[0] != nil {
		db = tx[0]
	}
	return db.WithContext(ctx)
}

// nextGuardrailID returns max(id)+1 for a guardrail table. The IDs are explicit (config.json
// seeds them) and the primary key is not autoincrement, so an insert without an ID must
// compute the next one inside the same transaction to stay collision-free.
func nextGuardrailID(tx *gorm.DB, model any) (uint, error) {
	var row struct{ ID uint }
	if err := tx.Model(model).Select("COALESCE(MAX(id), 0) AS id").Scan(&row).Error; err != nil {
		return 0, err
	}
	return row.ID + 1, nil
}

// GetGuardrailProviders loads every guardrail profile, ordered by id. A caller passing a
// transaction reads its own uncommitted writes, which is what keeps a reference check and the
// write it guards in one serializable window.
func (s *RDBConfigStore) GetGuardrailProviders(ctx context.Context, tx ...*gorm.DB) ([]tables.TableGuardrailProvider, error) {
	var providers []tables.TableGuardrailProvider
	if err := s.ScopedDB(ctx, tx...).Order("id ASC").Find(&providers).Error; err != nil {
		return nil, s.parseGormError(err)
	}
	return providers, nil
}

// GetGuardrailProvider loads one guardrail profile, ErrNotFound when it does not exist.
func (s *RDBConfigStore) GetGuardrailProvider(ctx context.Context, id uint, tx ...*gorm.DB) (*tables.TableGuardrailProvider, error) {
	var provider tables.TableGuardrailProvider
	if err := s.ScopedDB(ctx, tx...).First(&provider, "id = ?", id).Error; err != nil {
		return nil, s.parseGormError(err)
	}
	return &provider, nil
}

// CreateGuardrailProvider persists a profile, assigning the next explicit ID when it is zero.
func (s *RDBConfigStore) CreateGuardrailProvider(ctx context.Context, provider *tables.TableGuardrailProvider, tx ...*gorm.DB) error {
	// BeforeSave encrypts ConfigJSON in place; restore it so the caller keeps plaintext in
	// memory and never echoes ciphertext back to an API response.
	plainConfig := provider.ConfigJSON
	defer func() { provider.ConfigJSON = plainConfig }()
	return s.parseGormError(s.guardrailDB(ctx, tx...).Transaction(func(tx *gorm.DB) error {
		if provider.ID == 0 {
			id, err := nextGuardrailID(tx, &tables.TableGuardrailProvider{})
			if err != nil {
				return err
			}
			provider.ID = id
		}
		return tx.Create(provider).Error
	}))
}

// UpdateGuardrailProvider replaces a profile's settings, ErrNotFound when it does not exist.
func (s *RDBConfigStore) UpdateGuardrailProvider(ctx context.Context, provider *tables.TableGuardrailProvider, tx ...*gorm.DB) error {
	plainConfig := provider.ConfigJSON
	defer func() { provider.ConfigJSON = plainConfig }()
	return s.parseGormError(s.guardrailDB(ctx, tx...).Transaction(func(tx *gorm.DB) error {
		var existing tables.TableGuardrailProvider
		if err := dbForUpdate(tx).First(&existing, "id = ?", provider.ID).Error; err != nil {
			return err
		}
		// created_at is immutable: Save writes every column, so a caller passing a struct it
		// did not read from the DB would otherwise zero it out.
		provider.CreatedAt = existing.CreatedAt
		return tx.Save(provider).Error
	}))
}

// DeleteGuardrailProvider removes a profile, ErrNotFound when it does not exist.
func (s *RDBConfigStore) DeleteGuardrailProvider(ctx context.Context, id uint, tx ...*gorm.DB) error {
	return s.parseGormError(s.guardrailDB(ctx, tx...).Transaction(func(tx *gorm.DB) error {
		res := tx.Delete(&tables.TableGuardrailProvider{}, "id = ?", id)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrNotFound
		}
		return nil
	}))
}

// GetGuardrailRules lists guardrail rules ordered by id with the total row count. A Limit of 0
// returns every rule.
func (s *RDBConfigStore) GetGuardrailRules(ctx context.Context, params GuardrailRulesQueryParams, tx ...*gorm.DB) ([]tables.TableGuardrailRule, int64, error) {
	query := s.ScopedDB(ctx, tx...).Model(&tables.TableGuardrailRule{})
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, s.parseGormError(err)
	}
	if params.Limit > 0 {
		query = query.Limit(params.Limit)
	}
	if params.Offset > 0 {
		query = query.Offset(params.Offset)
	}
	var rules []tables.TableGuardrailRule
	if err := query.Order("id ASC").Find(&rules).Error; err != nil {
		return nil, 0, s.parseGormError(err)
	}
	return rules, total, nil
}

// GetGuardrailRule loads one rule, ErrNotFound when it does not exist.
func (s *RDBConfigStore) GetGuardrailRule(ctx context.Context, id uint, tx ...*gorm.DB) (*tables.TableGuardrailRule, error) {
	var rule tables.TableGuardrailRule
	if err := s.ScopedDB(ctx, tx...).First(&rule, "id = ?", id).Error; err != nil {
		return nil, s.parseGormError(err)
	}
	return &rule, nil
}

// CreateGuardrailRule persists a rule, assigning the next explicit ID when it is zero.
func (s *RDBConfigStore) CreateGuardrailRule(ctx context.Context, rule *tables.TableGuardrailRule, tx ...*gorm.DB) error {
	return s.parseGormError(s.guardrailDB(ctx, tx...).Transaction(func(tx *gorm.DB) error {
		if rule.ID == 0 {
			id, err := nextGuardrailID(tx, &tables.TableGuardrailRule{})
			if err != nil {
				return err
			}
			rule.ID = id
		}
		return tx.Create(rule).Error
	}))
}

// UpdateGuardrailRule replaces a rule's settings, ErrNotFound when it does not exist.
func (s *RDBConfigStore) UpdateGuardrailRule(ctx context.Context, rule *tables.TableGuardrailRule, tx ...*gorm.DB) error {
	return s.parseGormError(s.guardrailDB(ctx, tx...).Transaction(func(tx *gorm.DB) error {
		var existing tables.TableGuardrailRule
		if err := dbForUpdate(tx).First(&existing, "id = ?", rule.ID).Error; err != nil {
			return err
		}
		rule.CreatedAt = existing.CreatedAt
		return tx.Save(rule).Error
	}))
}

// DeleteGuardrailRule removes a rule, ErrNotFound when it does not exist.
func (s *RDBConfigStore) DeleteGuardrailRule(ctx context.Context, id uint, tx ...*gorm.DB) error {
	return s.parseGormError(s.guardrailDB(ctx, tx...).Transaction(func(tx *gorm.DB) error {
		res := tx.Delete(&tables.TableGuardrailRule{}, "id = ?", id)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrNotFound
		}
		return nil
	}))
}
