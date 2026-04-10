package lint

import (
	"fmt"
	"strings"
)

const (
	ruleExpectedPath    = "S35/expected-path-node-names"
	ruleHandlerResolve  = "S36/circuit-handler-resolution"
	ruleDeadNode        = "S37/dead-node-detection"
	ruleMediatorBackend = "S38/mediator-backend-coverage"
	rulePortConsistency = "S39/port-type-consistency"
)

// --- S35: expected-path-node-names ---

// ExpectedPathNodeNames checks that expected_path elements in scenario files
// are valid node names from the circuit definition.
type ExpectedPathNodeNames struct{}

func (r *ExpectedPathNodeNames) ID() string { return ruleExpectedPath }
func (r *ExpectedPathNodeNames) Description() string {
	return "expected_path elements must be valid circuit node names"
}
func (r *ExpectedPathNodeNames) Severity() Severity { return SeverityError }
func (r *ExpectedPathNodeNames) Tags() []string     { return []string{"scenario", "cross-ref"} }

func (r *ExpectedPathNodeNames) Check(ctx *LintContext) []Finding {
	if ctx.Def == nil || len(ctx.ProjectFiles) == 0 {
		return nil
	}

	// Collect valid node names from the circuit definition.
	nodeNames := make(map[string]bool, len(ctx.Def.Nodes))
	for i := range ctx.Def.Nodes {
		nodeNames[string(ctx.Def.Nodes[i].Name)] = true
	}
	// Include start and done as valid names.
	if ctx.Def.Start != "" {
		nodeNames[string(ctx.Def.Start)] = true
	}
	if ctx.Def.Done != "" {
		nodeNames[string(ctx.Def.Done)] = true
	}

	if len(nodeNames) == 0 {
		return nil
	}

	var findings []Finding
	for _, pf := range ctx.ProjectFiles["scenario"] {
		paths := extractExpectedPaths(pf.Data)
		for _, ep := range paths {
			if !nodeNames[ep] {
				findings = append(findings, Finding{
					RuleID:   r.ID(),
					Severity: r.Severity(),
					Message:  fmt.Sprintf("expected_path element %q in %s is not a valid circuit node name; known nodes: %s", ep, pf.File, sortedKeys(nodeNames)),
					File:     pf.File,
				})
			}
		}
	}
	return findings
}

