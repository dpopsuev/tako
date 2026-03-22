package circuit

// Category: DSL & Build — definition types for circuit YAML.

import (
	"fmt"
	"time"
)

// CircuitDef is the top-level DSL structure for declaring a circuit graph.
// Layout follows P3 (reading-first): circuit > zones > nodes > edges > start/done.
//
// When Import is set, this definition is an overlay on top of a base circuit
// provided by a schematic. Use LoadCircuitWithOverlay to resolve the import
// and merge overlay fields on top of the base.
type CircuitDef struct {
	Envelope    `yaml:",inline"`
	Circuit     string             `yaml:"circuit"`
	Description string             `yaml:"description,omitempty"`
	Import      string             `yaml:"import,omitempty"`
	Topology    string             `yaml:"topology,omitempty"`
	HandlerType string             `yaml:"handler_type,omitempty"`
	Timeout     string             `yaml:"timeout,omitempty"`
	Imports     []string           `yaml:"imports,omitempty"`
	Vars        map[string]any     `yaml:"vars,omitempty"`
	Extractors  []ExtractorDef     `yaml:"extractors,omitempty"`
	Ports       []PortDef          `yaml:"ports,omitempty"`
	Wiring      []WiringDef       `yaml:"wiring,omitempty"`
	Zones       map[string]ZoneDef `yaml:"zones,omitempty"`
	Nodes       []NodeDef          `yaml:"nodes"`
	Edges       []EdgeDef          `yaml:"edges"`
	Walkers     []WalkerDef        `yaml:"walkers,omitempty"`
	Start       string             `yaml:"start"`
	Done        string             `yaml:"done"`
	Scorecard   string             `yaml:"scorecard,omitempty"`
	Calibration *CalibrationContractDef `yaml:"calibration,omitempty"`
}

// CalibrationContractDef declares the calibration contract inline in circuit YAML.
type CalibrationContractDef struct {
	Inputs  []CalibrationFieldDef `yaml:"inputs,omitempty"`
	Outputs []CalibrationFieldDef `yaml:"outputs,omitempty"`
}

// CalibrationFieldDef maps a circuit output field to a scorer-addressable name.
type CalibrationFieldDef struct {
	Field      string `yaml:"field"`
	ScorerName string `yaml:"scorer_name"`
	Type       string `yaml:"type,omitempty"`
}

// PortDef declares a typed cross-circuit connection point.
type PortDef struct {
	Name        string `yaml:"name"`
	Direction   string `yaml:"direction"`             // "in", "out", or "loop"
	Type        string `yaml:"type,omitempty"`        // Go type for type-checking at wiring
	Description string `yaml:"description,omitempty"`
}

// WiringDef declares a consumer-level port connection between circuits.
type WiringDef struct {
	From    string `yaml:"from"`              // e.g. "rca.out:post-triage"
	To      string `yaml:"to"`                // e.g. "gnd.in:keywords"
	Adapter string `yaml:"adapter,omitempty"` // optional bridge transformer
}

// ExtractorDef declares a reusable extractor at the circuit level.
// Nodes reference extractors by name via handler: + handler_type: extractor.
// Type must be a built-in extractor type (json-schema, regex).
type ExtractorDef struct {
	Name    string          `yaml:"name"`
	Type    string          `yaml:"type"`
	Schema  *ArtifactSchema `yaml:"schema,omitempty"`
	Pattern string          `yaml:"pattern,omitempty"`
	OnError string          `yaml:"on_error,omitempty"`
}

// WalkerDef declares a walker (agent) in the circuit YAML.
// This is the "care, but in YAML" counterpart to DefaultWalker.
type WalkerDef struct {
	Name           string             `yaml:"name"`
	Approach       string             `yaml:"approach,omitempty"`
	Persona        string             `yaml:"persona,omitempty"`
	Preamble       string             `yaml:"preamble,omitempty"`
	OffsetPreamble string             `yaml:"offset_preamble,omitempty"`
	StepAffinity   map[string]float64 `yaml:"step_affinity,omitempty"`
	Role           string             `yaml:"role,omitempty"`
}

// ContextFilterDef declares which context keys are allowed or blocked
// when a walker transitions out of a zone. Implements the decoupling
// capacitor pattern: zone-local data stays local.
type ContextFilterDef struct {
	Pass  []string `yaml:"pass,omitempty"`
	Block []string `yaml:"block,omitempty"`
}

