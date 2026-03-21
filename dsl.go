package framework

// Category: DSL & Build

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
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

// rawCircuitDef is the intermediate representation that supports both
// verbose (top-level edges) and compact (node-scoped edges) forms.
// normalize() converts it to the canonical CircuitDef.
type rawCircuitDef struct {
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
	Nodes       []rawNodeDef       `yaml:"nodes"`
	Edges       []EdgeDef          `yaml:"edges,omitempty"`
	Walkers     []WalkerDef        `yaml:"walkers,omitempty"`
	Start       string             `yaml:"start"`
	Done        string             `yaml:"done"`
	Calibration *CalibrationContractDef `yaml:"calibration,omitempty"`
}

// rawNodeDef extends NodeDef with optional inline edges.
type rawNodeDef struct {
	NodeDef `yaml:",inline"`
	Edges   rawEdgeList `yaml:"edges,omitempty"`
}

// rawEdgeDef is the compact edge form nested under a node.
// From is implicit (parent node name) and ID is auto-generated.
type rawEdgeDef struct {
	Name        string `yaml:"name,omitempty"`
	DisplayName string `yaml:"display_name,omitempty"`
	To          string `yaml:"to,omitempty"`
	Shortcut    bool   `yaml:"shortcut,omitempty"`
	Loop        bool   `yaml:"loop,omitempty"`
	Parallel    bool   `yaml:"parallel,omitempty"`
	When        string `yaml:"when,omitempty"`
	Merge       string `yaml:"merge,omitempty"`
}

// rawEdgeList handles both flow-style string sequences (edges: [target])
// and mapping sequences (edges: [{name: ..., to: ..., when: ...}]).
type rawEdgeList []rawEdgeDef

func (l *rawEdgeList) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.SequenceNode {
		return fmt.Errorf("edges must be a sequence")
	}
	for _, item := range value.Content {
		switch item.Kind {
		case yaml.ScalarNode:
			*l = append(*l, rawEdgeDef{To: item.Value})
		case yaml.MappingNode:
			var ed rawEdgeDef
			if err := item.Decode(&ed); err != nil {
				return err
			}
			*l = append(*l, ed)
		default:
			return fmt.Errorf("edge element must be a string or mapping")
		}
	}
	return nil
}

func (raw *rawCircuitDef) normalize() (*CircuitDef, error) {
	def := &CircuitDef{
		Circuit:     raw.Circuit,
		Description: raw.Description,
		Import:      raw.Import,
		Topology:    raw.Topology,
		HandlerType: raw.HandlerType,
		Timeout:     raw.Timeout,
		Imports:     raw.Imports,
		Vars:        raw.Vars,
		Extractors:  raw.Extractors,
		Ports:       raw.Ports,
		Wiring:      raw.Wiring,
		Zones:       raw.Zones,
		Walkers:     raw.Walkers,
		Start:       raw.Start,
		Done:        raw.Done,
		Calibration: raw.Calibration,
	}

	def.Nodes = make([]NodeDef, 0, len(raw.Nodes))
	for i := range raw.Nodes {
		def.Nodes = append(def.Nodes, raw.Nodes[i].NodeDef)
	}

	edgeIDs := make(map[string]int)
	def.Edges = append(def.Edges, raw.Edges...)
	for _, e := range raw.Edges {
		edgeIDs[e.ID]++
	}

	for i := range raw.Nodes {
		nodeName := raw.Nodes[i].Name
		for _, re := range raw.Nodes[i].Edges {
			if re.To == "" {
				return nil, fmt.Errorf("node %q: inline edge missing 'to' field", nodeName)
			}
			id := generateEdgeID(nodeName, re, edgeIDs)
			def.Edges = append(def.Edges, EdgeDef{
				ID:          id,
				Name:        re.Name,
				DisplayName: re.DisplayName,
				From:        nodeName,
				To:          re.To,
				Shortcut:    re.Shortcut,
				Loop:        re.Loop,
				Parallel:    re.Parallel,
				When:        re.When,
				Merge:       re.Merge,
			})
		}
	}

	return def, nil
}

