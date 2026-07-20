package server

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/maximhq/bifrost/framework/configstore"
	"github.com/maximhq/bifrost/transports/bifrost-http/lib"
)

type configSyncTestRevisionStore struct {
	mu        sync.Mutex
	revisions []int64
	errors    []error
	calls     int
}

func (s *configSyncTestRevisionStore) GetConfigRevision(context.Context) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	index := s.calls
	s.calls++
	if index < len(s.errors) && s.errors[index] != nil {
		return 0, s.errors[index]
	}
	if index < len(s.revisions) {
		return s.revisions[index], nil
	}
	if len(s.revisions) == 0 {
		return 0, nil
	}
	return s.revisions[len(s.revisions)-1], nil
}

func (s *configSyncTestRevisionStore) ExecuteConfigMutation(
	context.Context,
	int64,
	func(context.Context) error,
) (int64, error) {
	return 0, errors.New("not implemented")
}

type configSyncTestReconciler struct {
	mu        sync.Mutex
	calls     []int64
	failCalls int
}

func (r *configSyncTestReconciler) Reconcile(_ context.Context, revision int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.calls = append(r.calls, revision)
	if len(r.calls) <= r.failCalls {
		return errors.New("reconcile failed")
	}
	return nil
}

func (r *configSyncTestReconciler) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls)
}

type configSyncModeTestStore struct {
	configstore.ConfigStore
	enabled bool
}

func (s *configSyncModeTestStore) SetConfigSyncEnabled(enabled bool) {
	s.enabled = enabled
}

func (s *configSyncModeTestStore) IsConfigSyncEnabled() bool {
	return s.enabled
}

func TestBifrostHTTPServerReadinessFollowsInitialSync(t *testing.T) {
	store := &configSyncModeTestStore{}
	server := &BifrostHTTPServer{Config: &lib.Config{ConfigStore: store}}

	if !server.isReady() {
		t.Fatal("server should be ready when config sync is disabled")
	}

	store.SetConfigSyncEnabled(true)
	if server.isReady() {
		t.Fatal("server should not be ready before the config sync worker succeeds")
	}

	server.configSyncWorker = &configSyncWorker{}
	server.configSyncWorker.ready.Store(true)
	if !server.isReady() {
		t.Fatal("server should be ready after the config sync worker succeeds")
	}
}

func TestConfigSyncWorkerImmediateSuccessAndStop(t *testing.T) {
	store := &configSyncTestRevisionStore{revisions: []int64{12}}
	reconciler := &configSyncTestReconciler{}
	waitStarted := make(chan time.Duration, 1)
	wait := func(ctx context.Context, duration time.Duration) error {
		waitStarted <- duration
		<-ctx.Done()
		return ctx.Err()
	}

	worker := newConfigSyncWorker(context.Background(), store, reconciler, wait)
	duration := receiveConfigSyncWait(t, waitStarted)
	if duration < configSyncPollMinInterval || duration > configSyncPollMaxInterval {
		t.Fatalf("success wait = %v, want between %v and %v", duration, configSyncPollMinInterval, configSyncPollMaxInterval)
	}
	if !worker.isReady() {
		t.Fatal("worker should be ready after its first successful reconciliation")
	}
	if got := worker.appliedRevision(); got != 12 {
		t.Fatalf("applied revision = %d, want 12", got)
	}
	if got := reconciler.callCount(); got != 1 {
		t.Fatalf("reconcile calls = %d, want 1", got)
	}

	assertConfigSyncWorkerStops(t, worker)
}

func TestConfigSyncWorkerRetainsReadinessAfterFailure(t *testing.T) {
	store := &configSyncTestRevisionStore{
		revisions: []int64{7, 7, 7},
		errors:    []error{nil, nil, errors.New("revision read failed")},
	}
	reconciler := &configSyncTestReconciler{failCalls: 1}
	waitStarted := make(chan time.Duration, 3)
	waitCalls := 0
	wait := func(ctx context.Context, duration time.Duration) error {
		waitStarted <- duration
		waitCalls++
		if waitCalls < 3 {
			return nil
		}
		<-ctx.Done()
		return ctx.Err()
	}

	worker := newConfigSyncWorker(context.Background(), store, reconciler, wait)
	if duration := receiveConfigSyncWait(t, waitStarted); duration != configSyncPollMinInterval {
		t.Fatalf("first retry wait = %v, want %v", duration, configSyncPollMinInterval)
	}
	if duration := receiveConfigSyncWait(t, waitStarted); duration < configSyncPollMinInterval || duration > configSyncPollMaxInterval {
		t.Fatalf("success wait = %v, want between %v and %v", duration, configSyncPollMinInterval, configSyncPollMaxInterval)
	}
	if duration := receiveConfigSyncWait(t, waitStarted); duration != configSyncPollMinInterval {
		t.Fatalf("retry wait after readiness = %v, want %v", duration, configSyncPollMinInterval)
	}

	if !worker.isReady() {
		t.Fatal("worker readiness should remain true after a later polling failure")
	}
	if got := worker.appliedRevision(); got != 7 {
		t.Fatalf("applied revision = %d, want 7", got)
	}
	if got := reconciler.callCount(); got != 2 {
		t.Fatalf("reconcile calls = %d, want 2", got)
	}

	assertConfigSyncWorkerStops(t, worker)
}

func TestConfigSyncWorkerRetryBackoffCapsAtFiveSeconds(t *testing.T) {
	store := &configSyncTestRevisionStore{
		errors: []error{
			errors.New("failure 1"),
			errors.New("failure 2"),
			errors.New("failure 3"),
			errors.New("failure 4"),
			errors.New("failure 5"),
		},
	}
	waitStarted := make(chan time.Duration, 5)
	waitCalls := 0
	wait := func(ctx context.Context, duration time.Duration) error {
		waitStarted <- duration
		waitCalls++
		if waitCalls < 5 {
			return nil
		}
		<-ctx.Done()
		return ctx.Err()
	}

	worker := newConfigSyncWorker(context.Background(), store, &configSyncTestReconciler{}, wait)
	want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 5 * time.Second, 5 * time.Second}
	for index, expected := range want {
		if duration := receiveConfigSyncWait(t, waitStarted); duration != expected {
			t.Fatalf("retry wait %d = %v, want %v", index+1, duration, expected)
		}
	}
	if worker.isReady() {
		t.Fatal("worker should not be ready without a successful reconciliation")
	}

	assertConfigSyncWorkerStops(t, worker)
}

func receiveConfigSyncWait(t *testing.T, waits <-chan time.Duration) time.Duration {
	t.Helper()
	select {
	case duration := <-waits:
		return duration
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for config sync worker")
		return 0
	}
}

func assertConfigSyncWorkerStops(t *testing.T, worker *configSyncWorker) {
	t.Helper()
	stopped := make(chan struct{})
	go func() {
		worker.stop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("config sync worker did not stop after cancellation")
	}
}
