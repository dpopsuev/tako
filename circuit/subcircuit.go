package circuit

// Category: DSL & Build — sub-circuit loading from filesystem.

import (
	"context"
	"io/fs"
	"log/slog"
	"path/filepath"
	"strings"
)

// LoadSubCircuitsFromFS loads sub-circuit definitions from a filesystem.
// It scans the "circuits/" directory for YAML files, resolves each via
// the matching AssetResolver (keyed by circuit name), and returns a map
// suitable for GraphRegistries.Circuits.
//
// The circuit name is derived from the filename: "circuits/gnd.yaml" -> "gnd".
// If no resolver exists for a circuit name, the YAML is loaded without overlay
// resolution (treated as a standalone circuit definition).
func LoadSubCircuitsFromFS(fsys fs.FS, resolvers map[string]AssetResolver) map[string]*CircuitDef {
	if fsys == nil {
		return nil
	}

	entries, err := fs.ReadDir(fsys, "circuits")
	if err != nil {
		return nil
	}

	circuits := make(map[string]*CircuitDef)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := filepath.Ext(e.Name())
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ext)

		data, err := fs.ReadFile(fsys, filepath.Join("circuits", e.Name()))
		if err != nil {
			continue
		}

		var def *CircuitDef
		if resolver, ok := resolvers[name]; ok {
			def, err = LoadCircuitWithOverlay(data, resolver)
		} else {
			def, err = LoadCircuit(data)
		}
		if err != nil {
			continue
		}

		// Skip circuits with unresolved imports — they can't be walked
		// locally. The mediator fallback in resolveHandler will handle
		// these via remote delegation instead.
		if def.Import != "" {
			if _, hasResolver := resolvers[name]; !hasResolver {
				slog.DebugContext(context.Background(), LogSubCircuitSkipped,
					slog.String(LogKeyComponent, LogComponentDSL),
					slog.String(LogKeyCircuit, name), slog.Any(LogKeyImport, def.Import),
				)
				continue
			}
		}

		circuits[name] = def
		slog.DebugContext(context.Background(), LogSubCircuitLoaded,
			slog.String(LogKeyComponent, LogComponentDSL),
			slog.String(LogKeyCircuit, name), slog.Any(LogKeyNodes, len(def.Nodes)))
	}

	if len(circuits) == 0 {
		return nil
	}
	return circuits
}
