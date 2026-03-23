// Command mediator runs the Origami Mediator — a session-aware MCP router
// that coordinates schematics via the Papercup protocol.
//
// Usage:
//
//	mediator [--port=9000] --backend rca=http://rca:9200/mcp --backend gnd:gnd=http://gnd:9100/mcp
//
// Backend format: name=url (default backend) or name:circuit_type=url (typed backend).
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

type backendFlags []string

func (b *backendFlags) String() string { return strings.Join(*b, ", ") }
func (b *backendFlags) Set(val string) error {
	*b = append(*b, val)
	return nil
}

func main() {
	port := flag.Int("port", 9000, "HTTP port for the mediator")
	healthz := flag.Bool("healthz", false, "probe /healthz and exit")
	var backends backendFlags
	flag.Var(&backends, "backend", "Backend: name=url or name:circuit_type=url (repeatable)")
	flag.Parse()

	if *healthz {
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/healthz", *port))
		if err != nil || resp.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		os.Exit(0)
	}

	if len(backends) == 0 {
		log.Fatal("at least one --backend is required")
	}

	var configs []mediator.BackendConfig
	for _, b := range backends {
		parts := strings.SplitN(b, "=", 2)
		if len(parts) != 2 {
			log.Fatalf("invalid backend format %q, expected name=url or name:circuit_type=url", b)
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
		log.Fatalf("start mediator: %v", err)
	}
	defer m.Stop(context.Background())

	addr := fmt.Sprintf(":%d", *port)
	httpServer := &http.Server{Addr: addr, Handler: m.Handler()}

	go func() {
		<-ctx.Done()
		_ = httpServer.Shutdown(context.Background())
	}()

	log.Printf("mediator listening on %s", addr)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}
