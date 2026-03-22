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

// SatisfiesDef declares that a connector provides a factory for a named socket.
type SatisfiesDef struct {
	Socket  string `yaml:"socket"`
	Factory string `yaml:"factory"`
	Wire    string `yaml:"wire,omitempty"` // "instance" (default): call factory, pass result; "factory": pass function reference
}

// WireMode returns the effective wire mode, defaulting to "instance".
func (s SatisfiesDef) WireMode() string {
	if s.Wire == "factory" {
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
	Requires struct {
		Origami string      `yaml:"origami,omitempty"`
		Sockets []SocketDef `yaml:"sockets,omitempty"`
	} `yaml:"requires,omitempty"`
	Satisfies []SatisfiesDef `yaml:"satisfies,omitempty"`
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
