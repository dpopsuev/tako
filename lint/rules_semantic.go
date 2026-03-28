package lint

import (
	"fmt"
	"strings"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
)

const (
	ruleOrphanNode   = "G1/orphan-node"
	ruleUnreachDone  = "G2/unreachable-done"
	ruleFanInMerge   = "G7/fan-in-without-merge"
)

// --- G1: orphan-node ---

type OrphanNode struct{}

func (r *OrphanNode) ID() string          { return ruleOrphanNode }
func (r *OrphanNode) Description() string { return "node not reachable from start via any edge path" }
func (r *OrphanNode) Severity() Severity   { return SeverityWarning }
func (r *OrphanNode) Tags() []string       { return []string{"semantic"} }

func (r *OrphanNode) Check(ctx *LintContext) []Finding {
	reachable := reachableNodes(ctx.Def)
	var out []Finding
	for i := range ctx.Def.Nodes {
		if !reachable[ctx.Def.Nodes[i].Name] {
			out = append(out, Finding{
				RuleID:   r.ID(),
				Severity: r.Severity(),
				Message:  fmt.Sprintf("node %q is not reachable from start node %q", ctx.Def.Nodes[i].Name, ctx.Def.Start),
				File:     ctx.File,
				Line:     ctx.NodeLine(ctx.Def.Nodes[i].Name),
			})
		}
	}
	return out
}

// --- G2: unreachable-done ---

type UnreachableDone struct{}

func (r *UnreachableDone) ID() string          { return ruleUnreachDone }
func (r *UnreachableDone) Description() string { return "no edge path from start reaches done" }
func (r *UnreachableDone) Severity() Severity   { return SeverityError }
func (r *UnreachableDone) Tags() []string       { return []string{"semantic"} }

func (r *UnreachableDone) Check(ctx *LintContext) []Finding {
	if ctx.Def.Done == "" || ctx.Def.Start == "" {
		return nil
	}
	adj := buildAdjacency(ctx.Def)
	visited := bfs(ctx.Def.Start, adj)
	if !visited[ctx.Def.Done] {
		return []Finding{{
			RuleID:   r.ID(),
			Severity: r.Severity(),
			Message:  fmt.Sprintf("no path from start %q reaches done node %q", ctx.Def.Start, ctx.Def.Done),
			File:     ctx.File,
			Line:     ctx.TopLevelLine("done"),
		}}
	}
	return nil
}

// --- G3: dead-edge ---

type DeadEdge struct{}

func (r *DeadEdge) ID() string          { return "G3/dead-edge" }
func (r *DeadEdge) Description() string { return "edge from unreachable node is dead" }
func (r *DeadEdge) Severity() Severity   { return SeverityWarning }
func (r *DeadEdge) Tags() []string       { return []string{"semantic"} }

func (r *DeadEdge) Check(ctx *LintContext) []Finding {
	reachable := reachableNodes(ctx.Def)
	var out []Finding
	for i := range ctx.Def.Edges {
		if !reachable[ctx.Def.Edges[i].From] {
			out = append(out, Finding{
				RuleID:   r.ID(),
				Severity: r.Severity(),
				Message:  fmt.Sprintf("edge %q originates from unreachable node %q", ctx.Def.Edges[i].ID, ctx.Def.Edges[i].From),
				File:     ctx.File,
				Line:     ctx.EdgeLine(ctx.Def.Edges[i].ID),
			})
		}
	}
	return out
}

// --- G4: shortcut-bypasses-required ---

type ShortcutBypassesRequired struct{}

func (r *ShortcutBypassesRequired) ID() string        { return "G4/shortcut-bypasses-required" }
func (r *ShortcutBypassesRequired) Description() string { return "shortcut edge skips a node with a schema" }
func (r *ShortcutBypassesRequired) Severity() Severity { return SeverityWarning }
func (r *ShortcutBypassesRequired) Tags() []string     { return []string{"semantic"} }

func (r *ShortcutBypassesRequired) Check(ctx *LintContext) []Finding {
	schemaNodes := make(map[string]bool)
	for i := range ctx.Def.Nodes {
		if ctx.Def.Nodes[i].Schema != nil {
			schemaNodes[ctx.Def.Nodes[i].Name] = true
		}
	}
	if len(schemaNodes) == 0 {
		return nil
	}

	normalAdj := make(map[string][]string)
	for i := range ctx.Def.Edges {
		if !ctx.Def.Edges[i].Shortcut {
			normalAdj[ctx.Def.Edges[i].From] = append(normalAdj[ctx.Def.Edges[i].From], ctx.Def.Edges[i].To)
		}
	}

	var out []Finding
	for i := range ctx.Def.Edges {
		ed := &ctx.Def.Edges[i]
		if !ed.Shortcut {
			continue
		}
		skipped := nodesOnPath(ed.From, ed.To, normalAdj)
		for name := range skipped {
			if schemaNodes[name] {
				out = append(out, Finding{
					RuleID:   r.ID(),
					Severity: r.Severity(),
					Message:  fmt.Sprintf("shortcut edge %q bypasses schema-bearing node %q", ed.ID, name),
					File:     ctx.File,
					Line:     ctx.EdgeLine(ed.ID),
				})
			}
		}
	}
	return out
}

