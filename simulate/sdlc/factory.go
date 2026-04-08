package sdlc

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami/instruments/gotools"
	oculusinst "github.com/dpopsuev/origami/instruments/oculus"
)

var errUnknownCircuit = errors.New("unknown circuit")

// Environment variable names.
const (
	envRepoPath = "SDLC_REPO_PATH"
	envMode     = "SDLC_MODE" // "stub", "real", or "" (defaults to stub)
)

// SessionFactory returns a SessionFactory for the SDLC circuit.
// Reads SDLC_REPO_PATH and SDLC_MODE from environment to configure
// instruments (stub vs real Oculus/go-build/go-test).
func SessionFactory() engine.SessionFactory {
	return &sdlcFactory{}
}

type sdlcFactory struct{}

func (f *sdlcFactory) CreateSession(_ context.Context, params *engine.SessionParams) (*engine.SessionConfig, error) {
	repoPath := os.Getenv(envRepoPath)
	if repoPath == "" {
		repoPath = "."
	}

	mode := os.Getenv(envMode)
	if m, ok := params.Extra[envMode].(string); ok && m != "" {
		mode = m
	}

	transformers := buildTransformers(repoPath, mode)

	def, err := LoadCircuit()
	if err != nil {
		return nil, fmt.Errorf("load sdlc circuit: %w", err)
	}

	cases := []engine.BatchCase{
		{ID: "sdlc-run", Context: map[string]any{}},
	}

	return &engine.SessionConfig{
		CircuitDef:   def,
		Transformers: transformers,
		Cases:        cases,
	}, nil
}

func buildTransformers(repoPath, mode string) engine.TransformerRegistry {
	if mode == "real" {
		return realTransformers(repoPath)
	}
	return StubTransformers(true)
}

func realTransformers(repoPath string) engine.TransformerRegistry {
	reg := StubTransformers(true)

	// Replace stubs with real instruments where available.
	reg["scan"] = oculusinst.NewScanTransformer(repoPath, oculusinst.WithLayers(OrigamiLayers))
	reg["build"] = gotools.NewBuildTransformer(repoPath)
	reg["test"] = gotools.NewTestTransformer(repoPath)

	// LLM fix stays as stub unless explicitly wired by the caller.
	// The Vertex provider requires runtime configuration that can't
	// be resolved from env alone — callers use WithTransformer().

	return reg
}

// SchematicResolver returns a circuit asset resolver for sub-circuit
// delegation. The SDLC circuit is self-contained (no sub-circuits),
// but fold requires this function when declared in component.yaml.
func SchematicResolver() circuit.AssetResolver {
	return func(name string) ([]byte, error) {
		if name == "sdlc" {
			return sdlcCircuitData, nil
		}
		return nil, fmt.Errorf("%w: %s", errUnknownCircuit, name)
	}
}
