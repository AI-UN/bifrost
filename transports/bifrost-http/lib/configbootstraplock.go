package lib

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/maximhq/bifrost/core/schemas"
	"github.com/maximhq/bifrost/framework/configstore"
)

const (
	configBootstrapLockKey       = "multinode-config-bootstrap"
	configBootstrapLockTTL       = 30 * time.Second
	configBootstrapRenewInterval = 10 * time.Second
)

type configBootstrapLock struct {
	lock   *configstore.DistributedLock
	ctx    context.Context
	cancel context.CancelCauseFunc
	done   chan struct{}
	once   sync.Once
	logger schemas.Logger
}

func acquireConfigBootstrapLock(
	ctx context.Context,
	store configstore.ConfigStore,
	logger schemas.Logger,
) (*configBootstrapLock, error) {
	mode, ok := store.(configstore.ConfigSyncMode)
	if !ok || !mode.IsConfigSyncEnabled() {
		return nil, nil
	}
	lockStore, ok := store.(configstore.LockStore)
	if !ok {
		return nil, fmt.Errorf("config sync requires distributed-lock support")
	}
	manager := configstore.NewDistributedLockManager(
		lockStore,
		logger,
		configstore.WithDefaultTTL(configBootstrapLockTTL),
		configstore.WithRetryInterval(250*time.Millisecond),
		configstore.WithMaxRetries(240),
	)
	lock, err := manager.NewLock(configBootstrapLockKey)
	if err != nil {
		return nil, fmt.Errorf("creating config bootstrap lock: %w", err)
	}
	if err := lock.Lock(ctx); err != nil {
		return nil, fmt.Errorf("acquiring config bootstrap lock: %w", err)
	}

	renewCtx, cancel := context.WithCancelCause(ctx)
	holder := &configBootstrapLock{
		lock:   lock,
		ctx:    renewCtx,
		cancel: cancel,
		done:   make(chan struct{}),
		logger: logger,
	}
	go holder.renew(renewCtx)
	return holder, nil
}

func (h *configBootstrapLock) renew(ctx context.Context) {
	defer close(h.done)
	ticker := time.NewTicker(configBootstrapRenewInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := h.lock.Extend(ctx); err != nil {
				h.logger.Error("failed to renew config bootstrap lock: %v", err)
				h.cancel(fmt.Errorf("config bootstrap lock renewal failed: %w", err))
				return
			}
		}
	}
}

func (h *configBootstrapLock) context() context.Context {
	if h == nil {
		return context.Background()
	}
	return h.ctx
}

func (h *configBootstrapLock) err() error {
	if h == nil {
		return nil
	}
	cause := context.Cause(h.ctx)
	if cause == nil || errors.Is(cause, context.Canceled) {
		return nil
	}
	return cause
}

func (h *configBootstrapLock) release() {
	if h == nil {
		return
	}
	h.once.Do(func() {
		h.cancel(context.Canceled)
		<-h.done
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := h.lock.Unlock(ctx); err != nil && !errors.Is(err, configstore.ErrLockNotHeld) {
			h.logger.Error("failed to release config bootstrap lock: %v", err)
		}
	})
}
