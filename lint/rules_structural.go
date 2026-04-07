package lint

import (
	"fmt"
	"strings"
	"time"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/circuit/def"
	"github.com/dpopsuev/origami/roster"
)

const (
	ruleInvalidApproach = "S2/invalid-approach"
	ruleInvalidMerge    = "S3/invalid-merge-strategy"
	ruleMissingEdgeName = "S4/missing-edge-name"
	ruleInvalidCacheTTL = "S7/invalid-cache-ttl"
	ruleMissingCircDesc = "S8/missing-circuit-description"
	ruleInvalidPersona  = "S11/invalid-walker-persona"
)

// isValidValue checks whether value is among the ValidValues declared for field
// in the given registry. Returns true when the field has no enum constraint
// (ValidValues == nil) or when a case-insensitive match is found.
func isValidValue(registry def.FieldRegistry, field, value string) bool {
	fd, ok := registry[field]
	if !ok || fd.ValidValues == nil {
		return true // no validation defined
	}
	for _, v := range fd.ValidValues {
		if strings.EqualFold(v, value) {
			return true
		}
	}
	return false
}

func knownPersonas() map[string]bool {
	all := roster.PersonaAll()
	m := make(map[string]bool)
	for i := range all {
		m[strings.ToLower(all[i].Name)] = true
	}
	return m
}

func approachSuggestion(val string) string {
	best, bestDist := "", 100
	for _, a := range def.ValidApproaches {
		d := levenshtein(strings.ToLower(val), a)
		if d < bestDist {
			bestDist = d
			best = a
		}
	}
	if bestDist <= 3 {
		return fmt.Sprintf("did you mean %q?", best)
	}
	return ""
}

// MissingNodeApproach checks that every node declares an approach.
type MissingNodeApproach struct{}

func (r *MissingNodeApproach) ID() string          { return "S1/missing-node-approach" }
func (r *MissingNodeApproach) Description() string { return "every node should declare an approach" }
func (r *MissingNodeApproach) Severity() Severity  { return SeverityWarning }
func (r *MissingNodeApproach) Tags() []string      { return []string{"structural"} }

func (r *MissingNodeApproach) Check(ctx *LintContext) []Finding {
	var out []Finding
	for i := range ctx.Def.Nodes {
		if ctx.Def.Nodes[i].Approach == "" {
			out = append(out, Finding{
				RuleID:   r.ID(),
				Severity: r.Severity(),
				Message:  fmt.Sprintf("node %q has no approach", string(ctx.Def.Nodes[i].Name)),
				File:     ctx.File,
				Line:     ctx.NodeLine(string(ctx.Def.Nodes[i].Name)),
			})
		}
	}
	return out
}

// InvalidApproach checks that node approach values are known approaches.
type InvalidApproach struct{}

func (r *InvalidApproach) ID() string          { return ruleInvalidApproach }
func (r *InvalidApproach) Description() string { return "approach value must be a known approach" }
func (r *InvalidApproach) Severity() Severity  { return SeverityError }
func (r *InvalidApproach) Tags() []string      { return []string{"structural"} }

func (r *InvalidApproach) Check(ctx *LintContext) []Finding {
	var out []Finding
	for i := range ctx.Def.Nodes {
		nd := &ctx.Def.Nodes[i]
		if nd.Approach != "" && !isValidValue(def.NodeFields, "approach", nd.Approach) {
			f := Finding{
				RuleID:   r.ID(),
				Severity: r.Severity(),
				Message:  fmt.Sprintf("node %q: unknown approach %q (valid: %s)", nd.Name, nd.Approach, strings.Join(def.ValidApproaches, ", ")),
				File:     ctx.File,
				Line:     ctx.NodeLine(string(nd.Name)),
			}
			if s := approachSuggestion(nd.Approach); s != "" {
				f.Suggestion = s
				f.FixAvailable = true
			}
			out = append(out, f)
		}
	}
	return out
}

