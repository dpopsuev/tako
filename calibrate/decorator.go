package calibrate

import (
	"github.com/dpopsuev/origami/circuit"
)

// CalibrationContract declares how a circuit's inputs and outputs map to
// scorer-addressable field names. Schematics embed this in their circuit.yaml
// so the calibration decorator knows which fields to measure.
type CalibrationContract struct {
	Inputs  []ContractField `yaml:"inputs,omitempty"`
	Outputs []ContractField `yaml:"outputs,omitempty"`
}

// ContractField maps a circuit artifact field to a scorer-addressable name.
type ContractField struct {
	Field      string `yaml:"field"`       // path in artifact (e.g. "output.defect_type")
	ScorerName string `yaml:"scorer_name"` // name used by scorer (e.g. "defect_type")
	Type       string `yaml:"type"`        // string, float, bool, array
}

// DecoratorConfig configures a calibration-decorated circuit.
type DecoratorConfig struct {
	Scorecard *ScorecardDef
	Contract  *CalibrationContract
}

// ScorecardDef is a reference to a scorecard configuration.
// The actual scorecard loading is handled by the calibration runner.
type ScorecardDef struct {
	Path string
}

// Wrap decorates a CircuitDef with calibration measurement points.
// The returned CircuitDef has the same topology but records node outputs
// for scoring. Wrap does NOT modify the original — it returns a new def.
//
// The decorator approach means the circuit itself is unchanged in production.
// Only when wrapped for calibration does measurement infrastructure activate.
func Wrap(base *circuit.CircuitDef, config DecoratorConfig) *circuit.CircuitDef {
	wrapped := *base

	// Shallow-copy slices to avoid mutating base
	wrapped.Nodes = make([]circuit.NodeDef, len(base.Nodes))
	copy(wrapped.Nodes, base.Nodes)

	wrapped.Edges = make([]circuit.EdgeDef, len(base.Edges))
	copy(wrapped.Edges, base.Edges)

	// Mark as calibration-wrapped via vars
	if wrapped.Vars == nil {
		wrapped.Vars = make(map[string]any)
	}
	wrapped.Vars["_calibration"] = true

	if config.Contract != nil {
		wrapped.Vars["_calibration_contract"] = config.Contract
	}

	return &wrapped
}

// IsCalibrationWrapped returns true if the circuit was decorated by Wrap.
func IsCalibrationWrapped(def *circuit.CircuitDef) bool {
	if def.Vars == nil {
		return false
	}
	v, ok := def.Vars["_calibration"]
	return ok && v == true
}
