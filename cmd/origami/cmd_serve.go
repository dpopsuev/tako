package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	battmcp "github.com/dpopsuev/battery/mcp"
	"github.com/dpopsuev/battery/tool"
	"github.com/dpopsuev/troupe/execution"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/dispatch"
	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami/engine/gate"
	"github.com/dpopsuev/origami/mcp"
	"github.com/dpopsuev/origami/simulate/sdlc"
	"github.com/dpopsuev/origami/toolkit"
	troupesignal "github.com/dpopsuev/troupe/signal"
)

var errUnknownCircuit = errors.New("unknown circuit")

const (
	logKeyCircuit  = "circuit"
	logKeyAddr     = "addr"
	logKeyEndpoint = "endpoint"
	logKeyService  = "service"
	logKeyTools    = "tools"
	logKeyError    = "error"
	logKeyModel    = "model"
	logKeyCount    = "count"

	serveReadHeaderTimeout = 10 * time.Second
)

func serveCmd(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	circuitName := fs.String("circuit", "sdlc", "circuit name (sdlc)")
	port := fs.Int("port", 9100, "HTTP port")
	stateDir := fs.String("state-dir", "", "directory for trace and run data")
	debug := fs.Bool("debug", false, "enable debug logging")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *debug {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// 1. Resolve circuit → SessionFactory.
	factory, err := resolveFactory(*circuitName)
	if err != nil {
		return err
	}

	// 2. Build CircuitConfig from factory.
	cfg := mcp.SessionFactoryToConfig(factory)
	cfg.Name = *circuitName
	cfg.Version = version
	if *stateDir != "" {
		cfg.StateDir = *stateDir
	}

	// 3. Wire approval gate.
	cfg.ApprovalStore = gate.NewMemoryStore()

	// 4. Connect to external MCP services via Battery MCPAdapter.
	cfg.Tools = connectExternalTools(ctx)

	// 5. Create LLM provider via Troupe execution (credential management).
	injectLLMProvider(&cfg)

	// 6. Auto-generate StepSchemas from circuit YAML output fields.
	repoPath := os.Getenv(sdlc.EnvRepoPath)
	if repoPath == "" {
		repoPath = "."
	}
	cfg.DomainFS = os.DirFS(repoPath)
	cfg.StepSchemas = generateStepSchemas(cfg.DomainFS)

	// 7. Create CircuitServer.
	srv := mcp.NewCircuitServer(&cfg)
	defer srv.Shutdown()

	// 6. Serve HTTP with graceful shutdown.
	addr := fmt.Sprintf(":%d", *port)
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: serveReadHeaderTimeout,
	}

	go func() {
		<-ctx.Done()
		_ = httpServer.Shutdown(context.Background())
	}()

	slog.InfoContext(ctx, "circuit server starting",
		slog.String(logKeyCircuit, *circuitName),
		slog.String(logKeyAddr, addr))

	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server: %w", err)
	}

	slog.InfoContext(ctx, "circuit server stopped")
	return nil
}

// connectExternalTools creates a Battery MCPAdapter and connects to any
// external MCP services specified via environment variables. Returns the
// shared tool.Registry (nil if no services configured).
func connectExternalTools(ctx context.Context) *tool.Registry {
	registry := tool.NewRegistry()
	adapter := battmcp.NewMCPAdapter(registry)

	services := []struct {
		name   string
		envVar string
	}{
		{"scribe", sdlc.EnvScribeEndpoint},
		{"locus", sdlc.EnvLocusEndpoint},
	}

	for _, svc := range services {
		endpoint := os.Getenv(svc.envVar)
		if endpoint == "" {
			continue
		}
		transport := &sdkmcp.StreamableClientTransport{Endpoint: endpoint}
		if err := adapter.RegisterMCP(ctx, svc.name, transport); err != nil {
			slog.WarnContext(ctx, "external service connection failed",
				slog.String(logKeyService, svc.name),
				slog.String(logKeyEndpoint, endpoint),
				slog.String(logKeyError, err.Error()))
		} else {
			slog.InfoContext(ctx, "external service connected",
				slog.String(logKeyService, svc.name),
				slog.String(logKeyEndpoint, endpoint))
		}
	}

	if len(registry.Names()) == 0 {
		return nil
	}

	slog.InfoContext(ctx, "Battery tools registered",
		slog.Any(logKeyTools, registry.Names()))

	return registry
}

