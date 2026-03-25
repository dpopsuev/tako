package circuit

// Category: DSL & Build — YAML envelope (self-identification header).

// Envelope is the standard header for all Origami YAML files.
// It provides self-identification (kind, version) and human-readable
// metadata so a parser can route by kind without knowing the file path.
//
// Envelope fields are optional during migration: files without kind
// are accepted (zero value). Lint rules warn on missing kind.
type Envelope struct {
	Kind     string   `yaml:"kind,omitempty" json:"kind,omitempty"`
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
var KnownKinds = map[string]bool{
	// DSL kinds — the three circuit kinds (KEYWORDS.md)
	"schematic": true, // CircuitSchematic — bare graph (nodes, edges)
	"component": true, // CircuitComponent — pluggable code (needs, gives)
	"board":     true, // CircuitBoard — composed, wired, runnable (uses, bind)

	// Framework kinds — parsed by Origami
	"circuit":      true, // legacy alias for schematic (migration)
	"store-schema": true,

	// Domain kinds — parsed by consumers
	"scorecard":       true,
	"scenario":        true,
	"artifact-schema": true,
	"report-template": true,
	"vocabulary":      true,
	"heuristic-rules": true,
	"source-pack":     true,
	"tuning":          true,
}
