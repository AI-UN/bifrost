package guardrails

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// v1 regex limits, enforced at write time so a profile cannot be saved that would make
// every request pay for an unbounded scan.
const (
	maxRegexPatterns     = 200
	maxRegexPatternBytes = 4 << 10
	defaultProfileTimeout = 30
	defaultRuleTimeout    = 60
)

// ValidateProfile rejects anything this build cannot run: an unknown provider, a config
// that does not decode, a v1-unsupported value, an unknown gitleaks rule, or a regex that
// does not compile. Unsupported values wrap ErrUnsupported.
func ValidateProfile(p Profile) error {
	switch p.ProviderName {
	case ProviderSecrets:
		var cfg SecretsConfig
		if err := decodeConfig(p.Config, &cfg); err != nil {
			return err
		}
		catalog, err := secretsRuleCatalog()
		if err != nil {
			return err
		}
		for _, id := range cfg.DisabledRules {
			if _, ok := catalog[id]; !ok {
				return fmt.Errorf("unknown gitleaks rule %q", id)
			}
		}
		for id, entropy := range cfg.RuleEntropy {
			if _, ok := catalog[id]; !ok {
				return fmt.Errorf("unknown gitleaks rule %q", id)
			}
			if entropy < 0 || entropy > 6 {
				return fmt.Errorf("rule_entropy for %q must be between 0 and 6, got %v", id, entropy)
			}
		}
		return validateCommon(cfg.Action, cfg.RedactionStrategy, cfg.RedactionMode)
	case ProviderRegex:
		var cfg RegexConfig
		if err := decodeConfig(p.Config, &cfg); err != nil {
			return err
		}
		if len(cfg.Patterns) > maxRegexPatterns {
			return fmt.Errorf("a regex profile allows at most %d patterns, got %d", maxRegexPatterns, len(cfg.Patterns))
		}
		for i, pattern := range cfg.Patterns {
			if pattern.Pattern == "" {
				return fmt.Errorf("pattern %d: pattern is required", i)
			}
			if len(pattern.Pattern) > maxRegexPatternBytes {
				return fmt.Errorf("pattern %d: pattern exceeds %d bytes", i, maxRegexPatternBytes)
			}
			if _, err := compilePattern(pattern); err != nil {
				return fmt.Errorf("pattern %d: %w", i, err)
			}
			if err := validateCommon(pattern.Action, pattern.RedactionStrategy, pattern.RedactionMode); err != nil {
				return fmt.Errorf("pattern %d: %w", i, err)
			}
		}
		return nil
	case ProviderPresidio:
		var cfg PresidioConfig
		if err := decodeConfig(p.Config, &cfg); err != nil {
			return err
		}
		if err := validateAnalyzerURL(cfg.AnalyzerURL); err != nil {
			return err
		}
		return validateCommon(cfg.Action, cfg.RedactionStrategy, cfg.RedactionMode)
	default:
		return fmt.Errorf("%w: unknown guardrail provider %q", ErrUnsupported, p.ProviderName)
	}
}

// ValidateRule rejects a rule this build cannot evaluate. Unsupported values wrap
// ErrUnsupported.
func ValidateRule(r Rule) error {
	if target := r.Target; target != "" && target != TargetLLM {
		return fmt.Errorf("%w: target %q", ErrUnsupported, target)
	}
	if applyTo := r.ApplyTo; applyTo != "" && applyTo != ApplyToInput {
		return fmt.Errorf("%w: apply_to %q", ErrUnsupported, applyTo)
	}
	if r.SamplingRate != nil && (*r.SamplingRate < 0 || *r.SamplingRate > 100) {
		return fmt.Errorf("sampling_rate must be between 0 and 100, got %d", *r.SamplingRate)
	}
	if r.Timeout < 0 {
		return fmt.Errorf("timeout must not be negative, got %d", r.Timeout)
	}
	if _, err := compileCEL(r.CELExpression); err != nil {
		return err
	}
	return nil
}

// decodeConfig decodes a profile's stored config. Unknown keys are left to the HTTP layer:
// provider configs are written by this build's own API, which owns rejecting the fields
// this version does not run.
func decodeConfig(raw json.RawMessage, dst any) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("config does not decode: %w", err)
	}
	return nil
}

// validateCommon checks the action settings every provider shares.
func validateCommon(action, strategy, mode string) error {
	switch action {
	case "", ActionDetectOnly, ActionBlock, ActionRedact:
	default:
		return fmt.Errorf("%w: action %q", ErrUnsupported, action)
	}
	switch strategy {
	case "", StrategyReplace, StrategyMask, StrategyHash:
	default:
		return fmt.Errorf("%w: redaction_strategy %q", ErrUnsupported, strategy)
	}
	switch mode {
	case "", RedactionModeRuntime:
	default:
		return fmt.Errorf("%w: redaction_mode %q", ErrUnsupported, mode)
	}
	return nil
}

// validateAnalyzerURL accepts only an absolute http(s) URL: the Analyzer is dialled from the
// gateway, so any other scheme is a configuration mistake, not a request-time failure.
func validateAnalyzerURL(raw string) error {
	if raw == "" {
		return fmt.Errorf("analyzer_url is required")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("analyzer_url is invalid: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("analyzer_url must be an http(s) URL, got %q", raw)
	}
	if parsed.Host == "" {
		return fmt.Errorf("analyzer_url has no host: %q", raw)
	}
	return nil
}

// compilePattern compiles a custom regex. Findings are matched with the standard library's
// RE2 engine only (never regexp2's backtracking), so a hostile pattern cannot run away.
func compilePattern(p RegexPattern) (*regexp.Regexp, error) {
	expr := p.Pattern
	flags := make([]byte, 0, 3)
	for i := range len(p.Flags) {
		switch flag := p.Flags[i]; flag {
		case 'i', 'm', 's':
			if !bytes.ContainsRune(flags, rune(flag)) {
				flags = append(flags, flag)
			}
		default:
			return nil, fmt.Errorf("%w: regex flag %q (expected i, m or s)", ErrUnsupported, string(flag))
		}
	}
	if len(flags) > 0 {
		expr = "(?" + string(flags) + ")" + expr
	}
	compiled, err := regexp.Compile(expr)
	if err != nil {
		return nil, fmt.Errorf("pattern does not compile: %w", err)
	}
	if compiled.MatchString("") {
		// A pattern that matches the empty string reports a zero-width hit on every
		// segment, which can never be redacted and would flood the findings.
		return nil, fmt.Errorf("pattern matches the empty string")
	}
	return compiled, nil
}

// profileTimeout resolves a profile's timeout in seconds.
func profileTimeout(p Profile) int {
	if p.Timeout > 0 {
		return p.Timeout
	}
	return defaultProfileTimeout
}

// ruleTimeout resolves a rule's timeout in seconds.
func ruleTimeout(r Rule) int {
	if r.Timeout > 0 {
		return r.Timeout
	}
	return defaultRuleTimeout
}

// sanitizeEntityType keeps an entity type usable as a placeholder label and as a log field:
// configuration is trusted, but a description used as an entity type is free text.
func sanitizeEntityType(entityType string) string {
	var b strings.Builder
	for _, r := range entityType {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-', r == '.':
			b.WriteRune(r)
		case r == ' ':
			b.WriteByte('_')
		}
		if b.Len() >= 64 {
			break
		}
	}
	if b.Len() == 0 {
		return "ENTITY"
	}
	return b.String()
}