// ZoneDef declares a meta-phase zone (P7: optional, progressive disclosure).
type ZoneDef struct {
	Nodes         []string          `yaml:"nodes"`
	Approach      string            `yaml:"approach,omitempty"`
	Stickiness    int               `yaml:"stickiness,omitempty"`
	Domain        string            `yaml:"domain,omitempty"`
	ContextFilter *ContextFilterDef `yaml:"context_filter,omitempty"`
}

// HandlerType constants for the handler_type field.
const (
	HandlerTypeTransformer = "transformer"
	HandlerTypeExtractor   = "extractor"
	HandlerTypeRenderer    = "renderer"
	HandlerTypeNode        = "node"
	HandlerTypeDelegate    = "delegate"
	HandlerTypeCircuit     = "circuit"
)

// OutputField holds the structured output declaration for a node.
type OutputField struct {
	Name     string `yaml:"name"`
	Type     string `yaml:"type"`                // string, float, bool, int, array, object
	Required bool   `yaml:"required,omitempty"`
}

// NodeDef declares a node in the circuit.
//
// Resolution uses handler_type + handler (explicit, no cascade).
// Legacy fields (family, transformer, extractor, renderer, delegate+generator)
// were removed in TSK-218; YAML files using them will fail to resolve.
type NodeDef struct {
	Name        string          `yaml:"name"`
	Description string          `yaml:"description,omitempty"`
	Approach    string          `yaml:"approach,omitempty"`
	HandlerType string          `yaml:"handler_type,omitempty"`
	Handler     string          `yaml:"handler,omitempty"`

	Timeout      string          `yaml:"timeout,omitempty"`
	Provider     string          `yaml:"provider,omitempty"`
	Prompt       string          `yaml:"prompt,omitempty"`
	OutputSchema string          `yaml:"output_schema,omitempty"`
	Input        string          `yaml:"input,omitempty"`
	Before      []string        `yaml:"before,omitempty"`
	After       []string        `yaml:"after,omitempty"`
	Schema      *ArtifactSchema `yaml:"schema,omitempty"`
	Cache       *CacheDef       `yaml:"cache,omitempty"`
	Meta        map[string]any  `yaml:"meta,omitempty"`

	// Vocabulary and output schema
	Code        string        `yaml:"code,omitempty"`         // machine code (e.g. "F0")
	DisplayName string        `yaml:"display_name,omitempty"` // human name (e.g. "Recall")
	Output      []OutputField `yaml:"output,omitempty"`
}

// EffectiveHandlerType returns the handler type for this node, resolving
// the node-level override or circuit-level default.
func (nd NodeDef) EffectiveHandlerType(circuitDefault string) string {
	if nd.HandlerType != "" {
		return nd.HandlerType
	}
	if nd.Handler != "" && circuitDefault != "" {
		return circuitDefault
	}
	return ""
}

// EffectiveHandler returns the handler name for this node.
// Falls back to the node name when handler is not set.
func (nd NodeDef) EffectiveHandler() string {
	if nd.Handler != "" {
		return nd.Handler
	}
	return nd.Name
}

// OutputFields returns the output field declarations, or nil if none declared.
func (nd NodeDef) OutputFields() []OutputField {
	return nd.Output
}

// ValidateOutput checks that a map of output values satisfies the declared schema.
// Returns nil if no schema declared or all required fields present with correct types.
func (nd NodeDef) ValidateOutput(output map[string]any) error {
	if len(nd.Output) == 0 {
		return nil
	}
	for _, f := range nd.Output {
		val, exists := output[f.Name]
		if !exists && f.Required {
			return fmt.Errorf("node %q: required output field %q missing", nd.Name, f.Name)
		}
		if exists && !checkOutputType(val, f.Type) {
			return fmt.Errorf("node %q: output field %q: expected type %s, got %T", nd.Name, f.Name, f.Type, val)
		}
	}
	return nil
}

func checkOutputType(val any, expected string) bool {
	switch expected {
	case "string":
		_, ok := val.(string)
		return ok
	case "float":
		_, ok := val.(float64)
		return ok
	case "bool":
		_, ok := val.(bool)
		return ok
	case "int":
		switch val.(type) {
		case int, int64, float64:
			return true
		}
		return false
	case "array":
		switch val.(type) {
		case []any, []string, []float64, []int:
			return true
		}
		return false
	case "object":
		_, ok := val.(map[string]any)
		return ok
	default:
		return true
	}
}

// EffectiveTimeout returns the timeout for this node, resolving the
// node-level override against the circuit-level default. Returns 0 if
// neither is set.
func (nd NodeDef) EffectiveTimeout(circuitDefault string) (time.Duration, error) {
	raw := nd.Timeout
	if raw == "" {
		raw = circuitDefault
	}
	if raw == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("node %q: invalid timeout %q: %w", nd.Name, raw, err)
	}
	return d, nil
}

