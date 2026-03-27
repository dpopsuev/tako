package mcp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/dpopsuev/origami/testkit/contracts"
	"github.com/dpopsuev/origami/toolkit"
)

func TestMCPTransport_SatisfiesTransportInterface(t *testing.T) {
	var _ toolkit.Transport = (*MCPTransport)(nil)
}

func TestMCPTransport_PassesTransportContract(t *testing.T) {
	contracts.RunTransportContract(t, func() toolkit.Transport {
		return NewMCPTransport(MCPTransportConfig{Port: 0})
	})
}

func TestMCPTransport_ServesHTTP(t *testing.T) {
	tr := NewMCPTransport(MCPTransportConfig{Port: 0}) // port 0 = ephemeral

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- tr.Serve(ctx, nil)
	}()

	// Wait for server to start
	time.Sleep(50 * time.Millisecond)

	port := tr.Port()
	if port == 0 {
		t.Fatal("Port() returned 0 after Serve started")
	}

	// Health check
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/healthz", port))
	if err != nil {
		t.Fatalf("healthz GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("healthz status = %d, want 200", resp.StatusCode)
	}

	// Shutdown
	if err := tr.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	cancel()
	if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, context.Canceled) {
		t.Errorf("Serve error: %v", err)
	}
}
