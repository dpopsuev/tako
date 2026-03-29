package topology

import "fmt"

// NodeInfo is the minimal node representation for topology validation.
type NodeInfo struct {
	Name    string
	Inputs  int // number of incoming edges (excluding shortcut/loop)
	Outputs int // number of outgoing edges (excluding shortcut/loop)
}

// GraphShape is the minimal graph interface needed for topology validation.
// The framework provides an adapter from its Graph interface.
type GraphShape interface {
	StartNode() string
	DoneNode() string
	NodeInfos() []NodeInfo
	NodeCount() int
}

// Validate checks a graph's shape against a topology definition.
// It validates both node count and per-node input/output cardinality.
func Validate(shape GraphShape, def *TopologyDef) *ValidationResult {
	result := &ValidationResult{}

	if def.MinNodes > 0 && shape.NodeCount() < def.MinNodes {
		result.Violations = append(result.Violations, Violation{
			Field:    "node_count",
			Expected: fmt.Sprintf(">= %d", def.MinNodes),
			Actual:   shape.NodeCount(),
		})
	}
	if def.MaxNodes > 0 && shape.NodeCount() > def.MaxNodes {
		result.Violations = append(result.Violations, Violation{
			Field:    "node_count",
			Expected: fmt.Sprintf("<= %d", def.MaxNodes),
			Actual:   shape.NodeCount(),
		})
	}

	start := shape.StartNode()
	done := shape.DoneNode()

	for _, ni := range shape.NodeInfos() {
		pos := classifyPosition(ni.Name, start, done, ni.Outputs)
		rule := def.RuleFor(pos)
		if rule == nil {
			continue
		}

		if rule.MinInputs >= 0 && ni.Inputs < rule.MinInputs {
			result.Violations = append(result.Violations, Violation{
				NodeName: ni.Name,
				Position: pos,
				Field:    "inputs",
				Expected: cardinalityRange(rule.MinInputs, rule.MaxInputs),
				Actual:   ni.Inputs,
			})
		}
		if rule.MaxInputs >= 0 && ni.Inputs > rule.MaxInputs {
			result.Violations = append(result.Violations, Violation{
				NodeName: ni.Name,
				Position: pos,
				Field:    "inputs",
				Expected: cardinalityRange(rule.MinInputs, rule.MaxInputs),
				Actual:   ni.Inputs,
			})
		}
		if rule.MinOutputs >= 0 && ni.Outputs < rule.MinOutputs {
			result.Violations = append(result.Violations, Violation{
				NodeName: ni.Name,
				Position: pos,
				Field:    "outputs",
				Expected: cardinalityRange(rule.MinOutputs, rule.MaxOutputs),
				Actual:   ni.Outputs,
			})
		}
		if rule.MaxOutputs >= 0 && ni.Outputs > rule.MaxOutputs {
			result.Violations = append(result.Violations, Violation{
				NodeName: ni.Name,
				Position: pos,
				Field:    "outputs",
				Expected: cardinalityRange(rule.MinOutputs, rule.MaxOutputs),
				Actual:   ni.Outputs,
			})
		}
	}

	return result
}

func classifyPosition(name, start, done string, outputs int) Position {
	if name == start {
		return PositionEntry
	}
	if outputs == 0 || name == done {
		return PositionExit
	}
	return PositionIntermediate
}

func cardinalityRange(lo, hi int) string {
	if lo == hi {
		return fmt.Sprintf("exactly %d", lo)
	}
	if hi < 0 {
		return fmt.Sprintf(">= %d", lo)
	}
	return fmt.Sprintf("%d..%d", lo, hi)
}
