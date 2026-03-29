package lint

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

const kindCircuit = "circuit"

// CrossRefRule declares a cross-file symbol validation rule.
// File A exports names at a YAML path; file B references them.
// The engine validates that references resolve to exports.
type CrossRefRule struct {
	RuleID      string
	Desc        string
	RuleSev     Severity
	ExportKind  string // YAML kind: field of the exporting file (e.g., "circuit")
	ExportPath  string // dotted path to extract exports (e.g., "calibration.outputs[].scorer_name")
	RefKind     string // YAML kind: field of the referencing file
	RefPaths    string // comma-separated dotted paths to extract references
	CheckType   string // "refs_subset_of_exports" or "bidirectional"
	ExportLabel string // human label for the export set (e.g., "calibration contract")
	RefLabel    string // human label for the reference set (e.g., "scorecard params")
	ExcludeKind string // optional: kind of file containing exclusion list
	ExcludePath string // optional: path to values excluded from reference checking (adapter-provided fields)
}

// CrossRefEngine implements the Rule interface by processing N declarative
// CrossRefRules against project-level YAML files. It extracts values at
// dotted paths from files indexed by kind, then checks that references
// resolve to exports.
type CrossRefEngine struct {
	Rules []CrossRefRule
}

func (e *CrossRefEngine) ID() string          { return "S30+/cross-ref-engine" }
func (e *CrossRefEngine) Description() string { return "declarative cross-file symbol validation" }
func (e *CrossRefEngine) Severity() Severity  { return SeverityError }
func (e *CrossRefEngine) Tags() []string      { return []string{"structural", "crossref"} }

// Check runs all cross-ref rules against the project files in ctx.
func (e *CrossRefEngine) Check(ctx *LintContext) []Finding {
	if len(ctx.ProjectFiles) == 0 {
		return nil
	}

	var findings []Finding
	for i := range e.Rules {
		findings = append(findings, e.checkRule(&e.Rules[i], ctx)...)
	}
	return findings
}

//nolint:gocyclo // cross-file validation with multiple check types
func (e *CrossRefEngine) checkRule(rule *CrossRefRule, ctx *LintContext) []Finding {
	// Extract exports from all files matching ExportKind.
	exports := make(map[string]bool)
	for _, doc := range ctx.ProjectFiles[rule.ExportKind] {
		for _, v := range ExtractPath(doc.Data, rule.ExportPath) {
			exports[fmt.Sprint(v)] = true
		}
	}
	if len(exports) == 0 {
		return nil // no exports found — rule doesn't apply
	}

	// Extract exclusions (adapter-provided fields that should not be checked).
	excluded := make(map[string]bool)
	if rule.ExcludePath != "" {
		excludeKind := rule.ExcludeKind
		if excludeKind == "" {
			excludeKind = rule.ExportKind
		}
		for _, doc := range ctx.ProjectFiles[excludeKind] {
			for _, v := range ExtractPath(doc.Data, rule.ExcludePath) {
				excluded[fmt.Sprint(v)] = true
			}
		}
	}

	// Extract references from all files matching RefKind.
	type ref struct {
		value string
		file  string
	}
	var refs []ref
	for _, refPath := range strings.Split(rule.RefPaths, ",") {
		refPath = strings.TrimSpace(refPath)
		for _, doc := range ctx.ProjectFiles[rule.RefKind] {
			for _, v := range ExtractPath(doc.Data, refPath) {
				refs = append(refs, ref{value: fmt.Sprint(v), file: doc.File})
			}
		}
	}

	// Run check.
	var findings []Finding
	switch rule.CheckType {
	case "refs_subset_of_exports":
		for _, r := range refs {
			if !exports[r.value] && !excluded[r.value] {
				findings = append(findings, Finding{
					RuleID:   rule.RuleID,
					Severity: rule.RuleSev,
					Message: fmt.Sprintf("unresolved %s reference %q in %s; available %s: %s",
						rule.RefLabel, r.value, r.file, rule.ExportLabel, sortedKeys(exports)),
					File:       r.file,
					Suggestion: closestMatch(r.value, exports),
				})
			}
		}
	case "bidirectional":
		refSet := make(map[string]string) // value → file
		for _, r := range refs {
			refSet[r.value] = r.file
		}
		// Check exports referenced.
		for exp := range exports {
			if _, ok := refSet[exp]; !ok {
				findings = append(findings, Finding{
					RuleID:   rule.RuleID,
					Severity: rule.RuleSev,
					Message:  fmt.Sprintf("%s %q declared but not wired", rule.ExportLabel, exp),
					File:     ctx.File,
				})
			}
		}
		// Check references resolve.
		for _, r := range refs {
			if !exports[r.value] {
				findings = append(findings, Finding{
					RuleID:   rule.RuleID,
					Severity: rule.RuleSev,
					Message: fmt.Sprintf("wiring references undeclared %s %q; available: %s",
						rule.ExportLabel, r.value, sortedKeys(exports)),
					File: r.file,
				})
			}
		}
	}
	return findings
}

