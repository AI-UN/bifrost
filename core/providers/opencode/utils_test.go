package opencode

import (
	"context"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/maximhq/bifrost/core/schemas"
)

// newTestOpencodeCtx builds a BifrostContext carrying the request headers the
// HTTP transport would have captured.
func newTestOpencodeCtx(t *testing.T, requestHeaders map[string]string) *schemas.BifrostContext {
	t.Helper()
	ctx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
	if len(requestHeaders) > 0 {
		ctx.SetValue(schemas.BifrostContextKeyRequestHeaders, requestHeaders)
	}
	return ctx
}

// opencodeTestIdentity is the least a settled identity needs for session
// scoping: the resolved virtual-key row and user IDs, plus a presented
// credential a test uses to prove the credential value never becomes a
// namespace. The remaining Identity methods are promoted from the embedded
// interface; these tests never call them.
type opencodeTestIdentity struct {
	schemas.Identity
	virtualKeyID    string
	userID          string
	credentialKind  string
	credentialValue string
}

func (i *opencodeTestIdentity) Credential() schemas.Credential {
	return schemas.Credential{Kind: i.credentialKind, Value: i.credentialValue}
}

func (i *opencodeTestIdentity) VirtualKey() *schemas.EntityRef {
	if i.virtualKeyID == "" {
		return nil
	}
	return &schemas.EntityRef{ID: i.virtualKeyID}
}

func (i *opencodeTestIdentity) User() *schemas.UserRef {
	if i.userID == "" {
		return nil
	}
	return &schemas.UserRef{ID: i.userID}
}

// opencodeTestGrant carries only an identity; the other Grant sections are
// promoted from the embedded interface and are never read here.
type opencodeTestGrant struct {
	schemas.Grant
	identity schemas.Identity
}

func (g *opencodeTestGrant) Identity() schemas.Identity { return g.identity }

// opencodeCtxWithIdentity installs a grant attributing the request to the given
// virtual-key row and/or user.
func opencodeCtxWithIdentity(t testing.TB, ctx *schemas.BifrostContext, virtualKeyID, userID string) *schemas.BifrostContext {
	t.Helper()
	ctx.SetGrant(&opencodeTestGrant{identity: &opencodeTestIdentity{virtualKeyID: virtualKeyID, userID: userID}})
	return ctx
}

var uuidValueRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// TestIsSafeOpencodeSessionValue verifies header-injection protection: CR, LF,
// and other control bytes are rejected while ordinary printable and unicode
// values are accepted.
func TestIsSafeOpencodeSessionValue(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "plain value", value: "session-abc-123", want: true},
		{name: "value with spaces", value: "a b c", want: true},
		{name: "value with colon", value: "vk-1:session-1", want: true},
		{name: "value with unicode", value: "séance-☃", want: true},
		{name: "carriage return rejected", value: "abc\r\nInjected: x", want: false},
		{name: "line feed rejected", value: "abc\nInjected", want: false},
		{name: "control byte rejected", value: "abc\x01def", want: false},
		{name: "tab rejected", value: "abc\tdef", want: false},
		{name: "DEL rejected", value: "abc\x7fdef", want: false},
		{name: "NUL rejected", value: "abc\x00def", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSafeOpencodeSessionValue(tt.value); got != tt.want {
				t.Errorf("isSafeOpencodeSessionValue(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

// TestNormalizeOpencodeSessionValue verifies every source shares one rule:
// surrounding whitespace is trimmed, the session length bound applies, and a
// value carrying control bytes is unusable.
func TestNormalizeOpencodeSessionValue(t *testing.T) {
	t.Run("trims surrounding whitespace", func(t *testing.T) {
		got, ok := normalizeOpencodeSessionValue("  session-a\n")
		if !ok || got != "session-a" {
			t.Errorf("normalizeOpencodeSessionValue() = %q, %v, want %q, true", got, ok, "session-a")
		}
	})

	t.Run("white-space only is absent", func(t *testing.T) {
		if got, ok := normalizeOpencodeSessionValue("   "); ok || got != "" {
			t.Errorf("normalizeOpencodeSessionValue() = %q, %v, want empty, false", got, ok)
		}
	})

	t.Run("keeps a value at the length bound", func(t *testing.T) {
		value := strings.Repeat("a", schemas.MaxSessionIDLength)
		if got, ok := normalizeOpencodeSessionValue(value); !ok || got != value {
			t.Errorf("normalizeOpencodeSessionValue() rejected a value at the length bound")
		}
	})

	t.Run("rejects a value past the length bound", func(t *testing.T) {
		if got, ok := normalizeOpencodeSessionValue(strings.Repeat("a", schemas.MaxSessionIDLength+1)); ok || got != "" {
			t.Errorf("normalizeOpencodeSessionValue() = %q, %v, want empty, false", got, ok)
		}
	})

	t.Run("rejects control bytes", func(t *testing.T) {
		if got, ok := normalizeOpencodeSessionValue("abc\x01def"); ok || got != "" {
			t.Errorf("normalizeOpencodeSessionValue() = %q, %v, want empty, false", got, ok)
		}
	})
}

// TestResolveOpencodeSessionClientHeaderWins verifies the client-sent
// x-opencode-session header is forwarded verbatim (namespaced per granted
// identity) and takes precedence over every later source.
func TestResolveOpencodeSessionClientHeaderWins(t *testing.T) {
	t.Run("forwarded verbatim and namespaced per granted virtual key", func(t *testing.T) {
		ctx := newTestOpencodeCtx(t, map[string]string{
			OpencodeSessionHeader: "client-sess-abc",
		})
		opencodeCtxWithIdentity(t, ctx, "vk-1", "")

		if got := ResolveOpencodeSession(ctx); got != "vk:vk-1:client-sess-abc" {
			t.Errorf("ResolveOpencodeSession() = %q, want %q", got, "vk:vk-1:client-sess-abc")
		}
	})

	t.Run("forwarded verbatim without a grant", func(t *testing.T) {
		ctx := newTestOpencodeCtx(t, map[string]string{
			OpencodeSessionHeader: "client-sess-abc",
		})
		if got := ResolveOpencodeSession(ctx); got != "client-sess-abc" {
			t.Errorf("ResolveOpencodeSession() = %q, want %q", got, "client-sess-abc")
		}
	})

	t.Run("client header beats x-bf-session-id", func(t *testing.T) {
		ctx := newTestOpencodeCtx(t, map[string]string{
			OpencodeSessionHeader: "client-sess-abc",
		})
		ctx.SetValue(schemas.BifrostContextKeySessionID, "bf-sess-123")
		if got := ResolveOpencodeSession(ctx); got != "client-sess-abc" {
			t.Errorf("ResolveOpencodeSession() = %q, want client header to win over %q", got, "bf-sess-123")
		}
	})

	t.Run("client header beats a forwarded extra header", func(t *testing.T) {
		ctx := newTestOpencodeCtx(t, map[string]string{
			OpencodeSessionHeader: "client-sess-abc",
		})
		ctx.SetValue(schemas.BifrostContextKeyExtraHeaders, map[string][]string{
			OpencodeSessionHeader: {"escape-sess"},
		})
		if got := ResolveOpencodeSession(ctx); got != "client-sess-abc" {
			t.Errorf("ResolveOpencodeSession() = %q, want the client header to win over the escape hatch", got)
		}
	})
}

// TestResolveOpencodeSessionForwardedExtraHeader verifies the per-request
// x-bf-eh-* escape hatch is the second source: it is used when the client sent
// no header of its own, normalized like the client header, and never confused
// with the still-prefixed raw request header.
func TestResolveOpencodeSessionForwardedExtraHeader(t *testing.T) {
	t.Run("forwarded extra header is used and namespaced", func(t *testing.T) {
		ctx := newTestOpencodeCtx(t, nil)
		ctx.SetValue(schemas.BifrostContextKeyExtraHeaders, map[string][]string{
			OpencodeSessionHeader: {"escape-sess"},
		})
		ctx.SetValue(schemas.BifrostContextKeySessionID, "bf-sess-123")
		opencodeCtxWithIdentity(t, ctx, "vk-1", "")
		if got := ResolveOpencodeSession(ctx); got != "vk:vk-1:escape-sess" {
			t.Errorf("ResolveOpencodeSession() = %q, want %q", got, "vk:vk-1:escape-sess")
		}
	})

	t.Run("case-variant key is accepted", func(t *testing.T) {
		ctx := newTestOpencodeCtx(t, nil)
		ctx.SetValue(schemas.BifrostContextKeyExtraHeaders, map[string][]string{
			"X-Opencode-Session": {"escape-sess"},
		})
		if got := ResolveOpencodeSession(ctx); got != "escape-sess" {
			t.Errorf("ResolveOpencodeSession() = %q, want %q", got, "escape-sess")
		}
	})

	t.Run("first usable value wins", func(t *testing.T) {
		ctx := newTestOpencodeCtx(t, nil)
		ctx.SetValue(schemas.BifrostContextKeyExtraHeaders, map[string][]string{
			OpencodeSessionHeader: {"bad\r\nvalue", "escape-sess"},
		})
		if got := ResolveOpencodeSession(ctx); got != "escape-sess" {
			t.Errorf("ResolveOpencodeSession() = %q, want %q", got, "escape-sess")
		}
	})

	t.Run("unusable forwarded value falls through to the session id", func(t *testing.T) {
		ctx := newTestOpencodeCtx(t, nil)
		ctx.SetValue(schemas.BifrostContextKeyExtraHeaders, map[string][]string{
			OpencodeSessionHeader: {"bad\x01value"},
		})
		ctx.SetValue(schemas.BifrostContextKeySessionID, "bf-sess-123")
		if got := ResolveOpencodeSession(ctx); got != "bf-sess-123" {
			t.Errorf("ResolveOpencodeSession() = %q, want fallback %q", got, "bf-sess-123")
		}
	})

	t.Run("still-prefixed raw request header is not the escape hatch", func(t *testing.T) {
		// The transport puts the stripped name in the extra-header map and keeps
		// the prefixed name in the raw request-header map. Only the former is a
		// forwarded session; the prefixed one alone must not become one.
		ctx := newTestOpencodeCtx(t, map[string]string{
			"x-bf-eh-" + OpencodeSessionHeader: "escape-sess",
		})
		ctx.SetValue(schemas.BifrostContextKeySessionID, "bf-sess-123")
		if got := ResolveOpencodeSession(ctx); got != "bf-sess-123" {
			t.Errorf("ResolveOpencodeSession() = %q, want the session id %q", got, "bf-sess-123")
		}
	})
}

// TestResolveOpencodeSessionFallbackChain verifies the resolution order when the
// client header is absent, empty, poisoned, or over the length bound: the
// forwarded header is tried next, then the Bifrost session id, otherwise a UUID
// is synthesized.
func TestResolveOpencodeSessionFallbackChain(t *testing.T) {
	t.Run("session id used when client header absent", func(t *testing.T) {
		ctx := newTestOpencodeCtx(t, nil)
		ctx.SetValue(schemas.BifrostContextKeySessionID, "bf-sess-123")
		opencodeCtxWithIdentity(t, ctx, "vk-1", "")
		if got := ResolveOpencodeSession(ctx); got != "vk:vk-1:bf-sess-123" {
			t.Errorf("ResolveOpencodeSession() = %q, want %q", got, "vk:vk-1:bf-sess-123")
		}
	})

	t.Run("session id used when client header is empty", func(t *testing.T) {
		ctx := newTestOpencodeCtx(t, map[string]string{OpencodeSessionHeader: ""})
		ctx.SetValue(schemas.BifrostContextKeySessionID, "bf-sess-123")
		if got := ResolveOpencodeSession(ctx); got != "bf-sess-123" {
			t.Errorf("ResolveOpencodeSession() = %q, want %q", got, "bf-sess-123")
		}
	})

	t.Run("harness session id used when neither client header nor x-bf-session-id present", func(t *testing.T) {
		// The harness-vs-x-bf-session-id distinction is resolved by the ingress
		// (ResolveSessionIDFromHeaders); the egress only sees the outcome under
		// BifrostContextKeySessionID, so a harness-derived value flows through
		// the same path.
		ctx := newTestOpencodeCtx(t, nil)
		ctx.SetValue(schemas.BifrostContextKeySessionID, "harness-sess-42")
		if got := ResolveOpencodeSession(ctx); got != "harness-sess-42" {
			t.Errorf("ResolveOpencodeSession() = %q, want %q", got, "harness-sess-42")
		}
	})

	t.Run("poisoned client header falls back to session id", func(t *testing.T) {
		ctx := newTestOpencodeCtx(t, map[string]string{
			OpencodeSessionHeader: "evil\r\nInjected: x",
		})
		ctx.SetValue(schemas.BifrostContextKeySessionID, "bf-sess-123")
		if got := ResolveOpencodeSession(ctx); got != "bf-sess-123" {
			t.Errorf("ResolveOpencodeSession() = %q, want fallback %q", got, "bf-sess-123")
		}
	})

	t.Run("overlong client header falls back to session id", func(t *testing.T) {
		ctx := newTestOpencodeCtx(t, map[string]string{
			OpencodeSessionHeader: strings.Repeat("a", schemas.MaxSessionIDLength+1),
		})
		ctx.SetValue(schemas.BifrostContextKeySessionID, "bf-sess-123")
		if got := ResolveOpencodeSession(ctx); got != "bf-sess-123" {
			t.Errorf("ResolveOpencodeSession() = %q, want fallback %q", got, "bf-sess-123")
		}
	})

	t.Run("poisoned client header falls back to synthesized UUID", func(t *testing.T) {
		ctx := newTestOpencodeCtx(t, map[string]string{
			OpencodeSessionHeader: "abc\x01def",
		})
		got := ResolveOpencodeSession(ctx)
		if !uuidValueRe.MatchString(got) {
			t.Errorf("ResolveOpencodeSession() = %q, want synthesized UUID", got)
		}
	})

	t.Run("whitespace-only client header is treated as absent", func(t *testing.T) {
		ctx := newTestOpencodeCtx(t, map[string]string{
			OpencodeSessionHeader: "   ",
		})
		ctx.SetValue(schemas.BifrostContextKeySessionID, "bf-sess-123")
		if got := ResolveOpencodeSession(ctx); got != "bf-sess-123" {
			t.Errorf("ResolveOpencodeSession() = %q, want fallback %q", got, "bf-sess-123")
		}
	})

	t.Run("session id with control bytes falls back to synthesized UUID", func(t *testing.T) {
		ctx := newTestOpencodeCtx(t, nil)
		ctx.SetValue(schemas.BifrostContextKeySessionID, "abc\x01def")
		got := ResolveOpencodeSession(ctx)
		if !uuidValueRe.MatchString(got) {
			t.Errorf("ResolveOpencodeSession() = %q, want synthesized UUID for poisoned session id", got)
		}
	})

	t.Run("env-looking literal is forwarded literally", func(t *testing.T) {
		t.Setenv("OPENCODE_TEST_SESSION", "from-env")
		ctx := newTestOpencodeCtx(t, map[string]string{
			OpencodeSessionHeader: "env.OPENCODE_TEST_SESSION",
		})
		if got := ResolveOpencodeSession(ctx); got != "env.OPENCODE_TEST_SESSION" {
			t.Errorf("ResolveOpencodeSession() = %q, want the literal session value", got)
		}
	})
}

// TestResolveOpencodeSessionSynthesizesUUID verifies that with no session signal
// at all a fresh, per-request UUID is produced and still survives namespacing.
func TestResolveOpencodeSessionSynthesizesUUID(t *testing.T) {
	t.Run("valid UUID when no signal at all", func(t *testing.T) {
		ctx := newTestOpencodeCtx(t, nil)
		got := ResolveOpencodeSession(ctx)
		if !uuidValueRe.MatchString(got) {
			t.Errorf("ResolveOpencodeSession() = %q, want a UUID", got)
		}
	})

	t.Run("UUIDs differ across requests", func(t *testing.T) {
		first := ResolveOpencodeSession(newTestOpencodeCtx(t, nil))
		second := ResolveOpencodeSession(newTestOpencodeCtx(t, nil))
		if first == second {
			t.Errorf("ResolveOpencodeSession() returned the same synthesized UUID for two requests: %q", first)
		}
	})

	t.Run("namespaced UUID is still a UUID after the prefix", func(t *testing.T) {
		ctx := newTestOpencodeCtx(t, nil)
		opencodeCtxWithIdentity(t, ctx, "vk-1", "")
		got := ResolveOpencodeSession(ctx)
		prefix := "vk:vk-1:"
		if len(got) <= len(prefix) || got[:len(prefix)] != prefix {
			t.Fatalf("ResolveOpencodeSession() = %q, want %q prefix", got, prefix)
		}
		if !uuidValueRe.MatchString(got[len(prefix):]) {
			t.Errorf("ResolveOpencodeSession() = %q, want UUID after namespace prefix", got)
		}
	})
}

// TestResolveOpencodeSessionNamespacesPerIdentity verifies sessions are scoped
// with the granted identity — the same session under two virtual keys or users
// yields distinct values, the same identity stays stable across requests and
// credential rotation, and vk/user tags cannot collide.
func TestResolveOpencodeSessionNamespacesPerIdentity(t *testing.T) {
	build := func(virtualKeyID, userID string) *schemas.BifrostContext {
		ctx := newTestOpencodeCtx(t, map[string]string{OpencodeSessionHeader: "shared-session"})
		if virtualKeyID != "" || userID != "" {
			opencodeCtxWithIdentity(t, ctx, virtualKeyID, userID)
		}
		return ctx
	}

	vk1 := ResolveOpencodeSession(build("vk-1", ""))
	vk2 := ResolveOpencodeSession(build("vk-2", ""))
	if vk1 == vk2 {
		t.Errorf("ResolveOpencodeSession() produced the same value for two virtual keys: %q", vk1)
	}
	if vk1 != "vk:vk-1:shared-session" || vk2 != "vk:vk-2:shared-session" {
		t.Errorf("ResolveOpencodeSession() = %q and %q, want vk-scoped values", vk1, vk2)
	}
	if again := ResolveOpencodeSession(build("vk-1", "")); again != vk1 {
		t.Errorf("ResolveOpencodeSession() not stable for the same virtual key: %q vs %q", again, vk1)
	}

	t.Run("user identity is scoped with the user tag", func(t *testing.T) {
		if got := ResolveOpencodeSession(build("", "user-1")); got != "user:user-1:shared-session" {
			t.Errorf("ResolveOpencodeSession() = %q, want %q", got, "user:user-1:shared-session")
		}
	})

	t.Run("virtual key wins when both are settled", func(t *testing.T) {
		if got := ResolveOpencodeSession(build("vk-1", "user-1")); got != "vk:vk-1:shared-session" {
			t.Errorf("ResolveOpencodeSession() = %q, want the virtual-key scope", got)
		}
	})

	t.Run("the same ID under both tags does not collide", func(t *testing.T) {
		asKey := ResolveOpencodeSession(build("same-id", ""))
		asUser := ResolveOpencodeSession(build("", "same-id"))
		if asKey == asUser {
			t.Fatalf("virtual-key and user scopes collided: %q", asKey)
		}
		if asKey != "vk:same-id:shared-session" || asUser != "user:same-id:shared-session" {
			t.Errorf("scoped values = %q and %q, want tagged values", asKey, asUser)
		}
	})

	t.Run("credential rotation keeps the session stable", func(t *testing.T) {
		// The virtual key row is the same across a secret rotation, so the two
		// requests must produce one upstream session even though the presented
		// value changed.
		before := newTestOpencodeCtx(t, map[string]string{OpencodeSessionHeader: "shared-session"})
		before.SetValue(schemas.BifrostContextKeyVirtualKey, "sk-before-rotation")
		opencodeCtxWithIdentity(t, before, "vk-row-1", "")

		after := newTestOpencodeCtx(t, map[string]string{OpencodeSessionHeader: "shared-session"})
		after.SetValue(schemas.BifrostContextKeyVirtualKey, "sk-after-rotation")
		opencodeCtxWithIdentity(t, after, "vk-row-1", "")

		first := ResolveOpencodeSession(before)
		second := ResolveOpencodeSession(after)
		if first != second {
			t.Errorf("credential rotation changed the session: %q vs %q", first, second)
		}
		if first != "vk:vk-row-1:shared-session" {
			t.Errorf("ResolveOpencodeSession() = %q, want %q", first, "vk:vk-row-1:shared-session")
		}
	})

	t.Run("unsafe identity id leaves value unqualified", func(t *testing.T) {
		ctx := newTestOpencodeCtx(t, map[string]string{OpencodeSessionHeader: "shared-session"})
		opencodeCtxWithIdentity(t, ctx, "evil\r\nvk", "")
		if got := ResolveOpencodeSession(ctx); got != "shared-session" {
			t.Errorf("ResolveOpencodeSession() = %q, want unqualified value with unsafe identity id", got)
		}
	})

	t.Run("unsettled identity leaves value unqualified", func(t *testing.T) {
		ctx := newTestOpencodeCtx(t, map[string]string{OpencodeSessionHeader: "shared-session"})
		ctx.SetGrant(&opencodeTestGrant{})
		if got := ResolveOpencodeSession(ctx); got != "shared-session" {
			t.Errorf("ResolveOpencodeSession() = %q, want unqualified value with no identity", got)
		}
	})
}

// TestResolveOpencodeSessionResolvesOncePerRequest verifies the resolved value is
// cached on the context so repeated calls for one request return the same
// namespaced value instead of re-synthesizing.
func TestResolveOpencodeSessionResolvesOncePerRequest(t *testing.T) {
	ctx := newTestOpencodeCtx(t, nil)
	first := ResolveOpencodeSession(ctx)
	second := ResolveOpencodeSession(ctx)
	if first != second {
		t.Errorf("ResolveOpencodeSession() resolved twice: %q then %q, want a single cached value", first, second)
	}
	// The cached value must be the namespaced, final form.
	if !uuidValueRe.MatchString(first) {
		t.Errorf("ResolveOpencodeSession() = %q, want a UUID", first)
	}
}

// TestNamespaceOpencodeSessionNoCollision is a regression test for naive
// "scope:value" concatenation: component escaping keeps the mapping injective so
// ("a","b:c") and ("a:b","c") map to distinct values, and the vk/user tags keep
// the same identifier under different scopes distinct.
func TestNamespaceOpencodeSessionNoCollision(t *testing.T) {
	mk := func(virtualKeyID, userID string) *schemas.BifrostContext {
		ctx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
		return opencodeCtxWithIdentity(t, ctx, virtualKeyID, userID)
	}

	got1 := namespaceOpencodeSession(mk("a", ""), "b:c")
	got2 := namespaceOpencodeSession(mk("a:b", ""), "c")

	if got1 != "vk:a:b\\:c" {
		t.Errorf("(\"a\",\"b:c\") = %q, want %q", got1, "vk:a:b\\:c")
	}
	if got2 != "vk:a\\:b:c" {
		t.Errorf("(\"a:b\",\"c\") = %q, want %q", got2, "vk:a\\:b:c")
	}
	if got1 == got2 {
		t.Fatalf("collision: (\"a\",\"b:c\") and (\"a:b\",\"c\") both produced %q", got1)
	}

	asKey := namespaceOpencodeSession(mk("shared", ""), "session-1")
	asUser := namespaceOpencodeSession(mk("", "shared"), "session-1")
	if asKey == asUser {
		t.Fatalf("tag collision: vk and user scopes both produced %q", asKey)
	}
}

// TestResolveOpencodeSessionUnscopedCannotImpersonateScope pins that a caller
// without a grant cannot pick a session value that equals a scoped tenant's
// header. The unqualified value is escaped, so "vk:row:session" is sent as
// "vk\:row\:session" and differs from the scoped "vk:row:session".
func TestResolveOpencodeSessionUnscopedCannotImpersonateScope(t *testing.T) {
	t.Run("virtual key scope", func(t *testing.T) {
		unscopedCtx := newTestOpencodeCtx(t, map[string]string{
			OpencodeSessionHeader: "vk:vk-1:shared-session",
		})
		scopedCtx := newTestOpencodeCtx(t, map[string]string{
			OpencodeSessionHeader: "shared-session",
		})
		opencodeCtxWithIdentity(t, scopedCtx, "vk-1", "")

		unscoped := ResolveOpencodeSession(unscopedCtx)
		scoped := ResolveOpencodeSession(scopedCtx)
		if unscoped == scoped {
			t.Fatalf("unscoped session impersonated a scoped tenant: both %q", unscoped)
		}
		if unscoped != `vk\:vk-1\:shared-session` {
			t.Errorf("unscoped session = %q, want the escaped literal", unscoped)
		}
		if scoped != "vk:vk-1:shared-session" {
			t.Errorf("scoped session = %q, want %q", scoped, "vk:vk-1:shared-session")
		}
	})

	t.Run("user scope", func(t *testing.T) {
		unscopedCtx := newTestOpencodeCtx(t, map[string]string{
			OpencodeSessionHeader: "user:user-1:shared-session",
		})
		scopedCtx := newTestOpencodeCtx(t, map[string]string{
			OpencodeSessionHeader: "shared-session",
		})
		opencodeCtxWithIdentity(t, scopedCtx, "", "user-1")

		if unscoped, scoped := ResolveOpencodeSession(unscopedCtx), ResolveOpencodeSession(scopedCtx); unscoped == scoped {
			t.Fatalf("unscoped session impersonated a scoped user: both %q", unscoped)
		}
	})
}

// TestResolveOpencodeSessionNilContext verifies a nil context yields an empty
// session value instead of panicking.
func TestResolveOpencodeSessionNilContext(t *testing.T) {
	if got := ResolveOpencodeSession(nil); got != "" {
		t.Errorf("ResolveOpencodeSession(nil) = %q, want \"\"", got)
	}
}

// TestResolveOpencodeSessionStableUnderBlockRestrictedWrites verifies the cached
// session value survives BlockRestrictedWrites and is returned unchanged on the
// next resolution.
func TestResolveOpencodeSessionStableUnderBlockRestrictedWrites(t *testing.T) {
	ctx := newTestOpencodeCtx(t, nil)
	opencodeCtxWithIdentity(t, ctx, "vk-1", "")

	first := ResolveOpencodeSession(ctx)
	prefix := "vk:vk-1:"
	if !strings.HasPrefix(first, prefix) || !uuidValueRe.MatchString(strings.TrimPrefix(first, prefix)) {
		t.Fatalf("first = %q, want namespaced UUID", first)
	}

	ctx.BlockRestrictedWrites()
	second := ResolveOpencodeSession(ctx)
	if second != first {
		t.Errorf("under blockRestrictedWrites: second = %q, want %q (cached)", second, first)
	}
}

// TestResolveOpencodeSessionConcurrentSameContext verifies every concurrent
// resolver of one context returns the value the context stores, even though
// suppliers may run concurrently.
func TestResolveOpencodeSessionConcurrentSameContext(t *testing.T) {
	ctx := newTestOpencodeCtx(t, nil)
	opencodeCtxWithIdentity(t, ctx, "vk-1", "")

	const goroutines = 100
	results := make(chan string, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			results <- ResolveOpencodeSession(ctx)
		}()
	}
	wg.Wait()
	close(results)

	first := <-results
	for value := range results {
		if value != first {
			t.Fatalf("concurrent resolution was not stable: got %q and %q", first, value)
		}
	}
	prefix := "vk:vk-1:"
	if !strings.HasPrefix(first, prefix) || !uuidValueRe.MatchString(strings.TrimPrefix(first, prefix)) {
		t.Fatalf("first = %q, want namespaced UUID", first)
	}
}

// TestResolveOpencodeSessionDoesNotExposePresentedCredential pins that the
// credential a request presented is never used as a session namespace, whether
// it is still the raw context value or is carried on a settled identity. The
// resolved virtual-key row ID is the only key value that may scope a session.
func TestResolveOpencodeSessionDoesNotExposePresentedCredential(t *testing.T) {
	const secret = "sk-bf-private-gateway-credential"

	t.Run("raw context credential is not used", func(t *testing.T) {
		ctx := newTestOpencodeCtx(t, map[string]string{OpencodeSessionHeader: "conversation-a"})
		ctx.SetValue(schemas.BifrostContextKeyVirtualKey, secret)
		got := ResolveOpencodeSession(ctx)
		if got != "conversation-a" {
			t.Fatalf("unresolved credential must not become an upstream namespace: %q", got)
		}
		if strings.Contains(got, secret) {
			t.Fatalf("resolved session leaked the presented credential: %q", got)
		}
	})

	t.Run("settled identity credential value is not used", func(t *testing.T) {
		ctx := newTestOpencodeCtx(t, map[string]string{OpencodeSessionHeader: "conversation-a"})
		ctx.SetGrant(&opencodeTestGrant{identity: &opencodeTestIdentity{
			credentialKind:  "virtual_key",
			credentialValue: secret,
		}})
		got := ResolveOpencodeSession(ctx)
		if got != "conversation-a" {
			t.Fatalf("credential value must not become an upstream namespace: %q", got)
		}
	})

	t.Run("resolved key row id scopes without the credential", func(t *testing.T) {
		ctx := newTestOpencodeCtx(t, map[string]string{OpencodeSessionHeader: "conversation-a"})
		ctx.SetValue(schemas.BifrostContextKeyVirtualKey, secret)
		ctx.SetGrant(&opencodeTestGrant{identity: &opencodeTestIdentity{
			virtualKeyID:    "vk-row-1",
			credentialKind:  "virtual_key",
			credentialValue: secret,
		}})
		got := ResolveOpencodeSession(ctx)
		if got != "vk:vk-row-1:conversation-a" {
			t.Fatalf("ResolveOpencodeSession() = %q, want the key row scope", got)
		}
		if strings.Contains(got, secret) {
			t.Fatalf("resolved session leaked the presented credential: %q", got)
		}
	})
}

var benchmarkOpencodeSession string

func BenchmarkResolveOpencodeSession(b *testing.B) {
	for _, source := range []string{"client", "session", "generated", "cached"} {
		b.Run(source, func(b *testing.B) {
			newContext := func() *schemas.BifrostContext {
				ctx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
				switch source {
				case "client", "cached":
					ctx.SetValue(schemas.BifrostContextKeyRequestHeaders, map[string]string{OpencodeSessionHeader: "conversation-a"})
				case "session":
					ctx.SetValue(schemas.BifrostContextKeySessionID, "conversation-a")
				}
				return ctx
			}
			ctx := newContext()
			ResolveOpencodeSession(ctx)
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if source != "cached" {
					ctx = newContext()
				}
				benchmarkOpencodeSession = ResolveOpencodeSession(ctx)
			}
		})
	}
}