// --- G5: zone-approach-mismatch ---

type ZoneApproachMismatch struct{}

func (r *ZoneApproachMismatch) ID() string          { return "G5/zone-approach-mismatch" }
func (r *ZoneApproachMismatch) Description() string { return "zone approach differs from contained node approaches" }
func (r *ZoneApproachMismatch) Severity() Severity   { return SeverityInfo }
func (r *ZoneApproachMismatch) Tags() []string       { return []string{"semantic"} }

func (r *ZoneApproachMismatch) Check(ctx *LintContext) []Finding {
	nodeApproaches := make(map[string]string)
	for i := range ctx.Def.Nodes {
		if ctx.Def.Nodes[i].Approach != "" {
			nodeApproaches[ctx.Def.Nodes[i].Name] = strings.ToLower(ctx.Def.Nodes[i].Approach)
		}
	}

	var out []Finding
	for zoneName, z := range ctx.Def.Zones {
		if z.Approach == "" || len(z.Nodes) == 0 {
			continue
		}
		zoneApproach := strings.ToLower(z.Approach)

		anyMatch := false
		for _, nodeName := range z.Nodes {
			if na, ok := nodeApproaches[nodeName]; ok && na == zoneApproach {
				anyMatch = true
				break
			}
		}
		if !anyMatch {
			out = append(out, Finding{
				RuleID:   r.ID(),
				Severity: r.Severity(),
				Message:  fmt.Sprintf("zone %q declares approach %q but none of its nodes use that approach", zoneName, z.Approach),
				File:     ctx.File,
			})
		}
	}
	return out
}

// --- G6: expression-compile-error ---

type ExpressionCompileError struct{}

func (r *ExpressionCompileError) ID() string          { return "G6/expression-compile-error" }
func (r *ExpressionCompileError) Description() string { return "when expression does not compile" }
func (r *ExpressionCompileError) Severity() Severity   { return SeverityError }
func (r *ExpressionCompileError) Tags() []string       { return []string{"semantic"} }

func (r *ExpressionCompileError) Check(ctx *LintContext) []Finding {
	var out []Finding
	for i := range ctx.Def.Edges {
		if ctx.Def.Edges[i].When == "" {
			continue
		}
		if _, err := engine.CompileExpressionEdge(&ctx.Def.Edges[i], ctx.Def.Vars); err != nil {
			out = append(out, Finding{
				RuleID:   r.ID(),
				Severity: r.Severity(),
				Message:  fmt.Sprintf("edge %q: %v", ctx.Def.Edges[i].ID, err),
				File:     ctx.File,
				Line:     ctx.EdgeLine(ctx.Def.Edges[i].ID),
			})
		}
	}
	return out
}

// --- G7: fan-in-without-merge ---

type FanInWithoutMerge struct{}

func (r *FanInWithoutMerge) ID() string          { return ruleFanInMerge }
func (r *FanInWithoutMerge) Description() string { return "multiple edges converge on a node without merge strategy" }
func (r *FanInWithoutMerge) Severity() Severity   { return SeverityWarning }
func (r *FanInWithoutMerge) Tags() []string       { return []string{"semantic"} }

func (r *FanInWithoutMerge) Check(ctx *LintContext) []Finding {
	type edgeInfo struct {
		id          string
		conditional bool
	}
	inbound := make(map[string][]edgeInfo)
	for i := range ctx.Def.Edges {
		ed := &ctx.Def.Edges[i]
		conditional := ed.When != "" || ed.Condition != "" || ed.Shortcut || ed.Parallel
		inbound[ed.To] = append(inbound[ed.To], edgeInfo{id: ed.ID, conditional: conditional})
	}

	hasMerge := make(map[string]bool)
	for i := range ctx.Def.Edges {
		if ctx.Def.Edges[i].Merge != "" {
			hasMerge[ctx.Def.Edges[i].To] = true
		}
	}

	var out []Finding
	for node, edges := range inbound {
		if len(edges) <= 1 || hasMerge[node] || node == ctx.Def.Done {
			continue
		}
		// Only flag when at least two inbound edges could fire simultaneously
		// (unconditional edges). If all inbound edges are conditional/parallel/shortcut,
		// the fan-in is guarded by routing logic and merge is unnecessary.
		unconditional := 0
		for _, e := range edges {
			if !e.conditional {
				unconditional++
			}
		}
		if unconditional >= 2 {
			out = append(out, Finding{
				RuleID:   r.ID(),
				Severity: r.Severity(),
				Message:  fmt.Sprintf("node %q has %d unconditional inbound edges but no merge strategy", node, unconditional),
				File:     ctx.File,
				Line:     ctx.NodeLine(node),
			})
		}
	}
	return out
}

