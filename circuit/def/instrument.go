package def

// Category: DSL & Build — instrument manifest (YAML-level types only).
// Instruments are runtime-dispatched tools: exec (CLI), mcp (MCP server),
// or docker (container). They never compile into the circuit binary.

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// DispatchMode declares how an instrument is invoked at runtime.
type DispatchMode string

const (
	DispatchExec   DispatchMode = "exec"   // CLI subprocess
	DispatchMCP    DispatchMode = "mcp"    // MCP server
	DispatchDocker DispatchMode = "docker" // Docker container
)

// ValidDispatchModes enumerates the accepted dispatch mode values.
var ValidDispatchModes = []string{
	string(DispatchExec),
	string(DispatchMCP),
	string(DispatchDocker),
}

// InstrumentManifest is the parsed representation of an instrument.yaml file.
// Instruments are language-agnostic tools dispatched at runtime via exec, MCP,
// or Docker. They satisfy battery.Tool and are registered in the circuit's
// tool registry.
type InstrumentManifest struct {
	Kind         Kind         `yaml:"kind"`
	Name         string       `yaml:"name"`
	Namespace    string       `yaml:"namespace"`
	Version      string       `yaml:"version"`
	Description  string       `yaml:"description,omitempty"`
	Dispatch     DispatchMode `yaml:"dispatch"`
	Tune         string       `yaml:"tune"`                   // preflight command — must succeed before circuit starts
	Command      string       `yaml:"command,omitempty"`      // exec dispatch: command to run
	Endpoint     string       `yaml:"endpoint,omitempty"`     // mcp dispatch: server endpoint
	Image        string       `yaml:"image,omitempty"`        // docker dispatch: container image
	InputSchema  string       `yaml:"input_schema,omitempty"` // JSON Schema (draft 2020-12)
	OutputSchema string       `yaml:"output_schema,omitempty"`
}

// Validate checks that an instrument manifest is well-formed.
// Enforces: tune required, dispatch required and valid, dispatch-specific
// fields present.
func (m *InstrumentManifest) Validate(path string) error {
	if m.Name == "" {
		return fmt.Errorf("%w: %s: metadata.name is required", ErrInstrumentManifest, path)
	}
	if m.Namespace == "" {
		return fmt.Errorf("%w: %s: metadata.namespace is required", ErrInstrumentManifest, path)
	}
	if m.Tune == "" {
		return fmt.Errorf("%w: %s: spec.tune is required — instrument must declare a preflight check", ErrInstrumentManifest, path)
	}
	if !isValidDispatchMode(m.Dispatch) {
		return fmt.Errorf("%w: %s: spec.dispatch %q is not valid — must be one of: exec, mcp, docker", ErrInstrumentManifest, path, m.Dispatch)
	}

	switch m.Dispatch {
	case DispatchExec:
		if m.Command == "" {
			return fmt.Errorf("%w: %s: spec.command is required for exec dispatch", ErrInstrumentManifest, path)
		}
	case DispatchMCP:
		if m.Endpoint == "" {
			return fmt.Errorf("%w: %s: spec.endpoint is required for mcp dispatch", ErrInstrumentManifest, path)
		}
	case DispatchDocker:
		if m.Image == "" {
			return fmt.Errorf("%w: %s: spec.image is required for docker dispatch", ErrInstrumentManifest, path)
		}
	}
	return nil
}

func isValidDispatchMode(mode DispatchMode) bool {
	switch mode {
	case DispatchExec, DispatchMCP, DispatchDocker:
		return true
	}
	return false
}

// instrumentManifestYAML is the K8s-style YAML structure for instrument.yaml.
type instrumentManifestYAML struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name        string `yaml:"name"`
		Namespace   string `yaml:"namespace"`
		Description string `yaml:"description,omitempty"`
	} `yaml:"metadata"`
	Spec struct {
		Version      string       `yaml:"version,omitempty"`
		Dispatch     DispatchMode `yaml:"dispatch"`
		Tune         string       `yaml:"tune"`
		Command      string       `yaml:"command,omitempty"`
		Endpoint     string       `yaml:"endpoint,omitempty"`
		Image        string       `yaml:"image,omitempty"`
		InputSchema  string       `yaml:"input_schema,omitempty"`
		OutputSchema string       `yaml:"output_schema,omitempty"`
	} `yaml:"spec"`
}

// LoadInstrumentManifest reads and parses an instrument.yaml file.
// Accepts K8s-style format: apiVersion/kind/metadata/spec.
func LoadInstrumentManifest(path string) (*InstrumentManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read instrument manifest %s: %w", path, err)
	}
	return ParseInstrumentManifest(data, path)
}

// ParseInstrumentManifest parses raw YAML bytes into an InstrumentManifest.
// The path parameter is used only for error messages.
func ParseInstrumentManifest(data []byte, path string) (*InstrumentManifest, error) {
	var raw instrumentManifestYAML
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse instrument manifest %s: %w", path, err)
	}
	if raw.APIVersion == "" {
		return nil, fmt.Errorf("%w: %s: apiVersion is required", ErrInstrumentManifest, path)
	}
	kind, err := ParseKind(data)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", ErrInstrumentManifest, path, err)
	}
	if kind != KindInstrument {
		return nil, fmt.Errorf("%w: %s: kind must be %q, got %q", ErrInstrumentManifest, path, KindInstrument, kind)
	}

	m := &InstrumentManifest{
		Kind:         kind,
		Name:         raw.Metadata.Name,
		Namespace:    raw.Metadata.Namespace,
		Version:      raw.Spec.Version,
		Description:  raw.Metadata.Description,
		Dispatch:     raw.Spec.Dispatch,
		Tune:         raw.Spec.Tune,
		Command:      raw.Spec.Command,
		Endpoint:     raw.Spec.Endpoint,
		Image:        raw.Spec.Image,
		InputSchema:  raw.Spec.InputSchema,
		OutputSchema: raw.Spec.OutputSchema,
	}

	if err := m.Validate(path); err != nil {
		return nil, err
	}
	return m, nil
}