// injectLLMProvider creates an LLM provider via Troupe's execution package
// (credential management) and injects it into the CircuitConfig so the factory
// receives it via params.Extra. The serve command owns credentials — the factory
// never touches env vars or provider creation.
func injectLLMProvider(cfg *mcp.CircuitConfig) {
	providerName := os.Getenv(sdlc.EnvProvider)
	model := os.Getenv(sdlc.EnvModel)
	if providerName == "" {
		return
	}
	if model == "" {
		slog.WarnContext(context.Background(), "SDLC_PROVIDER set but SDLC_MODEL empty — LLM fix disabled")
		return
	}

	provider, err := execution.NewProviderWithConfig(providerName, execution.ProviderConfig{
		MaxTokens: sdlc.SDLCMaxTokens,
	})
	if err != nil {
		slog.ErrorContext(context.Background(), "LLM provider creation failed — fix transformer will use stub",
			slog.String(logKeyError, err.Error()))
		return
	}

	// Wrap CreateSession to inject provider into Extra before the factory sees it.
	origCreate := cfg.CreateSession
	cfg.CreateSession = func(ctx context.Context, params mcp.StartParams, disp *dispatch.MuxDispatcher, bus troupesignal.Bus) (mcp.RunFunc, mcp.SessionMeta, error) {
		if params.Extra == nil {
			params.Extra = make(map[string]any)
		}
		params.Extra[sdlc.ExtraKeyProvider] = provider
		params.Extra[sdlc.ExtraKeyModel] = model
		return origCreate(ctx, params, disp, bus)
	}

	slog.InfoContext(context.Background(), "LLM provider injected via Troupe execution",
		slog.String(logKeyService, providerName),
		slog.String(logKeyModel, model))
}

// generateStepSchemas loads the circuit YAML from DomainFS and harvests
// output field declarations into StepSchemas. Each node with output: fields
// becomes a StepSchema with FieldDefs. Sub-circuits are also harvested.
func generateStepSchemas(domainFS fs.FS) []mcp.StepSchema {
	// Load main circuit.
	def, err := sdlc.LoadCircuit(domainFS)
	if err != nil {
		slog.WarnContext(context.Background(), "failed to load circuit for StepSchemas",
			slog.String(logKeyError, err.Error()))
		return nil
	}

	schemas := make([]mcp.StepSchema, 0, len(def.Nodes))

	// Harvest from main circuit nodes.
	for i := range def.Nodes {
		node := &def.Nodes[i]
		if len(node.Output) == 0 {
			continue
		}
		schemas = append(schemas, nodeToStepSchema(string(node.Name), node.Output))
	}

	// Harvest from sub-circuit nodes.
	resolver := sdlc.SchematicResolver(domainFS)
	for i := range def.Nodes {
		node := &def.Nodes[i]
		if node.Instrument != "circuit" {
			continue
		}
		subData, err := resolver(node.Action)
		if err != nil {
			continue
		}
		subDef, err := circuit.LoadCircuit(subData)
		if err != nil {
			continue
		}
		for j := range subDef.Nodes {
			subNode := &subDef.Nodes[j]
			if len(subNode.Output) == 0 {
				continue
			}
			schemas = append(schemas, nodeToStepSchema(string(subNode.Name), subNode.Output))
		}
	}

	if len(schemas) > 0 {
		slog.InfoContext(context.Background(), "StepSchemas generated from circuit YAML",
			slog.Int(logKeyCount, len(schemas)))
	}

	return schemas
}

func nodeToStepSchema(name string, outputs []circuit.OutputField) mcp.StepSchema {
	defs := make([]toolkit.FieldDef, len(outputs))
	for i, o := range outputs {
		defs[i] = toolkit.FieldDef{
			Name:     o.Name,
			Type:     o.Type,
			Required: o.Required,
		}
	}
	return mcp.StepSchema{Name: name, Defs: defs}
}

func resolveFactory(name string) (engine.SessionFactory, error) {
	switch name {
	case "sdlc":
		return sdlc.SessionFactory(), nil
	default:
		return nil, fmt.Errorf("%w: %q; available: sdlc", errUnknownCircuit, name)
	}
}
