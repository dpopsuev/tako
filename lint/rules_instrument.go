package lint

// Instrument manifest lint rules (I-series).
// These rules fire only when the YAML kind is "Instrument".

import (
	"encoding/json"
	"fmt"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/circuit/def"
	"gopkg.in/yaml.v3"
)

// instrumentSpec mirrors the spec section of an instrument.yaml for lint-time parsing.
type instrumentSpec struct {
	Dispatch string                      `yaml:"dispatch"`
	Tune     string                      `yaml:"tune"`
	Endpoint string                      `yaml:"endpoint"`
	Image    string                      `yaml:"image"`
	Actions  map[string]instrumentAction `yaml:"actions"`
}

type instrumentAction struct {
	Command      string `yaml:"command"`
	GoFunc       string `yaml:"go_func"`
	InputSchema  string `yaml:"input_schema"`
	OutputSchema string `yaml:"output_schema"`
}

// instrumentManifestLint is the lint-time YAML shape for instrument.yaml.
type instrumentManifestLint struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name      string `yaml:"name"`
		Namespace string `yaml:"namespace"`
	} `yaml:"metadata"`
	Spec instrumentSpec `yaml:"spec"`
}

// parseInstrumentForLint returns the parsed manifest if the YAML is kind: Instrument.
// Returns nil if the kind is not Instrument.
func parseInstrumentForLint(ctx *LintContext) *instrumentManifestLint {
	if ctx.yamlRoot == nil {
		return nil
	}
	var m instrumentManifestLint
	if err := yaml.Unmarshal(ctx.Raw, &m); err != nil {
		return nil
	}
	if circuit.Kind(m.Kind) != circuit.KindInstrument {
		return nil
	}
	return &m
}

// --- I1: missing tune ---

// InstrumentMissingTune errors when an instrument manifest has no tune command.
type InstrumentMissingTune struct{}

func (r *InstrumentMissingTune) ID() string { return "I1/instrument-missing-tune" }
func (r *InstrumentMissingTune) Description() string {
	return "instrument must declare a preflight tune command"
}
func (r *InstrumentMissingTune) Severity() Severity { return SeverityError }
func (r *InstrumentMissingTune) Tags() []string     { return []string{"instrument"} }

func (r *InstrumentMissingTune) Check(ctx *LintContext) []Finding {
	m := parseInstrumentForLint(ctx)
	if m == nil {
		return nil
	}
	if m.Spec.Tune == "" {
		return []Finding{{
			RuleID:   r.ID(),
			Severity: r.Severity(),
			Message:  "instrument manifest missing spec.tune — must declare a preflight verification command",
			File:     ctx.File,
		}}
	}
	return nil
}

// --- I2: invalid dispatch ---

// InstrumentInvalidDispatch errors when an instrument's dispatch mode is not a valid enum.
type InstrumentInvalidDispatch struct{}

func (r *InstrumentInvalidDispatch) ID() string { return "I2/instrument-invalid-dispatch" }
func (r *InstrumentInvalidDispatch) Description() string {
	return "instrument dispatch must be a valid mode"
}
func (r *InstrumentInvalidDispatch) Severity() Severity { return SeverityError }
func (r *InstrumentInvalidDispatch) Tags() []string     { return []string{"instrument"} }

func (r *InstrumentInvalidDispatch) Check(ctx *LintContext) []Finding {
	m := parseInstrumentForLint(ctx)
	if m == nil {
		return nil
	}
	if m.Spec.Dispatch == "" {
		return []Finding{{
			RuleID:   r.ID(),
			Severity: r.Severity(),
			Message:  "instrument manifest missing spec.dispatch",
			File:     ctx.File,
		}}
	}
	for _, valid := range def.ValidDispatchModes {
		if m.Spec.Dispatch == valid {
			return nil
		}
	}
	return []Finding{{
		RuleID:   r.ID(),
		Severity: r.Severity(),
		Message:  fmt.Sprintf("instrument dispatch %q is not valid — must be one of: %s", m.Spec.Dispatch, validDispatchList()),
		File:     ctx.File,
	}}
}

