package circuit

// Category: DSL & Build — circuit YAML loading and overlay merge.

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
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

func (l *rawEdgeList) UnmarshalYAML(value *yamlNode) error {
	if value.Kind != yamlSequenceNode {
		return fmt.Errorf("edges must be a sequence")
	}
	for _, item := range value.Content {
		switch item.Kind {
		case yamlScalarNode:
			*l = append(*l, rawEdgeDef{To: item.Value})
		case yamlMappingNode:
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
		Start:       NodeName(raw.Start),
		Done:        NodeName(raw.Done),
		Calibration: raw.Calibration,
	}

	def.Nodes = make([]NodeDef, 0, len(raw.Nodes))
	for i := range raw.Nodes {
		def.Nodes = append(def.Nodes, raw.Nodes[i].NodeDef)
	}

	edgeIDs := make(map[string]int)
	def.Edges = append(def.Edges, raw.Edges...)
	for i := range raw.Edges {
		edgeIDs[raw.Edges[i].ID]++
	}

	for i := range raw.Nodes {
		nodeName := raw.Nodes[i].Name
		for _, re := range raw.Nodes[i].Edges {
			if re.To == "" {
				return nil, fmt.Errorf("node %q: inline edge missing 'to' field", nodeName)
			}
			id := generateEdgeID(string(nodeName), &re, edgeIDs)
			def.Edges = append(def.Edges, EdgeDef{
				ID:          id,
				Name:        re.Name,
				DisplayName: re.DisplayName,
				From:        nodeName,
				To:          NodeName(re.To),
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

func generateEdgeID(from string, e *rawEdgeDef, seen map[string]int) string {
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

// AssetResolver resolves a schematic name to its embedded circuit YAML.
// Implementations may read from an embed.FS, a manifest's asset map, or
// the local filesystem.
type AssetResolver func(schematicName string) ([]byte, error)

// LoadCircuit parses a YAML circuit definition and returns a CircuitDef.
// Supports both verbose (top-level edges) and compact (node-scoped edges) forms.
func LoadCircuit(data []byte) (*CircuitDef, error) {
	var raw rawCircuitDef
	if err := yamlUnmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse circuit YAML: %w", err)
	}
	return raw.normalize()
}

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

	slog.DebugContext(context.Background(), LogOverlayMerge, LogKeyComponent, LogComponentDSL,
		"base", overlay.Import,
		"base_nodes", len(base.Nodes),
		"overlay_nodes", len(overlay.Nodes),
		"overlay_edges", len(overlay.Edges))

	merged, err := mergeCircuits(base, overlay)
	if err != nil {
		return nil, fmt.Errorf("merge overlay onto %q: %w", overlay.Import, err)
	}

	slog.DebugContext(context.Background(), LogOverlayMergeComplete, LogKeyComponent, LogComponentDSL,
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
		baseNodeSet := make(map[NodeName]bool, len(merged.Nodes))
		for i := range merged.Nodes {
			baseNodeSet[merged.Nodes[i].Name] = true
		}
		for i := range overlay.Nodes {
			if baseNodeSet[overlay.Nodes[i].Name] {
				return nil, fmt.Errorf("overlay cannot override base node %q (append-only)", overlay.Nodes[i].Name)
			}
			merged.Nodes = append(merged.Nodes, overlay.Nodes[i])
		}
	}

	// Edges: overlay appends; matching IDs replace base edges
	if len(overlay.Edges) > 0 {
		baseEdgeIdx := make(map[string]int, len(merged.Edges))
		for i := range merged.Edges {
			baseEdgeIdx[merged.Edges[i].ID] = i
		}
		for i := range overlay.Edges {
			if idx, exists := baseEdgeIdx[overlay.Edges[i].ID]; exists {
				merged.Edges[idx] = overlay.Edges[i]
			} else {
				merged.Edges = append(merged.Edges, overlay.Edges[i])
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
