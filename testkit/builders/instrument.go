package builders

import (
	"github.com/dpopsuev/origami/circuit/def"
)

// InstrumentManifestBuilder constructs InstrumentManifest incrementally for tests.
type InstrumentManifestBuilder struct {
	m def.InstrumentManifest
}

// NewInstrumentManifest creates a builder with required defaults.
func NewInstrumentManifest(name string) *InstrumentManifestBuilder {
	return &InstrumentManifestBuilder{
		m: def.InstrumentManifest{
			Kind:      def.KindInstrument,
			Name:      name,
			Namespace: "test",
			Dispatch:  def.DispatchCLI,
			Binary:    "echo",
			Tune:      "--version",
			Actions: map[string]def.ActionDef{
				"default": {Command: "ok"},
			},
		},
	}
}

// WithNamespace sets the namespace.
func (b *InstrumentManifestBuilder) WithNamespace(ns string) *InstrumentManifestBuilder {
	b.m.Namespace = ns
	return b
}

// WithDispatch sets the dispatch mode.
func (b *InstrumentManifestBuilder) WithDispatch(mode def.DispatchMode) *InstrumentManifestBuilder {
	b.m.Dispatch = mode
	return b
}

// WithBinary sets the binary name.
func (b *InstrumentManifestBuilder) WithBinary(bin string) *InstrumentManifestBuilder {
	b.m.Binary = bin
	return b
}

// WithTune sets the tune command.
func (b *InstrumentManifestBuilder) WithTune(tune string) *InstrumentManifestBuilder {
	b.m.Tune = tune
	return b
}

// WithEndpoint sets the MCP endpoint.
func (b *InstrumentManifestBuilder) WithEndpoint(ep string) *InstrumentManifestBuilder {
	b.m.Endpoint = ep
	return b
}

// WithImage sets the Docker image.
func (b *InstrumentManifestBuilder) WithImage(img string) *InstrumentManifestBuilder {
	b.m.Image = img
	return b
}

// WithVersion sets the version.
func (b *InstrumentManifestBuilder) WithVersion(v string) *InstrumentManifestBuilder {
	b.m.Version = v
	return b
}

// WithDescription sets the description.
func (b *InstrumentManifestBuilder) WithDescription(desc string) *InstrumentManifestBuilder {
	b.m.Description = desc
	return b
}

// WithAction adds an action to the instrument.
func (b *InstrumentManifestBuilder) WithAction(name string, action def.ActionDef) *InstrumentManifestBuilder {
	if b.m.Actions == nil {
		b.m.Actions = make(map[string]def.ActionDef)
	}
	b.m.Actions[name] = action
	return b
}

// Build returns the constructed InstrumentManifest.
func (b *InstrumentManifestBuilder) Build() *def.InstrumentManifest {
	m := b.m
	// Deep copy actions map.
	if b.m.Actions != nil {
		m.Actions = make(map[string]def.ActionDef, len(b.m.Actions))
		for k, v := range b.m.Actions {
			m.Actions[k] = v
		}
	}
	return &m
}
