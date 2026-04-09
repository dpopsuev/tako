package def

// Category: DSL & Build — definition types for circuit YAML.

import (
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

// NodeName is a typed identifier for circuit nodes.
// Provides compile-time safety for node name usage in maps and function params.
type NodeName string

// String returns the node name as a plain string.
func (n NodeName) String() string { return string(n) }

const (
	outputTypeString = "string"
	outputTypeArray  = "array"
	outputTypeObject = "object"
)

// CircuitDef is the top-level DSL structure for declaring a circuit graph.
// Layout follows P3 (reading-first): circuit > zones > nodes > edges > start/done.
//
// When Import is set, this definition is an overlay on top of a base circuit
// provided by a schematic. Use LoadCircuitWithOverlay to resolve the import
// and merge overlay fields on top of the base.
type CircuitDef struct {
	Envelope    `yaml:",inline"`
	Circuit     string                  `yaml:"circuit"`
	Description string                  `yaml:"description,omitempty"`
	Import      string                  `yaml:"import,omitempty"`
	Topology    string                  `yaml:"topology,omitempty"`
	HandlerType string                  `yaml:"handler_type,omitempty"`
	Timeout     string                  `yaml:"timeout,omitempty"`
	Imports     []string                `yaml:"imports,omitempty"`
	Vars        map[string]any          `yaml:"vars,omitempty"`
	Extractors  []ExtractorDef          `yaml:"extractors,omitempty"`
	Ports       []PortDef               `yaml:"ports,omitempty"`
	Wiring      []WiringDef             `yaml:"wiring,omitempty"`
	Zones       map[string]ZoneDef      `yaml:"zones,omitempty"`
	Nodes       []NodeDef               `yaml:"nodes"`
	Edges       []EdgeDef               `yaml:"edges"`
	Walkers     []WalkerDef             `yaml:"walkers,omitempty"`
	Start       NodeName                `yaml:"start"`
	Done        NodeName                `yaml:"done"`
	Finally     NodeName                `yaml:"finally,omitempty"`
	Scorecard   string                  `yaml:"scorecard,omitempty"`
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
	Direction   string `yaml:"direction"`      // "in", "out", or "loop"
	Type        string `yaml:"type,omitempty"` // Go type for type-checking at wiring
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
	Nodes         []NodeName        `yaml:"nodes"`
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
	HandlerTypeInstrument  = "instrument"
)

// OutputField holds the structured output declaration for a node.
type OutputField struct {
	Name     string `yaml:"name"`
	Type     string `yaml:"type"` // string, float, bool, int, array, object
	Required bool   `yaml:"required,omitempty"`
}

// NodeConfig carries typed handler configuration for a circuit node.
// Replaces the former untyped Meta map[string]any field. Unknown keys go to Extras.
type NodeConfig struct {
	// Template params (template-params transformer)
	IncludeState  bool           `yaml:"include_state,omitempty"`
	IncludeConfig bool           `yaml:"include_config,omitempty"`
	Pick          []string       `yaml:"pick,omitempty"`
	Extra         map[string]any `yaml:"extra,omitempty"`

	// HTTP transformer
	URL     string            `yaml:"url,omitempty"`
	Method  string            `yaml:"method,omitempty"`
	Headers map[string]string `yaml:"headers,omitempty"`

	// JQ transformer
	Expr string `yaml:"expr,omitempty"`

	// Match transformer
	RuleSet string `yaml:"rule_set,omitempty"`
	Field   string `yaml:"field,omitempty"`

	// LLM transformer
	ArtifactPath string `yaml:"artifact_path,omitempty"`

	// Build / extractor
	Pattern string `yaml:"pattern,omitempty"`

	// Hook output
	OutputPath string `yaml:"output_path,omitempty"`

	// SQLite hook
	SQLiteQuery  string `yaml:"sqlite_query,omitempty"`
	SQLiteParams []any  `yaml:"sqlite_params,omitempty"`

	// Limits
	MaxRetries int `yaml:"max_retries,omitempty"`
	MaxTokens  int `yaml:"max_tokens,omitempty"`

	// Programmatic — not from YAML
	Evaluator any `yaml:"-"` // *toolkit.MatchEvaluator, set by consumer code

	// Controlled escape hatch for domain-specific keys
	Extras map[string]any `yaml:"extras,omitempty"`
}

// NodeDef declares a node in the circuit.
//
// Resolution uses handler_type + handler (explicit, no cascade).
// Legacy fields (family, transformer, extractor, renderer, delegate+generator)
// were removed in TSK-218; YAML files using them will fail to resolve.
type NodeDef struct {
	Name        NodeName `yaml:"name"`
	Description string   `yaml:"description,omitempty"`
	Approach    string   `yaml:"approach,omitempty"`
	HandlerType string   `yaml:"handler_type,omitempty"`
	Handler     string   `yaml:"handler,omitempty"`

	Timeout      string          `yaml:"timeout,omitempty"`
	Provider     string          `yaml:"provider,omitempty"`
	Prompt       string          `yaml:"prompt,omitempty"`
	OutputSchema string          `yaml:"output_schema,omitempty"`
	Input        string          `yaml:"input,omitempty"`
	Before       []string        `yaml:"before,omitempty"`
	After        []string        `yaml:"after,omitempty"`
	Schema       *ArtifactSchema `yaml:"schema,omitempty"`
	Cache        *CacheDef       `yaml:"cache,omitempty"`
	Config       *NodeConfig     `yaml:"meta,omitempty"`

	// Vocabulary and output schema
	Code        string        `yaml:"code,omitempty"`         // machine code (e.g. "F0")
	DisplayName string        `yaml:"display_name,omitempty"` // human name (e.g. "Recall")
	Output      []OutputField `yaml:"output,omitempty"`
}

// EffectiveConfig returns the node's typed config, or an empty NodeConfig if none set.
func (nd *NodeDef) EffectiveConfig() *NodeConfig {
	if nd.Config != nil {
		return nd.Config
	}
	return &NodeConfig{}
}

// EffectiveHandlerType returns the handler type for this node, resolving
// the node-level override or circuit-level default.
func (nd *NodeDef) EffectiveHandlerType(circuitDefault string) string {
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
func (nd *NodeDef) EffectiveHandler() string {
	if nd.Handler != "" {
		return nd.Handler
	}
	return string(nd.Name)
}

// OutputFields returns the output field declarations, or nil if none declared.
func (nd *NodeDef) OutputFields() []OutputField {
	return nd.Output
}

// ValidateOutput checks that a map of output values satisfies the declared schema.
// Returns nil if no schema declared or all required fields present with correct types.
func (nd *NodeDef) ValidateOutput(output map[string]any) error {
	if len(nd.Output) == 0 {
		return nil
	}
	for _, f := range nd.Output {
		val, exists := output[f.Name]
		if !exists && f.Required {
			return fmt.Errorf("%w: %q: required output field %q missing", ErrNode, string(nd.Name), f.Name)
		}
		if exists && !checkOutputType(val, f.Type) {
			return fmt.Errorf("%w: %q: output field %q: expected type %s, got %T", ErrNode, string(nd.Name), f.Name, f.Type, val)
		}
	}
	return nil
}

func checkOutputType(val any, expected string) bool {
	switch expected {
	case outputTypeString:
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
	case outputTypeArray:
		switch val.(type) {
		case []any, []string, []float64, []int:
			return true
		}
		return false
	case outputTypeObject:
		_, ok := val.(map[string]any)
		return ok
	default:
		return true
	}
}

// EffectiveTimeout returns the timeout for this node, resolving the
// node-level override against the circuit-level default. Returns 0 if
// neither is set.
func (nd *NodeDef) EffectiveTimeout(circuitDefault string) (time.Duration, error) {
	raw := nd.Timeout
	if raw == "" {
		raw = circuitDefault
	}
	if raw == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("node %q: invalid timeout %q: %w", string(nd.Name), raw, err)
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
	ID          string   `yaml:"id"`
	Name        string   `yaml:"name"`
	From        NodeName `yaml:"from"`
	To          NodeName `yaml:"to"`
	Shortcut    bool     `yaml:"shortcut,omitempty"`
	Loop        bool     `yaml:"loop,omitempty"`
	Parallel    bool     `yaml:"parallel,omitempty"`
	Condition   string   `yaml:"condition,omitempty"`
	When        string   `yaml:"when,omitempty"`
	Merge       string   `yaml:"merge,omitempty"`
	DisplayName string   `yaml:"display_name,omitempty"` // human name for edge conditions
}

// Merge strategy constants for fan-in edges.
const (
	MergeAppend = "append"
	MergeLatest = "latest"
	MergeCustom = "custom"
)

// Canonical enum values for DSL fields. Lint rules read from FieldRegistry
// instead of maintaining duplicate maps. Add a value here → lint validates it.
var (
	ValidApproaches      = []string{"rapid", "aggressive", "methodical", "rigorous", "analytical", "holistic"}
	ValidZoneDomains     = []string{"unstructured", "structured", "hybrid"}
	ValidPortDirections  = []string{"in", "out", "loop"}
	ValidHandlerTypes    = []string{HandlerTypeTransformer, HandlerTypeExtractor, HandlerTypeRenderer, HandlerTypeNode, HandlerTypeDelegate, HandlerTypeCircuit, HandlerTypeInstrument}
	ValidMergeStrategies = []string{MergeAppend, MergeLatest, MergeCustom}
)

// MarshalYAML serializes a CircuitDef back to YAML (P8: round-trip fidelity).
func (def *CircuitDef) MarshalYAML() ([]byte, error) {
	return yaml.Marshal(def)
}

// Validate checks referential integrity of the circuit definition:
//   - circuit name is non-empty
//   - at least one node and one edge exist
//   - start node exists in the node list
//   - all edge From/To reference existing nodes (or the done pseudo-node)
//   - all zone node references exist
func (def *CircuitDef) Validate() error {
	if def.Circuit == "" {
		return ErrCircuitNameIsRequired
	}
	if len(def.Nodes) == 0 {
		return ErrAtLeastOneNodeIsRequired
	}
	if len(def.Edges) == 0 {
		return ErrAtLeastOneEdgeIsRequired
	}
	if def.Start == "" {
		return ErrStartNodeIsRequired
	}
	if def.Done == "" {
		return ErrDoneNodeIsRequired
	}

	nodeSet := make(map[NodeName]bool, len(def.Nodes))
	for i := range def.Nodes {
		if def.Nodes[i].Name == "" {
			return ErrNodeNameIsRequired
		}
		if nodeSet[def.Nodes[i].Name] {
			return fmt.Errorf("%w: %q", ErrDuplicateNodeName, def.Nodes[i].Name)
		}
		nodeSet[def.Nodes[i].Name] = true
	}

	if !nodeSet[def.Start] {
		return fmt.Errorf("%w: %q not found in node list", ErrStartNode, def.Start)
	}
	if def.Finally != "" && !nodeSet[def.Finally] {
		return fmt.Errorf("%w: finally node %q not found in node list", ErrNode, def.Finally)
	}

	edgeIDs := make(map[string]bool, len(def.Edges))
	for i := range def.Edges {
		if def.Edges[i].ID == "" {
			return ErrEdgeIdIsRequired
		}
		if edgeIDs[def.Edges[i].ID] {
			return fmt.Errorf("%w: %q", ErrDuplicateEdgeId, def.Edges[i].ID)
		}
		edgeIDs[def.Edges[i].ID] = true

		if !nodeSet[def.Edges[i].From] {
			return fmt.Errorf("%w: %s references unknown source node %q", ErrEdge, def.Edges[i].ID, def.Edges[i].From)
		}
		if def.Edges[i].To != def.Done && !nodeSet[def.Edges[i].To] {
			return fmt.Errorf("%w: %s references unknown target node %q", ErrEdge, def.Edges[i].ID, def.Edges[i].To)
		}
	}

	for zoneName, z := range def.Zones {
		for _, nodeName := range z.Nodes {
			if !nodeSet[nodeName] {
				return fmt.Errorf("%w: %q references unknown node %q", ErrZone, zoneName, nodeName)
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
	for i := range def.Nodes {
		n := &def.Nodes[i]
		if n.Code != "" && n.DisplayName != "" {
			v.RegisterEntry(n.Code, VocabEntry{Short: n.Code, Long: n.DisplayName})
			alias := n.Code + "_" + toUpperReplace(n.DisplayName, " ", "_")
			v.RegisterEntry(alias, VocabEntry{Short: n.Code, Long: n.DisplayName})
		}
		if n.DisplayName != "" {
			v.RegisterEntry(string(n.Name), VocabEntry{Long: n.DisplayName})
		}
	}
	for i := range def.Edges {
		if def.Edges[i].DisplayName != "" {
			v.RegisterEntry(def.Edges[i].ID, VocabEntry{Long: def.Edges[i].DisplayName})
		}
	}
}
