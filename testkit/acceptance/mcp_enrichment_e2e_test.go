package acceptance_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/dpopsuev/tako/testkit/stubs"
	"github.com/dpopsuev/tako/tool"
	battmcp "github.com/dpopsuev/tako/tool/mcp"
)

// TestMCPEnrichment_E2E proves the wiring: Scribe + Lex + Locus
// connected via MCPAdapter, tools registered, callable through battery.Tool.
// This is the E2E skeleton — stubs return canned responses.
func TestMCPEnrichment_E2E(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	registry := tool.NewRegistry()
	adapter := battmcp.NewMCPAdapter(registry)

	// Connect all three MCP servers.
	if err := adapter.RegisterMCP(ctx, "scribe", stubs.StubScribeServer(t)); err != nil {
		t.Fatalf("register scribe: %v", err)
	}
	if err := adapter.RegisterMCP(ctx, "lex", stubs.StubLexServer(t)); err != nil {
		t.Fatalf("register lex: %v", err)
	}
	if err := adapter.RegisterMCP(ctx, "locus", stubs.StubLocusServer(t)); err != nil {
		t.Fatalf("register locus: %v", err)
	}

	// Verify all tools registered with prefixed names.
	names := registry.Names()
	t.Logf("registered tools: %v", names)

	// Should have: scribe.artifact, scribe.graph, scribe.admin,
	//              lex.lexicon, lex.config,
	//              locus.codograph, locus.analysis, locus.clinic, locus.lint, locus.triage
	if len(names) < 10 {
		t.Fatalf("expected >= 10 tools, got %d: %v", len(names), names)
	}

	// Verify Scribe tools work.
	result, err := registry.Execute(ctx, "scribe.artifact", json.RawMessage(`{"action":"list"}`))
	if err != nil {
		t.Fatalf("scribe.artifact: %v", err)
	}
	if result.Text() == "" {
		t.Error("scribe.artifact returned empty result")
	}
	t.Logf("scribe.artifact: %s", result.Text())

	// Verify Lex tools work.
	result, err = registry.Execute(ctx, "lex.lexicon", json.RawMessage(`{"action":"resolve"}`))
	if err != nil {
		t.Fatalf("lex.lexicon: %v", err)
	}
	t.Logf("lex.lexicon: %s", result.Text())

	// Verify Locus tools work.
	result, err = registry.Execute(ctx, "locus.analysis", json.RawMessage(`{"action":"deps"}`))
	if err != nil {
		t.Fatalf("locus.analysis: %v", err)
	}
	t.Logf("locus.analysis: %s", result.Text())

	// Verify cleanup.
	adapter.UnregisterMCP("scribe")
	adapter.UnregisterMCP("lex")
	adapter.UnregisterMCP("locus")

	if len(registry.Names()) != 0 {
		t.Errorf("after unregister: %v", registry.Names())
	}
}