// MissingEdgeID checks that every top-level edge has an explicit id.
// Without an id the graph builder rejects the circuit at runtime —
// catching this at lint time gives a faster, clearer error.
type MissingEdgeID struct{}

func (r *MissingEdgeID) ID() string          { return "S10/missing-edge-id" }
func (r *MissingEdgeID) Description() string { return "every edge must have an explicit id" }
func (r *MissingEdgeID) Severity() Severity  { return SeverityError }
func (r *MissingEdgeID) Tags() []string      { return []string{"structural"} }

func (r *MissingEdgeID) Check(ctx *LintContext) []Finding {
	var out []Finding
	for i := range ctx.Def.Edges {
		ed := &ctx.Def.Edges[i]
		if ed.ID == "" {
			msg := fmt.Sprintf("edge from %q to %q has no id", ed.From, ed.To)
			out = append(out, Finding{
				RuleID:       r.ID(),
				Severity:     r.Severity(),
				Message:      msg,
				File:         ctx.File,
				Suggestion:   fmt.Sprintf("add id: %s-%s", ed.From, ed.To),
				FixAvailable: true,
			})
		}
	}
	return out
}

// InvalidMergeStrategy checks that edge merge values are valid strategies.
type InvalidMergeStrategy struct{}

func (r *InvalidMergeStrategy) ID() string { return ruleInvalidMerge }
func (r *InvalidMergeStrategy) Description() string {
	return "merge strategy must be append, latest, or custom"
}
func (r *InvalidMergeStrategy) Severity() Severity { return SeverityError }
func (r *InvalidMergeStrategy) Tags() []string     { return []string{"structural"} }

func (r *InvalidMergeStrategy) Check(ctx *LintContext) []Finding {
	var out []Finding
	for i := range ctx.Def.Edges {
		if ctx.Def.Edges[i].Merge != "" && !isValidValue(def.EdgeFields, "merge", ctx.Def.Edges[i].Merge) {
			out = append(out, Finding{
				RuleID:   r.ID(),
				Severity: r.Severity(),
				Message:  fmt.Sprintf("edge %q: unknown merge strategy %q (valid: %s)", ctx.Def.Edges[i].ID, ctx.Def.Edges[i].Merge, strings.Join(def.ValidMergeStrategies, ", ")),
				File:     ctx.File,
				Line:     ctx.EdgeLine(ctx.Def.Edges[i].ID),
			})
		}
	}
	return out
}

// MissingEdgeName checks that every edge has a human-readable name.
type MissingEdgeName struct{}

func (r *MissingEdgeName) ID() string          { return ruleMissingEdgeName }
func (r *MissingEdgeName) Description() string { return "edges should have a human-readable name" }
func (r *MissingEdgeName) Severity() Severity  { return SeverityInfo }
func (r *MissingEdgeName) Tags() []string      { return []string{"structural"} }

func (r *MissingEdgeName) Check(ctx *LintContext) []Finding {
	var out []Finding
	for i := range ctx.Def.Edges {
		if ctx.Def.Edges[i].Name == "" {
			out = append(out, Finding{
				RuleID:       r.ID(),
				Severity:     r.Severity(),
				Message:      fmt.Sprintf("edge %q has no name", ctx.Def.Edges[i].ID),
				File:         ctx.File,
				Line:         ctx.EdgeLine(ctx.Def.Edges[i].ID),
				FixAvailable: true,
			})
		}
	}
	return out
}

// DuplicateEdgeCondition checks for edges from the same node sharing identical conditions.
type DuplicateEdgeCondition struct{}

func (r *DuplicateEdgeCondition) ID() string { return "S5/duplicate-edge-condition" }
func (r *DuplicateEdgeCondition) Description() string {
	return "edges from the same node should not have identical when expressions"
}
func (r *DuplicateEdgeCondition) Severity() Severity { return SeverityWarning }
func (r *DuplicateEdgeCondition) Tags() []string     { return []string{"structural"} }

