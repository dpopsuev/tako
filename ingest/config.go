package ingest

// Config holds domain-agnostic ingestion parameters.
// Domain-specific settings go in Extra.
type Config struct {
	LookbackDays int            `json:"lookback_days" yaml:"lookback_days"`
	OutputDir    string         `json:"output_dir" yaml:"output_dir"`
	Extra        map[string]any `json:"extra,omitempty" yaml:"extra,omitempty"`
}
