package sumi

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/kami"
	"github.com/dpopsuev/origami/view"
)

func testDef() *circuit.CircuitDef {
	return &circuit.CircuitDef{
		Circuit: "test",
		Nodes: []circuit.NodeDef{
			{Name: "recall"},
			{Name: "triage"},
		},
	}
}

func quietLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(nil, &slog.HandlerOptions{Level: slog.LevelError + 1}))
}

type logEntry struct {
	Level string
	Msg   string
	Attrs map[string]string
}

type logSink struct {
	mu      sync.Mutex
	entries []logEntry
}

func (s *logSink) append(e logEntry) {
	s.mu.Lock()
	s.entries = append(s.entries, e)
	s.mu.Unlock()
}

func (s *logSink) snapshot() []logEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]logEntry, len(s.entries))
	copy(cp, s.entries)
	return cp
}

func capturingLog() (*slog.Logger, *logSink) {
	sink := &logSink{}
	handler := &captureHandler{sink: sink}
	return slog.New(handler), sink
}

type captureHandler struct {
	sink  *logSink
	attrs []slog.Attr
}

func (h *captureHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }
func (h *captureHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &captureHandler{sink: h.sink, attrs: append(h.attrs, attrs...)}
}
func (h *captureHandler) WithGroup(_ string) slog.Handler { return h }
func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	e := logEntry{
		Level: r.Level.String(),
		Msg:   r.Message,
		Attrs: make(map[string]string),
	}
	for _, a := range h.attrs {
		e.Attrs[a.Key] = a.Value.String()
	}
	r.Attrs(func(a slog.Attr) bool {
		e.Attrs[a.Key] = a.Value.String()
		return true
	})
	h.sink.append(e)
	return nil
}

func TestSSEClient_ReceivesEvents(t *testing.T) {
	store := view.NewCircuitStore(testDef())
	defer store.Close()

	id, ch := store.Subscribe()
	defer store.Unsubscribe(id)

	evt := kami.Event{
		Type:      kami.EventNodeEnter,
		Node:      "recall",
		Agent:     "w1",
		Timestamp: time.Now(),
	}
	data, _ := json.Marshal(evt)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		fmt.Fprintf(w, "data: %s\n\n", data)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	addr := ts.Listener.Addr().String()
	go sseClientLoop(ctx, addr, store, quietLog())

	select {
	case diff := <-ch:
		if diff.Node != "recall" {
			t.Errorf("Node = %q, want recall", diff.Node)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for store event from SSE client")
	}
}

func TestSSEClient_ReconnectsOnClose(t *testing.T) {
	store := view.NewCircuitStore(testDef())
	defer store.Close()

	id, ch := store.Subscribe()
	defer store.Unsubscribe(id)

	var connectCount atomic.Int32

	evt := kami.Event{
		Type:      kami.EventNodeEnter,
		Node:      "triage",
		Agent:     "w1",
		Timestamp: time.Now(),
	}
	data, _ := json.Marshal(evt)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connectCount.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		fmt.Fprintf(w, "data: %s\n\n", data)
		w.(http.Flusher).Flush()
		// Close immediately to trigger reconnect.
	}))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	addr := ts.Listener.Addr().String()
	go sseClientLoop(ctx, addr, store, quietLog())

	received := 0
	timeout := time.After(5 * time.Second)
	for received < 3 {
		select {
		case <-ch:
			received++
		case <-timeout:
			t.Fatalf("received only %d events, wanted at least 3 (reconnect test)", received)
		}
	}

	connects := connectCount.Load()
	if connects < 2 {
		t.Errorf("expected at least 2 connections (reconnect), got %d", connects)
	}
	t.Logf("received %d store diffs across %d connections", received, connects)
}

func TestSSEClient_ContextCancellation(t *testing.T) {
	store := view.NewCircuitStore(testDef())
	defer store.Close()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		sseClientLoop(ctx, ts.Listener.Addr().String(), store, quietLog())
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("sseClientLoop did not exit after context cancellation")
	}
}

func TestSSEClient_ErrorStatus(t *testing.T) {
	store := view.NewCircuitStore(testDef())
	defer store.Close()

	var connectCount atomic.Int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := connectCount.Add(1)
		if n <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		evt := kami.Event{Type: kami.EventNodeEnter, Node: "recall", Agent: "w1"}
		data, _ := json.Marshal(evt)
		fmt.Fprintf(w, "data: %s\n\n", data)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	id, ch := store.Subscribe()
	defer store.Unsubscribe(id)

	go sseClientLoop(ctx, ts.Listener.Addr().String(), store, quietLog())

	select {
	case <-ch:
		connects := connectCount.Load()
		if connects < 3 {
			t.Errorf("expected at least 3 attempts, got %d", connects)
		}
		t.Logf("recovered after %d connection attempts", connects)
	case <-time.After(10 * time.Second):
		t.Fatal("SSE client did not recover from error status")
	}
}