func validDispatchList() string {
	s := ""
	for i, v := range def.ValidDispatchModes {
		if i > 0 {
			s += ", "
		}
		s += v
	}
	return s
}

// --- I3: missing namespace ---

// InstrumentMissingNamespace errors when an instrument has no namespace.
type InstrumentMissingNamespace struct{}

func (r *InstrumentMissingNamespace) ID() string { return "I3/instrument-missing-namespace" }
func (r *InstrumentMissingNamespace) Description() string {
	return "instrument must declare a namespace"
}
func (r *InstrumentMissingNamespace) Severity() Severity { return SeverityError }
func (r *InstrumentMissingNamespace) Tags() []string     { return []string{"instrument"} }

func (r *InstrumentMissingNamespace) Check(ctx *LintContext) []Finding {
	m := parseInstrumentForLint(ctx)
	if m == nil {
		return nil
	}
	if m.Metadata.Namespace == "" {
		return []Finding{{
			RuleID:   r.ID(),
			Severity: r.Severity(),
			Message:  "instrument manifest missing metadata.namespace",
			File:     ctx.File,
		}}
	}
	return nil
}

// --- I4: malformed schema ---

// InstrumentMalformedSchema errors when an action's input_schema or output_schema
// is present but not valid JSON.
type InstrumentMalformedSchema struct{}

func (r *InstrumentMalformedSchema) ID() string          { return "I4/instrument-malformed-schema" }
func (r *InstrumentMalformedSchema) Description() string { return "action schema must be valid JSON" }
func (r *InstrumentMalformedSchema) Severity() Severity  { return SeverityError }
func (r *InstrumentMalformedSchema) Tags() []string      { return []string{"instrument"} }

func (r *InstrumentMalformedSchema) Check(ctx *LintContext) []Finding {
	m := parseInstrumentForLint(ctx)
	if m == nil {
		return nil
	}
	var out []Finding
	for name, action := range m.Spec.Actions {
		if action.InputSchema != "" && !json.Valid([]byte(action.InputSchema)) {
			out = append(out, Finding{
				RuleID:   r.ID(),
				Severity: r.Severity(),
				Message:  fmt.Sprintf("action %q: input_schema is not valid JSON", name),
				File:     ctx.File,
			})
		}
		if action.OutputSchema != "" && !json.Valid([]byte(action.OutputSchema)) {
			out = append(out, Finding{
				RuleID:   r.ID(),
				Severity: r.Severity(),
				Message:  fmt.Sprintf("action %q: output_schema is not valid JSON", name),
				File:     ctx.File,
			})
		}
	}
	return out
}

// --- I5: missing schema ---

// InstrumentMissingSchema warns when an action is missing input_schema or output_schema.
type InstrumentMissingSchema struct{}

func (r *InstrumentMissingSchema) ID() string { return "I5/instrument-missing-schema" }
func (r *InstrumentMissingSchema) Description() string {
	return "actions should declare input and output schemas"
}
func (r *InstrumentMissingSchema) Severity() Severity { return SeverityWarning }
func (r *InstrumentMissingSchema) Tags() []string     { return []string{"instrument"} }

func (r *InstrumentMissingSchema) Check(ctx *LintContext) []Finding {
	m := parseInstrumentForLint(ctx)
	if m == nil {
		return nil
	}
	var out []Finding
	for name, action := range m.Spec.Actions {
		if action.InputSchema == "" {
			out = append(out, Finding{
				RuleID:   r.ID(),
				Severity: r.Severity(),
				Message:  fmt.Sprintf("action %q: missing input_schema — declare JSON Schema for input contract", name),
				File:     ctx.File,
			})
		}
		if action.OutputSchema == "" {
			out = append(out, Finding{
				RuleID:   r.ID(),
				Severity: r.Severity(),
				Message:  fmt.Sprintf("action %q: missing output_schema — declare JSON Schema for output contract", name),
				File:     ctx.File,
			})
		}
	}
	return out
}
