package lint

import (
	"fmt"
	"strings"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
	"gopkg.in/yaml.v3"
)

const (
	rulePreferWhen      = "B1/prefer-when-over-condition"
	ruleStochastic      = "B7/stochastic-transformer"
	ruleStochasticSumm  = "B7s/stochastic-summary"
	ruleMissingKind     = "B9/missing-kind"
	ruleDeprecatedArrow = "B10/deprecated-fk-arrow"
	yamlFieldKind       = "kind"
)

// --- B1: prefer-when-over-condition ---

type PreferWhenOverCondition struct{}

func (r *PreferWhenOverCondition) ID() string { return rulePreferWhen }
func (r *PreferWhenOverCondition) Description() string {
	return "use when instead of deprecated condition field"
}
func (r *PreferWhenOverCondition) Severity() Severity { return SeverityWarning }
func (r *PreferWhenOverCondition) Tags() []string     { return []string{"best-practice"} }

func (r *PreferWhenOverCondition) Check(ctx *LintContext) []Finding {
	var out []Finding
	for i := range ctx.Def.Edges {
		ed := &ctx.Def.Edges[i]
		if ed.Condition != "" && ed.When == "" && looksLikeExpression(ed.Condition) {
			out = append(out, Finding{
				RuleID:       r.ID(),
				Severity:     r.Severity(),
				Message:      fmt.Sprintf("edge %q: condition %q looks like an expression; use when for evaluated conditions", ed.ID, ed.Condition),
				File:         ctx.File,
				Line:         ctx.EdgeLine(ed.ID),
				FixAvailable: true,
			})
		}
	}
	return out
}

// looksLikeExpression returns true if the string contains operators or
// patterns typical of expr-lang expressions rather than human comments.
// looksLikeExpression returns true only when the string contains tokens
// that are clearly programmatic (expr-lang references or boolean operators).
// Comparison operators like ==, >= are excluded because they commonly appear
// in human-readable descriptions ("confidence >= 0.9", "verdict == affirm").
func looksLikeExpression(s string) bool {
	for _, op := range []string{"&&", "||", "output.", "state.", "config."} {
		if strings.Contains(s, op) {
			return true
		}
	}
	return false
}

// --- B2: name-your-edges ---

type NameYourEdges struct{}

func (r *NameYourEdges) ID() string { return "B2/name-your-edges" }
func (r *NameYourEdges) Description() string {
	return "circuits with many edges should name them for readability"
}
func (r *NameYourEdges) Severity() Severity { return SeverityInfo }
func (r *NameYourEdges) Tags() []string     { return []string{"best-practice"} }

func (r *NameYourEdges) Check(ctx *LintContext) []Finding {
	unnamed := 0
	for i := range ctx.Def.Edges {
		if ctx.Def.Edges[i].Name == "" {
			unnamed++
		}
	}
	if unnamed > 3 {
		return []Finding{{
			RuleID:   r.ID(),
			Severity: r.Severity(),
			Message:  fmt.Sprintf("circuit has %d unnamed edges; consider adding name fields for readability", unnamed),
			File:     ctx.File,
			Line:     ctx.TopLevelLine("edges"),
		}}
	}
	return nil
}

// --- B3: terminal-edge-to-done ---

type TerminalEdgeToDone struct{}

func (r *TerminalEdgeToDone) ID() string { return "B3/terminal-edge-to-done" }
func (r *TerminalEdgeToDone) Description() string {
	return "terminal nodes should have an edge to done"
}
func (r *TerminalEdgeToDone) Severity() Severity { return SeverityWarning }
func (r *TerminalEdgeToDone) Tags() []string     { return []string{"best-practice"} }

func (r *TerminalEdgeToDone) Check(ctx *LintContext) []Finding {
	hasOutgoing := make(map[string]bool)
	for i := range ctx.Def.Edges {
		hasOutgoing[string(ctx.Def.Edges[i].From)] = true
	}

	var out []Finding
	for i := range ctx.Def.Nodes {
		nd := &ctx.Def.Nodes[i]
		if !hasOutgoing[string(nd.Name)] {
			out = append(out, Finding{
				RuleID:   r.ID(),
				Severity: r.Severity(),
				Message:  fmt.Sprintf("node %q has no outgoing edges; add an edge to %q", nd.Name, ctx.Def.Done),
				File:     ctx.File,
				Line:     ctx.NodeLine(string(nd.Name)),
			})
		}
	}
	return out
}

// --- B4: zone-stickiness-without-provider ---

type ZoneStickinessWithoutProvider struct{}

func (r *ZoneStickinessWithoutProvider) ID() string { return "B4/zone-stickiness-without-provider" }
func (r *ZoneStickinessWithoutProvider) Description() string {
	return "zone with stickiness but no providers in its nodes"
}
func (r *ZoneStickinessWithoutProvider) Severity() Severity { return SeverityInfo }
func (r *ZoneStickinessWithoutProvider) Tags() []string     { return []string{"best-practice"} }

