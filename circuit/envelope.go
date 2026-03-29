package circuit

// Category: DSL & Build — YAML envelope (self-identification header).

// Kind is a typed identifier for Origami YAML document kinds.
// Provides compile-time safety for kind comparisons and map keys.
type Kind string

// Kind constants for all recognized Origami YAML document types.
const (
	KindSchematic      Kind = "schematic"
	KindComponent      Kind = "component"
	KindBoard          Kind = "board"
	KindCircuit        Kind = "circuit" // legacy alias for schematic (migration)
	KindStoreSchema    Kind = "store-schema"
	KindScorecard      Kind = "scorecard"
	KindScenario       Kind = "scenario"
	KindArtifactSchema Kind = "artifact-schema"
	KindReportTemplate Kind = "report-template"
	KindVocabulary     Kind = "vocabulary"
	KindHeuristicRules Kind = "heuristic-rules"
	KindSourcePack     Kind = "source-pack"
	KindTuning         Kind = "tuning"
	KindDataset        Kind = "dataset"
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

// ParseEnvelope extracts just the envelope fields from raw YAML bytes.
// It never fails on missing fields — the returned Envelope simply has
// zero-value fields for anything not present.
func ParseEnvelope(data []byte) (*Envelope, error) {
	var env Envelope
	if err := yamlUnmarshal(data, &env); err != nil {
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
}
