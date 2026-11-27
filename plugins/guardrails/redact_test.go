package guardrails

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var testHMACKey = []byte("guardrails-test-hmac-key-32-bytes")

func TestPlaceholderStrategies(t *testing.T) {
	require.Equal(t, "[EMAIL_ADDRESS]", placeholder("EMAIL_ADDRESS", StrategyReplace, "alex@example.com", testHMACKey))
	require.Equal(t, "[EMAIL_ADDRESS:****************]", placeholder("EMAIL_ADDRESS", StrategyMask, "alex@example.com", testHMACKey))
	require.Equal(t, "[EMAIL_ADDRESS]", placeholder("EMAIL_ADDRESS", "", "alex@example.com", testHMACKey))
}

func TestMaskCountsRunesNotBytes(t *testing.T) {
	// Three Chinese characters are nine bytes but three runes.
	require.Equal(t, "[NAME:***]", placeholder("NAME", StrategyMask, "张三丰", testHMACKey))
	// An emoji is one rune per code point here.
	require.Equal(t, "[NAME:**]", placeholder("NAME", StrategyMask, "😀😀", testHMACKey))
}

func TestMaskIsCapped(t *testing.T) {
	require.Equal(t, "["+"NAME:"+strings.Repeat("*", maxMaskLength)+"]", placeholder("NAME", StrategyMask, strings.Repeat("a", maxMaskLength*4), testHMACKey))
}

func TestHashIsDeterministicAndKeyed(t *testing.T) {
	first := placeholder("EMAIL_ADDRESS", StrategyHash, "alex@example.com", testHMACKey)
	second := placeholder("EMAIL_ADDRESS", StrategyHash, "alex@example.com", testHMACKey)
	require.Equal(t, first, second)
	require.Regexp(t, `^\[EMAIL_ADDRESS:[0-9a-f]{16}\]$`, first)

	// A different key yields a different digest for the same value, so a dump of placeholders
	// cannot be attacked with a precomputed table.
	other := placeholder("EMAIL_ADDRESS", StrategyHash, "alex@example.com", []byte("another-key-0123456789abcdefghijk"))
	require.NotEqual(t, first, other)

	require.NotEqual(t, first, placeholder("EMAIL_ADDRESS", StrategyHash, "bob@example.com", testHMACKey))
}

func TestEffectiveActionAndStrategy(t *testing.T) {
	profile := &compiledProfile{action: ActionBlock, strategy: StrategyReplace}

	require.Equal(t, ActionBlock, effectiveAction(finding{}, profile))
	require.Equal(t, ActionRedact, effectiveAction(finding{action: ActionRedact}, profile))
	require.Equal(t, StrategyReplace, effectiveStrategy(finding{}, profile))
	require.Equal(t, StrategyMask, effectiveStrategy(finding{strategy: StrategyMask}, profile))
	require.Equal(t, StrategyReplace, effectiveStrategy(finding{}, &compiledProfile{}))
	require.Equal(t, ActionDetectOnly, effectiveAction(finding{}, nil))
}

func TestDefaultAction(t *testing.T) {
	require.Equal(t, ActionBlock, defaultAction("", ActionBlock))
	require.Equal(t, ActionRedact, defaultAction(ActionRedact, ActionBlock))
}