// --- Instrumentation tests ---
// These verify that the SSE pipeline emits structured log entries at key
// decision points, enabling debug-mode diagnosis of live runs.

func TestSSE_Logging_StreamConnectAndEvent(t *testing.T) {
	store := view.NewCircuitStore(testDef())
	defer store.Close()

	evt := kami.Event{
		Type:  kami.EventNodeEnter,
		Node:  "recall",
		Agent: "w1",
	}
	data, _ := json.Marshal(evt)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		fmt.Fprintf(w, "data: %s\n\n", data)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())

	log, entries := capturingLog()
	id, ch := store.Subscribe()
	defer store.Unsubscribe(id)

	go sseClientLoop(ctx, ts.Listener.Addr().String(), store, log)

	select {
	case <-ch:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout")
	}
	cancel()
	time.Sleep(100 * time.Millisecond)

	// Should have logged SSE connection and event receipt.
	captured := entries.snapshot()
	hasConnect := false
	hasEvent := false
	for _, e := range captured {
		if e.Msg == "SSE connected" {
			hasConnect = true
		}
		if e.Msg == "SSE event received" {
			hasEvent = true
		}
	}
	if !hasConnect {
		t.Error("missing 'SSE connected' log entry — streamSSE should log on successful connect")
	}
	if !hasEvent {
		t.Error("missing 'SSE event received' log entry — streamSSE should log each event")
	}
	t.Logf("captured %d log entries", len(captured))
}

func TestSSE_Logging_RebootstrapSuccess(t *testing.T) {
	store := view.NewCircuitStore(testDef())
	defer store.Close()

	bridge := kami.NewEventBridge(nil)
	defer bridge.Close()
	srv := kami.NewServer(kami.Config{Bridge: bridge, Store: store})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	httpAddr, _, err := srv.StartOnAvailablePort(ctx)
	if err != nil {
		t.Fatalf("kami start: %v", err)
	}

	clientStore := view.NewCircuitStore(&circuit.CircuitDef{Circuit: "empty"})
	defer clientStore.Close()

	log, entries := capturingLog()
	rebootstrapStore(httpAddr, clientStore, log)

	hasRebootstrap := false
	for _, e := range entries.snapshot() {
		if e.Msg == "re-bootstrapped store from snapshot" {
			hasRebootstrap = true
			if e.Attrs["circuit"] != "test" {
				t.Errorf("circuit = %q, want 'test'", e.Attrs["circuit"])
			}
			if e.Attrs["nodes"] != "2" {
				t.Errorf("nodes = %q, want '2'", e.Attrs["nodes"])
			}
		}
	}
	if !hasRebootstrap {
		t.Error("missing 're-bootstrapped store from snapshot' log entry")
	}
}

func TestSSE_Logging_RebootstrapFailure(t *testing.T) {
	clientStore := view.NewCircuitStore(&circuit.CircuitDef{Circuit: "empty"})
	defer clientStore.Close()

	log, entries := capturingLog()
	rebootstrapStore("127.0.0.1:1", clientStore, log)

	hasFailure := false
	for _, e := range entries.snapshot() {
		if e.Msg == "re-bootstrap snapshot unavailable" {
			hasFailure = true
		}
	}
	if !hasFailure {
		t.Error("missing 're-bootstrap snapshot unavailable' log entry on connection failure")
	}
}

func TestSSE_Logging_ReconnectCycle(t *testing.T) {
	store := view.NewCircuitStore(testDef())
	defer store.Close()

	var connectCount atomic.Int32

	evt := kami.Event{
		Type:  kami.EventNodeEnter,
		Node:  "recall",
		Agent: "w1",
	}
	data, _ := json.Marshal(evt)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := connectCount.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		fmt.Fprintf(w, "data: %s\n\n", data)
		w.(http.Flusher).Flush()
		if n < 3 {
			return // close to trigger reconnect
		}
		<-r.Context().Done()
	}))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())

	log, entries := capturingLog()
	id, ch := store.Subscribe()
	defer store.Unsubscribe(id)

	go sseClientLoop(ctx, ts.Listener.Addr().String(), store, log)

	received := 0
	timeout := time.After(5 * time.Second)
	for received < 3 {
		select {
		case <-ch:
			received++
		case <-timeout:
			t.Fatalf("timeout with %d events", received)
		}
	}
	cancel()
	time.Sleep(200 * time.Millisecond)

	// Count log entries by message type.
	counts := map[string]int{}
	for _, e := range entries.snapshot() {
		counts[e.Msg]++
	}

	// Should see at least 1 reconnect attempt (SSE reconnecting log).
	if counts["SSE reconnecting"] < 1 {
		t.Error("expected at least 1 'SSE reconnecting' log")
	}

	// Should see at least 2 connections.
	if counts["SSE connected"] < 2 {
		t.Errorf("expected >= 2 SSE connections, got %d", counts["SSE connected"])
	}

	// Should see re-bootstrap attempts on reconnects (not first connect).
	// The test server doesn't serve /api/snapshot, so re-bootstrap
	// attempts produce decode failures, which still proves the attempt was made.
	rebootstrapAttempts := counts["re-bootstrapped store from snapshot"] +
		counts["re-bootstrap snapshot decode failed"] +
		counts["re-bootstrap snapshot unavailable"] +
		counts["re-bootstrap snapshot non-200"]
	if rebootstrapAttempts < 1 {
		t.Error("expected at least 1 re-bootstrap attempt on reconnect")
	}

	t.Logf("log counts: %v", counts)
}