// ProjectFile is a parsed YAML file indexed by kind for cross-file validation.
type ProjectFile struct {
	File string
	Kind string
	Data map[string]any
}

// ExtractPath extracts string values from a parsed YAML document at a dotted
// path. Supports [] for array iteration and * for map key/value wildcard.
//
// Examples:
//
//	"calibration.outputs[].scorer_name" → all scorer_name values
//	"metrics[].params.*"               → all values from all params maps
//	"repos[].name"                     → all repo names
func ExtractPath(doc map[string]any, path string) []any {
	segments := splitPath(path)
	return extractSegments([]any{doc}, segments)
}

func extractSegments(current []any, segments []string) []any {
	if len(segments) == 0 {
		return current
	}

	seg := segments[0]
	rest := segments[1:]

	var next []any
	for _, v := range current {
		switch seg {
		case "[]":
			// Iterate array elements.
			if arr, ok := toSlice(v); ok {
				next = append(next, arr...)
			}
		case "*":
			// Wildcard: collect all values from map.
			if m, ok := v.(map[string]any); ok {
				for _, val := range m {
					next = append(next, val)
				}
			}
		default:
			// Key lookup in map.
			if m, ok := v.(map[string]any); ok {
				if val, exists := m[seg]; exists {
					next = append(next, val)
				}
			}
		}
	}
	return extractSegments(next, rest)
}

func splitPath(path string) []string {
	// Split on "." but keep "[]" as a single segment.
	var segments []string
	for _, part := range strings.Split(path, ".") {
		if strings.HasSuffix(part, "[]") {
			segments = append(segments, strings.TrimSuffix(part, "[]"), "[]")
		} else {
			segments = append(segments, part)
		}
	}
	return segments
}

func toSlice(v any) ([]any, bool) {
	switch s := v.(type) {
	case []any:
		return s, true
	case []map[string]any:
		out := make([]any, len(s))
		for i, m := range s {
			out[i] = m
		}
		return out, true
	}
	return nil, false
}

func sortedKeys(m map[string]bool) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		if strings.HasPrefix(k, "_") {
			continue // skip internal fields like _path
		}
		keys = append(keys, k)
	}
	if len(keys) == 0 {
		return "(none)"
	}
	// Simple sort for readability in error messages.
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return "[" + strings.Join(keys, ", ") + "]"
}

func closestMatch(value string, candidates map[string]bool) string {
	best, bestDist := "", 100
	for c := range candidates {
		if strings.HasPrefix(c, "_") {
			continue
		}
		d := levenshtein(strings.ToLower(value), strings.ToLower(c))
		if d < bestDist {
			bestDist = d
			best = c
		}
	}
	if bestDist <= 3 && best != "" {
		return fmt.Sprintf("did you mean %q?", best)
	}
	return ""
}

// LoadProjectFiles parses all YAML files and indexes them by kind.
func LoadProjectFiles(files map[string][]byte) map[string][]ProjectFile {
	index := make(map[string][]ProjectFile)
	for file, raw := range files {
		var doc map[string]any
		if err := yaml.Unmarshal(raw, &doc); err != nil {
			continue
		}
		kind, _ := doc["kind"].(string)
		if kind == "" {
			// Infer kind from common fields.
			if _, ok := doc[kindCircuit]; ok {
				kind = kindCircuit
			} else if _, ok := doc["scorecard"]; ok {
				kind = "scorecard"
			} else if _, ok := doc["rcas"]; ok {
				kind = "scenario"
			}
		}
		if kind == "" {
			continue
		}
		index[kind] = append(index[kind], ProjectFile{
			File: file,
			Kind: kind,
			Data: doc,
		})
	}
	return index
}