// extractExpectedPaths extracts all expected_path string values from a
// scenario document. Supports both cases[].expected_path (array of strings)
// and rcas[].expected_path.
func extractExpectedPaths(doc map[string]any) []string {
	var result []string

	// Try cases[].expected_path[]
	casePaths := ExtractPath(doc, "cases[].expected_path[]")
	for _, v := range casePaths {
		if s, ok := v.(string); ok {
			result = append(result, s)
		}
	}
	if len(result) > 0 {
		return result
	}

	// Fallback: try rcas[].expected_path[]
	rcaPaths := ExtractPath(doc, "rcas[].expected_path[]")
	for _, v := range rcaPaths {
		if s, ok := v.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

// --- S36: circuit-handler-resolution ---

// CircuitHandlerResolution checks that nodes with instrument=circuit reference
// a resolvable circuit file.
type CircuitHandlerResolution struct{}

func (r *CircuitHandlerResolution) ID() string { return ruleHandlerResolve }
func (r *CircuitHandlerResolution) Description() string {
	return "instrument=circuit action must resolve to a known circuit file"
}
func (r *CircuitHandlerResolution) Severity() Severity { return SeverityError }
func (r *CircuitHandlerResolution) Tags() []string     { return []string{"structural", "cross-ref"} }

func (r *CircuitHandlerResolution) Check(ctx *LintContext) []Finding {
	if ctx.Def == nil {
		return nil
	}

	// Build set of known circuit names from ProjectFiles.
	knownCircuits := make(map[string]bool)
	for _, pf := range ctx.ProjectFiles["circuit"] {
		if name, ok := pf.Data["circuit"].(string); ok {
			knownCircuits[name] = true
		}
	}

	// Also check Registries.Circuits if available.
	if ctx.Registries != nil {
		for name := range ctx.Registries.Circuits {
			knownCircuits[name] = true
		}
	}

	findings := make([]Finding, 0, len(ctx.Def.Nodes))
	for i := range ctx.Def.Nodes {
		nd := &ctx.Def.Nodes[i]
		if nd.Instrument != kindCircuit {
			continue
		}
		action := nd.Action
		if action == "" {
			action = string(nd.Name)
		}
		if action == "" {
			continue
		}
		if knownCircuits[action] {
			continue
		}

		msg := fmt.Sprintf("node %q: instrument=circuit action=%q does not resolve to any known circuit file", nd.Name, action)

		// Check if the referenced circuit uses import: which may need a resolver.
		for _, pf := range ctx.ProjectFiles["circuit"] {
			if cname, ok := pf.Data["circuit"].(string); ok && cname == action {
				if imp, ok := pf.Data["import"].(string); ok && imp != "" {
					msg += fmt.Sprintf(" (circuit %q has import: %q, may need a resolver)", action, imp)
				}
			}
		}

		findings = append(findings, Finding{
			RuleID:   r.ID(),
			Severity: r.Severity(),
			Message:  msg,
			File:     ctx.File,
			Line:     ctx.NodeLine(string(nd.Name)),
		})
	}
	return findings
}

// --- S37: dead-node-detection ---

// DeadNodeDetection finds nodes that appear in edges (reachable) but never
// appear in any scenario expected_path (untested).
type DeadNodeDetection struct{}

func (r *DeadNodeDetection) ID() string { return ruleDeadNode }
func (r *DeadNodeDetection) Description() string {
	return "reachable nodes not covered by any scenario expected_path are untested"
}
func (r *DeadNodeDetection) Severity() Severity { return SeverityWarning }
func (r *DeadNodeDetection) Tags() []string     { return []string{"scenario", "cross-ref"} }

func (r *DeadNodeDetection) Check(ctx *LintContext) []Finding {
	if ctx.Def == nil || len(ctx.ProjectFiles) == 0 {
		return nil
	}

	// Collect all nodes referenced in edges (reachable graph nodes).
	edgeNodes := make(map[string]bool)
	for i := range ctx.Def.Edges {
		edgeNodes[string(ctx.Def.Edges[i].From)] = true
		edgeNodes[string(ctx.Def.Edges[i].To)] = true
	}

	// Collect all nodes from all scenario expected_path arrays.
	testedNodes := make(map[string]bool)
	for _, pf := range ctx.ProjectFiles["scenario"] {
		for _, ep := range extractExpectedPaths(pf.Data) {
			testedNodes[ep] = true
		}
	}

	if len(testedNodes) == 0 {
		return nil // no scenarios with expected_path — rule doesn't apply
	}

	// Exclude start, done, and internal nodes.
	excluded := map[string]bool{
		"_done": true,
		"start": true,
	}
	if ctx.Def.Start != "" {
		excluded[string(ctx.Def.Start)] = true
	}
	if ctx.Def.Done != "" {
		excluded[string(ctx.Def.Done)] = true
	}

	findings := make([]Finding, 0, len(ctx.Def.Nodes))
	for i := range ctx.Def.Nodes {
		if excluded[string(ctx.Def.Nodes[i].Name)] {
			continue
		}
		if !edgeNodes[string(ctx.Def.Nodes[i].Name)] {
			continue // not reachable via edges — already caught by G1/orphan-node
		}
		if testedNodes[string(ctx.Def.Nodes[i].Name)] {
			continue // covered by at least one scenario
		}
		findings = append(findings, Finding{
			RuleID:   r.ID(),
			Severity: r.Severity(),
			Message:  fmt.Sprintf("node %q is reachable via edges but never appears in any scenario expected_path", ctx.Def.Nodes[i].Name),
			File:     ctx.File,
			Line:     ctx.NodeLine(string(ctx.Def.Nodes[i].Name)),
		})
	}
	return findings
}

// --- S38: mediator-backend-coverage ---

// MediatorBackendCoverage warns when instrument=circuit nodes have neither
// a local circuit definition nor a registered resolver, meaning they depend
// on a mediator endpoint at runtime.
type MediatorBackendCoverage struct{}

func (r *MediatorBackendCoverage) ID() string { return ruleMediatorBackend }
func (r *MediatorBackendCoverage) Description() string {
	return "instrument=circuit nodes should have a local circuit or registered resolver"
}
func (r *MediatorBackendCoverage) Severity() Severity { return SeverityWarning }
func (r *MediatorBackendCoverage) Tags() []string     { return []string{"structural"} }

func (r *MediatorBackendCoverage) Check(ctx *LintContext) []Finding {
	if ctx.Def == nil {
		return nil
	}

	// Build set of locally known circuits from ProjectFiles.
	localCircuits := make(map[string]bool)
	for _, pf := range ctx.ProjectFiles["circuit"] {
		if name, ok := pf.Data["circuit"].(string); ok {
			localCircuits[name] = true
		}
	}

	// Check Registries.Circuits for resolver coverage.
	hasResolver := func(name string) bool {
		if ctx.Registries == nil {
			return false
		}
		if _, ok := ctx.Registries.Circuits[name]; ok {
			return true
		}
		return false
	}

	hasMediatorEndpoint := ctx.Registries != nil && ctx.Registries.MediatorEndpoint != ""

	findings := make([]Finding, 0, len(ctx.Def.Nodes))
	for i := range ctx.Def.Nodes {
		nd := &ctx.Def.Nodes[i]
		if nd.Instrument != kindCircuit {
			continue
		}
		action := nd.Action
		if action == "" {
			action = string(nd.Name)
		}
		if action == "" {
			continue
		}
		if localCircuits[action] || hasResolver(action) {
			continue
		}

		msg := fmt.Sprintf("node %q: instrument=circuit action=%q has no local circuit definition and no registered resolver", nd.Name, action)
		if hasMediatorEndpoint {
			msg += fmt.Sprintf("; will fall back to mediator endpoint %s at runtime", ctx.Registries.MediatorEndpoint)
		} else {
			msg += "; no mediator endpoint configured — resolution will fail at runtime"
		}

		findings = append(findings, Finding{
			RuleID:   r.ID(),
			Severity: r.Severity(),
			Message:  msg,
			File:     ctx.File,
			Line:     ctx.NodeLine(string(nd.Name)),
		})
	}
	return findings
}

// --- S39: port-type-consistency ---

// PortTypeConsistency checks that wiring entries connect ports with matching types.
type PortTypeConsistency struct{}

func (r *PortTypeConsistency) ID() string { return rulePortConsistency }
func (r *PortTypeConsistency) Description() string {
	return "wiring from/to ports must have matching types"
}
func (r *PortTypeConsistency) Severity() Severity { return SeverityWarning }
func (r *PortTypeConsistency) Tags() []string     { return []string{"structural"} }

func (r *PortTypeConsistency) Check(ctx *LintContext) []Finding {
	if ctx.Def == nil || len(ctx.Def.Wiring) == 0 {
		return nil
	}

	// Build a map of port declarations: circuit_name → port_name → PortDef.
	// Local circuit ports.
	localPorts := make(map[string]string) // port_name → type
	for _, p := range ctx.Def.Ports {
		localPorts[p.Name] = p.Type
	}

	// Collect ports from project files (other circuits).
	circuitPorts := make(map[string]map[string]string) // circuit_name → port_name → type
	circuitPorts[ctx.Def.Circuit] = localPorts

	for _, pf := range ctx.ProjectFiles["circuit"] {
		cname, _ := pf.Data["circuit"].(string)
		if cname == "" {
			continue
		}
		ports := extractPortTypes(pf.Data)
		if len(ports) > 0 {
			circuitPorts[cname] = ports
		}
	}

	var findings []Finding
	for _, w := range ctx.Def.Wiring {
		fromCircuit, _, fromPort := parseWiringRef(w.From)
		toCircuit, _, toPort := parseWiringRef(w.To)

		if fromCircuit == "" || toCircuit == "" || fromPort == "" || toPort == "" {
			continue // malformed wiring entry — handled elsewhere
		}

		fromPorts, fromOK := circuitPorts[fromCircuit]
		toPorts, toOK := circuitPorts[toCircuit]
		if !fromOK || !toOK {
			continue // circuit not found — can't check
		}

		fromType, fromExists := fromPorts[fromPort]
		toType, toExists := toPorts[toPort]
		if !fromExists || !toExists {
			continue // port not declared — can't check type
		}

		if fromType == "" || toType == "" {
			continue // no type declared — nothing to compare
		}

		if fromType != toType {
			findings = append(findings, Finding{
				RuleID:   r.ID(),
				Severity: r.Severity(),
				Message: fmt.Sprintf("wiring %s → %s: port type mismatch: %s has type %q but %s has type %q",
					w.From, w.To, w.From, fromType, w.To, toType),
				File: ctx.File,
			})
		}
	}
	return findings
}

// parseWiringRef parses a wiring reference like "rca.out:post-triage"
// into (circuit, direction, port_name).
func parseWiringRef(ref string) (circuit, direction, port string) {
	// Format: circuit.direction:port_name
	dotIdx := strings.Index(ref, ".")
	if dotIdx < 0 {
		return "", "", ""
	}
	circuit = ref[:dotIdx]
	rest := ref[dotIdx+1:]
	direction, port, _ = strings.Cut(rest, ":")
	return circuit, direction, port
}

// extractPortTypes extracts port name → type mappings from a parsed
// circuit document's ports array.
func extractPortTypes(doc map[string]any) map[string]string {
	result := make(map[string]string)
	portsRaw, ok := doc["ports"]
	if !ok {
		return result
	}
	portsSlice, ok := toSlice(portsRaw)
	if !ok {
		return result
	}
	for _, item := range portsSlice {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name, _ := m["name"].(string)
		ptype, _ := m["type"].(string)
		if name != "" {
			result[name] = ptype
		}
	}
	return result
}
