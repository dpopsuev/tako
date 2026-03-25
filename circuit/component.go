package circuit

// Category: DSL & Build — component manifest (YAML-level types only).
// The live Component struct with runtime registries stays in the root package.

import (
	"fmt"
	"os"
)

// SocketDef declares a typed dependency slot that a schematic requires.
// Connectors satisfy sockets by declaring a matching factory in their manifest.
type SocketDef struct {
	Name        string `yaml:"name"`
	Type        string `yaml:"type"`
	Option      string `yaml:"option,omitempty"` // With* function name on the server
	Description string `yaml:"description,omitempty"`
	Schematic   string `yaml:"schematic,omitempty"` // if set, satisfied by another schematic
	Optional    bool   `yaml:"optional,omitempty"`  // true if socket has a built-in default
}

// GivesDef declares that a component provides a factory for a named socket.
type GivesDef struct {
	Socket  string `yaml:"socket"`
	Factory string `yaml:"factory"`
	Wire    string `yaml:"wire,omitempty"` // "instance" (default): call factory, pass result; "factory": pass function reference
}

// WireMode returns the effective wire mode, defaulting to "instance".
func (g GivesDef) WireMode() string {
	if g.Wire == "factory" {
		return "factory"
	}
	return "instance"
}

// ComponentManifest is the YAML schema for component.yaml files.
type ComponentManifest struct {
	Component   string `yaml:"component"`
	Module      string `yaml:"module"` // Go import path (e.g. github.com/dpopsuev/origami/connectors/rp)
	Namespace   string `yaml:"namespace"`
	Version     string `yaml:"version"`
	Description string `yaml:"description,omitempty"`
	Factory     string `yaml:"factory,omitempty"`  // schematic constructor (e.g. NewRouter, NewServer)
	Resolver    string `yaml:"resolver,omitempty"` // circuit overlay resolver function (e.g. SchematicResolver)
	Adapter     string `yaml:"adapter,omitempty"`  // optional adapter for subprocess mode
	Serve       string `yaml:"serve,omitempty"`    // path to serve command for subprocess mode
	Provides    struct {
		Transformers []string `yaml:"transformers,omitempty"`
		Extractors   []string `yaml:"extractors,omitempty"`
		Hooks        []string `yaml:"hooks,omitempty"`
	} `yaml:"provides"`
	Needs struct {
		Origami    string      `yaml:"origami,omitempty"`
		Transports []SocketDef `yaml:"transports,omitempty"` // Transport, Trigger
		Sources    []SocketDef `yaml:"sources,omitempty"`    // SourceReader, SourceCatalog
		Storage    []SocketDef `yaml:"storage,omitempty"`    // Driver
	} `yaml:"needs,omitempty"`
	Gives []GivesDef `yaml:"gives,omitempty"`

	// MCP server configuration — fold reads these to generate CircuitConfig.
	Params   []ParamDef  `yaml:"params,omitempty"`  // extra start_circuit parameters
	Schemas  []string    `yaml:"schemas,omitempty"`  // step schema paths (relative to domain FS)
	Report   string      `yaml:"report,omitempty"`   // report template YAML path
	Dispatch DispatchDef `yaml:"dispatch,omitempty"` // dispatch provider config
	Hooks    string      `yaml:"hooks,omitempty"`    // Go symbol: "rca.Hooks()"
}

// ParamDef declares an extra parameter for MCP start_circuit.
type ParamDef struct {
	Name     string   `yaml:"name"`
	Type     string   `yaml:"type"`
	Desc     string   `yaml:"description,omitempty"`
	Required bool     `yaml:"required,omitempty"`
	Enum     []string `yaml:"enum,omitempty"`
}

// DispatchDef declares how the schematic dispatches LLM prompts.
type DispatchDef struct {
	Provider string `yaml:"provider,omitempty"` // "cli" (default), "http"
	Workers  int    `yaml:"workers,omitempty"`  // parallel workers (default 1)
	Timeout  string `yaml:"timeout,omitempty"`  // per-dispatch timeout (e.g. "5m")
}

// LoadComponentManifest reads and parses a component.yaml file.
func LoadComponentManifest(path string) (*ComponentManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read component manifest %s: %w", path, err)
	}
	var m ComponentManifest
	if err := yamlUnmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse component manifest %s: %w", path, err)
	}
	if m.Namespace == "" {
		return nil, fmt.Errorf("component manifest %s: namespace is required", path)
	}
	return &m, nil
}
