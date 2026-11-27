package guardrails

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"unicode/utf8"
)

// maxMaskLength caps the asterisks a mask placeholder emits. A mask keeps a value's rough
// length, but a detector can return a very long match, and the placeholder would then be as
// large as the secret it replaces.
const maxMaskLength = 64

// placeholder renders the replacement for one literal. Formats mirror the enterprise build:
// "[EMAIL]", "[EMAIL:****]" and "[EMAIL:<16 hex>]" where the 16 hex characters are an
// HMAC-SHA256 prefix of the value, so an email or phone number cannot be recovered by
// enumerating a bare hash of the same value.
func placeholder(entityType, strategy, literal string, hmacKey []byte) string {
	label := sanitizeEntityType(entityType)
	switch strategy {
	case StrategyMask:
		maskLength := min(utf8.RuneCountInString(literal), maxMaskLength)
		return "[" + label + ":" + strings.Repeat("*", maskLength) + "]"
	case StrategyHash:
		mac := hmac.New(sha256.New, hmacKey)
		mac.Write([]byte(literal))
		return "[" + label + ":" + hex.EncodeToString(mac.Sum(nil)[:8]) + "]"
	default:
		return "[" + label + "]"
	}
}

// effectiveStrategy resolves a finding's redaction strategy: the pattern's own when it sets
// one, else the profile's, else replace.
func effectiveStrategy(f finding, profile *compiledProfile) string {
	if f.strategy != "" {
		return f.strategy
	}
	if profile != nil && profile.strategy != "" {
		return profile.strategy
	}
	return StrategyReplace
}

// effectiveAction resolves a finding's action: the pattern's own when it sets one (custom
// regex), else the profile's.
func effectiveAction(f finding, profile *compiledProfile) string {
	if f.action != "" {
		return f.action
	}
	if profile != nil {
		return profile.action
	}
	return ActionDetectOnly
}

// defaultAction resolves an empty configured action.
func defaultAction(action, fallback string) string {
	if action == "" {
		return fallback
	}
	return action
}
