package server

import (
	"context"
	"fmt"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"time"

	"github.com/maximhq/bifrost/framework/configstore"
)

const (
	configSyncPollMinInterval = time.Second
	configSyncPollMaxInterval = 5 * time.Second
	configSyncRetryMaxBackoff = 5 * time.Second
)

type configSyncWaitFunc func(context.Context, time.Duration) error

type configSyncWorker struct {
	revisionStore configstore.ConfigRevisionStore
	reconciler    ConfigSnapshotReconciler
	wait          configSyncWaitFunc

	lastApplied atomic.Int64
	ready       atomic.Bool

	cancel   context.CancelFunc
	done     chan struct{}
	stopOnce sync.Once
}

func newConfigSyncWorker(
	ctx context.Context,
	revisionStore configstore.ConfigRevisionStore,
	reconciler ConfigSnapshotReconciler,
	wait configSyncWaitFunc,
) *configSyncWorker {
	if wait == nil {
		wait = waitForConfigSync
	}

	workerCtx, cancel := context.WithCancel(ctx)
	worker := &configSyncWorker{
		revisionStore: revisionStore,
		reconciler:    reconciler,
		wait:          wait,
		cancel:        cancel,
		done:          make(chan struct{}),
	}
	go worker.run(workerCtx)
	return worker
}

func (w *configSyncWorker) run(ctx context.Context) {
	defer close(w.done)

	retryBackoff := configSyncPollMinInterval
	for {
		succeeded := w.poll(ctx)
		if ctx.Err() != nil {
			return
		}

		waitDuration := configSyncPollInterval()
		if succeeded {
			retryBackoff = configSyncPollMinInterval
		} else {
			waitDuration = retryBackoff
			retryBackoff = min(retryBackoff*2, configSyncRetryMaxBackoff)
		}

		if err := w.wait(ctx, waitDuration); err != nil {
			return
		}
	}
}

func (w *configSyncWorker) poll(ctx context.Context) bool {
	revision, err := w.revisionStore.GetConfigRevision(ctx)
	if err != nil {
		w.logFailure("failed to read config revision", err)
		return false
	}

	if w.ready.Load() && revision == w.lastApplied.Load() {
		return true
	}

	if err := w.reconciler.Reconcile(ctx, revision); err != nil {
		w.logFailure(fmt.Sprintf("failed to reconcile config revision %d", revision), err)
		return false
	}

	w.lastApplied.Store(revision)
	w.ready.Store(true)
	return true
}

func (w *configSyncWorker) logFailure(message string, err error) {
	if logger != nil {
		logger.Warn("config sync worker: %s: %v", message, err)
	}
}

func (w *configSyncWorker) stop() {
	if w == nil {
		return
	}
	w.stopOnce.Do(w.cancel)
	<-w.done
}

func (w *configSyncWorker) isReady() bool {
	return w != nil && w.ready.Load()
}

func (w *configSyncWorker) appliedRevision() int64 {
	if w == nil {
		return 0
	}
	return w.lastApplied.Load()
}

func waitForConfigSync(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func configSyncPollInterval() time.Duration {
	span := configSyncPollMaxInterval - configSyncPollMinInterval
	return configSyncPollMinInterval + time.Duration(rand.Int64N(int64(span)+1))
}

func (s *BifrostHTTPServer) startConfigSyncWorker(ctx context.Context) error {
	if s.configSyncWorker != nil {
		return nil
	}

	if s.Config == nil || s.Config.ConfigStore == nil {
		return nil
	}

	syncMode, ok := s.Config.ConfigStore.(configstore.ConfigSyncMode)
	if !ok || !syncMode.IsConfigSyncEnabled() {
		return nil
	}

	revisionStore, ok := s.Config.ConfigStore.(configstore.ConfigRevisionStore)
	if !ok {
		return fmt.Errorf("config sync is enabled but the config store has no revision support")
	}

	s.ConfigSnapshotReconciler = NewConfigSnapshotReconciler(s)
	if s.ConfigSnapshotReconciler == nil {
		return fmt.Errorf("config sync is enabled but the snapshot reconciler is unavailable")
	}

	s.configSyncWorker = newConfigSyncWorker(
		ctx,
		revisionStore,
		s.ConfigSnapshotReconciler,
		nil,
	)
	return nil
}

func (s *BifrostHTTPServer) stopConfigSyncWorker() {
	if s.configSyncWorker == nil {
		return
	}
	s.configSyncWorker.stop()
	s.configSyncWorker = nil
}

func (s *BifrostHTTPServer) isReady() bool {
	if s.Config == nil || s.Config.ConfigStore == nil {
		return true
	}

	syncMode, ok := s.Config.ConfigStore.(configstore.ConfigSyncMode)
	if !ok || !syncMode.IsConfigSyncEnabled() {
		return true
	}

	return s.configSyncWorker.isReady()
}
