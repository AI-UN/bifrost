package opencode

import (
	"strings"

	"github.com/google/uuid"
	"github.com/maximhq/bifrost/core/schemas"
)

// OpencodeSessionHeader is the header the OpenCode Zen/Go gateways use to
// identify a conversation/session. Starting 2026-09-06 the Go gateway requires
// it and may reject requests that omit it. Bifrost resolves a session identity
// once per request (see ResolveOpencodeSession) and attaches it only on
// upstream calls to the OpenCode provider family.
const OpencodeSessionHeader = "x-opencode-session"

// isSafeOpencodeSessionValue reports whether a caller-supplied session value is
// safe to forward as an HTTP header value: it must contain no CR, LF, or any
// other control character. This is header-injection protection — a value that
// fails validation is treated as absent and the fallback chain applies instead
// of the raw value ever reaching the wire.
func isSafeOpencodeSessionValue(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] < 0x20 || value[i] == 0x7f {
			return false
		}
	}
	return true
}

// normalizeOpencodeSessionValue trims a caller-supplied session value, applies
// the shared session-identifier length bound, and rejects any value that cannot
// be a header value. Every session source goes through it, so the same literal
// normalizes the same way whichever layer supplied it, and a value that fails
// any check is treated as absent rather than retained or truncated.
func normalizeOpencodeSessionValue(value string) (string, bool) {
	value, ok := schemas.NormalizeSessionID(value)
	if !ok || !isSafeOpencodeSessionValue(value) {
		return "", false
	}
	return value, true
}

// ResolveOpencodeSession resolves the x-opencode-session value for a request
// and returns it, namespaced per granted identity. The result is resolved
// exactly once per request: the first call computes it and caches it on the
// context (BifrostContextKeyOpencodeSession), and every later call — retries,
// zen→go fallbacks, subsequent MCP agent-loop turns on the same request —
// returns the cached value, so the upstream sees one stable identity for the
// whole request.
//
// Resolution order, preserving the existing "client wins" and "poison-to-none"
// session identity rules from core/schemas:
//
//	a) the client-sent x-opencode-session request header, trimmed and bounded by
//	   schemas.NormalizeSessionID and charset-validated
//	   (isSafeOpencodeSessionValue). A value failing either check is treated as
//	   absent.
//	b) the caller's forwarded x-opencode-session (the x-bf-eh- escape hatch,
//	   or direct forwarding when an allowlist is configured), normalized the
//	   same way.
//	c) the request's session identity as already resolved by Bifrost
//	   (BifrostContextKeySessionID): x-bf-session-id wins, then the coding
//	   harness's own session headers (Claude Code, Codex CLI, OpenCode, ...).
//	d) if no signal at all: a fresh per-request UUID. This keeps the request
//	   compliant with the gateway requirement but provides no affinity across
//	   turns — multi-turn session tracking remains the client's responsibility.
//
// The resolved value is qualified with the ID of the virtual key (or user) the
// request's Grant identity is attributed to — never the credential itself — so
// sessions from different tenants cannot collide upstream.
func ResolveOpencodeSession(ctx *schemas.BifrostContext) string {
	if ctx == nil {
		return ""
	}
	return ctx.GetOrStoreOpencodeSession(func() string {
		return resolveOpencodeSessionValue(ctx)
	})
}

// resolveOpencodeSessionValue computes the x-opencode-session value for a
// request by walking the resolution chain (client header, forwarded header,
// Bifrost session id, then a synthesized per-request UUID), namespacing
// whichever signal wins per granted identity. It is called at most once per
// request by ResolveOpencodeSession and is read-only: it never writes to the
// context, so a concurrent supplier cannot change what another one observes.
func resolveOpencodeSessionValue(ctx *schemas.BifrostContext) string {
	// a) Client-sent header wins. The HTTP transport captures every request
	// header into BifrostContextKeyRequestHeaders (lowercased keys), so the raw
	// x-opencode-session is visible here even though ingress does not otherwise
	// special-case it. The value goes through NormalizeSessionID, the same trim
	// and MaxSessionIDLength bound every other session ingestion path uses.
	if headers, ok := ctx.Value(schemas.BifrostContextKeyRequestHeaders).(map[string]string); ok {
		if value, ok := normalizeOpencodeSessionValue(headers[OpencodeSessionHeader]); ok {
			return namespaceOpencodeSession(ctx, value)
		}
	}
	// b) Caller-forwarded header: the x-bf-eh-* escape hatch (or direct
	// allowlisted forwarding), already stripped and lower-cased by the
	// transport. Checked after the client's own header so the escape hatch
	// cannot override it.
	if value, ok := forwardedOpencodeSessionValue(ctx); ok {
		return namespaceOpencodeSession(ctx, value)
	}
	// c) BifrostContextKeySessionID already encodes the shared session identity
	// rules (x-bf-session-id wins, poison-to-none on an invalid explicit value,
	// then harness session headers), so reuse it rather than re-deriving. It is
	// normalized like the header sources: a value carrying control bytes or
	// exceeding the session bound falls through to a synthesized UUID.
	if sessionID, ok := ctx.Value(schemas.BifrostContextKeySessionID).(string); ok {
		if value, ok := normalizeOpencodeSessionValue(sessionID); ok {
			return namespaceOpencodeSession(ctx, value)
		}
	}
	// d) No signal at all: synthesize a fresh per-request UUID so the request
	// stays compliant. No affinity — the caller must send its own session
	// identity for multi-turn tracking.
	return namespaceOpencodeSession(ctx, uuid.New().String())
}

