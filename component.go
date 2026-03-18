package framework

// Category: DSL & Build

import (
	"fmt"
	"log/slog"
	"os"

	"gopkg.in/yaml.v3"
)

// Component bundles reusable plumbing (transformers, extractors, hooks) under
// a namespace. Consumers merge components into their registries at build time.
type Component struct {
	Namespace    string
	Name         string
	Version      string
	Description  string
	Transformers TransformerRegistry
	Extractors   ExtractorRegistry
	Hooks        HookRegistry
}

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
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse component manifest %s: %w", path, err)
	}
	if m.Namespace == "" {
		return nil, fmt.Errorf("component manifest %s: namespace is required", path)
	}
	return &m, nil
}

// MergeComponents merges one or more components into a base GraphRegistries.
// Each component's items are registered under their FQCN (namespace.name).
// Short names are also registered if no collision with the base or earlier components.
// Returns an error if two components provide the same FQCN.
func MergeComponents(base GraphRegistries, components ...*Component) (GraphRegistries, error) {
	merged := GraphRegistries{
		Transformers:     cloneMap(base.Transformers),
		Extractors:       cloneMap(base.Extractors),
		Hooks:            cloneMap(base.Hooks),
		Nodes:            base.Nodes,
		Edges:            base.Edges,
		Circuits:         base.Circuits,
		MediatorEndpoint: base.MediatorEndpoint,
	}

	slog.Debug("merge components",
		"component", "registry",
		"base_circuits", len(base.Circuits),
		"mediator_endpoint", base.MediatorEndpoint,
		"components", len(components),
	)

	for _, a := range components {
		if err := mergeTransformers(merged.Transformers, a); err != nil {
			return GraphRegistries{}, err
		}
		if err := mergeExtractors(merged.Extractors, a); err != nil {
			return GraphRegistries{}, err
		}
		if err := mergeHooks(merged.Hooks, a); err != nil {
			return GraphRegistries{}, err
		}
	}
	return merged, nil
}

func mergeTransformers(dst TransformerRegistry, a *Component) error {
	for name, t := range a.Transformers {
		fqcn := a.Namespace + "." + name
		if _, exists := dst[fqcn]; exists {
			return fmt.Errorf("transformer %q collision (component %s)", fqcn, a.Namespace)
		}
		dst[fqcn] = t
		if _, exists := dst[name]; !exists {
			dst[name] = t
		}
	}
	return nil
}

func mergeExtractors(dst ExtractorRegistry, a *Component) error {
	for name, e := range a.Extractors {
		fqcn := a.Namespace + "." + name
		if _, exists := dst[fqcn]; exists {
			return fmt.Errorf("extractor %q collision (component %s)", fqcn, a.Namespace)
		}
		dst[fqcn] = e
		if _, exists := dst[name]; !exists {
			dst[name] = e
		}
	}
	return nil
}

func mergeHooks(dst HookRegistry, a *Component) error {
	for name, h := range a.Hooks {
		fqcn := a.Namespace + "." + name
		if _, exists := dst[fqcn]; exists {
			return fmt.Errorf("hook %q collision (component %s)", fqcn, a.Namespace)
		}
		dst[fqcn] = h
		if _, exists := dst[name]; !exists {
			dst[name] = h
		}
	}
	return nil
}

func cloneMap[K comparable, V any](src map[K]V) map[K]V {
	if src == nil {
		return make(map[K]V)
	}
	dst := make(map[K]V, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
