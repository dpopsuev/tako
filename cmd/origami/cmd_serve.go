package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami/engine/gate"
	"github.com/dpopsuev/origami/mcp"
	"github.com/dpopsuev/origami/simulate/sdlc"
)

var errUnknownCircuit = errors.New("unknown circuit")

const (
	logKeyCircuit = "circuit"
	logKeyAddr    = "addr"

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

	// 4. Create CircuitServer.
	srv := mcp.NewCircuitServer(&cfg)
	defer srv.Shutdown()

	// 5. Serve HTTP with graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

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

func resolveFactory(name string) (engine.SessionFactory, error) {
	switch name {
	case "sdlc":
		return sdlc.SessionFactory(), nil
	default:
		return nil, fmt.Errorf("%w: %q; available: sdlc", errUnknownCircuit, name)
	}
}
