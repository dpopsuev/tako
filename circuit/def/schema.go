package def

// ArtifactSchema declares the expected shape of a node's output artifact.
// It is optional (P7: progressive disclosure) and validated by Runner after
// each node completes. Domain tools declare schemas in YAML; the framework
// enforces them so malformed artifacts fail fast with clear diagnostics.
type ArtifactSchema struct {
	Type     string                 `yaml:"type" json:"type"`
	Required []string               `yaml:"required,omitempty" json:"required,omitempty"`
	Fields   map[string]FieldSchema `yaml:"fields,omitempty" json:"fields,omitempty"`
}

// FieldSchema describes a single field in an artifact object.
type FieldSchema struct {
	Type     string                 `yaml:"type" json:"type"`
	Required []string               `yaml:"required,omitempty" json:"required,omitempty"`
	Fields   map[string]FieldSchema `yaml:"fields,omitempty" json:"fields,omitempty"`
	Items    *FieldSchema           `yaml:"items,omitempty" json:"items,omitempty"`
}