func (r *ZoneStickinessWithoutProvider) Check(ctx *LintContext) []Finding {
	// Providers are often runtime-configured and stickiness is valid
	// as declarative intent even when YAML-level provider fields are absent.
	// Only flag when stickiness is "exclusive" (3) and zero providers exist
	// anywhere in the circuit, suggesting a configuration oversight.
	anyProvider := false
	for i := range ctx.Def.Nodes {
		if ctx.Def.Nodes[i].Provider != "" {
			anyProvider = true
			break
		}
	}
	if anyProvider {
		return nil
	}

	out := make([]Finding, 0, len(ctx.Def.Zones))
	for zoneName, z := range ctx.Def.Zones {
		if z.Stickiness < 3 {
			continue
		}
		out = append(out, Finding{
			RuleID:   r.ID(),
			Severity: r.Severity(),
			Message:  fmt.Sprintf("zone %q has exclusive stickiness=%d but no nodes in the entire circuit declare a provider", zoneName, z.Stickiness),
			File:     ctx.File,
		})
	}
	return out
}

// --- B5: large-circuit-no-zones ---

type LargeCircuitNoZones struct{}

func (r *LargeCircuitNoZones) ID() string { return "B5/large-circuit-no-zones" }
func (r *LargeCircuitNoZones) Description() string {
	return "large circuits should define zones for organization"
}
func (r *LargeCircuitNoZones) Severity() Severity { return SeverityInfo }
func (r *LargeCircuitNoZones) Tags() []string     { return []string{"best-practice"} }

func (r *LargeCircuitNoZones) Check(ctx *LintContext) []Finding {
	if len(ctx.Def.Nodes) > 6 && len(ctx.Def.Zones) == 0 {
		return []Finding{{
			RuleID:   r.ID(),
			Severity: r.Severity(),
			Message:  fmt.Sprintf("circuit has %d nodes but no zones; consider adding zones for organization", len(ctx.Def.Nodes)),
			File:     ctx.File,
			Line:     ctx.TopLevelLine("nodes"),
		}}
	}
	return nil
}

// --- B6: approach-affinity-chain ---

type ApproachAffinityChain struct{}

func (r *ApproachAffinityChain) ID() string { return "B6/approach-affinity-chain" }
func (r *ApproachAffinityChain) Description() string {
	return "three or more consecutive nodes with the same approach"
}
func (r *ApproachAffinityChain) Severity() Severity { return SeverityInfo }
func (r *ApproachAffinityChain) Tags() []string     { return []string{"best-practice"} }

func (r *ApproachAffinityChain) Check(ctx *LintContext) []Finding {
	nodeApproaches := make(map[string]string)
	for i := range ctx.Def.Nodes {
		if ctx.Def.Nodes[i].Approach != "" {
			nodeApproaches[string(ctx.Def.Nodes[i].Name)] = strings.ToLower(ctx.Def.Nodes[i].Approach)
		}
	}

	adj := make(map[string][]string)
	for i := range ctx.Def.Edges {
		if !ctx.Def.Edges[i].Shortcut && !ctx.Def.Edges[i].Loop {
			adj[string(ctx.Def.Edges[i].From)] = append(adj[string(ctx.Def.Edges[i].From)], string(ctx.Def.Edges[i].To))
		}
	}

	var out []Finding
	reported := make(map[string]bool)
	for i := range ctx.Def.Nodes {
		nd := &ctx.Def.Nodes[i]
		approach := nodeApproaches[string(nd.Name)]
		if approach == "" {
			continue
		}
		chain := findApproachChain(string(nd.Name), approach, nodeApproaches, adj)
		if len(chain) >= 3 && !reported[approach+":"+chain[0]] {
			reported[approach+":"+chain[0]] = true
			out = append(out, Finding{
				RuleID:   r.ID(),
				Severity: r.Severity(),
				Message:  fmt.Sprintf("%d consecutive %s nodes: %s; consider varying approaches for balance", len(chain), approach, strings.Join(chain, " → ")),
				File:     ctx.File,
				Line:     ctx.NodeLine(chain[0]),
			})
		}
	}
	return out
}

func findApproachChain(start, approach string, nodeApproaches map[string]string, adj map[string][]string) []string {
	chain := []string{start}
	curr := start
	visited := map[string]bool{start: true}
	for {
		nexts := adj[curr]
		extended := false
		for _, next := range nexts {
			if visited[next] || nodeApproaches[next] != approach {
				continue
			}
			chain = append(chain, next)
			visited[next] = true
			curr = next
			extended = true
			break
		}
		if !extended {
			break
		}
	}
	return chain
}

// --- B7: stochastic-transformer ---