func (r *DuplicateEdgeCondition) Check(ctx *LintContext) []Finding {
	type key struct{ from, when string }
	seen := make(map[key]string)
	var out []Finding
	for i := range ctx.Def.Edges {
		ed := &ctx.Def.Edges[i]
		if ed.When == "" || ed.Parallel {
			continue
		}
		k := key{string(ed.From), ed.When}
		if prev, ok := seen[k]; ok {
			out = append(out, Finding{
				RuleID:   r.ID(),
				Severity: r.Severity(),
				Message:  fmt.Sprintf("edge %q has same when expression as edge %q (from node %q)", ed.ID, prev, ed.From),
				File:     ctx.File,
				Line:     ctx.EdgeLine(ed.ID),
			})
		} else {
			seen[k] = ed.ID
		}
	}
	return out
}

// EmptyPrompt checks that nodes with an LLM transformer have a non-empty prompt.
type EmptyPrompt struct{}

func (r *EmptyPrompt) ID() string { return "S6/empty-prompt" }
func (r *EmptyPrompt) Description() string {
	return "node with no prompt, transformer, extractor, or renderer may produce empty output"
}
func (r *EmptyPrompt) Severity() Severity { return SeverityWarning }
func (r *EmptyPrompt) Tags() []string     { return []string{"structural"} }

func (r *EmptyPrompt) Check(ctx *LintContext) []Finding {
	var out []Finding
	for i := range ctx.Def.Nodes {
		nd := &ctx.Def.Nodes[i]
		ht := nd.EffectiveHandlerType(ctx.Def.HandlerType)
		// node/delegate/circuit/extractor/renderer handler types provide their own logic
		if ht == circuit.HandlerTypeNode || ht == circuit.HandlerTypeDelegate ||
			ht == circuit.HandlerTypeCircuit || ht == circuit.HandlerTypeExtractor ||
			ht == circuit.HandlerTypeRenderer {
			continue
		}
		// handler: set means resolution is explicit — skip this check
		if nd.Handler != "" {
			continue
		}
		if nd.Prompt == "" {
			out = append(out, Finding{
				RuleID:   r.ID(),
				Severity: r.Severity(),
				Message:  fmt.Sprintf("node %q has no prompt, transformer, extractor, or renderer", nd.Name),
				File:     ctx.File,
				Line:     ctx.NodeLine(string(nd.Name)),
			})
		}
	}
	return out
}

// InvalidCacheTTL checks that node cache TTL values are valid Go durations.
type InvalidCacheTTL struct{}

func (r *InvalidCacheTTL) ID() string          { return ruleInvalidCacheTTL }
func (r *InvalidCacheTTL) Description() string { return "cache TTL must be a valid Go duration" }
func (r *InvalidCacheTTL) Severity() Severity  { return SeverityError }
func (r *InvalidCacheTTL) Tags() []string      { return []string{"structural"} }

func (r *InvalidCacheTTL) Check(ctx *LintContext) []Finding {
	var out []Finding
	for i := range ctx.Def.Nodes {
		nd := &ctx.Def.Nodes[i]
		if nd.Cache != nil && nd.Cache.TTL != "" {
			if _, err := time.ParseDuration(nd.Cache.TTL); err != nil {
				out = append(out, Finding{
					RuleID:   r.ID(),
					Severity: r.Severity(),
					Message:  fmt.Sprintf("node %q: invalid cache TTL %q: %v", nd.Name, nd.Cache.TTL, err),
					File:     ctx.File,
					Line:     ctx.NodeLine(string(nd.Name)),
				})
			}
		}
	}
	return out
}

// MissingCircuitDescription checks that the circuit has a description field.
type MissingCircuitDescription struct{}

