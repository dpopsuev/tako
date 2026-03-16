package calibrate

import (
	"fmt"
	"strings"

	framework "github.com/dpopsuev/origami"
)

// ExtractFields uses a CalibrationContract to extract scorer-addressable
// values from a BatchWalkResult. Each contract output field maps a path like
// "investigate.defect_type" (node.field) to a scorer name like
// "actual_defect_type". The result is a flat map[string]any keyed by scorer
// name, ready for scorecard evaluation.
func ExtractFields(contract *CalibrationContract, result framework.BatchWalkResult) map[string]any {
	if contract == nil {
		return nil
	}
	extracted := make(map[string]any, len(contract.Outputs))
	for _, out := range contract.Outputs {
		val := resolveFieldPath(out.Field, result)
		if val != nil {
			extracted[out.ScorerName] = val
		}
	}
	// Always include the walker path — universally useful for calibration.
	extracted["_path"] = result.Path
	return extracted
}

// resolveFieldPath resolves a dotted path like "investigate.defect_type"
// against a BatchWalkResult. The first segment is the node name (looked up
// in StepArtifacts), remaining segments walk into the artifact's map
// representation. Special prefix "state." reads from WalkerState.
func resolveFieldPath(path string, result framework.BatchWalkResult) any {
	parts := strings.SplitN(path, ".", 2)
	if len(parts) == 0 {
		return nil
	}
	root := parts[0]

	if root == "state" {
		return resolveStatePath(parts, result.State)
	}

	art, ok := result.StepArtifacts[root]
	if !ok || art == nil {
		return nil
	}
	m := asAnyMap(art.Raw())
	if m == nil {
		if len(parts) == 1 {
			return art.Raw()
		}
		return nil
	}
	if len(parts) == 1 {
		return m
	}
	return walkMap(m, parts[1])
}

// resolveStatePath extracts values from WalkerState for paths like
// "state.path", "state.loops.investigate".
func resolveStatePath(parts []string, state *framework.WalkerState) any {
	if state == nil || len(parts) < 2 {
		return nil
	}
	subpath := parts[1]
	switch {
	case subpath == "path":
		history := make([]string, len(state.History))
		for i, h := range state.History {
			history[i] = h.Node
		}
		return history
	case strings.HasPrefix(subpath, "loops."):
		key := strings.TrimPrefix(subpath, "loops.")
		return state.LoopCounts[key]
	case subpath == "current_node":
		return state.CurrentNode
	case subpath == "status":
		return state.Status
	}
	return nil
}

// walkMap traverses a nested map using a dotted path like "evidence_refs" or
// "gap_brief.verdict".
func walkMap(m map[string]any, path string) any {
	parts := strings.Split(path, ".")
	current := any(m)
	for _, key := range parts {
		cm, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current, ok = cm[key]
		if !ok {
			return nil
		}
	}
	return current
}

// asAnyMap converts an arbitrary value to map[string]any if possible.
func asAnyMap(v any) map[string]any {
	switch m := v.(type) {
	case map[string]any:
		return m
	case map[any]any:
		result := make(map[string]any, len(m))
		for k, val := range m {
			result[fmt.Sprint(k)] = val
		}
		return result
	}
	return nil
}

// FoldContracts merges multiple calibration contracts into a single contract
// with namespace-prefixed scorer names. This enables multi-circuit calibration
// where each circuit's contract fields are distinguishable.
//
// Example: FoldContracts(map[string]*CalibrationContract{
//
//	"rca":       rcaContract,
//	"harvester": harvesterContract,
//
// }) produces outputs like "rca.actual_defect_type", "harvester.actual_files_found".
func FoldContracts(contracts map[string]*CalibrationContract) *CalibrationContract {
	if len(contracts) == 0 {
		return nil
	}
	if len(contracts) == 1 {
		for _, c := range contracts {
			return c
		}
	}
	folded := &CalibrationContract{}
	for ns, c := range contracts {
		for _, inp := range c.Inputs {
			folded.Inputs = append(folded.Inputs, ContractField{
				Field:      ns + "." + inp.Field,
				ScorerName: ns + "." + inp.ScorerName,
				Type:       inp.Type,
			})
		}
		for _, out := range c.Outputs {
			folded.Outputs = append(folded.Outputs, ContractField{
				Field:      ns + "." + out.Field,
				ScorerName: ns + "." + out.ScorerName,
				Type:       out.Type,
			})
		}
	}
	return folded
}

// ContractFromDef converts the DSL CalibrationContractDef to the calibrate
// package's CalibrationContract. Returns nil if def is nil.
func ContractFromDef(def *framework.CalibrationContractDef) *CalibrationContract {
	if def == nil {
		return nil
	}
	contract := &CalibrationContract{
		Inputs:  make([]ContractField, len(def.Inputs)),
		Outputs: make([]ContractField, len(def.Outputs)),
	}
	for i, f := range def.Inputs {
		contract.Inputs[i] = ContractField{
			Field:      f.Field,
			ScorerName: f.ScorerName,
			Type:       f.Type,
		}
	}
	for i, f := range def.Outputs {
		contract.Outputs[i] = ContractField{
			Field:      f.Field,
			ScorerName: f.ScorerName,
			Type:       f.Type,
		}
	}
	return contract
}