func generateEdgeID(from string, e rawEdgeDef, seen map[string]int) string {
	base := from + "-" + e.To
	if e.Name != "" {
		base = from + "-" + strings.ReplaceAll(e.Name, " ", "-")
	}
	seen[base]++
	if seen[base] > 1 {
		return fmt.Sprintf("%s-%d", base, seen[base])
	}
	return base
}

// InferTopology computes shortcut and loop flags from graph topology.
// Pass 1: DFS cycle detection marks back edges as loops.
// Pass 2: for each non-loop forward edge, checks whether an indirect path
// (length >= 2) exists — if so, the edge is a shortcut.
// Edges to the done pseudo-node are excluded from shortcut inference.
func InferTopology(def *CircuitDef) {
	if def.Start == "" || len(def.Nodes) == 0 {
		return
	}

	edgesByNode := make(map[string][]int)
	for i, e := range def.Edges {
		edgesByNode[e.From] = append(edgesByNode[e.From], i)
	}

	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int)

	var dfs func(node string)
	dfs = func(node string) {
		color[node] = gray
		for _, idx := range edgesByNode[node] {
			e := &def.Edges[idx]
			if e.Loop {
				continue
			}
			switch color[e.To] {
			case white:
				dfs(e.To)
			case gray:
				e.Loop = true
			}
		}
		color[node] = black
	}
	dfs(def.Start)

	dagAdj := make(map[string][]string)
	for _, e := range def.Edges {
		if !e.Loop {
			dagAdj[e.From] = append(dagAdj[e.From], e.To)
		}
	}

	for i := range def.Edges {
		e := &def.Edges[i]
		if e.Loop || e.Shortcut || e.To == def.Done {
			continue
		}
		if hasIndirectPath(e.From, e.To, dagAdj) {
			e.Shortcut = true
		}
	}
}

// hasIndirectPath returns true if `to` is reachable from `from` via a path
// of length >= 2 (through at least one intermediate node).
func hasIndirectPath(from, to string, adj map[string][]string) bool {
	visited := map[string]bool{from: true}
	var queue []string
	for _, next := range adj[from] {
		if next != to && !visited[next] {
			visited[next] = true
			queue = append(queue, next)
		}
	}
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		if node == to {
			return true
		}
		for _, next := range adj[node] {
			if !visited[next] {
				visited[next] = true
				queue = append(queue, next)
			}
		}
	}
	return false
}

// NodeRegistry maps node family names to Node factory functions.
type NodeRegistry map[string]func(def NodeDef) Node

// EdgeFactory maps edge IDs to Edge factory functions.
type EdgeFactory map[string]func(def EdgeDef) Edge

// LoadCircuit parses a YAML circuit definition and returns a CircuitDef.
// Supports both verbose (top-level edges) and compact (node-scoped edges) forms.
func LoadCircuit(data []byte) (*CircuitDef, error) {
	var raw rawCircuitDef
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse circuit YAML: %w", err)
	}
	return raw.normalize()
}

// AssetResolver resolves a schematic name to its embedded circuit YAML.
// Implementations may read from an embed.FS, a manifest's asset map, or
// the local filesystem.
type AssetResolver func(schematicName string) ([]byte, error)