func (r *MissingCircuitDescription) ID() string          { return ruleMissingCircDesc }
func (r *MissingCircuitDescription) Description() string { return "circuit should have a description" }
func (r *MissingCircuitDescription) Severity() Severity  { return SeverityInfo }
func (r *MissingCircuitDescription) Tags() []string      { return []string{"structural"} }

func (r *MissingCircuitDescription) Check(ctx *LintContext) []Finding {
	if ctx.Def.Description == "" {
		return []Finding{{
			RuleID:       r.ID(),
			Severity:     r.Severity(),
			Message:      "circuit has no description",
			File:         ctx.File,
			Line:         ctx.TopLevelLine("circuit"),
			FixAvailable: true,
		}}
	}
	return nil
}

// UnnamedNode checks that no node has an empty name.
type UnnamedNode struct{}

func (r *UnnamedNode) ID() string          { return "S9/unnamed-node" }
func (r *UnnamedNode) Description() string { return "every node must have a name" }
func (r *UnnamedNode) Severity() Severity  { return SeverityError }
func (r *UnnamedNode) Tags() []string      { return []string{"structural"} }

func (r *UnnamedNode) Check(ctx *LintContext) []Finding {
	var out []Finding
	for i := range ctx.Def.Nodes {
		if ctx.Def.Nodes[i].Name == "" {
			out = append(out, Finding{
				RuleID:   r.ID(),
				Severity: r.Severity(),
				Message:  fmt.Sprintf("node at index %d has no name", i),
				File:     ctx.File,
			})
		}
	}
	return out
}

// InvalidWalkerElement checks that walker element values are known elements.
type InvalidWalkerApproach struct{}

func (r *InvalidWalkerApproach) ID() string { return "S10/invalid-walker-approach" }
func (r *InvalidWalkerApproach) Description() string {
	return "walker approach must be a known approach"
}
func (r *InvalidWalkerApproach) Severity() Severity { return SeverityError }
func (r *InvalidWalkerApproach) Tags() []string     { return []string{"structural"} }

func (r *InvalidWalkerApproach) Check(ctx *LintContext) []Finding {
	var out []Finding
	for _, w := range ctx.Def.Walkers {
		if w.Approach != "" && !isValidValue(def.WalkerFields, "approach", w.Approach) {
			f := Finding{
				RuleID:   r.ID(),
				Severity: r.Severity(),
				Message:  fmt.Sprintf("walker %q: unknown approach %q", w.Name, w.Approach),
				File:     ctx.File,
				Line:     ctx.WalkerLine(w.Name),
			}
			if s := approachSuggestion(w.Approach); s != "" {
				f.Suggestion = s
				f.FixAvailable = true
			}
			out = append(out, f)
		}
	}
	return out
}

// InvalidWalkerPersona checks that walker persona values are known personas.
type InvalidWalkerPersona struct{}

func (r *InvalidWalkerPersona) ID() string          { return ruleInvalidPersona }
func (r *InvalidWalkerPersona) Description() string { return "walker persona must be a known persona" }
func (r *InvalidWalkerPersona) Severity() Severity  { return SeverityError }
func (r *InvalidWalkerPersona) Tags() []string      { return []string{"structural"} }

func (r *InvalidWalkerPersona) Check(ctx *LintContext) []Finding {
	personas := knownPersonas()
	var out []Finding
	for _, w := range ctx.Def.Walkers {
		if w.Persona != "" && !personas[strings.ToLower(w.Persona)] {
			out = append(out, Finding{
				RuleID:   r.ID(),
				Severity: r.Severity(),
				Message:  fmt.Sprintf("walker %q: unknown persona %q", w.Name, w.Persona),
				File:     ctx.File,
				Line:     ctx.WalkerLine(w.Name),
			})
		}
	}
	return out
}

// SchemaInUnstructuredZone checks for artifact schemas on nodes in zones without structured extractors.
type SchemaInUnstructuredZone struct{}