// --- G8: unacknowledged-shortcut ---

type UnacknowledgedShortcut struct{}

func (r *UnacknowledgedShortcut) ID() string          { return "G8/unacknowledged-shortcut" }
func (r *UnacknowledgedShortcut) Description() string { return "edge is topologically a shortcut but not declared as such" }
func (r *UnacknowledgedShortcut) Severity() Severity   { return SeverityWarning }
func (r *UnacknowledgedShortcut) Tags() []string       { return []string{"semantic"} }

func (r *UnacknowledgedShortcut) Check(ctx *LintContext) []Finding {
	inferred := inferEdgeTopology(ctx.Def)
	var out []Finding
	for i := range ctx.Def.Edges {
		orig := &ctx.Def.Edges[i]
		inf := inferred[i]
		if inf.Shortcut && !orig.Shortcut {
			out = append(out, Finding{
				RuleID:     r.ID(),
				Severity:   r.Severity(),
				Message:    fmt.Sprintf("edge %q (%s -> %s) is a topological shortcut but lacks shortcut: true", orig.ID, orig.From, orig.To),
				File:       ctx.File,
				Line:       ctx.EdgeLine(orig.ID),
				Suggestion: "add 'shortcut: true' to acknowledge this forward skip",
			})
		}
		if !inf.Shortcut && orig.Shortcut {
			out = append(out, Finding{
				RuleID:   r.ID(),
				Severity: SeverityError,
				Message:  fmt.Sprintf("edge %q (%s -> %s) declares shortcut: true but is not a topological shortcut", orig.ID, orig.From, orig.To),
				File:     ctx.File,
				Line:     ctx.EdgeLine(orig.ID),
			})
		}
	}
	return out
}

// --- G9: unacknowledged-loop ---

type UnacknowledgedLoop struct{}

func (r *UnacknowledgedLoop) ID() string          { return "G9/unacknowledged-loop" }
func (r *UnacknowledgedLoop) Description() string { return "edge is topologically a loop but not declared as such" }
func (r *UnacknowledgedLoop) Severity() Severity   { return SeverityWarning }
func (r *UnacknowledgedLoop) Tags() []string       { return []string{"semantic"} }

func (r *UnacknowledgedLoop) Check(ctx *LintContext) []Finding {
	inferred := inferEdgeTopology(ctx.Def)
	var out []Finding
	for i := range ctx.Def.Edges {
		orig := &ctx.Def.Edges[i]
		inf := inferred[i]
		if inf.Loop && !orig.Loop {
			out = append(out, Finding{
				RuleID:     r.ID(),
				Severity:   r.Severity(),
				Message:    fmt.Sprintf("edge %q (%s -> %s) is a topological loop but lacks loop: true", orig.ID, orig.From, orig.To),
				File:       ctx.File,
				Line:       ctx.EdgeLine(orig.ID),
				Suggestion: "add 'loop: true' to acknowledge this backward edge",
			})
		}
		if !inf.Loop && orig.Loop {
			out = append(out, Finding{
				RuleID:   r.ID(),
				Severity: SeverityError,
				Message:  fmt.Sprintf("edge %q (%s -> %s) declares loop: true but is not a topological loop", orig.ID, orig.From, orig.To),
				File:     ctx.File,
				Line:     ctx.EdgeLine(orig.ID),
			})
		}
	}
	return out
}

// inferEdgeTopology runs InferTopology on a copy of the circuit def
// and returns the inferred edge flags without mutating the original.
func inferEdgeTopology(def *circuit.CircuitDef) []circuit.EdgeDef {
	cp := *def
	cp.Edges = make([]circuit.EdgeDef, len(def.Edges))
	copy(cp.Edges, def.Edges)
	circuit.InferTopology(&cp)
	return cp.Edges
}

// --- Graph helpers ---

func buildAdjacency(def *circuit.CircuitDef) map[string][]string {
	adj := make(map[string][]string)
	for i := range def.Edges {
		adj[def.Edges[i].From] = append(adj[def.Edges[i].From], def.Edges[i].To)
	}
	return adj
}

func bfs(start string, adj map[string][]string) map[string]bool {
	visited := map[string]bool{start: true}
	queue := []string{start}
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		for _, next := range adj[curr] {
			if !visited[next] {
				visited[next] = true
				queue = append(queue, next)
			}
		}
	}
	return visited
}

func reachableNodes(def *circuit.CircuitDef) map[string]bool {
	if def.Start == "" {
		return nil
	}
	return bfs(def.Start, buildAdjacency(def))
}

// nodesOnPath returns nodes between from and to (exclusive of both) reachable
// via BFS through the adjacency map. Used by shortcut-bypasses-required.
func nodesOnPath(from, to string, adj map[string][]string) map[string]bool {
	visited := bfs(from, adj)
	skipped := make(map[string]bool)
	for node := range visited {
		if node != from && node != to {
			skipped[node] = true
		}
	}
	return skipped
}