// CacheDef configures node-level caching via the DSL.
type CacheDef struct {
	TTL string `yaml:"ttl,omitempty"`
}

// EdgeDef declares a conditional edge between two nodes.
// P5: both id (machine) and name (human) are present.
// When is an expression evaluated by expr-lang/expr against {output, state, config}.
// Condition is a human-readable comment (not evaluated).
type EdgeDef struct {
	ID          string `yaml:"id"`
	Name        string `yaml:"name"`
	From        string `yaml:"from"`
	To          string `yaml:"to"`
	Shortcut    bool   `yaml:"shortcut,omitempty"`
	Loop        bool   `yaml:"loop,omitempty"`
	Parallel    bool   `yaml:"parallel,omitempty"`
	Condition   string `yaml:"condition,omitempty"`
	When        string `yaml:"when,omitempty"`
	Merge       string `yaml:"merge,omitempty"`
	DisplayName string `yaml:"display_name,omitempty"` // human name for edge conditions
}

// Merge strategy constants for fan-in edges.
const (
	MergeAppend = "append"
	MergeLatest = "latest"
	MergeCustom = "custom"
)

// MarshalYAML serializes a CircuitDef back to YAML (P8: round-trip fidelity).
func (def *CircuitDef) MarshalYAML() ([]byte, error) {
	return yamlMarshal(def)
}

// Validate checks referential integrity of the circuit definition:
//   - circuit name is non-empty
//   - at least one node and one edge exist
//   - start node exists in the node list
//   - all edge From/To reference existing nodes (or the done pseudo-node)
//   - all zone node references exist
func (def *CircuitDef) Validate() error {
	if def.Circuit == "" {
		return fmt.Errorf("circuit name is required")
	}
	if len(def.Nodes) == 0 {
		return fmt.Errorf("at least one node is required")
	}
	if len(def.Edges) == 0 {
		return fmt.Errorf("at least one edge is required")
	}
	if def.Start == "" {
		return fmt.Errorf("start node is required")
	}
	if def.Done == "" {
		return fmt.Errorf("done node is required")
	}

	nodeSet := make(map[string]bool, len(def.Nodes))
	for _, n := range def.Nodes {
		if n.Name == "" {
			return fmt.Errorf("node name is required")
		}
		if nodeSet[n.Name] {
			return fmt.Errorf("duplicate node name %q", n.Name)
		}
		nodeSet[n.Name] = true
	}

	if !nodeSet[def.Start] {
		return fmt.Errorf("start node %q not found in node list", def.Start)
	}

	edgeIDs := make(map[string]bool, len(def.Edges))
	for _, e := range def.Edges {
		if e.ID == "" {
			return fmt.Errorf("edge id is required")
		}
		if edgeIDs[e.ID] {
			return fmt.Errorf("duplicate edge id %q", e.ID)
		}
		edgeIDs[e.ID] = true

		if !nodeSet[e.From] {
			return fmt.Errorf("edge %s references unknown source node %q", e.ID, e.From)
		}
		if e.To != def.Done && !nodeSet[e.To] {
			return fmt.Errorf("edge %s references unknown target node %q", e.ID, e.To)
		}
	}

	for zoneName, z := range def.Zones {
		for _, nodeName := range z.Nodes {
			if !nodeSet[nodeName] {
				return fmt.Errorf("zone %q references unknown node %q", zoneName, nodeName)
			}
		}
	}

	return nil
}

// RegisterVocabulary populates the given vocabulary with entries derived
// from circuit nodes and edges. Nodes with Code/DisplayName register both
// the code->name and a "code_NAME" alias. Edges with DisplayName register
// the edge ID->name mapping.
func (def *CircuitDef) RegisterVocabulary(v *RichMapVocabulary) {
	for _, n := range def.Nodes {
		if n.Code != "" && n.DisplayName != "" {
			v.RegisterEntry(n.Code, VocabEntry{Short: n.Code, Long: n.DisplayName})
			alias := n.Code + "_" + toUpperReplace(n.DisplayName, " ", "_")
			v.RegisterEntry(alias, VocabEntry{Short: n.Code, Long: n.DisplayName})
		}
		if n.DisplayName != "" {
			v.RegisterEntry(n.Name, VocabEntry{Long: n.DisplayName})
		}
	}
	for _, e := range def.Edges {
		if e.DisplayName != "" {
			v.RegisterEntry(e.ID, VocabEntry{Long: e.DisplayName})
		}
	}
}