// --- Server-restart reconnect tests ---

// TestSSE_ServerRestart_Reconnects simulates MCP disable→re-enable: the server
// goes away (connection refused), then comes back on the same port with a new
// circuit definition. The SSE client must reconnect, re-bootstrap the store from
// the new server's snapshot, and deliver events from the new session.
func TestSSE_ServerRestart_Reconnects(t *testing.T) {
	def := testDef()
	clientStore := view.NewCircuitStore(def)
	defer clientStore.Close()

	id, ch := clientStore.Subscribe()
	defer clientStore.Unsubscribe(id)

	// Phase 1: start a server, send one event, then shut it down.
	phase1Done := make(chan struct{})
	ts1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/snapshot":
			snap := view.CircuitSnapshot{
				CircuitName: "test",
				Nodes: map[string]view.NodeState{
					"recall": {Name: "recall", State: view.NodeIdle},
					"triage": {Name: "triage", State: view.NodeIdle},
				},
				Walkers: map[string]view.WalkerPosition{},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(snap)
		case "/events/stream":
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(200)
			evt := kami.Event{Type: kami.EventNodeEnter, Node: "recall", Agent: "w1"}
			data, _ := json.Marshal(evt)
			fmt.Fprintf(w, "data: %s\n\n", data)
			w.(http.Flusher).Flush()
			<-phase1Done
		default:
			w.WriteHeader(404)
		}
	}))

	ctx, cancel := context.WithCancel(context.Background())

	addr := ts1.Listener.Addr().String()
	go sseClientLoop(ctx, addr, clientStore, quietLog())

	// Wait for the phase-1 event.
	drainUntil(t, ch, 3*time.Second, func() bool {
		snap := clientStore.Snapshot()
		_, ok := snap.Walkers["w1"]
		return ok
	})

	// Kill the server — SSE loop will start failing.
	close(phase1Done)
	ts1.Close()

	// Phase 2: start a NEW server on the SAME address with an expanded circuit.
	ln, err := newListenerOnAddr(addr)
	if err != nil {
		t.Skipf("cannot rebind addr %s: %v", addr, err)
	}

	ts2 := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/snapshot":
			snap := view.CircuitSnapshot{
				CircuitName: "circuit-v2",
				Nodes: map[string]view.NodeState{
					"recall":      {Name: "recall", State: view.NodeIdle},
					"triage":      {Name: "triage", State: view.NodeIdle},
					"investigate": {Name: "investigate", State: view.NodeIdle},
				},
				Walkers: map[string]view.WalkerPosition{},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(snap)
		case "/events/stream":
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(200)
			evt := kami.Event{Type: kami.EventNodeEnter, Node: "recall", Agent: "w2"}
			data, _ := json.Marshal(evt)
			fmt.Fprintf(w, "data: %s\n\n", data)
			w.(http.Flusher).Flush()
			<-r.Context().Done()
		default:
			w.WriteHeader(404)
		}
	}))
	ts2.Listener.Close()
	ts2.Listener = ln
	ts2.Start()

	// Poll until the store reflects the new circuit, nodes, AND walker.
	drainUntil(t, ch, 15*time.Second, func() bool {
		snap := clientStore.Snapshot()
		_, hasW2 := snap.Walkers["w2"]
		return snap.CircuitName == "circuit-v2" && len(snap.Nodes) == 3 && hasW2
	})

	// Cancel SSE client before closing server to avoid ts2.Close() blocking.
	cancel()

	snap := clientStore.Snapshot()
	if snap.CircuitName != "circuit-v2" {
		t.Errorf("circuit = %q, want circuit-v2", snap.CircuitName)
	}
	if len(snap.Nodes) != 3 {
		t.Errorf("nodes = %d, want 3", len(snap.Nodes))
	}
	if _, ok := snap.Walkers["w2"]; !ok {
		t.Error("walker w2 should exist from new session")
	}

	ts2.Close()
}

