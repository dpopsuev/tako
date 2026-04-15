package circuit

import (
	"errors"
	"fmt"
)

// ErrStep is returned for step schema validation failures.
var ErrStep = errors.New("step")

// FieldDef describes a single field in a step's artifact schema.
type FieldDef struct {
	Name     string // field name, e.g. "confidence"
	Type     string // type hint: "string", "bool", "float", "object", "array"
	Required bool   // if true, submit_step rejects artifacts missing this field
	Desc     string // optional human-readable description
}

// StepSchema declares what a single circuit step expects in its artifact.
// Used for runtime validation in submit_step and to auto-generate worker
// prompt step-schema tables.
type StepSchema struct {
	Name string     // e.g. "F0_RECALL", "scan"
	Defs []FieldDef // structured field definitions for runtime validation
}

// ValidateFields checks that fields satisfies the schema's Defs.
func (s StepSchema) ValidateFields(fields map[string]any) error {
	for _, def := range s.Defs {
		v, ok := fields[def.Name]
		if !ok && def.Required {
			return fmt.Errorf("%w: %s: missing required field %q", ErrStep, s.Name, def.Name)
		}
		if ok && v == nil && def.Required {
			return fmt.Errorf("%w: %s: field %q is null", ErrStep, s.Name, def.Name)
		}
	}
	return nil
}

// ExtraParamDef describes one domain-specific parameter inside the
// start_circuit "extra" field. Domains register these so the MCP schema
// tells callers exactly what keys are expected.
type ExtraParamDef struct {
	Name        string   // JSON key inside extra (e.g. "scenario")
	Type        string   // JSON Schema type: "string", "integer", "boolean", "object"
	Description string   // Human-readable description shown in MCP schema
	Required    bool     // If true, start_circuit rejects calls missing this key
	Enum        []string // If non-empty, allowed values (e.g. ["offline","online"])
}