// knownStochasticTransformers is the fallback list used when no
// TransformerRegistry is available at lint time (static YAML analysis).
var knownStochasticTransformers = map[string]bool{
	"core.llm": true,
	"llm":      true,
}

type StochasticTransformer struct{}

func (r *StochasticTransformer) ID() string { return ruleStochastic }
func (r *StochasticTransformer) Description() string {
	return "node uses a stochastic (non-deterministic) transformer"
}
func (r *StochasticTransformer) Severity() Severity { return SeverityInfo }
func (r *StochasticTransformer) Tags() []string     { return []string{"best-practice", "determinism"} }

func (r *StochasticTransformer) Check(ctx *LintContext) []Finding {
	var reg engine.TransformerRegistry
	if ctx.Registries != nil {
		reg = ctx.Registries.Transformers
	}

	var out []Finding
	for i := range ctx.Def.Nodes {
		nd := &ctx.Def.Nodes[i]
		if nd.Instrument != "" && nd.Instrument != "transformer" {
			continue
		}
		name := nd.Action
		if name == "" {
			name = string(nd.Name)
		}
		if name != "" && isStochastic(name, reg) {
			out = append(out, Finding{
				RuleID:   r.ID(),
				Severity: r.Severity(),
				Message:  fmt.Sprintf("node %q uses stochastic transformer %q", nd.Name, name),
				File:     ctx.File,
				Line:     ctx.NodeLine(string(nd.Name)),
			})
		}
	}
	return out
}

func isStochastic(name string, reg engine.TransformerRegistry) bool {
	if reg != nil {
		if t, err := reg.Get(name); err == nil {
			return !engine.IsDeterministic(t)
		}
	}
	return knownStochasticTransformers[name]
}

// --- B7s: stochastic-summary ---

type StochasticSummary struct{}

func (r *StochasticSummary) ID() string { return ruleStochasticSumm }
func (r *StochasticSummary) Description() string {
	return "aggregate count of stochastic nodes in circuit"
}
func (r *StochasticSummary) Severity() Severity { return SeverityInfo }
func (r *StochasticSummary) Tags() []string     { return []string{"best-practice", "determinism"} }

func (r *StochasticSummary) Check(ctx *LintContext) []Finding {
	var reg engine.TransformerRegistry
	if ctx.Registries != nil {
		reg = ctx.Registries.Transformers
	}

	totalWithTransformer := 0
	var stochasticNames []string
	for i := range ctx.Def.Nodes {
		nd := &ctx.Def.Nodes[i]
		if nd.Instrument != "" && nd.Instrument != "transformer" {
			continue
		}
		name := nd.Action
		if name == "" {
			name = string(nd.Name)
		}
		if name == "" {
			continue
		}
		totalWithTransformer++
		if isStochastic(name, reg) {
			stochasticNames = append(stochasticNames, string(nd.Name))
		}
	}

	if len(stochasticNames) == 0 {
		return nil
	}

	return []Finding{{
		RuleID:   r.ID(),
		Severity: r.Severity(),
		Message:  fmt.Sprintf("circuit has %d stochastic node(s) out of %d transformer-bound: %s", len(stochasticNames), totalWithTransformer, strings.Join(stochasticNames, ", ")),
		File:     ctx.File,
	}}
}

// --- B9: missing-kind ---

type MissingKind struct{}

func (r *MissingKind) ID() string          { return ruleMissingKind }
func (r *MissingKind) Description() string { return "YAML file should have a top-level kind field" }
func (r *MissingKind) Severity() Severity  { return SeverityWarning }
func (r *MissingKind) Tags() []string      { return []string{"best-practice", "envelope"} }

func (r *MissingKind) Check(ctx *LintContext) []Finding {
	if ctx.yamlRoot == nil {
		return nil
	}
	for i := 0; i+1 < len(ctx.yamlRoot.Content); i += 2 {
		if ctx.yamlRoot.Content[i].Kind == yaml.ScalarNode && ctx.yamlRoot.Content[i].Value == yamlFieldKind {
			val := ctx.yamlRoot.Content[i+1].Value
			if val != "" && !circuit.KnownKinds[circuit.Kind(val)] {
				return []Finding{{
					RuleID:   r.ID(),
					Severity: SeverityInfo,
					Message:  fmt.Sprintf("unknown kind %q; known kinds: circuit, store-schema, scorecard, scenario, artifact-schema, report-template, vocabulary, heuristic-rules, source-pack, tuning", val),
					File:     ctx.File,
					Line:     ctx.yamlRoot.Content[i].Line,
				}}
			}
			return nil
		}
	}
	return []Finding{{
		RuleID:   r.ID(),
		Severity: r.Severity(),
		Message:  "YAML file has no kind field; add kind: <type> for self-identification",
		File:     ctx.File,
		Line:     1,
	}}
}

// --- B10: missing-kind-deprecated-arrow ---

