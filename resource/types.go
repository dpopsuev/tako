// Package resource provides a unified Resource API for all Origami YAML
// artifacts. Every YAML file with a kind: field can be loaded, validated,
// merged, and discovered through a single KindRegistry. This is the K8s
// CRD pattern applied to circuit DSL artifacts.
package resource

import (
	"github.com/dpopsuev/origami/circuit"
)

// Resource is the universal envelope for all Origami YAML artifacts.
// Extends circuit.Envelope with spec and source tracking.
type Resource struct {
	APIVersion string         `yaml:"apiVersion,omitempty" json:"apiVersion,omitempty"`
	Kind       circuit.Kind   `yaml:"kind" json:"kind"`
	Version    string         `yaml:"version,omitempty" json:"version,omitempty"`
	Metadata   Metadata       `yaml:"metadata,omitempty" json:"metadata,omitempty"`
	Spec       map[string]any `yaml:"spec,omitempty" json:"spec,omitempty"`
	Raw        []byte         `yaml:"-" json:"-"` // original bytes
	Source     string         `yaml:"-" json:"-"` // file path
}

// Metadata is the universal metadata block for resource identity.
type Metadata struct {
	Name        string            `yaml:"name,omitempty" json:"name,omitempty"`
	Module      string            `yaml:"module,omitempty" json:"module,omitempty"`
	Namespace   string            `yaml:"namespace,omitempty" json:"namespace,omitempty"`
	Description string            `yaml:"description,omitempty" json:"description,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
}

// DiffEntry describes a single difference between two resources.
type DiffEntry struct {
	Path string `json:"path"` // dotted field path
	A    any    `json:"a"`    // value in first resource
	B    any    `json:"b"`    // value in second resource
}
