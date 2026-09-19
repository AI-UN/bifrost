package configstore

import (
	"context"

	"gorm.io/gorm"
)

type transactionContextKey struct{}

func withTransaction(ctx context.Context, tx *gorm.DB) context.Context {
	return context.WithValue(ctx, transactionContextKey{}, tx)
}

func transactionFromContext(ctx context.Context) (*gorm.DB, bool) {
	if ctx == nil {
		return nil, false
	}
	tx, ok := ctx.Value(transactionContextKey{}).(*gorm.DB)
	return tx, ok && tx != nil
}

// dbForContext returns the active configuration mutation transaction when present.
func (s *RDBConfigStore) dbForContext(ctx context.Context) *gorm.DB {
	if tx, ok := transactionFromContext(ctx); ok {
		return tx.WithContext(ctx)
	}
	return s.DB().WithContext(ctx)
}
