package core

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// ErrMatchEvaluatorRuleSet is returned when a named rule set is not found.
var ErrMatchEvaluatorRuleSet = errors.New("match evaluator: rule set")

// MatchRule defines a single pattern-matching rule. Conditions are ANDed:
// all specified conditions must be satisfied for the rule to match.
//
// Operators:
//   - all_of:     every string must be present
//   - any_of:     at least one string must be present
//   - none_of:    no string may be present
//   - not_all_of: at least one string must be absent (negation of all_of)
//   - regex:      every pattern must match (RE2 syntax, no lookaheads)
//   - none_regex: no pattern may match (RE2 syntax)
type MatchRule struct {
	AllOf     []string `yaml:"all_of,omitempty" json:"all_of,omitempty"`
	AnyOf     []string `yaml:"any_of,omitempty" json:"any_of,omitempty"`
	NoneOf    []string `yaml:"none_of,omitempty" json:"none_of,omitempty"`
	NotAllOf  []string `yaml:"not_all_of,omitempty" json:"not_all_of,omitempty"`
	Regex     []string `yaml:"regex,omitempty" json:"regex,omitempty"`
	NoneRegex []string `yaml:"none_regex,omitempty" json:"none_regex,omitempty"`
	Result    any      `yaml:"result" json:"result"`
}

// Matches returns true if all conditions are satisfied against text.
// An empty rule (no conditions) always matches.
func (r *MatchRule) Matches(text string) bool {
	if len(r.AllOf) > 0 {
		for _, s := range r.AllOf {
			if !strings.Contains(text, s) {
				return false
			}
		}
	}
	if len(r.AnyOf) > 0 {
		found := false
		for _, s := range r.AnyOf {
			if strings.Contains(text, s) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	for _, s := range r.NoneOf {
		if strings.Contains(text, s) {
			return false
		}
	}
	if len(r.NotAllOf) > 0 {
		allPresent := true
		for _, s := range r.NotAllOf {
			if !strings.Contains(text, s) {
				allPresent = false
				break
			}
		}
		if allPresent {
			return false
		}
	}
	for _, pat := range r.Regex {
		re, err := regexp.Compile(pat)
		if err != nil {
			return false
		}
		if !re.MatchString(text) {
			return false
		}
	}
	for _, pat := range r.NoneRegex {
		re, err := regexp.Compile(pat)
		if err != nil {
			continue
		}
		if re.MatchString(text) {
			return false
		}
	}
	return true
}

// MatchRuleSet is an ordered collection of rules evaluated first-match-wins.
type MatchRuleSet struct {
	Rules   []MatchRule `yaml:"rules" json:"rules"`
	Default any         `yaml:"default,omitempty" json:"default,omitempty"`
}

// Evaluate returns the result of the first matching rule. If no rule matches,
// returns Default and false.
func (rs *MatchRuleSet) Evaluate(text string) (any, bool) {
	for i := range rs.Rules {
		if rs.Rules[i].Matches(text) {
			return rs.Rules[i].Result, true
		}
	}
	return rs.Default, false
}

// EvaluateString is a convenience that returns the result as a string.
func (rs *MatchRuleSet) EvaluateString(text string) string {
	result, _ := rs.Evaluate(text)
	if s, ok := result.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", result)
}

// MatchEvaluator loads and caches named rule sets from a YAML document.
// The YAML is a map of rule-set names to MatchRuleSet definitions.
type MatchEvaluator struct {
	sets map[string]*MatchRuleSet
}

// NewMatchEvaluator parses YAML bytes into named rule sets.
// Top-level keys that do not contain a valid MatchRuleSet (e.g. plain
// lists or scalar values used for other purposes) are silently skipped.
func NewMatchEvaluator(yamlData []byte) (*MatchEvaluator, error) {
	var raw map[string]yaml.Node
	if err := yaml.Unmarshal(yamlData, &raw); err != nil {
		return nil, fmt.Errorf("match evaluator: parse YAML: %w", err)
	}
	sets := make(map[string]*MatchRuleSet, len(raw))
	for k, node := range raw { //nolint:gocritic // rangeValCopy: map value; unavoidable copy
		if node.Kind != yaml.MappingNode {
			continue
		}
		var rs MatchRuleSet
		if err := node.Decode(&rs); err != nil || len(rs.Rules) == 0 {
			continue
		}
		sets[k] = &rs
	}
	return &MatchEvaluator{sets: sets}, nil
}

// Get returns the named rule set, or an error if not found.
func (e *MatchEvaluator) Get(name string) (*MatchRuleSet, error) {
	rs, ok := e.sets[name]
	if !ok {
		return nil, fmt.Errorf("%w: %q not found", ErrMatchEvaluatorRuleSet, name)
	}
	return rs, nil
}

// Evaluate runs the named rule set against text. Returns the matching
// result, or the rule set's default.
func (e *MatchEvaluator) Evaluate(ruleSetName, text string) (result any, matched bool, err error) {
	rs, err := e.Get(ruleSetName)
	if err != nil {
		return nil, false, err
	}
	result, matched = rs.Evaluate(text)
	return result, matched, nil
}

// Names returns all available rule set names.
func (e *MatchEvaluator) Names() []string {
	out := make([]string, 0, len(e.sets))
	for k := range e.sets {
		out = append(out, k)
	}
	return out
}
