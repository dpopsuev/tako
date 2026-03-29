package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dpopsuev/origami/autodoc"
	"github.com/dpopsuev/origami/circuit"
)

func autodocCmd(args []string) error {
	fs := flag.NewFlagSet("autodoc", flag.ContinueOnError)
	scaffold := fs.Bool("scaffold", false, "generate full docs tree (Kubernetes pattern)")
	format := fs.String("format", "mermaid", "output format: mermaid, markdown")
	output := fs.String("output", "", "output directory (default: docs/ in project root)")
	dsBoundary := fs.Bool("ds", false, "render D/S boundary visualization")
	contextFlow := fs.Bool("context-flow", false, "render context flow diagram")
	if err := fs.Parse(args); err != nil {
		return err
	}

	projectRoot := "."
	if fs.NArg() > 0 {
		projectRoot = fs.Arg(0)
	}
	projectRoot, _ = filepath.Abs(projectRoot)

	manifestPath := filepath.Join(projectRoot, "origami.yaml")
	manifest, err := autodoc.LoadManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("load manifest: %w", err)
	}

	circuitPaths, err := autodoc.DiscoverCircuits(projectRoot)
	if err != nil {
		return fmt.Errorf("discover circuits: %w", err)
	}
	if len(circuitPaths) == 0 {
		return fmt.Errorf("%w: %s/circuits/", ErrNoCircuitsFoundIn, projectRoot)
	}

	circuits := make([]*circuit.CircuitDef, 0, len(circuitPaths))
	for _, cp := range circuitPaths {
		data, err := os.ReadFile(cp)
		if err != nil {
			return fmt.Errorf("read circuit %s: %w", cp, err)
		}
		def, err := circuit.LoadCircuit(data)
		if err != nil {
			return fmt.Errorf("parse circuit %s: %w", cp, err)
		}
		circuits = append(circuits, def)
	}

	if *scaffold {
		scorecardPaths, _ := autodoc.DiscoverScorecards(projectRoot)

		cfg := autodoc.ScaffoldConfig{
			ProjectRoot: projectRoot,
			OutputDir:   *output,
			Manifest:    manifest,
			Circuits:    circuits,
			Scorecards:  scorecardPaths,
		}
		if cfg.OutputDir == "" {
			cfg.OutputDir = filepath.Join(projectRoot, "docs")
		}

		if err := autodoc.Scaffold(&cfg); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "scaffolded docs tree for %s (%d circuits)\n", manifest.Name, len(circuits))
		return nil
	}

	for _, def := range circuits {
		switch {
		case *contextFlow:
			fmt.Println(autodoc.RenderContextFlow(def))
		case *dsBoundary:
			fmt.Println(autodoc.RenderDSBoundary(def, nil))
		case *format == "markdown":
			fmt.Println(autodoc.RenderNodeTable(def, nil))
			fmt.Println(autodoc.RenderSummary(def, nil))
		default:
			fmt.Println(autodoc.RenderMermaid(def, nil))
		}
	}

	return nil
}
