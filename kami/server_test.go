package kami

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/view"
)

func TestServer_SSEStreamEvents(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit: "test",
		Nodes: []circuit.NodeDef{
			{Name: "triage"},
		},
	}
	store := view.NewCircuitStore(def)
	defer store.Close()

	bridge := NewEventBridge(nil)
	defer bridge.Close()

	srv := NewServer(Config{Bridge: bridge, Store: store})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	httpAddr, _, err := srv.StartOnAvailablePort(ctx)
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	sseURL := fmt.Sprintf("http://%s/events/stream", httpAddr)
	resp, err := http.Get(sseURL)
	if err != nil {
		t.Fatalf("GET /events/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("content-type = %q, want text/event-stream", ct)
	}

	type result struct {
		evt Event
		err error
	}
	ch := make(chan result, 1)
	go func() {
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			payload := strings.TrimPrefix(line, "data: ")
			var evt Event
			if err := json.Unmarshal([]byte(payload), &evt); err != nil {
				ch <- result{err: err}
				return
			}
			ch <- result{evt: evt}
			return
		}
		if err := scanner.Err(); err != nil {
			ch <- result{err: err}
		}
	}()

	time.Sleep(50 * time.Millisecond)
	store.OnEvent(circuit.WalkEvent{
		Type:   circuit.EventNodeEnter,
		Node:   "triage",
		Walker: "sentinel",
	})

	select {
	case r := <-ch:
		if r.err != nil {
			t.Fatalf("SSE read: %v", r.err)
		}
		if r.evt.Type != EventNodeEnter {
			t.Errorf("Type = %q, want %q", r.evt.Type, EventNodeEnter)
		}
		if r.evt.Node != "triage" {
			t.Errorf("Node = %q, want triage", r.evt.Node)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for SSE event")
	}
}

func TestServer_HealthEndpoint(t *testing.T) {
	bridge := NewEventBridge(nil)
	defer bridge.Close()

	srv := NewServer(Config{Bridge: bridge})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	httpAddr, _, err := srv.StartOnAvailablePort(ctx)
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	resp, err := http.Get(fmt.Sprintf("http://%s/api/health", httpAddr))
	if err != nil {
		t.Fatalf("GET /api/health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("status = %q, want ok", body["status"])
	}
}

func TestServer_SelectionStoredAndRetrieved(t *testing.T) {
	bridge := NewEventBridge(nil)
	defer bridge.Close()

	id, ch := bridge.Subscribe()
	defer bridge.Unsubscribe(id)

	srv := NewServer(Config{Bridge: bridge})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	httpAddr, _, err := srv.StartOnAvailablePort(ctx)
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	payload := `{"elements":[{"type":"node","id":"recall"},{"type":"agent","id":"Herald"}],"timestamp":"2026-02-25T10:00:00Z"}`
	resp, err := http.Post(
		fmt.Sprintf("http://%s/events/selection", httpAddr),
		"application/json",
		strings.NewReader(payload),
	)
	if err != nil {
		t.Fatalf("POST /events/selection: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}

	// Verify event was emitted to bridge
	select {
	case evt := <-ch:
		if evt.Type != "browser_selection" {
			t.Errorf("Type = %q, want browser_selection", evt.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for selection event")
	}

	// Verify selection is stored on the server
	sel := srv.GetSelection()
	if sel == nil {
		t.Fatal("GetSelection() returned nil")
	}
	elems, ok := sel["elements"]
	if !ok {
		t.Fatal("selection missing 'elements' key")
	}
	arr, ok := elems.([]any)
	if !ok {
		t.Fatalf("elements is %T, want []any", elems)
	}
	if len(arr) != 2 {
		t.Errorf("len(elements) = %d, want 2", len(arr))
	}
}

func TestServer_SelectionClearedOnEmpty(t *testing.T) {
	bridge := NewEventBridge(nil)
	defer bridge.Close()

	srv := NewServer(Config{Bridge: bridge})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	httpAddr, _, err := srv.StartOnAvailablePort(ctx)
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	// Set a selection
	resp, _ := http.Post(
		fmt.Sprintf("http://%s/events/selection", httpAddr),
		"application/json",
		strings.NewReader(`{"elements":[{"type":"node","id":"recall"}]}`),
	)
	resp.Body.Close()

	if srv.GetSelection() == nil {
		t.Fatal("expected selection to be set")
	}

	// Clear with empty elements
	resp, _ = http.Post(
		fmt.Sprintf("http://%s/events/selection", httpAddr),
		"application/json",
		strings.NewReader(`{"elements":[]}`),
	)
	resp.Body.Close()

	sel := srv.GetSelection()
	if sel == nil {
		t.Fatal("GetSelection() returned nil after clear")
	}
	elems := sel["elements"].([]any)
	if len(elems) != 0 {
		t.Errorf("expected empty elements after clear, got %d", len(elems))
	}
}

func TestServer_BrowserEventForwarded(t *testing.T) {
	bridge := NewEventBridge(nil)
	defer bridge.Close()

	id, ch := bridge.Subscribe()
	defer bridge.Unsubscribe(id)

	srv := NewServer(Config{Bridge: bridge})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	httpAddr, _, err := srv.StartOnAvailablePort(ctx)
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	resp, err := http.Post(
		fmt.Sprintf("http://%s/events/click", httpAddr),
		"application/json",
		strings.NewReader(`{"node":"triage","x":100,"y":200}`),
	)
	if err != nil {
		t.Fatalf("POST /events/click: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}

	select {
	case evt := <-ch:
		if evt.Type != "browser_click" {
			t.Errorf("Type = %q, want browser_click", evt.Type)
		}
		if evt.Data["node"] != "triage" {
			t.Errorf("Data[node] = %v, want triage", evt.Data["node"])
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for browser event")
	}
}
