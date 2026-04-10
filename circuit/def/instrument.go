package def

// Category: DSL & Build — instrument manifest (YAML-level types only).
// Instruments are the universal node dispatch model. Every node in a circuit
// references an instrument + action. Dispatch modes: exec (CLI), mcp (MCP
// server), docker (container), go (in-process function call).

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// DispatchMode declares how an instrument is invoked at runtime.
type DispatchMode string

const (
	DispatchCLI       DispatchMode = "cli"       // CLI tool subprocess
	DispatchMCP       DispatchMode = "mcp"       // MCP server
	DispatchContainer DispatchMode = "container" // Container execution
	DispatchInproc    DispatchMode = "inproc"    // In-process function call
)

// ValidDispatchModes enumerates the accepted dispatch mode values.
var ValidDispatchModes = []string{
	string(DispatchCLI),
	string(DispatchMCP),
	string(DispatchContainer),
	string(DispatchInproc),
}

// ActionDef declares a single action an instrument can perform.
// Each action has its own command and I/O schema contract.
type ActionDef struct {
	Command      string `yaml:"command"`                // exec/docker: shell command
	GoFunc       string `yaml:"go_func,omitempty"`      // go dispatch: qualified function name
	InputSchema  string `yaml:"input_schema,omitempty"` // JSON Schema (draft 2020-12)
	OutputSchema string `yaml:"output_schema,omitempty"`
}

// InstrumentManifest is the parsed representation of an instrument.yaml file.
// Instruments are the universal node dispatch model — every circuit node
// references an instrument by name and an action from its actions table.
type InstrumentManifest struct {
	Kind        Kind                 `yaml:"kind"`
	Name        string               `yaml:"name"`
	Namespace   string               `yaml:"namespace"`
	Version     string               `yaml:"version"`
	Description string               `yaml:"description,omitempty"`
	Dispatch    DispatchMode         `yaml:"dispatch"`
	Tune        string               `yaml:"tune"`               // preflight command — must succeed before circuit starts
	Endpoint    string               `yaml:"endpoint,omitempty"` // mcp dispatch: server endpoint
	Image       string               `yaml:"image,omitempty"`    // docker dispatch: container image
	Actions     map[string]ActionDef `yaml:"actions"`
}

// Validate checks that an instrument manifest is well-formed.
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
		return fmt.Errorf("%w: %s: spec.dispatch %q is not valid — must be one of: cli, mcp, container, inproc", ErrInstrumentManifest, path, m.Dispatch)
	}
	if len(m.Actions) == 0 {
		return fmt.Errorf("%w: %s: spec.actions must declare at least one action", ErrInstrumentManifest, path)
	}

	switch m.Dispatch {
	case DispatchMCP:
		if m.Endpoint == "" {
			return fmt.Errorf("%w: %s: spec.endpoint is required for mcp dispatch", ErrInstrumentManifest, path)
		}
	case DispatchContainer:
		if m.Image == "" {
			return fmt.Errorf("%w: %s: spec.image is required for container dispatch", ErrInstrumentManifest, path)
		}
	}

	for name, action := range m.Actions {
		switch m.Dispatch {
		case DispatchCLI, DispatchContainer:
			if action.Command == "" {
				return fmt.Errorf("%w: %s: action %q: command is required for %s dispatch", ErrInstrumentManifest, path, name, m.Dispatch)
			}
		case DispatchInproc:
			if action.GoFunc == "" {
				return fmt.Errorf("%w: %s: action %q: go_func is required for inproc dispatch", ErrInstrumentManifest, path, name)
			}
		}
	}

	return nil
}

// HasAction returns true if the instrument has the named action.
func (m *InstrumentManifest) HasAction(name string) bool {
	_, ok := m.Actions[name]
	return ok
}

// Action returns the named action definition or an error if not found.
func (m *InstrumentManifest) Action(name string) (ActionDef, error) {
	a, ok := m.Actions[name]
	if !ok {
		return ActionDef{}, fmt.Errorf("%w: instrument %q has no action %q", ErrInstrumentManifest, m.Name, name)
	}
	return a, nil
}

func isValidDispatchMode(mode DispatchMode) bool {
	switch mode {
	case DispatchCLI, DispatchMCP, DispatchContainer, DispatchInproc:
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
		Version  string               `yaml:"version,omitempty"`
		Dispatch DispatchMode         `yaml:"dispatch"`
		Tune     string               `yaml:"tune"`
		Endpoint string               `yaml:"endpoint,omitempty"`
		Image    string               `yaml:"image,omitempty"`
		Actions  map[string]ActionDef `yaml:"actions"`
	} `yaml:"spec"`
}

// LoadInstrumentManifest reads and parses an instrument.yaml file.
func LoadInstrumentManifest(path string) (*InstrumentManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read instrument manifest %s: %w", path, err)
	}
	return ParseInstrumentManifest(data, path)
}

// ParseInstrumentManifest parses raw YAML bytes into an InstrumentManifest.
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
		Kind:        kind,
		Name:        raw.Metadata.Name,
		Namespace:   raw.Metadata.Namespace,
		Version:     raw.Spec.Version,
		Description: raw.Metadata.Description,
		Dispatch:    raw.Spec.Dispatch,
		Tune:        raw.Spec.Tune,
		Endpoint:    raw.Spec.Endpoint,
		Image:       raw.Spec.Image,
		Actions:     raw.Spec.Actions,
	}

	if err := m.Validate(path); err != nil {
		return nil, err
	}
	return m, nil
}