type DeprecatedArrow struct{}

func (r *DeprecatedArrow) ID() string { return ruleDeprecatedArrow }
func (r *DeprecatedArrow) Description() string {
	return "use references instead of -> for foreign keys"
}
func (r *DeprecatedArrow) Severity() Severity { return SeverityWarning }
func (r *DeprecatedArrow) Tags() []string     { return []string{"best-practice", "schema", "envelope"} }

func (r *DeprecatedArrow) Check(ctx *LintContext) []Finding {
	if ctx.yamlRoot == nil {
		return nil
	}
	var out []Finding
	findArrows(ctx.yamlRoot, ctx.File, &out)
	return out
}

func findArrows(node *yaml.Node, file string, out *[]Finding) {
	if node == nil {
		return
	}
	if node.Kind == yaml.ScalarNode && strings.Contains(node.Value, " -> ") {
		*out = append(*out, Finding{
			RuleID:       ruleDeprecatedArrow,
			Severity:     SeverityWarning,
			Message:      fmt.Sprintf("deprecated -> syntax; use references instead: %s", node.Value),
			File:         file,
			Line:         node.Line,
			FixAvailable: true,
		})
	}
	for _, child := range node.Content {
		findArrows(child, file, out)
	}
}

// --- B11: missing-kind-domain-path ---

// domainDirs are directory prefixes where YAML files are expected to have a kind field.
var domainDirs = []string{
	"circuits/", "schemas/", "llm-output-schemas/", "scenarios/", "scorecards/",
	"reports/", "sources/", "tuning/", "domains/",
}

type MissingKindDomainPath struct{}

func (r *MissingKindDomainPath) ID() string { return "B11/missing-kind-domain-path" }
func (r *MissingKindDomainPath) Description() string {
	return "YAML in a domain directory should have a kind field"
}
func (r *MissingKindDomainPath) Severity() Severity { return SeverityWarning }
func (r *MissingKindDomainPath) Tags() []string     { return []string{"best-practice", "envelope"} }

func (r *MissingKindDomainPath) Check(ctx *LintContext) []Finding {
	if ctx.yamlRoot == nil || ctx.File == "" {
		return nil
	}
	isDomainPath := false
	for _, dir := range domainDirs {
		if strings.Contains(ctx.File, dir) {
			isDomainPath = true
			break
		}
	}
	if !isDomainPath {
		return nil
	}
	for i := 0; i+1 < len(ctx.yamlRoot.Content); i += 2 {
		if ctx.yamlRoot.Content[i].Kind == yaml.ScalarNode && ctx.yamlRoot.Content[i].Value == yamlFieldKind {
			return nil
		}
	}
	return []Finding{{
		RuleID:   r.ID(),
		Severity: r.Severity(),
		Message:  fmt.Sprintf("YAML file in domain directory (%s) has no kind field", ctx.File),
		File:     ctx.File,
		Line:     1,
	}}
}

// --- B12: manifest-missing-metadata ---

// manifestKinds are K8s-style manifest kinds that require metadata.name.
var manifestKinds = map[string]bool{
	"board":     true,
	"schematic": true,
	"component": true,
}

// ManifestMetadata checks that K8s-style manifests (board, schematic, component)
// have a metadata section with a name field.
type ManifestMetadata struct{}

func (r *ManifestMetadata) ID() string          { return "B12/manifest-missing-metadata" }
func (r *ManifestMetadata) Description() string { return "K8s-style manifest must have metadata.name" }
func (r *ManifestMetadata) Severity() Severity  { return SeverityWarning }
func (r *ManifestMetadata) Tags() []string      { return []string{"best-practice", "manifest", "k8s"} }

func (r *ManifestMetadata) Check(ctx *LintContext) []Finding {
	if ctx.yamlRoot == nil {
		return nil
	}

	var kind string
	var hasMetadata, hasMetadataName bool

	for i := 0; i+1 < len(ctx.yamlRoot.Content); i += 2 {
		key := ctx.yamlRoot.Content[i]
		val := ctx.yamlRoot.Content[i+1]

		if key.Value == "kind" {
			kind = val.Value
		}
		if key.Value == "metadata" && val.Kind == yaml.MappingNode {
			hasMetadata = true
			for j := 0; j+1 < len(val.Content); j += 2 {
				if val.Content[j].Value == "name" && val.Content[j+1].Value != "" {
					hasMetadataName = true
				}
			}
		}
	}

	if !manifestKinds[kind] {
		return nil // not a manifest kind, skip
	}

	if !hasMetadata || !hasMetadataName {
		return []Finding{{
			RuleID:   r.ID(),
			Severity: r.Severity(),
			Message:  fmt.Sprintf("%s manifest requires metadata.name", kind),
			File:     ctx.File,
			Line:     1,
		}}
	}
	return nil
}