// TestSSE_BackoffResetsAfterSuccess verifies that after a successful connection,
// the exponential backoff resets to the minimum so the next failure recovers fast.
func TestSSE_BackoffResetsAfterSuccess(t *testing.T) {
	store := view.NewCircuitStore(testDef())
	defer store.Close()

	id, ch := store.Subscribe()
	defer store.Unsubscribe(id)

	var sseConnCount atomic.Int32

	evt := kami.Event{Type: kami.EventNodeEnter, Node: "recall", Agent: "w1"}
	data, _ := json.Marshal(evt)

	// Path-aware server: /api/snapshot always returns a valid snapshot,
	// /events/stream follows the 503→503→success→close→success pattern.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/snapshot":
			snap := view.CircuitSnapshot{
				CircuitName: "test",
				Nodes: map[string]view.NodeState{
					"recall": {Name: "recall", State: view.NodeIdle},
					"triage": {Name: "triage", State: view.NodeIdle},
				},
				Walkers: map[string]view.WalkerPosition{},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(snap)
		case "/events/stream":
			n := sseConnCount.Add(1)
			switch {
			case n <= 2:
				w.WriteHeader(http.StatusServiceUnavailable)
			case n == 3:
				w.Header().Set("Content-Type", "text/event-stream")
				w.WriteHeader(200)
				fmt.Fprintf(w, "data: %s\n\n", data)
				w.(http.Flusher).Flush()
			default:
				w.Header().Set("Content-Type", "text/event-stream")
				w.WriteHeader(200)
				fmt.Fprintf(w, "data: %s\n\n", data)
				w.(http.Flusher).Flush()
				<-r.Context().Done()
			}
		default:
			w.WriteHeader(404)
		}
	}))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log, entries := capturingLog()
	go sseClientLoop(ctx, ts.Listener.Addr().String(), store, log)

	// Wait until at least 2 SSE events are received (connections 3 and 4).
	drainUntil(t, ch, 10*time.Second, func() bool {
		return sseConnCount.Load() >= 4
	})

	// Find the reconnect log for the iteration AFTER a successful connection.
	// Iterations: 1=first(no log), 2=reconnect(503), 3=reconnect(503), 4=reconnect(success→close), 5=reconnect.
	// With path-aware routing, SSE connections are: 1=503, 2=503, 3=success, 4=success.
	// sseClientLoop iterations: 1(no log,503), 2(log,503), 3(log,success+close), 4(log,success).
	// If backoff resets after success at iteration 3, iteration 4 should have backoff ~100ms.
	captured := entries.snapshot()
	var foundBackoff bool
	var backoffValue time.Duration
	for _, e := range captured {
		if e.Msg == "SSE reconnecting" {
			if b, ok := e.Attrs["backoff"]; ok {
				d, _ := time.ParseDuration(b)
				backoffValue = d
			}
		}
	}
	// The last logged backoff should be ≤200ms if reset worked.
	// Without reset, it grows: 100→200→400→800ms.
	foundBackoff = backoffValue > 0
	if !foundBackoff {
		t.Fatal("no 'SSE reconnecting' log entries with backoff found")
	}
	if backoffValue > 200*time.Millisecond {
		t.Errorf("last reconnect backoff = %v, want <= 200ms (backoff should reset after success)", backoffValue)
	}
}

// TestModel_KamiStatusTransitionsOnReconnect verifies that the Model's kamiStatus
// field transitions from KamiOffline to KamiConnected when a DiffReset arrives
// (indicating the SSE client reconnected and re-bootstrapped the store).
func TestModel_KamiStatusTransitionsOnReconnect(t *testing.T) {
	def := testDef()
	store := view.NewCircuitStore(def)
	defer store.Close()

	engine := &view.GridLayout{}
	layout, _ := engine.Layout(def)

	m := New(Config{
		Def:    def,
		Store:  store,
		Layout: layout,
	})

	if m.kamiStatus != KamiOffline {
		t.Fatalf("initial kamiStatus = %d, want KamiOffline", m.kamiStatus)
	}

	// Simulate a DiffReset from SSE reconnect (store was re-bootstrapped).
	m.applyDiff(view.StateDiff{Type: view.DiffReset})
	m.snap = store.Snapshot()

	if m.kamiStatus != KamiConnected {
		t.Errorf("kamiStatus after DiffReset = %d, want KamiConnected (%d)", m.kamiStatus, KamiConnected)
	}
}

// drainUntil reads diffs from ch until pred() returns true or the deadline expires.
func drainUntil(t *testing.T, ch <-chan view.StateDiff, deadline time.Duration, pred func() bool) {
	t.Helper()
	timeout := time.After(deadline)
	for {
		if pred() {
			return
		}
		select {
		case <-ch:
		case <-timeout:
			t.Fatalf("drainUntil: timed out after %v", deadline)
		}
	}
}

func newListenerOnAddr(addr string) (net.Listener, error) {
	return net.Listen("tcp", addr)
}
