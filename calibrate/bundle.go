package calibrate

import (
	"io/fs"
	"time"
)

// BundleConfig points to an offline bundle for decoupled calibration.
// Schematics embed this in their RunConfig to consume pre-captured data.
type BundleConfig struct {
	FS            fs.FS  `json:"-" yaml:"-"`
	SchemaVersion string `json:"schema_version" yaml:"schema_version"`
	Schematic     string `json:"schematic" yaml:"schematic"`
}

// CaptureConfig configures a bundle capture operation.
type CaptureConfig struct {
	Schematic  string `json:"schematic" yaml:"schematic"`
	SourcePack string `json:"source_pack" yaml:"source_pack"`
	OutputDir  string `json:"output_dir" yaml:"output_dir"`
	Overwrite  bool   `json:"overwrite" yaml:"overwrite"`
}

// Manifest records the provenance and integrity of a captured bundle.
type Manifest struct {
	SchemaVersion string      `json:"schema_version" yaml:"schema_version"`
	Schematic     string      `json:"schematic" yaml:"schematic"`
	CapturedAt    time.Time   `json:"captured_at" yaml:"captured_at"`
	Repos         []RepoEntry `json:"repos,omitempty" yaml:"repos,omitempty"`
	Docs          []DocEntry  `json:"docs,omitempty" yaml:"docs,omitempty"`
	ExtraFiles    []string    `json:"extra_files,omitempty" yaml:"extra_files,omitempty"`
}

// RepoEntry records a captured repository snapshot.
type RepoEntry struct {
	Name   string   `json:"name" yaml:"name"`
	Branch string   `json:"branch,omitempty" yaml:"branch,omitempty"`
	SHA    string   `json:"sha" yaml:"sha"`
	Files  []string `json:"files" yaml:"files"`
}

// DocEntry records a captured documentation file.
type DocEntry struct {
	Name      string `json:"name" yaml:"name"`
	LocalPath string `json:"local_path" yaml:"local_path"`
	SHA       string `json:"sha" yaml:"sha"`
}

const (
	ManifestFile = "manifest.yaml"
	SchemaV1     = "v1"
)
