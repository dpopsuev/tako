package lint

import (
	"fmt"
	"strings"
)

// ApplyFixes runs all fixable rules and returns both the modified YAML bytes
// and the fixes that were applied. The input raw YAML is not mutated.
func ApplyFixes(raw []byte, file string, opts ...Option) ([]byte, []Fix, error) {
	ctx, err := NewLintContext(raw, file)
	if err != nil {
		return nil, nil, err
	}

	cfg := runConfig{profile: ProfileModerate}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.registries != nil {
		ctx.Registries = cfg.registries
	}

	maxSev := cfg.profile.maxSeverity()
	var fixes []Fix
	for _, rule := range AllRules() {
		if rule.Severity() > maxSev {
			continue
		}
		if fixable, ok := rule.(Fixable); ok {
			fixes = append(fixes, fixable.Fix(ctx)...)
		}
	}

	if len(fixes) == 0 {
		return nil, nil, nil
	}

	result := applyLineReplacements(raw, fixes)
	return result, fixes, nil
}

// applyLineReplacements applies line-based replacements to raw YAML.
// Fixes are applied bottom-up to preserve line numbers.
func applyLineReplacements(raw []byte, fixes []Fix) []byte {
	lines := strings.Split(string(raw), "\n")

	// Sort fixes by StartLine descending so we apply from bottom up
	sorted := make([]Fix, len(fixes))
	copy(sorted, fixes)
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].StartLine > sorted[i].StartLine {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	for i := range sorted {
		fix := &sorted[i]
		if fix.StartLine < 1 || fix.StartLine > len(lines) {
			continue
		}
		start := fix.StartLine - 1
		end := fix.EndLine - 1
		if end < start {
			end = start
		}
		if end >= len(lines) {
			end = len(lines) - 1
		}

		replacement := strings.Split(fix.Replacement, "\n")
		newLines := make([]string, 0, len(lines)-((end-start)+1)+len(replacement))
		newLines = append(newLines, lines[:start]...)
		newLines = append(newLines, replacement...)
		newLines = append(newLines, lines[end+1:]...)
		lines = newLines
	}

	return []byte(strings.Join(lines, "\n"))
}

// --- Fixable rule implementations ---

// Fix for S2/invalid-approach: replace with closest valid approach via fuzzy match.
func (r *InvalidApproach) Fix(ctx *LintContext) []Fix {
	fixes := make([]Fix, 0, len(ctx.Def.Nodes))
	for i := range ctx.Def.Nodes {
		nd := &ctx.Def.Nodes[i]
		if nd.Approach == "" || validApproaches[strings.ToLower(nd.Approach)] {
			continue
		}
		suggestion := closestApproach(nd.Approach)
		if suggestion == "" {
			continue
		}
		line := ctx.NodeLine(string(nd.Name))
		if line == 0 {
			continue
		}
		fixLine := findFieldLine(ctx.Raw, line, "approach:")
		if fixLine == 0 {
			continue
		}
		fixes = append(fixes, Fix{
			Finding: Finding{
				RuleID:   r.ID(),
				Severity: r.Severity(),
				Message:  fmt.Sprintf("node %q: fix approach %q → %q", nd.Name, nd.Approach, suggestion),
				File:     ctx.File,
				Line:     fixLine,
			},
			Replacement: replaceFieldValue(ctx.Raw, fixLine, suggestion),
			StartLine:   fixLine,
			EndLine:     fixLine,
		})
	}
	return fixes
}

// Fix for B1/prefer-when-over-condition: rename condition → when.
func (r *PreferWhenOverCondition) Fix(ctx *LintContext) []Fix {
	fixes := make([]Fix, 0, len(ctx.Def.Edges))
	for i := range ctx.Def.Edges {
		ed := &ctx.Def.Edges[i]
		if ed.Condition == "" || ed.When != "" {
			continue
		}
		line := ctx.EdgeLine(ed.ID)
		if line == 0 {
			continue
		}
		fixLine := findFieldLine(ctx.Raw, line, "condition:")
		if fixLine == 0 {
			continue
		}
		fixes = append(fixes, Fix{
			Finding: Finding{
				RuleID:   r.ID(),
				Severity: r.Severity(),
				Message:  fmt.Sprintf("edge %q: rename condition → when", ed.ID),
				File:     ctx.File,
				Line:     fixLine,
			},
			Replacement: replaceFieldKey(ctx.Raw, fixLine, "condition:", "when:"),
			StartLine:   fixLine,
			EndLine:     fixLine,
		})
	}
	return fixes
}

