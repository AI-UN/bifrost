package guardrails

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

// testGitHubToken is derived at run time from a fixed seed, so the repository holds no literal
// credential, and its hex body is neither sequential nor repetitive (gitleaks' global
// allowlists reject both). Its entropy sits above the rule's floor of 3 and below 6, which the
// entropy test relies on.
var testGitHubToken = "gh" + "p_" + testTokenBody("guardrails", 36)

// testTokenBody returns the first length characters of a hex digest.
func testTokenBody(seed string, length int) string {
	sum := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(sum[:])[:length]
}

func mustSecretsDetector(t *testing.T, cfg SecretsConfig) detector {
	t.Helper()
	detector, err := newSecretsDetector(cfg)
	require.NoError(t, err)
	return detector
}

func TestSecretsDetectorFindsSecret(t *testing.T) {
	found := mustFind(t, mustSecretsDetector(t, SecretsConfig{}), []string{"token = " + testGitHubToken})
	require.NotEmpty(t, found)

	var match *finding
	for i := range found {
		if found[i].entityType == "GITHUB-PAT" {
			match = &found[i]
			break
		}
	}
	require.NotNil(t, match, "expected the github-pat rule, got %v", found)
	require.Equal(t, testGitHubToken, match.literal)
}

func TestSecretsDetectorIgnoredKeywords(t *testing.T) {
	// A keyword found anywhere in a matched secret allows it, like the enterprise build's
	// allowlist stopwords.
	detector := mustSecretsDetector(t, SecretsConfig{IgnoredSecretKeywords: []string{testGitHubToken[4:10]}})
	require.Empty(t, mustFind(t, detector, []string{"token = " + testGitHubToken}))
}

func TestSecretsDetectorDisabledRules(t *testing.T) {
	detector := mustSecretsDetector(t, SecretsConfig{DisabledRules: []string{"github-pat"}})
	require.NotContains(t, entityTypesOf(mustFind(t, detector, []string{"token = " + testGitHubToken})), "GITHUB-PAT")
}

func TestSecretsDetectorRuleEntropyOverride(t *testing.T) {
	// The default entropy floor is 3; raising it past the value's own entropy makes the same
	// text stop matching.
	detector := mustSecretsDetector(t, SecretsConfig{RuleEntropy: map[string]float64{"github-pat": 6}})
	require.NotContains(t, entityTypesOf(mustFind(t, detector, []string{"token = " + testGitHubToken})), "GITHUB-PAT")
}

func TestSecretsDetectorUsesProfileScopedConfig(t *testing.T) {
	claiming := mustSecretsDetector(t, SecretsConfig{})
	disabling := mustSecretsDetector(t, SecretsConfig{DisabledRules: []string{"github-pat"}})
	text := []string{"token = " + testGitHubToken}

	require.Contains(t, entityTypesOf(mustFind(t, claiming, text)), "GITHUB-PAT")
	require.NotContains(t, entityTypesOf(mustFind(t, disabling, text)), "GITHUB-PAT",
		"one profile's disabled rules must not reach another detector")
}

func TestSecretsDetectorIgnoresGitleaksAllowAnnotation(t *testing.T) {
	// gitleaks drops a finding whose line carries its own source-code suppression annotation.
	// Prompt text is untrusted, so the annotation must not hide a secret.
	found := mustFind(t, mustSecretsDetector(t, SecretsConfig{}), []string{"token = " + testGitHubToken + " # gitleaks:allow"})
	require.Contains(t, entityTypesOf(found), "GITHUB-PAT")
}

func TestSecretsDetectorHonoursContextCancellation(t *testing.T) {
	detector := mustSecretsDetector(t, SecretsConfig{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// An interrupted scan is an incomplete answer: it must be an error so an enforcing profile
	// fails closed, not an empty result that looks clean.
	found, err := detector.Detect(ctx, []string{"token = " + testGitHubToken})
	require.ErrorIs(t, err, context.Canceled)
	require.Empty(t, found)
}

func TestSecretsRuleCatalog(t *testing.T) {
	catalog := SecretsRuleCatalog()
	require.NotEmpty(t, catalog)

	ids := make([]string, 0, len(catalog))
	for _, rule := range catalog {
		require.NotEmpty(t, rule.ID)
		require.NotEmpty(t, rule.Description)
		ids = append(ids, rule.ID)
	}
	require.Contains(t, ids, "github-pat")
	require.IsIncreasing(t, ids)

	// The list is a copy: a caller mutating it must not reach the shared catalog.
	catalog[0].ID = "mutated"
	require.Equal(t, ids[0], SecretsRuleCatalog()[0].ID)
}

// mustFind runs a detector and fails the test on an error.
func mustFind(t *testing.T, detector detector, segments []string) []finding {
	t.Helper()
	found, err := detector.Detect(context.Background(), segments)
	require.NoError(t, err)
	return found
}

// entityTypesOf lists the entity types of findings, for contains-style assertions.
func entityTypesOf(hits []finding) []string {
	types := make([]string, 0, len(hits))
	for _, hit := range hits {
		types = append(types, hit.entityType)
	}
	return types
}
