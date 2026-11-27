package guardrails

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateProfileRejectsUnknownProvider(t *testing.T) {
	err := ValidateProfile(Profile{ProviderName: "noma"})
	require.ErrorIs(t, err, ErrUnsupported)
}

func TestValidateProfileRejectsUnsupportedValues(t *testing.T) {
	tests := map[string]Profile{
		"action":   {ProviderName: ProviderSecrets, Config: json.RawMessage(`{"action":"transform"}`)},
		"strategy": {ProviderName: ProviderSecrets, Config: json.RawMessage(`{"redaction_strategy":"tokenize"}`)},
		"mode":     {ProviderName: ProviderSecrets, Config: json.RawMessage(`{"redaction_mode":"logs_only"}`)},
		"presidio mode": {
			ProviderName: ProviderPresidio,
			Config:       json.RawMessage(`{"analyzer_url":"http://presidio:3000","redaction_mode":"runtime_reversible"}`),
		},
	}
	for name, profile := range tests {
		t.Run(name, func(t *testing.T) {
			require.ErrorIs(t, ValidateProfile(profile), ErrUnsupported)
		})
	}
}

func TestValidateProfileUnknownGitleaksRule(t *testing.T) {
	err := ValidateProfile(Profile{ProviderName: ProviderSecrets, Config: json.RawMessage(`{"disabled_rules":["not-a-rule"]}`)})
	require.Error(t, err)
}

func TestValidateProfileRuleEntropyBounds(t *testing.T) {
	require.NoError(t, ValidateProfile(Profile{
		ProviderName: ProviderSecrets,
		Config:       json.RawMessage(`{"rule_entropy":{"aws-access-token":4.5}}`),
	}))
	require.Error(t, ValidateProfile(Profile{
		ProviderName: ProviderSecrets,
		Config:       json.RawMessage(`{"rule_entropy":{"aws-access-token":7}}`),
	}))
}

func TestValidateProfileRegexLimits(t *testing.T) {
	patterns := make([]RegexPattern, maxRegexPatterns+1)
	for i := range patterns {
		patterns[i] = RegexPattern{Pattern: "a"}
	}
	tooMany, err := json.Marshal(RegexConfig{Patterns: patterns})
	require.NoError(t, err)
	require.Error(t, ValidateProfile(Profile{ProviderName: ProviderRegex, Config: tooMany}))

	oversized, err := json.Marshal(RegexConfig{Patterns: []RegexPattern{{Pattern: strings.Repeat("a", maxRegexPatternBytes+1)}}})
	require.NoError(t, err)
	require.Error(t, ValidateProfile(Profile{ProviderName: ProviderRegex, Config: oversized}))

	broken, err := json.Marshal(RegexConfig{Patterns: []RegexPattern{{Pattern: "(["}}})
	require.NoError(t, err)
	require.Error(t, ValidateProfile(Profile{ProviderName: ProviderRegex, Config: broken}))

	emptyMatch, err := json.Marshal(RegexConfig{Patterns: []RegexPattern{{Pattern: "a*"}}})
	require.NoError(t, err)
	require.Error(t, ValidateProfile(Profile{ProviderName: ProviderRegex, Config: emptyMatch}))

	require.NoError(t, ValidateProfile(Profile{
		ProviderName: ProviderRegex,
		Config:       json.RawMessage(`{"patterns":[{"pattern":"[a-z]+@[a-z]+","entity_type":"EMAIL_ADDRESS","flags":"i","action":"redact","redaction_strategy":"mask"}]}`),
	}))
}

func TestValidateProfilePresidioAnalyzerURL(t *testing.T) {
	for _, url := range []string{"", "presidio:3000", "file:///tmp/x", "ftp://presidio"} {
		raw, err := json.Marshal(PresidioConfig{AnalyzerURL: url})
		require.NoError(t, err)
		require.Error(t, ValidateProfile(Profile{ProviderName: ProviderPresidio, Config: raw}), "url %q must be rejected", url)
	}
	raw, err := json.Marshal(PresidioConfig{AnalyzerURL: "http://presidio:3000"})
	require.NoError(t, err)
	require.NoError(t, ValidateProfile(Profile{ProviderName: ProviderPresidio, Config: raw}))
}

func TestValidateProfileRejectsUndecodableConfig(t *testing.T) {
	require.Error(t, ValidateProfile(Profile{ProviderName: ProviderRegex, Config: json.RawMessage(`{"patterns":"nope"}`)}))
	require.Error(t, ValidateProfile(Profile{ProviderName: ProviderSecrets, Config: json.RawMessage(`{`)}))
}

func TestValidateRule(t *testing.T) {
	require.NoError(t, ValidateRule(Rule{Enabled: true}))

	tests := map[string]Rule{
		"target":         {Target: "mcp"},
		"apply to":       {ApplyTo: "output"},
		"sampling high":  {SamplingRate: new(101)},
		"sampling low":   {SamplingRate: new(-1)},
		"timeout":        {Timeout: -1},
		"cel syntax":     {CELExpression: "provider =="},
		"cel not a bool": {CELExpression: `provider`},
		"unknown field":  {CELExpression: `mcp_tool == "x"`},
	}
	for name, rule := range tests {
		t.Run(name, func(t *testing.T) {
			require.Error(t, ValidateRule(rule))
		})
	}

	require.ErrorIs(t, ValidateRule(Rule{Target: "mcp"}), ErrUnsupported)
	require.ErrorIs(t, ValidateRule(Rule{ApplyTo: "output"}), ErrUnsupported)
	require.NoError(t, ValidateRule(Rule{
		Target:        TargetLLM,
		ApplyTo:       ApplyToInput,
		CELExpression: `provider == "openai" && headers["x-env"] == "prod" && team == "platform"`,
		SamplingRate:  new(50),
		Timeout:       10,
	}))
}

func TestCompilePatternFlags(t *testing.T) {
	pattern, err := compilePattern(RegexPattern{Pattern: "abc", Flags: "is"})
	require.NoError(t, err)
	require.True(t, pattern.MatchString("xABC\ny"))
	// An unknown flag is a validation error, not a silently ignored one.
	_, err = compilePattern(RegexPattern{Pattern: "abc", Flags: "g"})
	require.ErrorIs(t, err, ErrUnsupported)
}

func TestSanitizeEntityType(t *testing.T) {
	require.Equal(t, "EMAIL_ADDRESS", sanitizeEntityType("EMAIL_ADDRESS"))
	require.Equal(t, "EMAIL_ADDRESS", sanitizeEntityType("EMAIL ADDRESS"))
	// Brackets and newlines are dropped so a description can never break a placeholder or a log
	// field; spaces become underscores.
	require.Equal(t, "weirdentity", sanitizeEntityType("weird]\n[entity"))
	require.Equal(t, "ENTITY", sanitizeEntityType("[]\n"))
}

func TestErrUnsupportedIsWrapped(t *testing.T) {
	err := ValidateProfile(Profile{ProviderName: "other"})
	require.True(t, errors.Is(err, ErrUnsupported))
}