// Fix for S4/missing-edge-name: generate name from "from → to".
func (r *MissingEdgeName) Fix(ctx *LintContext) []Fix {
	fixes := make([]Fix, 0, len(ctx.Def.Edges))
	for i := range ctx.Def.Edges {
		ed := &ctx.Def.Edges[i]
		if ed.Name != "" {
			continue
		}
		line := ctx.EdgeLine(ed.ID)
		if line == 0 {
			continue
		}
		name := fmt.Sprintf("%s to %s", ed.From, ed.To)
		idLine := findFieldLine(ctx.Raw, line, "id:")
		if idLine == 0 {
			continue
		}
		rawLines := strings.Split(string(ctx.Raw), "\n")
		if idLine-1 >= len(rawLines) {
			continue
		}
		indent := leadingWhitespace(rawLines[idLine-1])
		insertion := rawLines[idLine-1] + "\n" + indent + "name: " + name
		fixes = append(fixes, Fix{
			Finding: Finding{
				RuleID:   r.ID(),
				Severity: r.Severity(),
				Message:  fmt.Sprintf("edge %q: add name %q", ed.ID, name),
				File:     ctx.File,
				Line:     idLine,
			},
			Replacement: insertion,
			StartLine:   idLine,
			EndLine:     idLine,
		})
	}
	return fixes
}

// Fix for S8/missing-circuit-description: insert description placeholder.
func (r *MissingCircuitDescription) Fix(ctx *LintContext) []Fix {
	if ctx.Def.Description != "" {
		return nil
	}
	line := ctx.TopLevelLine("circuit")
	if line == 0 {
		return nil
	}
	rawLines := strings.Split(string(ctx.Raw), "\n")
	if line-1 >= len(rawLines) {
		return nil
	}
	insertion := rawLines[line-1] + "\ndescription: \"\""
	return []Fix{{
		Finding: Finding{
			RuleID:   r.ID(),
			Severity: r.Severity(),
			Message:  "add description placeholder",
			File:     ctx.File,
			Line:     line,
		},
		Replacement: insertion,
		StartLine:   line,
		EndLine:     line,
	}}
}

// --- Fix helpers ---

func closestApproach(val string) string {
	best, bestDist := "", 100
	lower := strings.ToLower(val)
	for e := range validApproaches {
		d := levenshtein(lower, e)
		if d < bestDist {
			bestDist = d
			best = e
		}
	}
	if bestDist <= 3 {
		return best
	}
	return ""
}

func findFieldLine(raw []byte, startLine int, field string) int {
	lines := strings.Split(string(raw), "\n")
	for i := startLine - 1; i < len(lines) && i < startLine+20; i++ {
		if strings.Contains(strings.TrimSpace(lines[i]), field) {
			return i + 1
		}
	}
	return 0
}

func replaceFieldValue(raw []byte, line int, newValue string) string {
	lines := strings.Split(string(raw), "\n")
	if line < 1 || line > len(lines) {
		return ""
	}
	l := lines[line-1]
	colonIdx := strings.Index(l, ":")
	if colonIdx < 0 {
		return l
	}
	return l[:colonIdx+1] + " " + newValue
}

func replaceFieldKey(raw []byte, line int, oldKey, newKey string) string {
	lines := strings.Split(string(raw), "\n")
	if line < 1 || line > len(lines) {
		return ""
	}
	return strings.Replace(lines[line-1], oldKey, newKey, 1)
}

func leadingWhitespace(s string) string {
	for i, c := range s {
		if c != ' ' && c != '\t' {
			return s[:i]
		}
	}
	return s
}
