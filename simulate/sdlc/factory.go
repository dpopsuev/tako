package sdlc

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"

	"github.com/dpopsuev/battery/tool"
	anyllm "github.com/mozilla-ai/any-llm-go/providers"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami/instruments/gotools"
	"github.com/dpopsuev/origami/instruments/llmfix"
	oculusinst "github.com/dpopsuev/origami/instruments/oculus"
	"github.com/dpopsuev/origami/instruments/selfreview"
)

var errUnknownCircuit = errors.New("unknown circuit")

// Environment variable names.
const (
	// EnvRepoPath is the environment variable for the repository path.
	EnvRepoPath = "SDLC_REPO_PATH"
	// EnvMode is the environment variable for the execution mode (stub/real).
	EnvMode = "SDLC_MODE"

	// EnvProvider is the LLM provider name (used by serve command, not factory).
	EnvProvider = "SDLC_PROVIDER"
	// EnvModel is the LLM model name (used by serve command, not factory).
	EnvModel = "SDLC_MODEL"

	// sdlcMaxTokens is the output token limit for all LLM calls in the SDLC circuit.
	// Exported so the serve command can use it when creating the provider.
	SDLCMaxTokens = 16384

	// EnvScribeEndpoint is the MCP endpoint for Scribe (used by serve command, not factory).
	EnvScribeEndpoint = "SCRIBE_ENDPOINT"
	// EnvLocusEndpoint is the MCP endpoint for Locus (used by serve command, not factory).
	EnvLocusEndpoint = "LOCUS_ENDPOINT"
	// EnvCircuitDir is the directory containing circuit YAML (used by serve command).
	// When different from EnvRepoPath (e.g., origami dogfooding itself).
	EnvCircuitDir = "SDLC_CIRCUIT_DIR"
	// EnvScope is the Scribe scope for this circuit (e.g., "origami", "asterisk").
	EnvScope = "CIRCUIT_SCOPE"

	// ExtraKeyProvider is the Extra map key for the injected anyllm.Provider.
	ExtraKeyProvider = "llm_provider"
	// ExtraKeyModel is the Extra map key for the LLM model name.
	ExtraKeyModel = "llm_model"
)

// SessionFactory returns a SessionFactory for the SDLC circuit.
// Reads SDLC_REPO_PATH and SDLC_MODE from environment to configure
// instruments (stub vs real Oculus/go-build/go-test).
//
// External dependencies are injected via params:
//   - params.Tools: Battery tool.Registry with Scribe/Locus (serve command owns connections)
//   - params.Extra[ExtraKeyProvider]: anyllm.Provider (serve command owns credentials)
//   - params.Extra[ExtraKeyModel]: LLM model name
//   - params.DomainFS: filesystem for circuit YAML (serve command owns)
func SessionFactory() engine.SessionFactory {
	return &sdlcFactory{}
}

type sdlcFactory struct{}

func (f *sdlcFactory) CreateSession(_ context.Context, params *engine.SessionParams) (*engine.SessionConfig, error) {
	repoPath := os.Getenv(EnvRepoPath)
	if repoPath == "" {
		repoPath = "."
	}

	mode := os.Getenv(EnvMode)
	if m, ok := params.Extra[EnvMode].(string); ok && m != "" {
		mode = m
	}

	// Extract injected provider from Extra (set by serve command).
	var provider anyllm.Provider
	var model string
	if p, ok := params.Extra[ExtraKeyProvider].(anyllm.Provider); ok {
		provider = p
	}
	if m, ok := params.Extra[ExtraKeyModel].(string); ok {
		model = m
	}

	transformers, err := buildTransformers(repoPath, mode, params.Tools, provider, model)
	if err != nil {
		return nil, err
	}

	domainFS := params.DomainFS
	if domainFS == nil {
		domainFS = os.DirFS(repoPath)
	}
	def, err := LoadCircuit(domainFS)
	if err != nil {
		return nil, fmt.Errorf("load sdlc circuit: %w", err)
	}

	caseCtx := map[string]any{}
	if scope := os.Getenv(EnvScope); scope != "" {
		caseCtx["scope"] = scope
	}

	cases := []engine.BatchCase{
		{ID: "sdlc-run", Context: caseCtx},
	}

	return &engine.SessionConfig{
		CircuitDef:   def,
		Transformers: transformers,
		Cases:        cases,
	}, nil
}

func buildTransformers(repoPath, mode string, tools *tool.Registry, provider anyllm.Provider, model string) (engine.TransformerRegistry, error) {
	if mode == "real" {
		return realTransformers(repoPath, tools, provider, model)
	}
	return StubTransformers(true), nil
}

func realTransformers(repoPath string, tools *tool.Registry, provider anyllm.Provider, model string) (engine.TransformerRegistry, error) {
	reg := StubTransformers(true)

	// Replace stubs with real instruments.
	reg["scan"] = oculusinst.NewScanTransformer(repoPath, oculusinst.WithLayers(OrigamiLayers))
	reg["build"] = gotools.NewBuildTransformer(repoPath)
	reg["test"] = gotools.NewTestTransformer(repoPath)

	// Wire LLM fix if provider was injected by serve command.
	if provider != nil && model != "" {
		reg["fix"] = llmfix.NewFixTransformer(provider, model, repoPath)
		slog.InfoContext(context.Background(), "LLM fix instrument wired via injected provider")
	}

	// Wire self-review if Scribe tools are available via Battery.
	// Day 1: no tools → stub (all_verified=true). Day 2: Scribe connected → real stamps.
	if tools != nil {
		if _, err := tools.Get("scribe.artifact"); err == nil {
			reg["self-review"] = selfreview.New(tools, repoPath)
			slog.InfoContext(context.Background(), "self-review wired via Battery tools")
		}
	}

	return reg, nil
}

// SchematicResolver returns a circuit asset resolver that reads sub-circuit
// YAML from the given filesystem. Resolves by name: planning, coding,
// verifying, operating, sdlc-v2.
func SchematicResolver(domainFS fs.FS) circuit.AssetResolver {
	return func(name string) ([]byte, error) {
		path := "circuits/" + name + ".yaml"
		data, err := fs.ReadFile(domainFS, path)
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %w", errUnknownCircuit, name, err)
		}
		return data, nil
	}
}
