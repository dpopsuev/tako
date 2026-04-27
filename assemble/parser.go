package assemble

import (
	"io"
)

// Diagnostic is a validation finding from the parser or validator.
type Diagnostic struct {
	Severity string
	Message  string
	Line     int
}

// Parser reads YAML and produces typed Kind values.
type Parser interface {
	Parse(r io.Reader) ([]Kind, error)
}

// Validator checks parsed Kinds for structural and semantic correctness.
type Validator interface {
	Validate(kinds []Kind) []Diagnostic
}
