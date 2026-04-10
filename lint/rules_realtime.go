package lint

import (
	"fmt"
	"strings"
)

const (
	ruleEdgeNodeRef = "S21/edge-node-reference"
	ruleHookRef     = "S22/hook-reference"
)

// --- S21: edge-node-reference ---

// EdgeNodeReference checks that every edge from/to value references a declared
// node name or the circuit's done sentinel. Catches typos like "recal" for "recall".
type EdgeNodeReference struct{}

func (r *EdgeNodeReference) ID() string { return ruleEdgeNodeRef }
func (r *EdgeNodeReference) Description() string {
	return "edge from/to must reference a declared node or the done sentinel"
}
func (r *EdgeNodeReference) Severity() Severity { return SeverityError }
func (r *EdgeNodeReference) Tags() []string     { return []string{"structural"} }

func (r *EdgeNodeReference) Check(ctx *LintContext) []Finding {
	if ctx.Def == nil {
		return nil
	}

	// Build set of valid node names.
	validNodes := make(map[string]bool, len(ctx.Def.Nodes)+2)
	for i := range ctx.Def.Nodes {
		validNodes[string(ctx.Def.Nodes[i].Name)] = true
	}
	// The done sentinel is always a valid edge target.
	if ctx.Def.Done != "" {
		validNodes[string(ctx.Def.Done)] = true
	}
	// The start node is always valid (usually declared, but be safe).
	if ctx.Def.Start != "" {
		validNodes[string(ctx.Def.Start)] = true
	}

	var out []Finding
	for i := range ctx.Def.Edges {
		ed := &ctx.Def.Edges[i]
		if ed.From != "" && !validNodes[string(ed.From)] {
			f := Finding{
				RuleID:   r.ID(),
				Severity: r.Severity(),
				Message:  fmt.Sprintf("edge %q: from=%q does not match any declared node", ed.ID, string(ed.From)),
				File:     ctx.File,
				Line:     ctx.EdgeLine(ed.ID),
			}
			if s := nodeSuggestion(string(ed.From), validNodes); s != "" {
				f.Suggestion = s
			}
			out = append(out, f)
		}
		if ed.To != "" && !validNodes[string(ed.To)] {
			f := Finding{
				RuleID:   r.ID(),
				Severity: r.Severity(),
				Message:  fmt.Sprintf("edge %q: to=%q does not match any declared node or done sentinel", ed.ID, string(ed.To)),
				File:     ctx.File,
				Line:     ctx.EdgeLine(ed.ID),
			}
			if s := nodeSuggestion(string(ed.To), validNodes); s != "" {
				f.Suggestion = s
			}
			out = append(out, f)
		}
	}
	return out
}

func nodeSuggestion(val string, valid map[string]bool) string {
	best, bestDist := "", 100
	for name := range valid {
		d := levenshtein(strings.ToLower(val), strings.ToLower(name))
		if d < bestDist {
			bestDist = d
			best = name
		}
	}
	if bestDist > 0 && bestDist <= 3 {
		return fmt.Sprintf("did you mean %q?", best)
	}
	return ""
}

// --- S22: hook-reference ---

// HookReference checks that before/after hook references on nodes follow
// the expected naming convention. Hooks are registered at runtime, so this
// rule uses a heuristic: names should be non-empty, should not contain
// spaces, and when a dot-qualified name is used (e.g. "inject.failure"),
// the segments should be non-empty. Also applies levenshtein distance
// checking against other hook references in the same circuit for likely typos.
type HookReference struct{}

func (r *HookReference) ID() string { return ruleHookRef }
func (r *HookReference) Description() string {
	return "hook references in before/after should be well-formed identifiers"
}
func (r *HookReference) Severity() Severity { return SeverityWarning }
func (r *HookReference) Tags() []string     { return []string{"structural"} }

func (r *HookReference) Check(ctx *LintContext) []Finding {
	if ctx.Def == nil {
		return nil
	}

	// Collect all distinct hook names used across the circuit.
	allHooks := make(map[string]bool)
	for i := range ctx.Def.Nodes {
		for _, h := range ctx.Def.Nodes[i].Before {
			allHooks[h] = true
		}
		for _, h := range ctx.Def.Nodes[i].After {
			allHooks[h] = true
		}
	}

	var out []Finding
	for i := range ctx.Def.Nodes {
		nd := &ctx.Def.Nodes[i]
		out = append(out, checkHookRefs(ctx, string(nd.Name), "before", nd.Before, allHooks)...)
		out = append(out, checkHookRefs(ctx, string(nd.Name), "after", nd.After, allHooks)...)
	}
	return out
}

func checkHookRefs(ctx *LintContext, nodeName, phase string, hooks []string, allHooks map[string]bool) []Finding {
	var out []Finding
	for _, hookName := range hooks {
		// Hooks should not contain spaces.
		if strings.ContainsAny(hookName, " \t") {
			out = append(out, Finding{
				RuleID:   ruleHookRef,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("node %q: %s hook %q contains whitespace; hook names should be identifiers", nodeName, phase, hookName),
				File:     ctx.File,
				Line:     ctx.NodeLine(nodeName),
			})
			continue
		}
		// Dot-qualified names should have non-empty segments.
		if strings.Contains(hookName, ".") {
			parts := strings.Split(hookName, ".")
			emptySegment := false
			for _, p := range parts {
				if p == "" {
					emptySegment = true
					break
				}
			}
			if emptySegment {
				out = append(out, Finding{
					RuleID:   ruleHookRef,
					Severity: SeverityWarning,
					Message:  fmt.Sprintf("node %q: %s hook %q has empty segment in dot-qualified name", nodeName, phase, hookName),
					File:     ctx.File,
					Line:     ctx.NodeLine(nodeName),
				})
				continue
			}
		}
		// Check for likely duplicates/typos: if this hook is close to but
		// not identical to another hook reference in the circuit, warn.
		for other := range allHooks {
			if other == hookName {
				continue
			}
			d := levenshtein(hookName, other)
			if d > 0 && d <= 2 {
				out = append(out, Finding{
					RuleID:     ruleHookRef,
					Severity:   SeverityWarning,
					Message:    fmt.Sprintf("node %q: %s hook %q is similar to %q (edit distance %d); possible typo?", nodeName, phase, hookName, other, d),
					File:       ctx.File,
					Line:       ctx.NodeLine(nodeName),
					Suggestion: fmt.Sprintf("did you mean %q?", other),
				})
			}
		}
	}
	return out
}