func (r *SchemaInUnstructuredZone) ID() string { return "S12/schema-in-unstructured-zone" }
func (r *SchemaInUnstructuredZone) Description() string {
	return "nodes with schema should not be in unstructured zones"
}
func (r *SchemaInUnstructuredZone) Severity() Severity { return SeverityWarning }
func (r *SchemaInUnstructuredZone) Tags() []string     { return []string{"structural"} }

func (r *SchemaInUnstructuredZone) Check(ctx *LintContext) []Finding {
	var out []Finding
	for zoneName, zd := range ctx.Def.Zones {
		if !strings.EqualFold(zd.Domain, "unstructured") {
			continue
		}
		nodeSet := make(map[string]bool, len(zd.Nodes))
		for _, n := range zd.Nodes {
			nodeSet[string(n)] = true
		}
		for j := range ctx.Def.Nodes {
			name := string(ctx.Def.Nodes[j].Name)
			if nodeSet[name] && ctx.Def.Nodes[j].Schema != nil {
				out = append(out, Finding{
					RuleID:   r.ID(),
					Severity: r.Severity(),
					Message:  fmt.Sprintf("node %q has schema but is in unstructured zone %q", name, zoneName),
					File:     ctx.File,
					Line:     ctx.NodeLine(name),
				})
			}
		}
	}
	return out
}

// MissingZoneDomain checks that zones declare a domain field.
type MissingZoneDomain struct{}

func (r *MissingZoneDomain) ID() string { return "S13/missing-zone-domain" }
func (r *MissingZoneDomain) Description() string {
	return "zones should declare a domain (unstructured, structured, hybrid)"
}
func (r *MissingZoneDomain) Severity() Severity { return SeverityInfo }
func (r *MissingZoneDomain) Tags() []string     { return []string{"structural"} }

func (r *MissingZoneDomain) Check(ctx *LintContext) []Finding {
	var out []Finding
	for zoneName, zd := range ctx.Def.Zones {
		if len(zd.Nodes) > 0 && zd.Domain == "" {
			out = append(out, Finding{
				RuleID:   r.ID(),
				Severity: r.Severity(),
				Message:  fmt.Sprintf("zone %q has no domain annotation (consider: unstructured, structured, hybrid)", zoneName),
				File:     ctx.File,
				Line:     ctx.TopLevelLine("zones"),
			})
		}
	}
	return out
}

// InvalidZoneDomain checks that zone domain values are valid predefined domains.
type InvalidZoneDomain struct{}

func (r *InvalidZoneDomain) ID() string { return "S14/invalid-zone-domain" }
func (r *InvalidZoneDomain) Description() string {
	return "zone domain must be unstructured, structured, or hybrid"
}
func (r *InvalidZoneDomain) Severity() Severity { return SeverityError }
func (r *InvalidZoneDomain) Tags() []string     { return []string{"structural"} }

func (r *InvalidZoneDomain) Check(ctx *LintContext) []Finding {
	var out []Finding
	for zoneName, zd := range ctx.Def.Zones {
		if zd.Domain != "" && !isValidValue(def.ZoneFields, "domain", zd.Domain) {
			out = append(out, Finding{
				RuleID:   r.ID(),
				Severity: r.Severity(),
				Message:  fmt.Sprintf("zone %q: unknown domain %q (valid: %s)", zoneName, zd.Domain, strings.Join(def.ValidZoneDomains, ", ")),
				File:     ctx.File,
				Line:     ctx.TopLevelLine("zones"),
			})
		}
	}
	return out
}

// levenshtein computes the edit distance between two strings.
func levenshtein(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	prev := make([]int, lb+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr := make([]int, lb+1)
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(curr[j-1]+1, min(prev[j]+1, prev[j-1]+cost))
		}
		prev = curr
	}
	return prev[lb]
}

// InvalidWalkerRole checks that walker role values are recognized roles.
type InvalidWalkerRole struct{}

