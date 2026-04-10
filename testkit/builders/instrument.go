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
			Dispatch:  def.DispatchExec,
			Tune:      "true", // always succeeds
			Command:   "echo",
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

// WithTune sets the tune command.
func (b *InstrumentManifestBuilder) WithTune(tune string) *InstrumentManifestBuilder {
	b.m.Tune = tune
	return b
}

// WithCommand sets the exec command.
func (b *InstrumentManifestBuilder) WithCommand(cmd string) *InstrumentManifestBuilder {
	b.m.Command = cmd
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

// WithInputSchema sets the input JSON Schema.
func (b *InstrumentManifestBuilder) WithInputSchema(schema string) *InstrumentManifestBuilder {
	b.m.InputSchema = schema
	return b
}

// WithOutputSchema sets the output JSON Schema.
func (b *InstrumentManifestBuilder) WithOutputSchema(schema string) *InstrumentManifestBuilder {
	b.m.OutputSchema = schema
	return b
}

// Build returns the constructed InstrumentManifest.
func (b *InstrumentManifestBuilder) Build() *def.InstrumentManifest {
	m := b.m
	return &m
}
