package guardrails

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/rs/zerolog"
	"github.com/spf13/viper"
	gitleaksconfig "github.com/zricethezav/gitleaks/v8/config"
	"github.com/zricethezav/gitleaks/v8/detect"
	"github.com/zricethezav/gitleaks/v8/logging"
)

// gitleaksDefaults is the parsed embedded gitleaks rule set, shared by every secrets profile.
// Each profile builds its own detector config from it, so one profile's disabled rules or
// entropy overrides never reach another's.
type gitleaksDefaults struct {
	config  gitleaksconfig.Config
	catalog map[string]SecretRuleInfo
	rules   []SecretRuleInfo
}

var gitleaksDefaultsOnce = sync.OnceValues(func() (*gitleaksDefaults, error) {
	// The embedded default TOML is parsed through a private viper instance:
	// detect.NewDetectorDefaultConfig parses through the process-global one, so any viper
	// state another part of the process sets could change which rules the gateway runs.
	v := viper.New()
	v.SetConfigType("toml")
	if err := v.ReadConfig(strings.NewReader(gitleaksconfig.DefaultConfig)); err != nil {
		return nil, fmt.Errorf("read gitleaks default config: %w", err)
	}
	var parsed gitleaksconfig.ViperConfig
	if err := v.Unmarshal(&parsed); err != nil {
		return nil, fmt.Errorf("parse gitleaks default config: %w", err)
	}
	cfg, err := parsed.Translate()
	if err != nil {
		return nil, fmt.Errorf("translate gitleaks default config: %w", err)
	}

	defaults := &gitleaksDefaults{config: cfg, catalog: make(map[string]SecretRuleInfo, len(cfg.Rules))}
	defaults.rules = make([]SecretRuleInfo, 0, len(cfg.Rules))
	for _, rule := range cfg.Rules {
		info := SecretRuleInfo{ID: rule.RuleID, Description: rule.Description, Entropy: rule.Entropy}
		defaults.catalog[info.ID] = info
		defaults.rules = append(defaults.rules, info)
	}
	sort.Slice(defaults.rules, func(i, j int) bool { return defaults.rules[i].ID < defaults.rules[j].ID })
	return defaults, nil
})

// secretsRuleCatalog returns the rule set, keyed by id, for validation.
func secretsRuleCatalog() (map[string]SecretRuleInfo, error) {
	defaults, err := gitleaksDefaultsOnce()
	if err != nil {
		return nil, err
	}
	return defaults.catalog, nil
}

// SecretsRuleCatalog lists every gitleaks rule this build ships, sorted by id.
func SecretsRuleCatalog() []SecretRuleInfo {
	defaults, err := gitleaksDefaultsOnce()
	if err != nil {
		return nil
	}
	return slices.Clone(defaults.rules)
}

// gitleaksLoggerOnce keeps the package-global logger assignment race-free across reloads.
var gitleaksLoggerOnce sync.Once

// secretsDetector scans text with the gitleaks rule set.
type secretsDetector struct {
	detector *detect.Detector
}

// newSecretsDetector builds one detector per profile: nothing is shared with gitleaks' global
// state, so concurrent requests only ever read it.
func newSecretsDetector(cfg SecretsConfig) (detector, error) {
	defaults, err := gitleaksDefaultsOnce()
	if err != nil {
		return nil, err
	}
	base := defaults.config

	rules := maps.Clone(base.Rules)
	for _, id := range cfg.DisabledRules {
		delete(rules, id)
	}
	for id, entropy := range cfg.RuleEntropy {
		rule, ok := rules[id]
		if !ok {
			continue
		}
		rule.Entropy = entropy
		rules[id] = rule
	}

	allowlists := slices.Clone(base.Allowlists)
	if stopWords := normalizeStopWords(cfg.IgnoredSecretKeywords); len(stopWords) > 0 {
		allowlists = append(allowlists, &gitleaksconfig.Allowlist{StopWords: stopWords})
	}

	// gitleaks logs matches, rules and decoded fragments through a package-global zerolog
	// logger. It must never write a matched secret into the gateway's log.
	gitleaksLoggerOnce.Do(func() { logging.Logger = zerolog.Nop() })

	detector := detect.NewDetector(gitleaksconfig.Config{
		Title:      base.Title,
		Rules:      rules,
		Keywords:   base.Keywords,
		Allowlists: allowlists,
	})
	// gitleaks discards a finding that shares its line with its own source-code suppression
	// annotation. Prompt text is untrusted, so a caller could append one and hide a secret:
	// every finding is reported.
	detector.IgnoreGitleaksAllow = true
	return &secretsDetector{detector: detector}, nil
}

// normalizeStopWords lowercases and de-duplicates the ignore keywords; gitleaks matches
// stopwords against a finding's secret case-insensitively.
func normalizeStopWords(keywords []string) []string {
	if len(keywords) == 0 {
		return nil
	}
	stopWords := make([]string, 0, len(keywords))
	seen := make(map[string]struct{}, len(keywords))
	for _, keyword := range keywords {
		keyword = strings.ToLower(strings.TrimSpace(keyword))
		if keyword == "" {
			continue
		}
		if _, ok := seen[keyword]; ok {
			continue
		}
		seen[keyword] = struct{}{}
		stopWords = append(stopWords, keyword)
	}
	return stopWords
}

// Detect scans every segment with the profile's rule set, honouring the caller's deadline
// between rules. The detector is read-only during a scan, so it is shared across goroutines.
// A scan whose deadline expires mid-way is reported as an error, not as the findings it had
// reached: an incomplete answer must fail closed.
func (d *secretsDetector) Detect(ctx context.Context, segments []string) ([]finding, error) {
	var found []finding
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for _, segment := range segments {
		if segment == "" {
			continue
		}
		for _, hit := range d.detector.DetectContext(ctx, detect.Fragment{Raw: segment}) {
			if hit.Secret == "" {
				continue
			}
			found = append(found, finding{entityType: strings.ToUpper(hit.RuleID), literal: hit.Secret})
		}
		// DetectContext stops scanning and returns what it collected so far when the context
		// ends, which is indistinguishable from a clean segment without this check.
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	return found, nil
}
