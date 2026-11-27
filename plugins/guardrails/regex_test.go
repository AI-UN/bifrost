package guardrails

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func mustRegexDetector(t *testing.T, patterns ...RegexPattern) detector {
	t.Helper()
	detector, err := newRegexDetector(RegexConfig{Patterns: patterns})
	require.NoError(t, err)
	return detector
}

func TestRegexDetectorFindsMatches(t *testing.T) {
	detector := mustRegexDetector(t, RegexPattern{Pattern: `[\w.+-]+@[\w-]+\.[\w.]+`, EntityType: "EMAIL_ADDRESS"})

	found, err := detector.Detect(context.Background(), []string{"mail alex@example.com", "nothing here", "bob@example.com"})
	require.NoError(t, err)
	require.Equal(t, []finding{
		{entityType: "EMAIL_ADDRESS", literal: "alex@example.com", action: ActionBlock},
		{entityType: "EMAIL_ADDRESS", literal: "bob@example.com", action: ActionBlock},
	}, found)
}

func TestRegexDetectorEntityFallbacks(t *testing.T) {
	found, err := mustRegexDetector(t,
		RegexPattern{Pattern: `aaa`, Description: "Bad Words"},
		RegexPattern{Pattern: `bbb`},
		RegexPattern{Pattern: `ccc`, EntityType: "CUSTOM", Description: "ignored"},
	).Detect(context.Background(), []string{"aaa bbb ccc"})
	require.NoError(t, err)
	require.Equal(t, []string{"Bad Words", "REGEX_MATCH", "CUSTOM"}, []string{
		found[0].entityType, found[1].entityType, found[2].entityType,
	})
}

func TestRegexDetectorAllowedValuesAreCaseInsensitive(t *testing.T) {
	detector := mustRegexDetector(t, RegexPattern{
		Pattern:       `[a-z]+@example\.com`,
		EntityType:    "EMAIL_ADDRESS",
		AllowedValues: []string{"ALEX@EXAMPLE.COM"},
	})
	found, err := detector.Detect(context.Background(), []string{"alex@example.com bob@example.com"})
	require.NoError(t, err)
	require.Equal(t, []finding{{entityType: "EMAIL_ADDRESS", literal: "bob@example.com", action: ActionBlock}}, found)
}

func TestRegexDetectorCarriesPatternActionAndStrategy(t *testing.T) {
	found, err := mustRegexDetector(t, RegexPattern{
		Pattern:           `\d{3}-\d{2}-\d{4}`,
		EntityType:        "SSN",
		Action:            ActionRedact,
		RedactionStrategy: StrategyMask,
	}).Detect(context.Background(), []string{"ssn 123-45-6789"})
	require.NoError(t, err)
	require.Len(t, found, 1)
	require.Equal(t, ActionRedact, found[0].action)
	require.Equal(t, StrategyMask, found[0].strategy)
}

func TestRegexDetectorFlagsAreInline(t *testing.T) {
	detector := mustRegexDetector(t, RegexPattern{Pattern: `^secret$`, Flags: "im", EntityType: "CUSTOM"})
	found, err := detector.Detect(context.Background(), []string{"line one\nSeCrEt\nline three"})
	require.NoError(t, err)
	require.Len(t, found, 1)
	require.Equal(t, "SeCrEt", found[0].literal)
}

func TestRegexDetectorBoundsMatchesPerPattern(t *testing.T) {
	// Detection work is bounded, not just the HTTP call: one broad pattern on one large segment
	// must not materialize unbounded findings. Exceeding the bound is an error, not a partial
	// answer, so an enforcing profile fails closed.
	detector := mustRegexDetector(t, RegexPattern{Pattern: `a`, EntityType: "A"})
	_, err := detector.Detect(context.Background(), []string{strings.Repeat("a", maxRegexMatchesPerPattern)})
	require.NoError(t, err)

	_, err = detector.Detect(context.Background(), []string{strings.Repeat("a", maxRegexMatchesPerPattern+1)})
	require.Error(t, err)
}

func TestRegexDetectorHonoursContextCancellation(t *testing.T) {
	detector := mustRegexDetector(t, RegexPattern{Pattern: `[\w.+-]+@[\w-]+\.[\w.]+`, EntityType: "EMAIL_ADDRESS"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := detector.Detect(ctx, []string{"alex@example.com"})
	require.ErrorIs(t, err, context.Canceled)
}

func TestRegexDetectorRejectsBadPattern(t *testing.T) {
	_, err := newRegexDetector(RegexConfig{Patterns: []RegexPattern{{Pattern: "(["}}})
	require.Error(t, err)
}
