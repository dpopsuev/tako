package sdlc

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"

	"github.com/dpopsuev/battery/tool"
	"github.com/dpopsuev/troupe/execution"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami/instruments/gotools"
	"github.com/dpopsuev/origami/instruments/llmfix"
	oculusinst "github.com/dpopsuev/origami/instruments/oculus"
	"github.com/dpopsuev/origami/instruments/selfreview"
)

var (
	errUnknownCircuit = errors.New("unknown circuit")
	errModelRequired  = errors.New("SDLC_MODEL is required when SDLC_PROVIDER is set")
	errProviderFailed = errors.New("failed to create LLM provider")
)

// Environment variable names.
const (
	// EnvRepoPath is the environment variable for the repository path.
	EnvRepoPath = "SDLC_REPO_PATH"
	// EnvMode is the environment variable for the execution mode (stub/real).
	EnvMode     = "SDLC_MODE"
	envProvider = "SDLC_PROVIDER" // "vertex-ai", "anthropic-api", etc.
	envModel    = "SDLC_MODEL"    // no default — fail fast

	// sdlcMaxTokens is the output token limit for all LLM calls in the SDLC circuit.
	// Set once here, propagated to all providers via ConfiguredProvider.
	// 16384 is enough for any single Go file (64k available on Sonnet 4.6).
	sdlcMaxTokens = 16384

	// EnvScribeEndpoint is the MCP endpoint for Scribe (used by serve command, not factory).
	EnvScribeEndpoint = "SCRIBE_ENDPOINT"
	// EnvLocusEndpoint is the MCP endpoint for Locus (used by serve command, not factory).
	EnvLocusEndpoint = "LOCUS_ENDPOINT"
	// EnvScope is the Scribe scope for this circuit (e.g., "origami", "asterisk").
	// Filters all Scribe queries so the circuit only sees its own artifacts.
	EnvScope = "CIRCUIT_SCOPE"

	logKeyProvider = "provider"
	logKeyModel    = "model"
)

// SessionFactory returns a SessionFactory for the SDLC circuit.
// Reads SDLC_REPO_PATH and SDLC_MODE from environment to configure
// instruments (stub vs real Oculus/go-build/go-test).
//
// External tools (Scribe, Locus) are injected via params.Tools by the
// serve command — the factory never connects to MCP services directly.
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

	transformers, err := buildTransformers(repoPath, mode, params.Tools)
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

func buildTransformers(repoPath, mode string, tools *tool.Registry) (engine.TransformerRegistry, error) {
	if mode == "real" {
		return realTransformers(repoPath, tools)
	}
	return StubTransformers(true), nil
}

func realTransformers(repoPath string, tools *tool.Registry) (engine.TransformerRegistry, error) {
	reg := StubTransformers(true)

	// Replace stubs with real instruments.
	reg["scan"] = oculusinst.NewScanTransformer(repoPath, oculusinst.WithLayers(OrigamiLayers))
	reg["build"] = gotools.NewBuildTransformer(repoPath)
	reg["test"] = gotools.NewTestTransformer(repoPath)

	// Wire LLM fix — explicit only, no defaults, fail fast.
	providerName := os.Getenv(envProvider)
	model := os.Getenv(envModel)
	if providerName != "" {
		if model == "" {
			return nil, fmt.Errorf("%w: %s=%q but %s is empty"+
				" (e.g. SDLC_MODEL=claude-sonnet-4-6 for Vertex, SDLC_MODEL=gpt-4o for OpenAI)",
				errModelRequired, envProvider, providerName, envModel)
		}
		provider, err := execution.NewProviderWithConfig(providerName, execution.ProviderConfig{
			MaxTokens: sdlcMaxTokens,
		})
		if err != nil {
			return nil, fmt.Errorf("%w: %q: %w", errProviderFailed, providerName, err)
		}
		slog.InfoContext(context.Background(), "LLM fix instrument wired",
			slog.String(logKeyProvider, providerName),
			slog.String(logKeyModel, model))
		reg["fix"] = llmfix.NewFixTransformer(provider, model, repoPath)
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
