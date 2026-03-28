package autodoc

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dpopsuev/origami/circuit"
)

const (
	markerBegin = "<!-- autodoc:begin -->"
	markerEnd   = "<!-- autodoc:end -->"
)

// ScaffoldConfig configures the docs tree scaffold generation.
type ScaffoldConfig struct {
	ProjectRoot string
	OutputDir   string // defaults to "docs" relative to ProjectRoot
	Manifest    *Manifest
	Circuits    []*circuit.CircuitDef
	Scorecards  []string // paths to scorecard YAML files
}

// Scaffold generates a Kubernetes-style docs tree. Auto-generated sections
// are delimited by markers for idempotent updates.
func Scaffold(cfg *ScaffoldConfig) error {
	if cfg.OutputDir == "" {
		cfg.OutputDir = filepath.Join(cfg.ProjectRoot, "docs")
	}

	dirs := []string{
		cfg.OutputDir,
		filepath.Join(cfg.OutputDir, "circuits"),
		filepath.Join(cfg.OutputDir, "reference"),
		filepath.Join(cfg.OutputDir, "concepts"),
		filepath.Join(cfg.OutputDir, "getting-started"),
		filepath.Join(cfg.OutputDir, "contributing"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("create dir %s: %w", d, err)
		}
	}

	if err := writeReadme(cfg); err != nil {
		return err
	}
	for _, c := range cfg.Circuits {
		if err := writeCircuitPage(cfg, c); err != nil {
			return err
		}
	}
	if err := writeCircuitIndex(cfg); err != nil {
		return err
	}
	if err := writeStubs(cfg); err != nil {
		return err
	}
	if len(cfg.Scorecards) > 0 {
		if err := writeScorecardRef(cfg); err != nil {
			return err
		}
	}
	return nil
}

func writeReadme(cfg *ScaffoldConfig) error {
	name := cfg.Manifest.Name
	desc := cfg.Manifest.Description
	if desc == "" {
		desc = name + " — an Origami circuit project"
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("# %s\n\n", capitalize(name)))
	b.WriteString(fmt.Sprintf("%s\n\n", desc))

	b.WriteString("## Prerequisites\n\n")
	b.WriteString("- Go 1.22+\n")
	b.WriteString("- [Origami CLI](https://github.com/dpopsuev/origami) (`go install github.com/dpopsuev/origami/cmd/origami@latest`)\n\n")

	b.WriteString("## Quick Start\n\n")
	b.WriteString("```bash\norigami validate circuits/<circuit>.yaml\n")
	b.WriteString("origami run circuits/<circuit>.yaml\n```\n\n")

	b.WriteString(markerBegin + "\n")
	b.WriteString("## Documentation Index\n\n")
	b.WriteString("| Section | Description |\n")
	b.WriteString("|---------|-------------|\n")
	b.WriteString(fmt.Sprintf("| [Circuits](docs/circuits/) | %d circuit(s) — topology diagrams and node reference |\n", len(cfg.Circuits)))
	b.WriteString("| [Concepts](docs/concepts/) | Architecture and pipeline concepts |\n")
	b.WriteString("| [Getting Started](docs/getting-started/) | Installation, quick start, configuration |\n")
	b.WriteString("| [Reference](docs/reference/) | CLI reference, scorecards |\n")
	b.WriteString("| [Contributing](docs/contributing/) | Development and conventions |\n")
	b.WriteString(markerEnd + "\n")

	return writeIdempotent(filepath.Join(cfg.ProjectRoot, "README.md"), b.String())
}

func writeCircuitPage(cfg *ScaffoldConfig, def *circuit.CircuitDef) error {
	path := filepath.Join(cfg.OutputDir, "circuits", def.Circuit+".md")

	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Circuit: %s\n\n", def.Circuit))
	if def.Description != "" {
		b.WriteString(fmt.Sprintf("%s\n\n", def.Description))
	}

	b.WriteString(markerBegin + "\n")
	b.WriteString("## Topology\n\n")
	b.WriteString("```mermaid\n")
	b.WriteString(RenderMermaid(def, nil))
	b.WriteString("```\n\n")

	b.WriteString("## Node Reference\n\n")
	b.WriteString(RenderNodeTable(def, nil))
	b.WriteString("\n")
	b.WriteString(RenderSummary(def, nil))
	b.WriteString(markerEnd + "\n")

	return writeIdempotent(path, b.String())
}

