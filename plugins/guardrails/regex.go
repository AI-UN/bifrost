package guardrails

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// regexDetector matches custom patterns. Patterns compile once per reload with the standard
// library's RE2 engine, which runs in linear time: a hostile expression cannot backtrack, so a
// request cannot be made to hang. regexp2 is never used.
type regexDetector struct {
	patterns []regexPattern
}

// regexPattern is one compiled custom pattern with the settings its findings resolve to.
type regexPattern struct {
	re       *regexp.Regexp
	entity   string
	action   string
	strategy string
	allowed  map[string]struct{} // lowercased whole-match allowlist; nil when unset
}

// newRegexDetector compiles a profile's patterns.
func newRegexDetector(cfg RegexConfig) (detector, error) {
	patterns := make([]regexPattern, 0, len(cfg.Patterns))
	for i, pattern := range cfg.Patterns {
		re, err := compilePattern(pattern)
		if err != nil {
			return nil, fmt.Errorf("pattern %d: %w", i, err)
		}
		// The entity type the placeholder and the detection entry name: the explicit type, else
		// the description, else a generic label.
		entity := pattern.EntityType
		if entity == "" {
			entity = pattern.Description
		}
		if entity == "" {
			entity = "REGEX_MATCH"
		}
		patterns = append(patterns, regexPattern{
			re:       re,
			entity:   entity,
			action:   defaultAction(pattern.Action, ActionBlock),
			strategy: pattern.RedactionStrategy,
			allowed:  lowerSet(pattern.AllowedValues),
		})
	}
	return &regexDetector{patterns: patterns}, nil
}

// lowerSet builds a case-insensitive whole-match set.
func lowerSet(values []string) map[string]struct{} {
	if len(values) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value = strings.ToLower(strings.TrimSpace(value)); value != "" {
			set[value] = struct{}{}
		}
	}
	return set
}

// maxRegexMatchesPerPattern bounds one pattern's matches on one segment. A broad expression on a
// large accepted request would otherwise materialize millions of findings before anything checks
// the deadline; exceeding the bound is a detector error, so an enforcing profile fails closed.
const maxRegexMatchesPerPattern = 1000

// Detect reports every match of every pattern, skipping values an allowlist covers. It checks
// the caller's deadline between patterns and segments, and never materializes more matches than
// the per-pattern bound allows.
func (d *regexDetector) Detect(ctx context.Context, segments []string) ([]finding, error) {
	var found []finding
	for _, segment := range segments {
		for i := range d.patterns {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			pattern := &d.patterns[i]
			matches := pattern.re.FindAllStringIndex(segment, maxRegexMatchesPerPattern+1)
			if len(matches) > maxRegexMatchesPerPattern {
				return nil, fmt.Errorf("pattern %s matches more than %d times in one segment", pattern.entity, maxRegexMatchesPerPattern)
			}
			for _, loc := range matches {
				literal := segment[loc[0]:loc[1]]
				if literal == "" {
					continue
				}
				if len(pattern.allowed) > 0 {
					if _, ok := pattern.allowed[strings.ToLower(literal)]; ok {
						continue
					}
				}
				found = append(found, finding{
					entityType: pattern.entity,
					literal:    literal,
					action:     pattern.action,
					strategy:   pattern.strategy,
				})
			}
		}
	}
	return found, nil
}
