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

// The tags below qualify a session with the identity the request was granted.
// A virtual key row ID and a user ID are different namespaces, so tagging
// keeps an identifier that is both from colliding across the two.
const (
	opencodeSessionScopeVirtualKey = "vk"
	opencodeSessionScopeUser       = "user"
)

// isSafeOpencodeSessionValue reports whether a caller-supplied session value is
// safe to forward as an HTTP header value: it must contain no CR, LF, or any
// other control character. This is header-injection protection — a value that
// fails validation is treated as absent and the fallback chain applies instead
// of the raw value ever reaching the wire.
func isSafeOpencodeSessionValue(value string) bool {
	for i := range len(value) {
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
// and returns it, namespaced per granted identity. The result is resolved once
// per request: the first call computes it and caches it on the context
// (BifrostContextKeyOpencodeSession), and every later call — retries,
// zen→go fallbacks, subsequent MCP agent-loop turns on the same request —
// returns the cached value, so the upstream sees one stable identity for the
// whole request. Concurrent first calls may each compute a candidate, but all
// of them return the one value the context stores.
//
// Resolution order, preserving the existing "client wins" and "poison-to-none"
// session identity rules from core/schemas:
//
//	a) the client-sent x-opencode-session request header, after normalization
//	   (length bound, trim) and charset validation
//	   (isSafeOpencodeSessionValue). A value failing either is treated as
//	   absent.
//	b) the per-request forwarded x-opencode-session carried in
//	   BifrostContextKeyExtraHeaders — the x-bf-eh-x-opencode-session escape
//	   hatch, and direct forwarding when an allowlist is configured — after the
//	   same normalization.
//	c) the request's session identity as already resolved by Bifrost
//	   (BifrostContextKeySessionID): x-bf-session-id wins, then the coding
//	   harness's own session headers (Claude Code, Codex CLI, OpenCode, ...).
//	d) if no signal at all: a fresh per-request UUID. This keeps the request
//	   compliant with the gateway requirement but provides no affinity across
//	   turns — multi-turn session tracking remains the client's responsibility.
//
// The resolved value is qualified with the request's granted virtual key row
// ID, or the granted user ID when no key is settled, so sessions from different
// tenants cannot collide upstream. Only those resolved identifiers are used:
// the presented credential, its kind, and its value never enter the session.
func ResolveOpencodeSession(ctx *schemas.BifrostContext) string {
	if ctx == nil {
		return ""
	}
	return ctx.GetOrStoreOpencodeSession(func() string {
		return resolveOpencodeSessionValue(ctx)
	})
}

// resolveOpencodeSessionValue computes the x-opencode-session value for a
// request by walking the resolution chain (client header, forwarded per-request
// header, Bifrost session id, then a synthesized per-request UUID), namespacing
// whichever signal wins per granted identity. It is called by
// ResolveOpencodeSession and is read-only: it never writes to the context, so a
// concurrent supplier cannot change what another one observes.
func resolveOpencodeSessionValue(ctx *schemas.BifrostContext) string {
	// a) Client-sent header wins. The HTTP transport captures request headers
	// into BifrostContextKeyRequestHeaders (lowercased keys) and the raw
	// x-opencode-session is visible here. A prefixed x-bf-eh-* copy is not this
	// header; it is read from BifrostContextKeyExtraHeaders below.
	if headers, ok := ctx.Value(schemas.BifrostContextKeyRequestHeaders).(map[string]string); ok {
		if value, ok := normalizeOpencodeSessionValue(headers[OpencodeSessionHeader]); ok {
			return namespaceOpencodeSession(ctx, value)
		}
	}
	// b) Per-request forwarded header: the x-bf-eh-* escape hatch (or direct
	// allowlisted forwarding), already stripped and lower-cased by the
	// transport. Checked after the client's own header so the escape hatch
	// cannot override it.
	if value, ok := forwardedOpencodeSessionValue(ctx); ok {
		return namespaceOpencodeSession(ctx, value)
	}
	// c) BifrostContextKeySessionID already encodes the shared session identity
	// rules (x-bf-session-id wins, poison-to-none on an invalid explicit value,
	// then harness session headers), so reuse it rather than re-deriving. It is
	// normalized the same way as the header sources: a value carrying control
	// bytes or exceeding the session bound falls through to a synthesized UUID.
	if sessionID, ok := ctx.Value(schemas.BifrostContextKeySessionID).(string); ok {
		if value, ok := normalizeOpencodeSessionValue(sessionID); ok {
			return namespaceOpencodeSession(ctx, value)
		}
	}
	// d) No signal at all: synthesize a fresh per-request UUID so the request
	// stays compliant. No affinity — the caller must send its own session
	// identity for multi-turn tracking.
	return namespaceOpencodeSession(ctx, uuid.NewString())
}

// forwardedOpencodeSessionValue returns the per-request x-opencode-session
// carried in BifrostContextKeyExtraHeaders. The transport lower-cases the
// stripped x-bf-eh-* names; a Go SDK caller may not, so a miss on the canonical
// key falls back to a case-insensitive scan. Values are read in order and the
// first usable one wins. A session entry that is unusable is left for the next
// source in the resolution chain.
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
// request's granted identity, so the same client session used under two
// different virtual keys (or users) cannot collide upstream. Only the resolved
// virtual key row ID and user ID are read from the grant — never the presented
// credential or its value — so rotating a key's secret does not change its
// session namespace. A request nothing governs, whose grant has no identity, or
// whose identity IDs are unusable leaves the value unqualified rather than
// dropping the request.
//
// Every component is escaped before joining so the only literal ":" in the
// result is a separator we insert. Without escaping, ("a", "b:c") and ("a:b",
// "c") both produce "a:b:c" — a collision. Escaping doubles every backslash and
// prefixes every colon with a backslash, making the mapping injective: distinct
// (scope, value) pairs always yield distinct namespaced strings. The escaped
// output stays within isSafeOpencodeSessionValue's charset (only printable
// ASCII is introduced).
//
// An unqualified value is escaped too. Otherwise a caller with no grant could
// send "vk:row:session" as its session and produce exactly the value a scoped
// request for that row and session produces, crossing tenant boundaries.
func namespaceOpencodeSession(ctx *schemas.BifrostContext, value string) string {
	value = escapeSessionComponent(value)
	if scope := opencodeSessionScope(ctx); scope != "" {
		return scope + ":" + value
	}
	return value
}

// opencodeSessionScope returns the tagged identity scope for a request, or ""
// when nothing identifies it. A virtual key row ID wins over a user ID because
// a request made with a key is attributed to that key first; the tag keeps an
// ID that exists in both namespaces from colliding.
func opencodeSessionScope(ctx *schemas.BifrostContext) string {
	grant := ctx.Grant()
	if grant == nil {
		return ""
	}
	identity := grant.Identity()
	if identity == nil {
		return ""
	}
	if key := identity.VirtualKey(); key != nil {
		if id, ok := safeOpencodeSessionScopeID(key.ID); ok {
			return opencodeSessionScopeVirtualKey + ":" + escapeSessionComponent(id)
		}
	}
	if user := identity.User(); user != nil {
		if id, ok := safeOpencodeSessionScopeID(user.ID); ok {
			return opencodeSessionScopeUser + ":" + escapeSessionComponent(id)
		}
	}
	return ""
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
