package handlers

import (
	"context"
	"testing"

	"github.com/maximhq/bifrost/framework/configstore"
	"github.com/valyala/fasthttp"
)

type configMutationTestStore struct {
	configstore.ConfigStore
	enabled  bool
	revision int64
}

func (s *configMutationTestStore) SetConfigSyncEnabled(enabled bool) {
	s.enabled = enabled
}

func (s *configMutationTestStore) IsConfigSyncEnabled() bool {
	return s.enabled
}

func (s *configMutationTestStore) GetConfigRevision(context.Context) (int64, error) {
	return s.revision, nil
}

func (s *configMutationTestStore) ExecuteConfigMutation(
	ctx context.Context,
	expectedRevision int64,
	mutate func(context.Context) error,
) (int64, error) {
	if expectedRevision != s.revision {
		return 0, &configstore.ConfigRevisionConflictError{
			Expected: expectedRevision,
			Actual:   s.revision,
		}
	}
	if err := mutate(ctx); err != nil {
		return 0, err
	}
	s.revision++
	return s.revision, nil
}

func TestPrepareConfigMutation_RequiresIfMatchWhenEnabled(t *testing.T) {
	store := &configMutationTestStore{enabled: true, revision: 4}
	ctx := &fasthttp.RequestCtx{}

	_, ok := prepareConfigMutation(ctx, store)
	if ok {
		t.Fatal("expected missing If-Match to reject the mutation")
	}
	if got := ctx.Response.StatusCode(); got != fasthttp.StatusPreconditionRequired {
		t.Fatalf("expected status 428, got %d", got)
	}
}

func TestCommitConfigMutation_RejectsStaleRevision(t *testing.T) {
	store := &configMutationTestStore{enabled: true, revision: 7}
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set(ifMatchHeader, "6")

	prepared, ok := prepareConfigMutation(ctx, store)
	if !ok {
		t.Fatal("expected If-Match to parse")
	}
	called := false
	_, err := commitConfigMutation(ctx, store, prepared, func(context.Context) error {
		called = true
		return nil
	})
	if err == nil {
		t.Fatal("expected stale revision conflict")
	}
	if called {
		t.Fatal("stale mutation callback must not execute")
	}
	if !handleConfigMutationError(ctx, err) {
		t.Fatal("expected conflict handler to handle revision error")
	}
	if got := ctx.Response.StatusCode(); got != fasthttp.StatusConflict {
		t.Fatalf("expected status 409, got %d", got)
	}
	if got := string(ctx.Response.Header.Peek(configRevisionHeader)); got != "7" {
		t.Fatalf("expected current revision header 7, got %q", got)
	}
}

func TestCommitConfigMutation_CommitsAndReturnsRevision(t *testing.T) {
	store := &configMutationTestStore{enabled: true, revision: 2}
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set(ifMatchHeader, `"2"`)

	prepared, ok := prepareConfigMutation(ctx, store)
	if !ok {
		t.Fatal("expected quoted If-Match to parse")
	}
	called := false
	revision, err := commitConfigMutation(ctx, store, prepared, func(context.Context) error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("commit config mutation: %v", err)
	}
	if !called {
		t.Fatal("expected mutation callback")
	}
	if revision != 3 {
		t.Fatalf("expected committed revision 3, got %d", revision)
	}
}

func TestSendConfigMutationPending_PreservesLegacyStatus(t *testing.T) {
	legacy := &fasthttp.RequestCtx{}
	sendConfigMutationPending(legacy, 0)
	if got := legacy.Response.StatusCode(); got != fasthttp.StatusInternalServerError {
		t.Fatalf("expected legacy status 500, got %d", got)
	}
	if len(legacy.Response.Header.Peek(configRevisionHeader)) != 0 {
		t.Fatal("legacy response must not emit revision zero")
	}

	synchronized := &fasthttp.RequestCtx{}
	sendConfigMutationPending(synchronized, 9)
	if got := synchronized.Response.StatusCode(); got != fasthttp.StatusAccepted {
		t.Fatalf("expected synchronized status 202, got %d", got)
	}
	if got := string(synchronized.Response.Header.Peek(configRevisionHeader)); got != "9" {
		t.Fatalf("expected revision header 9, got %q", got)
	}
}