func writeCircuitIndex(cfg *ScaffoldConfig) error {
	path := filepath.Join(cfg.OutputDir, "circuits", "index.md")

	var b strings.Builder
	b.WriteString("# Circuit Catalog\n\n")
	b.WriteString(markerBegin + "\n")
	b.WriteString("| Circuit | Nodes | Edges | Description |\n")
	b.WriteString("|---------|-------|-------|-------------|\n")
	for _, c := range cfg.Circuits {
		b.WriteString(fmt.Sprintf("| [%s](%s.md) | %d | %d | %s |\n",
			c.Circuit, c.Circuit, len(c.Nodes), len(c.Edges), c.Description))
	}
	b.WriteString(markerEnd + "\n")

	return writeIdempotent(path, b.String())
}

func writeStubs(cfg *ScaffoldConfig) error {
	stubs := map[string]string{
		filepath.Join(cfg.OutputDir, "concepts", "architecture.md"):          "# Architecture\n\n*TODO: Describe the project architecture.*\n",
		filepath.Join(cfg.OutputDir, "concepts", "pipeline-stages.md"):       "# Pipeline Stages\n\n*TODO: Explain the pipeline stages.*\n",
		filepath.Join(cfg.OutputDir, "getting-started", "installation.md"):   fmt.Sprintf("# Installation\n\n*TODO: Prerequisites and installation steps for %s.*\n", cfg.Manifest.Name),
		filepath.Join(cfg.OutputDir, "getting-started", "quick-start.md"):    fmt.Sprintf("# Quick Start\n\n*TODO: First analysis in N minutes with %s.*\n", cfg.Manifest.Name),
		filepath.Join(cfg.OutputDir, "getting-started", "configuration.md"):  "# Configuration\n\n*TODO: origami.yaml, credentials, workspace setup.*\n",
		filepath.Join(cfg.OutputDir, "contributing", "development.md"):       "# Development\n\n*TODO: How to modify circuits, run calibration.*\n",
		filepath.Join(cfg.OutputDir, "contributing", "conventions.md"):       "# Conventions\n\n*TODO: Project conventions.*\n",
		filepath.Join(cfg.OutputDir, "reference", "cli.md"):                  fmt.Sprintf("# CLI Reference\n\n*TODO: Command reference for %s.*\n", cfg.Manifest.Name),
	}

	for path, content := range stubs {
		if _, err := os.Stat(path); err == nil {
			continue // don't overwrite existing stubs with hand-written content
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write stub %s: %w", path, err)
		}
	}
	return nil
}

func writeScorecardRef(cfg *ScaffoldConfig) error {
	path := filepath.Join(cfg.OutputDir, "reference", "scorecards.md")

	var b strings.Builder
	b.WriteString("# Scorecards Reference\n\n")
	b.WriteString(markerBegin + "\n")
	b.WriteString("| Scorecard | Path |\n")
	b.WriteString("|-----------|------|\n")
	for _, sc := range cfg.Scorecards {
		name := strings.TrimSuffix(filepath.Base(sc), filepath.Ext(sc))
		rel, _ := filepath.Rel(cfg.ProjectRoot, sc)
		b.WriteString(fmt.Sprintf("| %s | `%s` |\n", name, rel))
	}
	b.WriteString(markerEnd + "\n")

	return writeIdempotent(path, b.String())
}

// writeIdempotent writes content to a file. If the file already exists and
// contains autodoc markers, only the marked section is replaced.
func writeIdempotent(path, content string) error {
	existing, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(existing), markerBegin) {
		return os.WriteFile(path, []byte(content), 0o644)
	}

	old := string(existing)
	beginIdx := strings.Index(old, markerBegin)
	endIdx := strings.Index(old, markerEnd)
	if beginIdx < 0 || endIdx < 0 || endIdx < beginIdx {
		return os.WriteFile(path, []byte(content), 0o644)
	}

	newBeginIdx := strings.Index(content, markerBegin)
	newEndIdx := strings.Index(content, markerEnd)
	if newBeginIdx < 0 || newEndIdx < 0 {
		return os.WriteFile(path, []byte(content), 0o644)
	}

	newSection := content[newBeginIdx : newEndIdx+len(markerEnd)]
	updated := old[:beginIdx] + newSection + old[endIdx+len(markerEnd):]
	return os.WriteFile(path, []byte(updated), 0o644)
}
