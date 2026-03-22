package lint

import (
	"fmt"
	"strings"

	"github.com/dpopsuev/origami/circuit"
)

// --- S21: edge-node-reference ---

// EdgeNodeReference checks that every edge from/to value references a declared
// node name or the circuit's done sentinel. Catches typos like "recal" for "recall".
type EdgeNodeReference struct{}

func (r *EdgeNodeReference) ID() string          { return "S21/edge-node-reference" }
func (r *EdgeNodeReference) Description() string { return "edge from/to must reference a declared node or the done sentinel" }
func (r *EdgeNodeReference) Severity() Severity  { return SeverityError }
func (r *EdgeNodeReference) Tags() []string      { return []string{"structural"} }

func (r *EdgeNodeReference) Check(ctx *LintContext) []Finding {
	if ctx.Def == nil {
		return nil
	}

	// Build set of valid node names.
	validNodes := make(map[string]bool, len(ctx.Def.Nodes)+2)
	for _, nd := range ctx.Def.Nodes {
		validNodes[nd.Name] = true
	}
	// The done sentinel is always a valid edge target.
	if ctx.Def.Done != "" {
		validNodes[ctx.Def.Done] = true
	}
	// The start node is always valid (usually declared, but be safe).
	if ctx.Def.Start != "" {
		validNodes[ctx.Def.Start] = true
	}

	var out []Finding
	for _, ed := range ctx.Def.Edges {
		if ed.From != "" && !validNodes[ed.From] {
			f := Finding{
				RuleID:   r.ID(),
				Severity: r.Severity(),
				Message:  fmt.Sprintf("edge %q: from=%q does not match any declared node", ed.ID, ed.From),
				File:     ctx.File,
				Line:     ctx.EdgeLine(ed.ID),
			}
			if s := nodeSuggestion(ed.From, validNodes); s != "" {
				f.Suggestion = s
			}
			out = append(out, f)
		}
		if ed.To != "" && !validNodes[ed.To] {
			f := Finding{
				RuleID:   r.ID(),
				Severity: r.Severity(),
				Message:  fmt.Sprintf("edge %q: to=%q does not match any declared node or done sentinel", ed.ID, ed.To),
				File:     ctx.File,
				Line:     ctx.EdgeLine(ed.ID),
			}
			if s := nodeSuggestion(ed.To, validNodes); s != "" {
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

func (r *HookReference) ID() string          { return "S22/hook-reference" }
func (r *HookReference) Description() string { return "hook references in before/after should be well-formed identifiers" }
func (r *HookReference) Severity() Severity  { return SeverityWarning }
func (r *HookReference) Tags() []string      { return []string{"structural"} }

func (r *HookReference) Check(ctx *LintContext) []Finding {
	if ctx.Def == nil {
		return nil
	}

	// Collect all distinct hook names used across the circuit.
	allHooks := make(map[string]bool)
	for _, nd := range ctx.Def.Nodes {
		for _, h := range nd.Before {
			allHooks[h] = true
		}
		for _, h := range nd.After {
			allHooks[h] = true
		}
	}

	var out []Finding
	for _, nd := range ctx.Def.Nodes {
		out = append(out, checkHookRefs(ctx, nd.Name, "before", nd.Before, allHooks)...)
		out = append(out, checkHookRefs(ctx, nd.Name, "after", nd.After, allHooks)...)
	}
	return out
}

func checkHookRefs(ctx *LintContext, nodeName, phase string, hooks []string, allHooks map[string]bool) []Finding {
	var out []Finding
	for _, hookName := range hooks {
		// Hooks should not contain spaces.
		if strings.ContainsAny(hookName, " \t") {
			out = append(out, Finding{
				RuleID:   "S22/hook-reference",
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
					RuleID:   "S22/hook-reference",
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
					RuleID:     "S22/hook-reference",
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

// --- S23: invalid-handler-type ---

// InvalidHandlerType checks that node-level and circuit-level handler_type
// values are recognized handler types.
type InvalidHandlerType struct{}

func (r *InvalidHandlerType) ID() string          { return "S23/invalid-handler-type" }
func (r *InvalidHandlerType) Description() string { return "handler_type must be a recognized type (transformer, extractor, renderer, node, delegate, circuit)" }
func (r *InvalidHandlerType) Severity() Severity  { return SeverityError }
func (r *InvalidHandlerType) Tags() []string      { return []string{"structural"} }

var validHandlerTypes = map[string]bool{
	circuit.HandlerTypeTransformer: true,
	circuit.HandlerTypeExtractor:   true,
	circuit.HandlerTypeRenderer:    true,
	circuit.HandlerTypeNode:        true,
	circuit.HandlerTypeDelegate:    true,
	circuit.HandlerTypeCircuit:     true,
}

func (r *InvalidHandlerType) Check(ctx *LintContext) []Finding {
	if ctx.Def == nil {
		return nil
	}

	var out []Finding

	// Check circuit-level handler_type.
	if ctx.Def.HandlerType != "" && !validHandlerTypes[ctx.Def.HandlerType] {
		out = append(out, Finding{
			RuleID:     r.ID(),
			Severity:   r.Severity(),
			Message:    fmt.Sprintf("circuit-level handler_type=%q is not a recognized type (valid: transformer, extractor, renderer, node, delegate, circuit)", ctx.Def.HandlerType),
			File:       ctx.File,
			Line:       ctx.TopLevelLine("handler_type"),
			Suggestion: handlerTypeSuggestion(ctx.Def.HandlerType),
		})
	}

	// Check node-level handler_type.
	for _, nd := range ctx.Def.Nodes {
		if nd.HandlerType != "" && !validHandlerTypes[nd.HandlerType] {
			out = append(out, Finding{
				RuleID:     r.ID(),
				Severity:   r.Severity(),
				Message:    fmt.Sprintf("node %q: handler_type=%q is not a recognized type (valid: transformer, extractor, renderer, node, delegate, circuit)", nd.Name, nd.HandlerType),
				File:       ctx.File,
				Line:       ctx.NodeLine(nd.Name),
				Suggestion: handlerTypeSuggestion(nd.HandlerType),
			})
		}
	}
	return out
}

func handlerTypeSuggestion(val string) string {
	best, bestDist := "", 100
	for ht := range validHandlerTypes {
		d := levenshtein(strings.ToLower(val), ht)
		if d < bestDist {
			bestDist = d
			best = ht
		}
	}
	if bestDist > 0 && bestDist <= 3 {
		return fmt.Sprintf("did you mean %q?", best)
	}
	return ""
}
