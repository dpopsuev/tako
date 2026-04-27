package assemble

import (
	"errors"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

var ErrUnknownKind = errors.New("assemble: unknown kind")

// StubParser parses minimal YAML into the 3 top-level kinds.
type StubParser struct{}

var _ Parser = StubParser{}

func (StubParser) Parse(r io.Reader) ([]Kind, error) {
	var raw struct {
		Kind string `yaml:"kind"`
		Name string `yaml:"name"`
	}
	if err := yaml.NewDecoder(r).Decode(&raw); err != nil {
		return nil, fmt.Errorf("assemble: parse: %w", err)
	}
	switch raw.Kind {
	case "Complex":
		return []Kind{Complex{Name: raw.Name}}, nil
	case "Fab":
		return parseFab(r, raw.Name)
	case "Rehearsal":
		return []Kind{Rehearsal{Name: raw.Name}}, nil
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnknownKind, raw.Kind)
	}
}

func parseFab(_ io.Reader, name string) ([]Kind, error) {
	return []Kind{Fab{Name: name}}, nil
}

// StubValidator accepts everything.
type StubValidator struct{}

var _ Validator = StubValidator{}

func (StubValidator) Validate(_ []Kind) []Diagnostic { return nil }