// forwardedOpencodeSessionValue returns the per-request x-opencode-session
// carried in BifrostContextKeyExtraHeaders. The transport lower-cases the
// stripped x-bf-eh-* names; a Go SDK caller may not, so a miss on the canonical
// key falls back to a case-insensitive scan. Values are read in order and the
// first usable one wins; an unusable entry is left for the next source in the
// resolution chain.
func forwardedOpencodeSessionValue(ctx *schemas.BifrostContext) (string, bool) {
	headers, ok := ctx.Value(schemas.BifrostContextKeyExtraHeaders).(map[string][]string)
	if !ok || len(headers) == 0 {
		return "", false
	}
	if value, ok := firstNormalizedOpencodeSession(headers[OpencodeSessionHeader]); ok {
		return value, true
	}
	for key, values := range headers {
		if !strings.EqualFold(key, OpencodeSessionHeader) {
			continue
		}
		if value, ok := firstNormalizedOpencodeSession(values); ok {
			return value, true
		}
	}
	return "", false
}

// firstNormalizedOpencodeSession returns the first value that normalizes as a
// session, so one malformed entry does not hide a usable one behind it.
func firstNormalizedOpencodeSession(values []string) (string, bool) {
	for _, value := range values {
		if normalized, ok := normalizeOpencodeSessionValue(value); ok {
			return normalized, true
		}
	}
	return "", false
}

// namespaceOpencodeSession qualifies a resolved session value with the
// identity the request's Grant is attributed to, so the same client session
// used under two different virtual keys (or users) cannot collide upstream. It
// never uses a presented credential: that is a Bifrost secret and must not
// reach a third-party gateway. Without a settled identity (or with an unsafe
// ID) the value is left unqualified rather than dropping the request.
//
// Each component is escaped before joining so the only literal ":" in the
// result is a separator we insert. Without escaping, ("a", "b:c") and
// ("a:b", "c") both produce "a:b:c" — a collision. Escaping doubles every
// backslash and prefixes every colon with a backslash, making the mapping
// injective: distinct (scope, value) pairs always yield distinct namespaced
// strings. The escaped output stays within isSafeOpencodeSessionValue's
// charset (only printable ASCII is introduced).
//
// An unqualified value is escaped too. Otherwise a caller with no grant could
// send "row:session" as its session and produce exactly the value a scoped
// request for that row and session produces, crossing tenant boundaries.
func namespaceOpencodeSession(ctx *schemas.BifrostContext, value string) string {
	value = escapeSessionComponent(value)
	if scope, ok := opencodeSessionScope(ctx); ok {
		return scope + ":" + value
	}
	return value
}

// opencodeSessionScope returns the escaped identity scope for a request, or
// ok=false when nothing identifies it. A virtual key row ID wins over a user ID
// because a request made with a key is attributed to that key first.
//
// shortcut: a key row ID and a user ID share one namespace, so two different
// tenants whose IDs are the same string would share a scope; upgrade if IDs can
// repeat across the two tables.
func opencodeSessionScope(ctx *schemas.BifrostContext) (string, bool) {
	grant := ctx.Grant()
	if grant == nil {
		return "", false
	}
	identity := grant.Identity()
	if identity == nil {
		return "", false
	}
	if key := identity.VirtualKey(); key != nil {
		if id, ok := safeOpencodeSessionScopeID(key.ID); ok {
			return escapeSessionComponent(id), true
		}
	}
	if user := identity.User(); user != nil {
		if id, ok := safeOpencodeSessionScopeID(user.ID); ok {
			return escapeSessionComponent(id), true
		}
	}
	return "", false
}

// safeOpencodeSessionScopeID validates an identity ID before it is joined into
// the header value. IDs come from resolved rows rather than callers, but the
// same charset rule applies so an unexpectedly shaped ID cannot inject a header
// line or make the namespace ambiguous.
func safeOpencodeSessionScopeID(id string) (string, bool) {
	id = strings.TrimSpace(id)
	if id == "" || !isSafeOpencodeSessionValue(id) {
		return "", false
	}
	return id, true
}

// escapeSessionComponent makes a single session component unambiguous when
// joined with ":" as a separator. Backslashes are doubled first, then colons
// are prefixed with a backslash, so the only literal ":" in the final
// namespaced string is a separator inserted by namespaceOpencodeSession.
func escapeSessionComponent(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, ":", "\\:")
	return s
}
