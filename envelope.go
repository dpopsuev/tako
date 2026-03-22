package framework

// Category: DSL & Build — aliases to circuit/ package.

import "github.com/dpopsuev/origami/circuit"

type Envelope = circuit.Envelope
type Metadata = circuit.Metadata

// ParseEnvelope extracts just the envelope fields from raw YAML bytes.
func ParseEnvelope(data []byte) (*Envelope, error) { return circuit.ParseEnvelope(data) }

// KnownKinds enumerates the recognized kind values for Origami YAML files.
var KnownKinds = circuit.KnownKinds
