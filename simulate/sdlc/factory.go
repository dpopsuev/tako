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
	"github.com/dpopsuev/origami/instruments/enrichment"
	"github.com/dpopsuev/origami/instruments/gitops"
	"github.com/dpopsuev/origami/instruments/gotools"
	"github.com/dpopsuev/origami/instruments/llmfix"
	oculusinst "github.com/dpopsuev/origami/instruments/oculus"
	"github.com/dpopsuev/origami/instruments/scribeops"
	"github.com/dpopsuev/origami/instruments/selfreview"
	"github.com/dpopsuev/origami/instruments/tdd"
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
	// ExtraKeyTokenBudget is the Extra map key for the shared TokenBudget.
	ExtraKeyTokenBudget = "token_budget"
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

	// Extract injected provider + budget from Extra (set by serve command).
	var provider anyllm.Provider
	var model string
	var budget tdd.TokenBudget
	if p, ok := params.Extra[ExtraKeyProvider].(anyllm.Provider); ok {
		provider = p
	}
	if m, ok := params.Extra[ExtraKeyModel].(string); ok {
		model = m
	}
	if tb, ok := params.Extra[ExtraKeyTokenBudget].(tdd.TokenBudget); ok {
		budget = tb
	}

	transformers, err := buildTransformers(repoPath, mode, params.Tools, provider, model, budget)
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

func buildTransformers(repoPath, mode string, tools *tool.Registry, provider anyllm.Provider, model string, budget tdd.TokenBudget) (engine.TransformerRegistry, error) {
	if mode == "real" {
		return realTransformers(repoPath, tools, provider, model, budget)
	}
	return StubTransformers(true), nil
}

func realTransformers(repoPath string, tools *tool.Registry, provider anyllm.Provider, model string, budget tdd.TokenBudget) (engine.TransformerRegistry, error) {
	reg := StubTransformers(true)

	// Replace stubs with real instruments.
	reg["scan"] = oculusinst.NewScanTransformer(repoPath, oculusinst.WithLayers(OrigamiLayers))
	reg["build"] = gotools.NewBuildTransformer(repoPath)
	reg["test"] = gotools.NewTestTransformer(repoPath)
	reg["lint"] = gotools.NewLintTransformer(repoPath)
	reg["security-scan"] = gotools.NewSecurityScanTransformer(repoPath)
	reg["create-worktree"] = gitops.NewCreateWorktree(repoPath)
	reg["release"] = gitops.NewRelease(repoPath)

	// Wire LLM-backed instruments if provider was injected by serve command.
	if provider != nil && model != "" {
		var fixOpts []llmfix.FixOption
		if budget != nil {
			fixOpts = append(fixOpts, llmfix.WithTokenBudget(budget))
		}
		reg["fix"] = llmfix.NewFixTransformer(provider, model, repoPath, fixOpts...)
		reg["write-test"] = tdd.NewWriteTest(provider, model, repoPath, budget)
		reg["write-code"] = tdd.NewWriteCode(provider, model, repoPath, budget)
		reg["refactor"] = tdd.NewRefactor(provider, model, repoPath, budget)
		slog.InfoContext(context.Background(), "LLM instruments wired via injected provider")
	}

	// Wire Scribe-backed transformers if Battery tools are available.
	// Day 1: no tools → stubs. Day 2: Scribe connected → real.
	if tools != nil {
		if _, err := tools.Get("scribe.artifact"); err == nil {
			reg["self-review"] = selfreview.New(tools, repoPath)
			reg["poll-scribe"] = scribeops.NewPollScribe(tools)
			reg["mark-done"] = scribeops.NewMarkDone(tools)
			reg["file-bug"] = scribeops.NewFileBug(tools)
			reg["plan-review"] = scribeops.NewGatePassthrough("plan-review")
			reg["diff-review"] = scribeops.NewGatePassthrough("diff-review")
			reg["resolve-context"] = enrichment.NewResolveContext(tools)
			slog.InfoContext(context.Background(), "Scribe + enrichment transformers wired via Battery tools")
		}
	}

	return reg, nil
}

// SchematicResolver returns a circuit asset resolver that reads sub-circuit
// YAML from the given filesystem. Resolves by name: planning, coding,
// verifying, publishing.
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
