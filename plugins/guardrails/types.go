// Package guardrails is the OSS built-in guardrails plugin. It inspects the input text of
// an LLM request with secret, custom-regex and Presidio detectors, and can detect, block or
// redact what it matches. Detection runs in PreRequestHook, and again in PreLLMHook only when a
// later turn changed the request's text (a prompt template, an MCP tool result), so a plain
// fallback replay runs no detector. PreLLMHook short-circuits whatever that inspection blocked.
package guardrails

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/maximhq/bifrost/core/schemas"
)

// PluginName is the name the plugin registers under.
const PluginName = "guardrails"

// Guardrail providers (profiles) this build supports.
const (
	ProviderSecrets  = "secrets"
	ProviderRegex    = "regex"
	ProviderPresidio = "presidio"
)

// Actions a matched finding resolves to.
const (
	ActionDetectOnly = "detect_only"
	ActionBlock      = "block"
	ActionRedact     = "redact"
)

// Redaction strategies. The placeholder formats mirror the enterprise build's.
const (
	StrategyReplace = "replace"
	StrategyMask    = "mask"
	StrategyHash    = "hash"
)

// v1 scope: runtime redaction only, LLM target only, input phase only.
const (
	RedactionModeRuntime = "runtime"
	TargetLLM            = "llm"
	ApplyToInput         = "input"
)

// ErrUnsupported is wrapped by every validator rejection of a value this build does not
// implement, so the HTTP layer can answer 400 "not supported in this build" instead of
// silently accepting a setting it ignores.
var ErrUnsupported = errors.New("not supported in this build")

// Profile is one configured guardrail provider (one detector plus its settings).
type Profile struct {
	ID           uint
	ProviderName string
	PolicyName   string
	Enabled      bool
	Timeout      int             // seconds, 0 => 30
	Config       json.RawMessage // provider config JSON, snake_case keys
}

// Rule selects which requests are inspected and which profiles run on them.
type Rule struct {
	ID            uint
	Name          string
	Description   string
	Enabled       bool
	Target        string // "" or "llm"
	CELExpression string
	ApplyTo       string // "input"
	SamplingRate  *int   // nil => 100
	Timeout       int    // seconds, 0 => 60
	ProfileIDs    []uint // execution order
}

// SecretsConfig is a secrets profile's config.
type SecretsConfig struct {
	IgnoredSecretKeywords []string           `json:"ignored_secret_keywords,omitempty"`
	DisabledRules         []string           `json:"disabled_rules,omitempty"`
	RuleEntropy           map[string]float64 `json:"rule_entropy,omitempty"`
	Action                string             `json:"action,omitempty"`
	RedactionStrategy     string             `json:"redaction_strategy,omitempty"`
	RedactionMode         string             `json:"redaction_mode,omitempty"`
}

// RegexPattern is one custom regular expression.
type RegexPattern struct {
	Pattern           string   `json:"pattern"`
	Description       string   `json:"description,omitempty"`
	EntityType        string   `json:"entity_type,omitempty"`
	Flags             string   `json:"flags,omitempty"`
	Action            string   `json:"action,omitempty"`
	RedactionStrategy string   `json:"redaction_strategy,omitempty"`
	RedactionMode     string   `json:"redaction_mode,omitempty"`
	AllowedValues     []string `json:"allowed_values,omitempty"`
}

// RegexConfig is a regex profile's config.
type RegexConfig struct {
	Patterns []RegexPattern `json:"patterns"`
}

// PresidioConfig is a presidio profile's config. FailOpen is this fork's extension: it
// decides whether an unreachable Analyzer blocks (default) or only logs.
type PresidioConfig struct {
	AnalyzerURL       string             `json:"analyzer_url"`
	APIKey            *schemas.SecretVar `json:"api_key,omitempty"`
	Language          string             `json:"language,omitempty"`
	ScoreThreshold    *float64           `json:"score_threshold,omitempty"`
	Entities          []string           `json:"entities,omitempty"`
	Action            string             `json:"action,omitempty"`
	RedactionStrategy string             `json:"redaction_strategy,omitempty"`
	RedactionMode     string             `json:"redaction_mode,omitempty"`
	FailOpen          bool               `json:"fail_open,omitempty"`
}

// SecretRuleInfo describes one gitleaks rule, for the rules picker.
type SecretRuleInfo struct {
	ID          string  `json:"id"`
	Description string  `json:"description"`
	Entropy     float64 `json:"entropy"`
}

// Config is the plugin's row config. Empty in v1: everything lives in the profiles and rules.
type Config struct{}

// Loader reads the current profiles and rules. The host supplies it, so the plugin holds no
// database handle.
type Loader func(ctx context.Context) ([]Profile, []Rule, error)

// finding is one detector hit. action and strategy name the pattern's own overrides when it
// has any (custom regex), and are empty when the profile's apply.
type finding struct {
	entityType string
	literal    string
	action     string
	strategy   string
}

// detector is the single internal interface the three providers implement.
type detector interface {
	Detect(ctx context.Context, segments []string) ([]finding, error)
}
