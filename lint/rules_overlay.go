package lint

import (
	"fmt"
	"strings"

	"github.com/dpopsuev/origami/circuit"
)

const (
	ruleImportOverlay  = "S18/import-overlay"
	rulePortValidation = "S19/port-validation"
	ruleCalibContract  = "S20/calibration-contract"
	msgFieldRequired   = ": field is required"
	msgScorerRequired  = ": scorer_name is required"
)

var validPortDirections = map[string]bool{
	"in": true, "out": true, "loop": true,
}

// --- S18: import-overlay ---

// ImportOverlay validates circuits with import: directives (overlay-aware linting).
type ImportOverlay struct{}

func (r *ImportOverlay) ID() string { return ruleImportOverlay }
func (r *ImportOverlay) Description() string {
	return "import: must be non-empty if present; overlay should not redefine start/done"
}
func (r *ImportOverlay) Severity() Severity { return SeverityWarning }
func (r *ImportOverlay) Tags() []string     { return []string{"structural"} }

func (r *ImportOverlay) Check(ctx *LintContext) []Finding {
	if ctx.Def == nil {
		return nil
	}
	var out []Finding

	// import: present but empty
	if ctx.TopLevelLine("import") > 0 && strings.TrimSpace(ctx.Def.Import) == "" {
		out = append(out, Finding{
			RuleID:   r.ID(),
			Severity: SeverityError,
			Message:  "import: is present but value is empty",
			File:     ctx.File,
			Line:     ctx.TopLevelLine("import"),
		})
	}

	// circuit with import: redefining start: or done: (usually inherited from base)
	if ctx.Def.Import != "" {
		if ctx.Def.Start != "" {
			out = append(out, Finding{
				RuleID:   r.ID(),
				Severity: r.Severity(),
				Message:  fmt.Sprintf("circuit with import: %q redefines start: %q (usually inherited from base)", ctx.Def.Import, ctx.Def.Start),
				File:     ctx.File,
				Line:     ctx.TopLevelLine("start"),
			})
		}
		if ctx.Def.Done != "" {
			out = append(out, Finding{
				RuleID:   r.ID(),
				Severity: r.Severity(),
				Message:  fmt.Sprintf("circuit with import: %q redefines done: %q (usually inherited from base)", ctx.Def.Import, ctx.Def.Done),
				File:     ctx.File,
				Line:     ctx.TopLevelLine("done"),
			})
		}
	}
	return out
}

// --- S19: port-validation ---

// PortValidation checks port direction and uniqueness.
type PortValidation struct{}

func (r *PortValidation) ID() string { return rulePortValidation }
func (r *PortValidation) Description() string {
	return "port direction must be in/out/loop; port names must be unique"
}
func (r *PortValidation) Severity() Severity { return SeverityError }
func (r *PortValidation) Tags() []string     { return []string{"structural"} }

func (r *PortValidation) Check(ctx *LintContext) []Finding {
	if ctx.Def == nil {
		return nil
	}
	var out []Finding
	seen := make(map[string]int) // name -> first occurrence index

	for i, p := range ctx.Def.Ports {
		// direction must be in, out, or loop
		dir := strings.ToLower(strings.TrimSpace(p.Direction))
		if dir == "" || !validPortDirections[dir] {
			msg := fmt.Sprintf("port %q: direction must be one of in, out, loop", p.Name)
			if dir != "" {
				msg = fmt.Sprintf("port %q: direction %q must be one of in, out, loop", p.Name, p.Direction)
			}
			out = append(out, Finding{
				RuleID:   r.ID(),
				Severity: r.Severity(),
				Message:  msg,
				File:     ctx.File,
				Line:     ctx.PortLine(p.Name),
			})
		}

		// port names must be unique
		if p.Name != "" {
			if first, ok := seen[p.Name]; ok {
				out = append(out, Finding{
					RuleID:   r.ID(),
					Severity: r.Severity(),
					Message:  fmt.Sprintf("port name %q is duplicated (first at index %d)", p.Name, first),
					File:     ctx.File,
					Line:     ctx.PortLine(p.Name),
				})
			} else {
				seen[p.Name] = i
			}
		}
	}
	return out
}

// --- S20: calibration-contract ---

// CalibrationContract validates calibration inputs/outputs when calibration: is declared.
type CalibrationContract struct{}

func (r *CalibrationContract) ID() string { return ruleCalibContract }
func (r *CalibrationContract) Description() string {
	return "calibration inputs/outputs must have non-empty field and scorer_name"
}
func (r *CalibrationContract) Severity() Severity { return SeverityError }
func (r *CalibrationContract) Tags() []string     { return []string{"structural"} }

func (r *CalibrationContract) Check(ctx *LintContext) []Finding {
	if ctx.Def == nil || ctx.Def.Calibration == nil {
		return nil
	}
	cal := ctx.Def.Calibration
	var out []Finding
	out = r.checkCalibFields(ctx, cal.Inputs, "input", out)
	out = r.checkCalibFields(ctx, cal.Outputs, "output", out)
	return out
}

func (r *CalibrationContract) checkCalibFields(ctx *LintContext, fields []circuit.CalibrationFieldDef, kind string, out []Finding) []Finding {
	for i, f := range fields {
		if strings.TrimSpace(f.Field) == "" || strings.TrimSpace(f.ScorerName) == "" {
			msg := "calibration " + kind
			if f.Field != "" || f.ScorerName != "" {
				msg = fmt.Sprintf("calibration %s %q", kind, f.Field)
			}
			if strings.TrimSpace(f.Field) == "" {
				msg += msgFieldRequired
			} else {
				msg += msgScorerRequired
			}
			out = append(out, Finding{
				RuleID:   r.ID(),
				Severity: r.Severity(),
				Message:  msg,
				File:     ctx.File,
				Line:     ctx.CalibrationFieldLine(kind, i),
			})
		}
	}
	return out
}
