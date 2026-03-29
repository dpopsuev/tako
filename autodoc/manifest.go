package autodoc

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ErrMissingName is returned when a manifest is missing the required name field.
var ErrMissingName = errors.New("manifest missing required field: name")

// Manifest represents an origami.yaml project manifest.
type Manifest struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Version     string   `yaml:"version"`
	Imports     []string `yaml:"imports,omitempty"`
}

// LoadManifest reads and parses an origami.yaml file.
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	if m.Name == "" {
		return nil, ErrMissingName
	}
	return &m, nil
}

// DiscoverCircuits finds all *.yaml files in the internal/circuits/ subdirectory
// relative to the given project root.
func DiscoverCircuits(projectRoot string) ([]string, error) {
	return discoverYAML(filepath.Join(projectRoot, "internal", "circuits"))
}

// DiscoverScorecards finds all *.yaml files in the internal/scorecards/ subdirectory
// relative to the given project root.
func DiscoverScorecards(projectRoot string) ([]string, error) {
	return discoverYAML(filepath.Join(projectRoot, "internal", "scorecards"))
}

func discoverYAML(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read directory %s: %w", dir, err)
	}
	var paths []string
	for _, e := range entries {
		if !e.IsDir() && (filepath.Ext(e.Name()) == ".yaml" || filepath.Ext(e.Name()) == ".yml") {
			paths = append(paths, filepath.Join(dir, e.Name()))
		}
	}
	return paths, nil
}
