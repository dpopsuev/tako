package def

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Category: DSL & Build — YAML envelope (self-identification header).

// Kind is a typed identifier for Origami YAML document kinds.
// Provides compile-time safety for kind comparisons and map keys.
type Kind string

// Kind constants for all recognized Origami YAML document types.
const (
	KindSchematic      Kind = "Schematic"
	KindComponent      Kind = "Component"
	KindBoard          Kind = "Board"
	KindCircuit        Kind = "Circuit" // legacy alias for schematic (migration)
	KindStoreSchema    Kind = "StoreSchema"
	KindScorecard      Kind = "Scorecard"
	KindScenario       Kind = "Scenario"
	KindArtifactSchema Kind = "ArtifactSchema"
	KindReportTemplate Kind = "ReportTemplate"
	KindVocabulary     Kind = "Vocabulary"
	KindHeuristicRules Kind = "HeuristicRules"
	KindSourcePack     Kind = "SourcePack"
	KindInstrument     Kind = "Instrument"
	KindTuning         Kind = "Tuning"
	KindDataset        Kind = "Dataset"
	KindPrompt         Kind = "Prompt"
)

// Envelope is the standard header for all Origami YAML files.
// It provides self-identification (kind, version) and human-readable
// metadata so a parser can route by kind without knowing the file path.
//
// Envelope fields are optional during migration: files without kind
// are accepted (zero value). Lint rules warn on missing kind.
type Envelope struct {
	Kind     Kind     `yaml:"kind,omitempty" json:"kind,omitempty"`
	Version  string   `yaml:"version,omitempty" json:"version,omitempty"`
	Metadata Metadata `yaml:"metadata,omitempty" json:"metadata,omitempty"`
}

// Metadata carries the human-readable identity of a YAML document.
type Metadata struct {
	Name        string `yaml:"name,omitempty" json:"name,omitempty"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
}

// ParseKind extracts and validates the kind field from raw YAML bytes.
// This is the single parsing gateway — all YAML loaders must call this
// to obtain a Kind value. Returns ErrUnknownKind if the kind is not
// registered in KnownKinds.
//
// An empty kind is returned as-is (zero value) without error — callers
// decide whether an empty kind is acceptable for their context.
func ParseKind(data []byte) (Kind, error) {
	var probe struct {
		Kind Kind `yaml:"kind"`
	}
	if err := yaml.Unmarshal(data, &probe); err != nil {
		return "", fmt.Errorf("parse kind: %w", err)
	}
	if probe.Kind == "" {
		return "", nil
	}
	if !KnownKinds[probe.Kind] {
		return "", fmt.Errorf("%w: %q", ErrUnknownKind, probe.Kind)
	}
	return probe.Kind, nil
}

// ParseEnvelope extracts just the envelope fields from raw YAML bytes.
// It never fails on missing or unrecognized kind — it is a reader, not
// a validator. Use ParseKind when strict kind validation is needed.
func ParseEnvelope(data []byte) (*Envelope, error) {
	var env Envelope
	if err := yaml.Unmarshal(data, &env); err != nil {
		return nil, err
	}
	return &env, nil
}

// KnownKinds enumerates the recognized kind values for Origami YAML files.
// Framework kinds (circuit, store-schema) are parsed by Origami directly.
// Domain kinds (scenario, vocabulary, etc.) are defined by consumers but
// listed here so lint rules and tooling can recognize them.
var KnownKinds = map[Kind]bool{
	// DSL kinds — the three circuit kinds (KEYWORDS.md)
	KindSchematic: true, // CircuitSchematic — bare graph (nodes, edges)
	KindComponent: true, // CircuitComponent — pluggable code (needs, gives)
	KindBoard:     true, // CircuitBoard — composed, wired, runnable (uses, bind)

	// Framework kinds — parsed by Origami
	KindCircuit:     true, // legacy alias for schematic (migration)
	KindStoreSchema: true,

	// Instrument kind — runtime-dispatched tools (exec/MCP/Docker)
	KindInstrument: true,

	// Domain kinds — parsed by consumers
	KindScorecard:      true,
	KindScenario:       true,
	KindArtifactSchema: true,
	KindReportTemplate: true,
	KindVocabulary:     true,
	KindHeuristicRules: true,
	KindSourcePack:     true,
	KindTuning:         true,
	KindDataset:        true,
	KindPrompt:         true,
}
