package lint

// DefaultCrossRefRules returns the built-in cross-file validation rules.
// Each rule is a declarative pair of (export path, reference path) — no
// custom Go logic. The CrossRefEngine extracts values at these paths and
// checks that references resolve to exports.
func DefaultCrossRefRules() []CrossRefRule {
	return []CrossRefRule{
		{
			// S30: check all scorer actual-value params against the calibration
			// contract + adapter_fields exclusion list. Params.expected comes
			// from ground truth (scenario data), not circuit outputs.
			RuleID:      "S30/calibration-fields",
			Desc:        "scorecard actual params must reference fields declared in the circuit's calibration contract or adapter_fields",
			RuleSev:     SeverityWarning,
			ExportKind:  kindCircuit,
			ExportPath:  "calibration.outputs[].scorer_name",
			RefKind:     "scorecard",
			RefPaths:    "metrics[].params.actual,metrics[].params.actual_field,metrics[].params.text_field,metrics[].params.x_field,metrics[].params.numerator_field,metrics[].params.field",
			CheckType:   "refs_subset_of_exports",
			ExportLabel: "calibration contract",
			RefLabel:    "scorecard param",
			ExcludeKind: kindCircuit,
			ExcludePath: "calibration.adapter_fields[]",
		},
		{
			RuleID:      "S32/scenario-sources",
			Desc:        "scenario relevant_repos must exist in source-pack declarations",
			RuleSev:     SeverityWarning,
			ExportKind:  "source-pack",
			ExportPath:  "repos[].name",
			RefKind:     "scenario",
			RefPaths:    "rcas[].relevant_repos[]",
			CheckType:   "refs_subset_of_exports",
			ExportLabel: "source-pack repos",
			RefLabel:    "scenario repo reference",
		},
		// S34 (port-wiring) deferred: wiring uses namespaced references
		// (e.g., "alpha.out:post-triage") which need namespace-aware matching
		// against bare port names. Requires a custom check function.
	}
}