// LoadCircuitWithOverlay parses a consumer overlay YAML and, if it declares
// an import: field, loads the base circuit from the resolver and merges the
// overlay on top.
//
// Merge semantics:
//   - circuit, topology, handler_type, timeout, start, done, scorecard —
//     inherited from base unless explicitly set in overlay
//   - description — overlay wins if non-empty
//   - vars — shallow merge; overlay keys win
//   - nodes — overlay appends new nodes (by name); cannot override base nodes
//   - edges — overlay appends; edges whose ID matches a base edge replace it
//   - zones — shallow merge by zone name; overlay wins
//   - extractors — overlay appends new extractors; name collision replaces base
//   - walkers — overlay appends new walkers; name collision replaces base
//   - imports (component imports) — union, deduplicated
func LoadCircuitWithOverlay(overlayData []byte, resolver AssetResolver) (*CircuitDef, error) {
	overlay, err := LoadCircuit(overlayData)
	if err != nil {
		return nil, fmt.Errorf("parse overlay: %w", err)
	}

	if overlay.Import == "" {
		return overlay, nil
	}

	if resolver == nil {
		return nil, fmt.Errorf("overlay imports %q but no asset resolver provided", overlay.Import)
	}

	baseData, err := resolver(overlay.Import)
	if err != nil {
		return nil, fmt.Errorf("resolve import %q: %w", overlay.Import, err)
	}

	base, err := LoadCircuit(baseData)
	if err != nil {
		return nil, fmt.Errorf("parse base circuit %q: %w", overlay.Import, err)
	}

	slog.Debug(LogOverlayMerge, LogKeyComponent, LogComponentDSL,
		"base", overlay.Import,
		"base_nodes", len(base.Nodes),
		"overlay_nodes", len(overlay.Nodes),
		"overlay_edges", len(overlay.Edges))

	merged, err := mergeCircuits(base, overlay)
	if err != nil {
		return nil, fmt.Errorf("merge overlay onto %q: %w", overlay.Import, err)
	}

	slog.Debug(LogOverlayMergeComplete, LogKeyComponent, LogComponentDSL,
		"merged_nodes", len(merged.Nodes),
		"merged_edges", len(merged.Edges),
		"start", merged.Start,
		"done", merged.Done)

	return merged, nil
}

