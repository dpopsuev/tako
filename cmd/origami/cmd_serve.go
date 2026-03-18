package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"

	"github.com/dpopsuev/origami/mediator"
)

type serveBackendFlags []string

func (b *serveBackendFlags) String() string { return strings.Join(*b, ", ") }
func (b *serveBackendFlags) Set(val string) error {
	*b = append(*b, val)
	return nil
}

func serveCmd(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	port := fs.Int("port", 9000, "HTTP port for the mediator")
	var backends serveBackendFlags
	fs.Var(&backends, "backend", "Backend: name=url or name:circuit_type=url (repeatable)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if len(backends) == 0 {
		return fmt.Errorf("at least one --backend is required\nusage: origami serve --port 9000 --backend rca=http://localhost:9200/mcp")
	}

	var configs []mediator.BackendConfig
	for _, b := range backends {
		parts := strings.SplitN(b, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid backend format %q, expected name=url or name:circuit_type=url", b)
		}
		cfg := mediator.BackendConfig{Endpoint: parts[1]}
		nameOrTyped := parts[0]
		if typeParts := strings.SplitN(nameOrTyped, ":", 2); len(typeParts) == 2 {
			cfg.Name = typeParts[0]
			cfg.CircuitType = typeParts[1]
		} else {
			cfg.Name = nameOrTyped
		}
		configs = append(configs, cfg)
	}

	m := mediator.New(configs)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := m.Start(ctx); err != nil {
		return fmt.Errorf("start mediator: %w", err)
	}
	defer m.Stop(context.Background())

	addr := fmt.Sprintf(":%d", *port)
	httpServer := &http.Server{Addr: addr, Handler: m.Handler()}

	go func() {
		<-ctx.Done()
		httpServer.Shutdown(context.Background())
	}()

	log.Printf("origami mediator listening on %s", addr)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}
	return nil
}