func (r *InvalidWalkerRole) ID() string { return "S16/invalid-walker-role" }
func (r *InvalidWalkerRole) Description() string {
	return "walker role must be a recognized role (worker, manager, enforcer, broker)"
}
func (r *InvalidWalkerRole) Severity() Severity { return SeverityError }
func (r *InvalidWalkerRole) Tags() []string     { return []string{"structural"} }

func (r *InvalidWalkerRole) Check(ctx *LintContext) []Finding {
	validRoles := map[string]bool{
		"worker": true, "manager": true, "enforcer": true, "broker": true,
	}
	var out []Finding
	for _, w := range ctx.Def.Walkers {
		if w.Role != "" && !validRoles[strings.ToLower(w.Role)] {
			out = append(out, Finding{
				RuleID:   r.ID(),
				Severity: r.Severity(),
				Message:  fmt.Sprintf("walker %q: unknown role %q (valid: worker, manager, enforcer, broker)", w.Name, w.Role),
				File:     ctx.File,
				Line:     ctx.WalkerLine(w.Name),
			})
		}
	}
	return out
}

// DelegateWithoutGenerator warns when a delegate node lacks a generator.
type DelegateWithoutGenerator struct{}

func (r *DelegateWithoutGenerator) ID() string { return "S15/delegate-without-generator" }
func (r *DelegateWithoutGenerator) Description() string {
	return "delegate node requires a handler (generator transformer)"
}
func (r *DelegateWithoutGenerator) Severity() Severity { return SeverityWarning }
func (r *DelegateWithoutGenerator) Tags() []string     { return []string{"structural"} }

func (r *DelegateWithoutGenerator) Check(ctx *LintContext) []Finding {
	var out []Finding
	for i := range ctx.Def.Nodes {
		nd := &ctx.Def.Nodes[i]
		ht := nd.EffectiveHandlerType(ctx.Def.HandlerType)
		if ht == circuit.HandlerTypeDelegate && nd.Handler == "" {
			out = append(out, Finding{
				RuleID:   r.ID(),
				Severity: r.Severity(),
				Message:  fmt.Sprintf("delegate node %q requires handler: <name> (generator transformer)", nd.Name),
				File:     ctx.File,
				Line:     ctx.NodeLine(string(nd.Name)),
			})
		}
	}
	return out
}

// DeprecatedHandlerFields errors when nodes use removed legacy handler fields
// (family, transformer, extractor, renderer, delegate, generator) in YAML.
// These fields are no longer recognized; use handler: + handler_type: instead.
type DeprecatedHandlerFields struct{}

func (r *DeprecatedHandlerFields) ID() string { return "S17/deprecated-handler-fields" }
func (r *DeprecatedHandlerFields) Description() string {
	return "use handler: + handler_type: instead of family/transformer/extractor/renderer/delegate+generator"
}
func (r *DeprecatedHandlerFields) Severity() Severity { return SeverityError }
func (r *DeprecatedHandlerFields) Tags() []string     { return []string{"structural"} }

var removedNodeFields = []string{"family", "transformer", "extractor", "renderer", "delegate", "generator"}

func (r *DeprecatedHandlerFields) Check(ctx *LintContext) []Finding {
	var out []Finding
	for i := range ctx.Def.Nodes {
		nd := &ctx.Def.Nodes[i]
		keys := ctx.NodeYAMLKeys(string(nd.Name))
		if keys == nil {
			continue
		}
		var deprecated []string
		for _, f := range removedNodeFields {
			if keys[f] {
				deprecated = append(deprecated, f)
			}
		}
		if len(deprecated) > 0 {
			out = append(out, Finding{
				RuleID:   r.ID(),
				Severity: r.Severity(),
				Message:  fmt.Sprintf("node %q uses removed field(s) %s; migrate to handler: + handler_type:", nd.Name, strings.Join(deprecated, ", ")),
				File:     ctx.File,
				Line:     ctx.NodeLine(string(nd.Name)),
			})
		}
	}
	return out
}