func mergeCircuits(base, overlay *CircuitDef) (*CircuitDef, error) {
	merged := *base
	merged.Import = ""

	if overlay.Circuit != "" {
		merged.Circuit = overlay.Circuit
	}
	if overlay.Description != "" {
		merged.Description = overlay.Description
	}
	if overlay.Topology != "" {
		merged.Topology = overlay.Topology
	}
	if overlay.HandlerType != "" {
		merged.HandlerType = overlay.HandlerType
	}
	if overlay.Timeout != "" {
		merged.Timeout = overlay.Timeout
	}
	if overlay.Start != "" {
		merged.Start = overlay.Start
	}
	if overlay.Done != "" {
		merged.Done = overlay.Done
	}
	if overlay.Scorecard != "" {
		merged.Scorecard = overlay.Scorecard
	}

	// Calibration: overlay wins (if overlay has calibration, it replaces base)
	if overlay.Calibration != nil {
		merged.Calibration = overlay.Calibration
	}

	// Vars: shallow merge, overlay wins
	if len(overlay.Vars) > 0 {
		if merged.Vars == nil {
			merged.Vars = make(map[string]any)
		}
		for k, v := range overlay.Vars {
			merged.Vars[k] = v
		}
	}

	// Zones: overlay wins per zone name
	if len(overlay.Zones) > 0 {
		if merged.Zones == nil {
			merged.Zones = make(map[string]ZoneDef)
		}
		for k, v := range overlay.Zones {
			merged.Zones[k] = v
		}
	}

	// Nodes: overlay appends new nodes by name
	if len(overlay.Nodes) > 0 {
		baseNodeSet := make(map[string]bool, len(merged.Nodes))
		for _, n := range merged.Nodes {
			baseNodeSet[n.Name] = true
		}
		for _, n := range overlay.Nodes {
			if baseNodeSet[n.Name] {
				return nil, fmt.Errorf("overlay cannot override base node %q (append-only)", n.Name)
			}
			merged.Nodes = append(merged.Nodes, n)
		}
	}

	// Edges: overlay appends; matching IDs replace base edges
	if len(overlay.Edges) > 0 {
		baseEdgeIdx := make(map[string]int, len(merged.Edges))
		for i, e := range merged.Edges {
			baseEdgeIdx[e.ID] = i
		}
		for _, oe := range overlay.Edges {
			if idx, exists := baseEdgeIdx[oe.ID]; exists {
				merged.Edges[idx] = oe
			} else {
				merged.Edges = append(merged.Edges, oe)
			}
		}
	}

	// Extractors: overlay appends; name collision replaces
	if len(overlay.Extractors) > 0 {
		baseExtIdx := make(map[string]int, len(merged.Extractors))
		for i, e := range merged.Extractors {
			baseExtIdx[e.Name] = i
		}
		for _, oe := range overlay.Extractors {
			if idx, exists := baseExtIdx[oe.Name]; exists {
				merged.Extractors[idx] = oe
			} else {
				merged.Extractors = append(merged.Extractors, oe)
			}
		}
	}

	// Walkers: overlay appends; name collision replaces
	if len(overlay.Walkers) > 0 {
		baseWalkerIdx := make(map[string]int, len(merged.Walkers))
		for i, w := range merged.Walkers {
			baseWalkerIdx[w.Name] = i
		}
		for _, ow := range overlay.Walkers {
			if idx, exists := baseWalkerIdx[ow.Name]; exists {
				merged.Walkers[idx] = ow
			} else {
				merged.Walkers = append(merged.Walkers, ow)
			}
		}
	}

	// Ports: base ports inherited; overlay appends new ports (name collision = error)
	if len(overlay.Ports) > 0 {
		basePortSet := make(map[string]bool, len(merged.Ports))
		for _, p := range merged.Ports {
			basePortSet[p.Name] = true
		}
		for _, op := range overlay.Ports {
			if basePortSet[op.Name] {
				return nil, fmt.Errorf("overlay cannot override base port %q (append-only)", op.Name)
			}
			merged.Ports = append(merged.Ports, op)
		}
	}

	// Wiring: overlay-only (appended as-is)
	if len(overlay.Wiring) > 0 {
		merged.Wiring = append(merged.Wiring, overlay.Wiring...)
	}

	// Component imports: union, deduplicated
	if len(overlay.Imports) > 0 {
		seen := make(map[string]bool, len(merged.Imports))
		for _, imp := range merged.Imports {
			seen[imp] = true
		}
		for _, imp := range overlay.Imports {
			if !seen[imp] {
				merged.Imports = append(merged.Imports, imp)
				seen[imp] = true
			}
		}
	}

	return &merged, nil
}

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
// the code→name and a "code_NAME" alias. Edges with DisplayName register
// the edge ID→name mapping.
func (def *CircuitDef) RegisterVocabulary(v *RichMapVocabulary) {
	for _, n := range def.Nodes {
		if n.Code != "" && n.DisplayName != "" {
			v.RegisterEntry(n.Code, VocabEntry{Short: n.Code, Long: n.DisplayName})
			alias := n.Code + "_" + strings.ToUpper(strings.ReplaceAll(n.DisplayName, " ", "_"))
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

// ComponentLoader resolves an import name (e.g. "core", "vendor.rca-tools")
// to a live Component with populated registries. BuildGraph calls the loader
// for each entry in CircuitDef.Imports.
type ComponentLoader func(name string) (*Component, error)

// GraphRegistries bundles all optional registries for BuildGraph.
// Fields are optional; BuildGraph resolves nodes by priority:
// Transformer > Extractor > NodeRegistry (Family/Name).
type GraphRegistries struct {
	Nodes        NodeRegistry
	Edges        EdgeFactory
	Extractors   ExtractorRegistry
	Renderers    RendererRegistry
	Transformers TransformerRegistry
	Hooks        HookRegistry
	Components   ComponentLoader
	Circuits         map[string]*CircuitDef
	MediatorEndpoint string // MCP endpoint for remote sub-circuit delegation
}

// BuildGraph constructs a Graph from a CircuitDef using the full registries bundle.
// Node resolution priority: Transformer > Extractor > NodeRegistry (Family/Name).
// Edge resolution priority: expressionEdge (When) > EdgeFactory > dslEdge.
// When CircuitDef.Imports is non-empty and reg.Components is set, imported
// components are loaded and merged into the registries before node resolution.
func (def *CircuitDef) BuildGraph(reg GraphRegistries) (Graph, error) {
	if err := def.Validate(); err != nil {
		return nil, fmt.Errorf("validate: %w", err)
	}

	if len(def.Imports) > 0 && reg.Components != nil {
		comps := make([]*Component, 0, len(def.Imports))
		for _, imp := range def.Imports {
			c, err := reg.Components(imp)
			if err != nil {
				return nil, fmt.Errorf("import %q: %w", imp, err)
			}
			comps = append(comps, c)
		}
		merged, err := MergeComponents(reg, comps...)
		if err != nil {
			return nil, fmt.Errorf("merge imports: %w", err)
		}
		reg.Transformers = merged.Transformers
		reg.Extractors = merged.Extractors
		reg.Hooks = merged.Hooks
	}

	fwNodes := make([]Node, 0, len(def.Nodes))
	for _, nd := range def.Nodes {
		node, err := def.resolveNode(nd, reg)
		if err != nil {
			return nil, err
		}
		fwNodes = append(fwNodes, node)
	}

	fwEdges := make([]Edge, 0, len(def.Edges))
	for _, ed := range def.Edges {
		if ed.When != "" {
			exprEdge, err := CompileExpressionEdge(ed, def.Vars)
			if err != nil {
				return nil, fmt.Errorf("edge %s: %w", ed.ID, err)
			}
			fwEdges = append(fwEdges, exprEdge)
		} else if reg.Edges != nil {
			if factory, ok := reg.Edges[ed.ID]; ok {
				fwEdges = append(fwEdges, factory(ed))
			} else {
				fwEdges = append(fwEdges, &dslEdge{def: ed})
			}
		} else {
			fwEdges = append(fwEdges, &dslEdge{def: ed})
		}
	}

	fwZones := make([]Zone, 0, len(def.Zones))
	for name, zd := range def.Zones {
		elem, _ := ResolveApproach(strings.ToLower(zd.Approach))
		fwZones = append(fwZones, Zone{
			Name:            name,
			NodeNames:       zd.Nodes,
			ElementAffinity: elem,
			Stickiness:      zd.Stickiness,
			Domain:          strings.ToLower(zd.Domain),
			ContextFilter:   zd.ContextFilter,
		})
	}

	var timeouts map[string]time.Duration
	for _, nd := range def.Nodes {
		d, err := nd.EffectiveTimeout(def.Timeout)
		if err != nil {
			return nil, err
		}
		if d > 0 {
			if timeouts == nil {
				timeouts = make(map[string]time.Duration)
			}
			timeouts[nd.Name] = d
		}
	}

	opts := []GraphOption{WithDoneNode(def.Done)}
	if len(timeouts) > 0 {
		opts = append(opts, WithNodeTimeouts(timeouts))
	}

	g, err := NewGraph(def.Circuit, fwNodes, fwEdges, fwZones, opts...)
	if err != nil {
		return nil, err
	}
	g.registries = &reg

	if def.Topology != "" {
		if err := validateTopology(g, def); err != nil {
			return nil, err
		}
	}

	runBuildDiagnostics(def, reg)

	return g, nil
}

// TopologyValidator validates a built graph's structure against a named topology.
// When nil, topology validation is skipped. The topology/ package provides the
// default implementation via RegisterTopologyValidator.
type TopologyValidator func(topoName string, shape GraphShape) error

// GraphShape describes the structural properties of a graph for topology validation.
type GraphShape struct {
	StartNode string
	DoneNode  string
	Nodes     []GraphNodeInfo
}

// GraphNodeInfo describes a single node's edge cardinality.
type GraphNodeInfo struct {
	Name    string
	Inputs  int
	Outputs int
}

// DefaultTopologyValidator is the active topology validation function.
// It is nil until a topology package registers itself via
// RegisterTopologyValidator. When nil, BuildGraph skips topology checks.
var DefaultTopologyValidator TopologyValidator

// RegisterTopologyValidator sets the default topology validator.
// Called by topology/ init() to wire in the built-in topology registry.
func RegisterTopologyValidator(v TopologyValidator) {
	DefaultTopologyValidator = v
}

// validateTopology checks the graph against the declared topology.
func validateTopology(g *DefaultGraph, def *CircuitDef) error {
	v := DefaultTopologyValidator
	if v == nil {
		slog.Warn("topology validator not registered, skipping validation",
			"component", "build",
			"topology", def.Topology,
			"circuit", def.Circuit,
		)
		return nil
	}
	shape := buildGraphShape(g, def)
	return v(def.Topology, shape)
}

func buildGraphShape(g *DefaultGraph, def *CircuitDef) GraphShape {
	nodes := make([]GraphNodeInfo, 0, len(g.nodes))
	for _, n := range g.nodes {
		inputs := 0
		for _, e := range g.edges {
			if e.To() == n.Name() && !e.IsShortcut() && !e.IsLoop() {
				inputs++
			}
		}
		outputs := 0
		for _, e := range g.edges {
			if e.From() == n.Name() && !e.IsShortcut() && !e.IsLoop() {
				outputs++
			}
		}
		nodes = append(nodes, GraphNodeInfo{
			Name:    n.Name(),
			Inputs:  inputs,
			Outputs: outputs,
		})
	}
	return GraphShape{
		StartNode: def.Start,
		DoneNode:  g.doneNode,
		Nodes:     nodes,
	}
}

// resolveNode creates a Node from a NodeDef using handler + handler_type.
func (def *CircuitDef) resolveNode(nd NodeDef, reg GraphRegistries) (Node, error) {
	elem, _ := ResolveApproach(strings.ToLower(nd.Approach))
	return def.resolveHandler(nd, reg, elem)
}

// resolveHandler resolves a node using the explicit handler + handler_type path.
func (def *CircuitDef) resolveHandler(nd NodeDef, reg GraphRegistries, elem Element) (Node, error) {
	handler := nd.Handler
	if handler == "" {
		handler = nd.Name
	}
	ht := nd.HandlerType
	if ht == "" {
		ht = def.HandlerType
	}
	if ht == "" {
		return nil, fmt.Errorf("node %q: handler %q specified but no handler_type on node or circuit", nd.Name, handler)
	}

	switch ht {
	case HandlerTypeTransformer:
		t, err := def.resolveTransformerByName(handler, nd.Name, reg)
		if err != nil {
			return nil, err
		}
		return &transformerNode{
			name:     nd.Name,
			element:  elem,
			trans:    t,
			prompt:   nd.Prompt,
			input:    nd.Input,
			provider: nd.Provider,
			config:   def.Vars,
			meta:     nd.Meta,
		}, nil

	case HandlerTypeExtractor:
		ext, err := def.resolveExtractor(handler, nd, reg)
		if err != nil {
			return nil, err
		}
		return &extractorNode{
			name:    nd.Name,
			element: elem,
			ext:     ext,
			meta:    nd.Meta,
		}, nil

	case HandlerTypeRenderer:
		rnd, err := def.resolveRenderer(handler, nd, reg)
		if err != nil {
			return nil, err
		}
		return &rendererNode{
			name:    nd.Name,
			element: elem,
			rnd:     rnd,
			meta:    nd.Meta,
		}, nil

	case HandlerTypeNode:
		if reg.Nodes == nil {
			return nil, fmt.Errorf("node %q: handler %q not found (node registry is nil)", nd.Name, handler)
		}
		factory, ok := reg.Nodes[handler]
		if !ok {
			return nil, fmt.Errorf("node %q: handler %q not found in node registry", nd.Name, handler)
		}
		return factory(nd), nil

	case HandlerTypeDelegate:
		if reg.Transformers == nil {
			return nil, fmt.Errorf("node %q: delegate handler %q not found (transformer registry is nil)", nd.Name, handler)
		}
		gen, err := reg.Transformers.Get(handler)
		if err != nil {
			return nil, fmt.Errorf("node %q: delegate handler: %w", nd.Name, err)
		}
		return &dslDelegateNode{
			name:    nd.Name,
			element: elem,
			gen:     gen,
			config:  def.Vars,
			meta:    nd.Meta,
		}, nil

	case HandlerTypeCircuit:
		slog.Debug("resolve circuit handler",
			"component", "build",
			"node", nd.Name,
			"handler", handler,
			"circuits_nil", reg.Circuits == nil,
			"circuits_count", len(reg.Circuits),
			"mediator_endpoint", reg.MediatorEndpoint,
		)
		// Local resolution first.
		if reg.Circuits != nil {
			if cd, ok := reg.Circuits[handler]; ok {
				slog.Debug("circuit handler resolved locally",
					"component", "build",
					"node", nd.Name,
					"handler", handler,
				)
				return &circuitRefNode{
					name:       nd.Name,
					element:    elem,
					circuitDef: cd,
					meta:       nd.Meta,
				}, nil
			}
		}
		// Mediator fallback: delegate to remote schematic via MCP.
		if reg.MediatorEndpoint != "" {
			slog.Debug("circuit handler delegating to mediator",
				"component", "build",
				"node", nd.Name,
				"handler", handler,
				"endpoint", reg.MediatorEndpoint,
			)
			return &transformerNode{
				name:    nd.Name,
				element: elem,
				trans:   &mcpCircuitTransformer{circuitType: handler, endpoint: reg.MediatorEndpoint},
				config:  def.Vars,
				meta:    nd.Meta,
			}, nil
		}
		return nil, fmt.Errorf("node %q: circuit handler %q not found (no local circuit and no mediator endpoint)", nd.Name, handler)

	default:
		return nil, fmt.Errorf("node %q: unknown handler_type %q", nd.Name, ht)
	}
}

// resolveTransformerByName resolves a transformer by name, checking builtins first.
func (def *CircuitDef) resolveTransformerByName(name, nodeName string, reg GraphRegistries) (Transformer, error) {
	switch name {
	case BuiltinTransformerGoTemplate:
		return &goTemplateTransformer{}, nil
	case BuiltinTransformerPassthrough:
		return &passthroughTransformer{}, nil
	}
	if reg.Transformers == nil {
		return nil, fmt.Errorf("node %q: transformer %q not found (registry is nil)", nodeName, name)
	}
	return reg.Transformers.Get(name)
}

// dslEdge is a default Edge implementation created from an EdgeDef when
// no custom factory is registered. It always matches (returns a transition).
type dslEdge struct {
	def EdgeDef
}

func (e *dslEdge) ID() string         { return e.def.ID }
func (e *dslEdge) From() string       { return e.def.From }
func (e *dslEdge) To() string         { return e.def.To }
func (e *dslEdge) IsShortcut() bool   { return e.def.Shortcut }
func (e *dslEdge) IsLoop() bool       { return e.def.Loop }
func (e *dslEdge) IsParallel() bool   { return e.def.Parallel }
func (e *dslEdge) Evaluate(_ Artifact, _ *WalkerState) *Transition {
	return &Transition{
		NextNode:    e.def.To,
		Explanation: e.def.Condition,
	}
}

// resolveExtractor resolves an extractor by name.
// Priority: built-in name → circuit-level ExtractorDef → ExtractorRegistry.
func (def *CircuitDef) resolveExtractor(name string, nd NodeDef, reg GraphRegistries) (Extractor, error) {
	switch name {
	case BuiltinExtractorJSONSchema:
		return &JSONSchemaExtractor{schema: nd.Schema}, nil
	case BuiltinExtractorRegex:
		pattern, _ := nd.Meta["pattern"].(string)
		if pattern == "" {
			return nil, fmt.Errorf("node %q: regex extractor requires meta.pattern", nd.Name)
		}
		return NewRegexExtractor(nd.Name, pattern)
	}

	for _, ed := range def.Extractors {
		if ed.Name != name {
			continue
		}
		switch ed.Type {
		case BuiltinExtractorJSONSchema:
			schema := ed.Schema
			if nd.Schema != nil {
				schema = nd.Schema
			}
			return &JSONSchemaExtractor{schema: schema}, nil
		case BuiltinExtractorRegex:
			if ed.Pattern == "" {
				return nil, fmt.Errorf("extractor %q: regex type requires pattern", ed.Name)
			}
			return NewRegexExtractor(ed.Name, ed.Pattern)
		default:
			return nil, fmt.Errorf("extractor %q: unknown type %q", ed.Name, ed.Type)
		}
	}

	if reg.Extractors != nil {
		ext, err := reg.Extractors.Get(name)
		if err == nil {
			return ext, nil
		}
	}
	return nil, fmt.Errorf("node %q: extractor %q not found", nd.Name, name)
}

// resolveRenderer resolves a renderer by name.
// Priority: built-in name → RendererRegistry.
func (def *CircuitDef) resolveRenderer(name string, nd NodeDef, reg GraphRegistries) (Renderer, error) {
	if name == BuiltinRendererTemplate {
		return &TemplateRenderer{Template: nd.Prompt}, nil
	}
	if reg.Renderers != nil {
		rnd, err := reg.Renderers.Get(name)
		if err == nil {
			return rnd, nil
		}
	}
	return nil, fmt.Errorf("node %q: renderer %q not found", nd.Name, name)
}
