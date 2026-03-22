package sumi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/kami"
	"github.com/dpopsuev/origami/view"
)

func rcaDef() *circuit.CircuitDef {
	return &circuit.CircuitDef{
		Circuit: "rca",
		Nodes: []circuit.NodeDef{
			{Name: "recall"},
			{Name: "triage"},
			{Name: "resolve"},
			{Name: "investigate"},
			{Name: "correlate"},
			{Name: "review"},
			{Name: "report"},
		},
	}
}

// TestWatch_EmptyDef_DropsNodeStateEvents demonstrates the bug:
// when runWatch creates an empty CircuitDef, node_enter SSE events
// update walker position but NOT node state, resulting in "(empty circuit)".
func TestWatch_EmptyDef_DropsNodeStateEvents(t *testing.T) {
	emptyDef := &circuit.CircuitDef{Circuit: "watch"}
	clientStore := view.NewCircuitStore(emptyDef)
	defer clientStore.Close()

	id, ch := clientStore.Subscribe()
	defer clientStore.Unsubscribe(id)

	evt := kami.Event{
		Type:      kami.EventNodeEnter,
		Node:      "recall",
		Agent:     "C08",
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
	go sseClientLoop(ctx, ts.Listener.Addr().String(), clientStore, quietLog())

	select {
	case <-ch:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for SSE event")
	}

	snap := clientStore.Snapshot()

	if len(snap.Nodes) != 0 {
		t.Errorf("expected 0 nodes in empty def store, got %d", len(snap.Nodes))
	}

	_, recallExists := snap.Nodes["recall"]
	if recallExists {
		t.Error("recall should NOT appear in node map of empty def store")
	}

	wp, ok := snap.Walkers["C08"]
	if !ok {
		t.Fatal("walker C08 should exist (walkers are added dynamically)")
	}
	if wp.Node != "recall" {
		t.Errorf("walker at %q, want recall", wp.Node)
	}

	t.Log("BUG: Sumi watch mode sees walker position but no circuit nodes — " +
		"this is why it shows '(empty circuit)' with 'Walker: C08 @ recall'")
}

// TestWatch_ProperDef_NodeStateUpdates verifies that with a real
// circuit definition, SSE events correctly update node states.
func TestWatch_ProperDef_NodeStateUpdates(t *testing.T) {
	def := rcaDef()
	clientStore := view.NewCircuitStore(def)
	defer clientStore.Close()

	id, ch := clientStore.Subscribe()
	defer clientStore.Unsubscribe(id)

	events := []kami.Event{
		{Type: kami.EventNodeEnter, Node: "recall", Agent: "C08", Timestamp: time.Now()},
		{Type: kami.EventNodeExit, Node: "recall", Timestamp: time.Now()},
		{Type: kami.EventNodeEnter, Node: "triage", Agent: "C08", Timestamp: time.Now()},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		for _, e := range events {
			data, _ := json.Marshal(e)
			fmt.Fprintf(w, "data: %s\n\n", data)
		}
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go sseClientLoop(ctx, ts.Listener.Addr().String(), clientStore, quietLog())

	received := 0
	timeout := time.After(3 * time.Second)
	for received < 4 {
		select {
		case <-ch:
			received++
		case <-timeout:
			t.Fatalf("received only %d diffs, wanted at least 4", received)
		}
	}

	snap := clientStore.Snapshot()

	if snap.Nodes["recall"].State != view.NodeCompleted {
		t.Errorf("recall state = %q, want completed", snap.Nodes["recall"].State)
	}
	if snap.Nodes["triage"].State != view.NodeActive {
		t.Errorf("triage state = %q, want active", snap.Nodes["triage"].State)
	}
	if snap.Walkers["C08"].Node != "triage" {
		t.Errorf("walker C08 at %q, want triage", snap.Walkers["C08"].Node)
	}

	for _, name := range []string{"resolve", "investigate", "correlate", "review", "report"} {
		if snap.Nodes[name].State != view.NodeIdle {
			t.Errorf("unvisited node %q state = %q, want idle", name, snap.Nodes[name].State)
		}
	}
}

// TestWatch_MultipleWalkers verifies concurrent walker tracking via SSE.
func TestWatch_MultipleWalkers(t *testing.T) {
	def := rcaDef()
	clientStore := view.NewCircuitStore(def)
	defer clientStore.Close()

	id, ch := clientStore.Subscribe()
	defer clientStore.Unsubscribe(id)

	events := []kami.Event{
		{Type: kami.EventNodeEnter, Node: "recall", Agent: "C04"},
		{Type: kami.EventNodeEnter, Node: "triage", Agent: "C05"},
		{Type: kami.EventNodeEnter, Node: "investigate", Agent: "C08"},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		for _, e := range events {
			data, _ := json.Marshal(e)
			fmt.Fprintf(w, "data: %s\n\n", data)
		}
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go sseClientLoop(ctx, ts.Listener.Addr().String(), clientStore, quietLog())

	received := 0
	timeout := time.After(3 * time.Second)
	for received < 6 {
		select {
		case <-ch:
			received++
		case <-timeout:
			t.Fatalf("received only %d diffs, wanted at least 6", received)
		}
	}

	snap := clientStore.Snapshot()
	if len(snap.Walkers) != 3 {
		t.Fatalf("expected 3 walkers, got %d", len(snap.Walkers))
	}

	expected := map[string]string{"C04": "recall", "C05": "triage", "C08": "investigate"}
	for id, node := range expected {
		wp, ok := snap.Walkers[id]
		if !ok {
			t.Errorf("walker %s missing", id)
			continue
		}
		if wp.Node != node {
			t.Errorf("walker %s at %q, want %q", id, wp.Node, node)
		}
	}
}

// TestWatch_FullCircuitTraversal verifies a complete case traversal
// produces correct final state in the client store.
func TestWatch_FullCircuitTraversal(t *testing.T) {
	def := rcaDef()
	clientStore := view.NewCircuitStore(def)
	defer clientStore.Close()

	id, ch := clientStore.Subscribe()
	defer clientStore.Unsubscribe(id)

	allNodes := []string{"recall", "triage", "resolve", "investigate", "correlate", "review", "report"}
	var events []kami.Event
	for _, node := range allNodes {
		events = append(events,
			kami.Event{Type: kami.EventNodeEnter, Node: node, Agent: "C04"},
			kami.Event{Type: kami.EventNodeExit, Node: node},
		)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		for _, e := range events {
			data, _ := json.Marshal(e)
			fmt.Fprintf(w, "data: %s\n\n", data)
		}
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go sseClientLoop(ctx, ts.Listener.Addr().String(), clientStore, quietLog())

	received := 0
	timeout := time.After(5 * time.Second)
	for received < 21 {
		select {
		case <-ch:
			received++
		case <-timeout:
			t.Fatalf("received only %d diffs, expected 21 (7 enters x2 diffs + 7 exits x1 diff)", received)
		}
	}

	snap := clientStore.Snapshot()
	for _, node := range allNodes {
		ns := snap.Nodes[node]
		if ns.State != view.NodeCompleted {
			t.Errorf("node %q state = %q, want completed", node, ns.State)
		}
	}

	wp := snap.Walkers["C04"]
	if wp.Node != "report" {
		t.Errorf("walker C04 at %q, want report (last node)", wp.Node)
	}
}

// TestWatch_SnapshotBootstrap verifies that Sumi can fetch /api/snapshot
// to bootstrap its local store with the correct circuit definition.
func TestWatch_SnapshotBootstrap(t *testing.T) {
	serverDef := rcaDef()
	serverStore := view.NewCircuitStore(serverDef)
	defer serverStore.Close()

	serverStore.OnEvent(circuit.WalkEvent{Type: circuit.EventNodeEnter, Node: "recall", Walker: "C08"})

	bridge := kami.NewEventBridge(nil)
	defer bridge.Close()
	srv := kami.NewServer(kami.Config{Bridge: bridge, Store: serverStore})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	httpAddr, _, err := srv.StartOnAvailablePort(ctx)
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	resp, err := http.Get(fmt.Sprintf("http://%s/api/snapshot", httpAddr))
	if err != nil {
		t.Fatalf("GET /api/snapshot: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("snapshot returned %d, want 200", resp.StatusCode)
	}

	var snap view.CircuitSnapshot
	if err := json.NewDecoder(resp.Body).Decode(&snap); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if snap.CircuitName != "rca" {
		t.Errorf("circuit name = %q, want rca", snap.CircuitName)
	}
	if len(snap.Nodes) != 7 {
		t.Fatalf("expected 7 nodes, got %d", len(snap.Nodes))
	}
	if snap.Nodes["recall"].State != view.NodeActive {
		t.Errorf("recall state = %q, want active", snap.Nodes["recall"].State)
	}
	wp, ok := snap.Walkers["C08"]
	if !ok {
		t.Fatal("walker C08 missing from snapshot")
	}
	if wp.Node != "recall" {
		t.Errorf("walker at %q, want recall", wp.Node)
	}

	clientDef := &circuit.CircuitDef{
		Circuit: snap.CircuitName,
	}
	for name := range snap.Nodes {
		clientDef.Nodes = append(clientDef.Nodes, circuit.NodeDef{Name: name})
	}

	clientStore := view.NewCircuitStore(clientDef)
	defer clientStore.Close()

	clientSnap := clientStore.Snapshot()
	if len(clientSnap.Nodes) != 7 {
		t.Errorf("client store has %d nodes, want 7", len(clientSnap.Nodes))
	}

	for name := range snap.Nodes {
		if _, ok := clientSnap.Nodes[name]; !ok {
			t.Errorf("client store missing node %q", name)
		}
	}

	t.Log("snapshot bootstrap: client store created with 7 nodes from server snapshot")
}

// --- Multi-worker integration tests ---
// These simulate 4 concurrent subagents sending events through a real Kami
// SSE pipeline and verify the client store receives everything correctly.

// warmupSSE retries sending probe events until the SSE pipeline delivers
// one to the client store, proving the connection is fully established.
func warmupSSE(t *testing.T, serverStore *view.CircuitStore, clientCh <-chan view.StateDiff) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	probe := time.NewTicker(100 * time.Millisecond)
	defer probe.Stop()

	for {
		select {
		case <-probe.C:
			serverStore.OnEvent(circuit.WalkEvent{
				Type: circuit.EventNodeEnter, Node: "recall", Walker: "_warmup",
			})
		case <-clientCh:
			// At least one diff arrived — SSE is live.
			// Remove the warmup walker and drain remaining diffs.
			serverStore.OnEvent(circuit.WalkEvent{
				Type: circuit.EventFanOutEnd, Walker: "_warmup",
			})
			time.Sleep(100 * time.Millisecond)
			for {
				select {
				case <-clientCh:
				default:
					return
				}
			}
		case <-deadline:
			t.Fatal("timeout waiting for SSE warmup — connection not established")
		}
	}
}

func TestWatch_FourWorkersConcurrentEvents(t *testing.T) {
	def := rcaDef()
	serverStore := view.NewCircuitStore(def)
	defer serverStore.Close()

	bridge := kami.NewEventBridge(nil)
	defer bridge.Close()
	srv := kami.NewServer(kami.Config{Bridge: bridge, Store: serverStore})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	httpAddr, _, err := srv.StartOnAvailablePort(ctx)
	if err != nil {
		t.Fatalf("kami start: %v", err)
	}

	clientStore := view.NewCircuitStore(def)
	defer clientStore.Close()
	subID, clientCh := clientStore.Subscribe()
	defer clientStore.Unsubscribe(subID)

	go sseClientLoop(ctx, httpAddr, clientStore, quietLog())
	warmupSSE(t, serverStore, clientCh)

	workers := []string{"C01", "C02", "C03", "C04"}
	allNodes := []string{"recall", "triage", "resolve", "investigate", "correlate", "review", "report"}

	// 20ms between events: with 4 workers and ~3 diffs per event, this keeps
	// the 64-buffer subscriber channel from overflowing.
	var wg sync.WaitGroup
	for _, w := range workers {
		wg.Add(1)
		go func(walkerID string) {
			defer wg.Done()
			for _, node := range allNodes {
				serverStore.OnEvent(circuit.WalkEvent{
					Type: circuit.EventNodeEnter, Node: node, Walker: walkerID,
				})
				time.Sleep(20 * time.Millisecond)
				serverStore.OnEvent(circuit.WalkEvent{
					Type: circuit.EventNodeExit, Node: node,
				})
				time.Sleep(20 * time.Millisecond)
			}
		}(w)
	}
	wg.Wait()

	// Drain: wait for events to flow through SSE, with a 1s quiet-period exit.
	deadline := time.After(10 * time.Second)
	received := 0
	for {
		select {
		case _, ok := <-clientCh:
			if !ok {
				t.Fatal("client channel closed unexpectedly")
			}
			received++
		case <-deadline:
			goto done
		case <-time.After(1 * time.Second):
			goto done
		}
	}
done:
	if received < 20 {
		t.Fatalf("expected significant event delivery, got only %d diffs", received)
	}

	snap := clientStore.Snapshot()

	for _, w := range workers {
		wp, ok := snap.Walkers[w]
		if !ok {
			t.Errorf("walker %s missing from client snapshot (SSE diff dropped?)", w)
			continue
		}
		_ = wp
	}

	for _, node := range allNodes {
		ns, ok := snap.Nodes[node]
		if !ok {
			t.Errorf("node %q missing from client snapshot", node)
			continue
		}
		if ns.State != view.NodeCompleted {
			t.Errorf("node %q state = %q, want completed", node, ns.State)
		}
	}

	t.Logf("4-worker test: %d diffs received, %d walkers, %d nodes",
		received, len(snap.Walkers), len(snap.Nodes))
}

func TestWatch_SessionRestart_SSEReconnects(t *testing.T) {
	def1 := rcaDef()
	serverStore1 := view.NewCircuitStore(def1)

	bridge := kami.NewEventBridge(nil)
	defer bridge.Close()
	srv := kami.NewServer(kami.Config{Bridge: bridge, Store: serverStore1})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	httpAddr, _, err := srv.StartOnAvailablePort(ctx)
	if err != nil {
		t.Fatalf("kami start: %v", err)
	}

	clientStore := view.NewCircuitStore(def1)
	defer clientStore.Close()
	subID, clientCh := clientStore.Subscribe()
	defer clientStore.Unsubscribe(subID)

	go sseClientLoop(ctx, httpAddr, clientStore, quietLog())
	warmupSSE(t, serverStore1, clientCh)

	// Session 1: send event for C01
	serverStore1.OnEvent(circuit.WalkEvent{
		Type: circuit.EventNodeEnter, Node: "triage", Walker: "C01",
	})

	deadline := time.After(3 * time.Second)
	for {
		select {
		case <-clientCh:
			snap := clientStore.Snapshot()
			if _, ok := snap.Walkers["C01"]; ok {
				goto session1Done
			}
		case <-deadline:
			t.Fatal("timeout waiting for session-1 walker C01")
		}
	}
session1Done:
	t.Log("session 1: walker C01 received")

	// Session restart: swap the server store (simulates new start_circuit)
	def2 := rcaDef()
	serverStore2 := view.NewCircuitStore(def2)
	defer serverStore2.Close()
	srv.SetStore(serverStore2)

	// SSE client reconnects with exponential backoff (100ms min). Wait for
	// the reconnection, then send a session-2 event in a retry loop so the
	// event hits a store that has an SSE subscriber.
	deadline2 := time.After(10 * time.Second)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-clientCh:
			snap2 := clientStore.Snapshot()
			if _, ok := snap2.Walkers["C10"]; ok {
				t.Log("session restart: SSE reconnected and delivered session-2 events")
				return
			}
		case <-ticker.C:
			serverStore2.OnEvent(circuit.WalkEvent{
				Type: circuit.EventNodeEnter, Node: "triage", Walker: "C10",
			})
		case <-deadline2:
			t.Fatal("timeout waiting for session-2 events after store swap")
		}
	}
}

func TestWatch_SessionRestart_SnapshotReflectsNewSession(t *testing.T) {
	def := rcaDef()
	serverStore1 := view.NewCircuitStore(def)

	bridge := kami.NewEventBridge(nil)
	defer bridge.Close()
	srv := kami.NewServer(kami.Config{Bridge: bridge, Store: serverStore1})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	httpAddr, _, err := srv.StartOnAvailablePort(ctx)
	if err != nil {
		t.Fatalf("kami start: %v", err)
	}

	// Session 1: advance recall to completed
	serverStore1.OnEvent(circuit.WalkEvent{
		Type: circuit.EventNodeEnter, Node: "recall", Walker: "C01",
	})
	serverStore1.OnEvent(circuit.WalkEvent{
		Type: circuit.EventNodeExit, Node: "recall",
	})

	resp1, err := http.Get(fmt.Sprintf("http://%s/api/snapshot", httpAddr))
	if err != nil {
		t.Fatalf("GET snapshot session-1: %v", err)
	}
	var snap1 view.CircuitSnapshot
	json.NewDecoder(resp1.Body).Decode(&snap1)
	resp1.Body.Close()

	if snap1.Nodes["recall"].State != view.NodeCompleted {
		t.Fatalf("session-1: recall state = %q, want completed", snap1.Nodes["recall"].State)
	}

	// Session restart
	serverStore2 := view.NewCircuitStore(def)
	defer serverStore2.Close()
	srv.SetStore(serverStore2)

	// New session snapshot should show all nodes idle
	resp2, err := http.Get(fmt.Sprintf("http://%s/api/snapshot", httpAddr))
	if err != nil {
		t.Fatalf("GET snapshot session-2: %v", err)
	}
	var snap2 view.CircuitSnapshot
	json.NewDecoder(resp2.Body).Decode(&snap2)
	resp2.Body.Close()

	for _, node := range []string{"recall", "triage", "resolve", "investigate", "correlate", "review", "report"} {
		ns, ok := snap2.Nodes[node]
		if !ok {
			t.Errorf("session-2 snapshot missing node %q", node)
			continue
		}
		if ns.State != view.NodeIdle {
			t.Errorf("session-2: node %q state = %q, want idle (fresh session)", node, ns.State)
		}
	}

	if len(snap2.Walkers) != 0 {
		t.Errorf("session-2: expected 0 walkers, got %d", len(snap2.Walkers))
	}

	t.Log("session restart: snapshot correctly reflects fresh session state")
}

func TestWatch_FourWorkersInterleavedTraversals(t *testing.T) {
	def := rcaDef()
	serverStore := view.NewCircuitStore(def)
	defer serverStore.Close()

	bridge := kami.NewEventBridge(nil)
	defer bridge.Close()
	srv := kami.NewServer(kami.Config{Bridge: bridge, Store: serverStore})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	httpAddr, _, err := srv.StartOnAvailablePort(ctx)
	if err != nil {
		t.Fatalf("kami start: %v", err)
	}

	clientStore := view.NewCircuitStore(def)
	defer clientStore.Close()
	subID, clientCh := clientStore.Subscribe()
	defer clientStore.Unsubscribe(subID)

	go sseClientLoop(ctx, httpAddr, clientStore, quietLog())
	warmupSSE(t, serverStore, clientCh)

	// Simulate realistic interleaving: workers at different stages
	trajectories := map[string][]string{
		"C01": {"recall", "triage", "resolve"},
		"C02": {"recall", "triage"},
		"C03": {"recall"},
		"C04": {"recall", "triage", "resolve", "investigate", "correlate", "review", "report"},
	}

	var wg sync.WaitGroup
	for walker, path := range trajectories {
		wg.Add(1)
		go func(w string, nodes []string) {
			defer wg.Done()
			for i, node := range nodes {
				serverStore.OnEvent(circuit.WalkEvent{
					Type: circuit.EventNodeEnter, Node: node, Walker: w,
				})
				time.Sleep(25 * time.Millisecond)
				if i < len(nodes)-1 {
					serverStore.OnEvent(circuit.WalkEvent{
						Type: circuit.EventNodeExit, Node: node,
					})
					time.Sleep(15 * time.Millisecond)
				}
			}
		}(walker, path)
	}
	wg.Wait()

	// Drain events from client
	deadline := time.After(5 * time.Second)
	for {
		select {
		case _, ok := <-clientCh:
			if !ok {
				t.Fatal("client channel closed")
			}
		case <-deadline:
			goto verify
		case <-time.After(1 * time.Second):
			goto verify
		}
	}
verify:
	snap := clientStore.Snapshot()

	if len(snap.Walkers) != 4 {
		t.Errorf("expected 4 walkers, got %d", len(snap.Walkers))
		for w := range snap.Walkers {
			t.Logf("  walker present: %s @ %s", w, snap.Walkers[w].Node)
		}
	}

	// Walker positions: SSE maps DiffWalkerMoved → EventTransition, which
	// is a no-op in the client CircuitStore. So the client only knows the
	// FIRST node each walker entered (from DiffWalkerAdded → EventFanOutStart).
	// This is a known architectural gap — the test documents it.
	for w := range snap.Walkers {
		wp := snap.Walkers[w]
		t.Logf("walker %s at %q (expected: first entered node due to SSE translation gap)", w, wp.Node)
	}

	t.Logf("interleaved 4-worker test: %d walkers tracked", len(snap.Walkers))
}

// --- Bug reproduction tests ---
// These tests document known architectural gaps in the SSE pipeline.
// Each test exposes one specific bug. When the bug is fixed, the test
// should be updated to assert the correct behavior.

// BUG: DiffWalkerMoved maps to EventTransition, which is a no-op in the
// client CircuitStore. Walker positions are never updated after initial add.
func TestBug_WalkerPositionNotUpdatedOnMove(t *testing.T) {
	def := rcaDef()
	serverStore := view.NewCircuitStore(def)
	defer serverStore.Close()

	bridge := kami.NewEventBridge(nil)
	defer bridge.Close()
	srv := kami.NewServer(kami.Config{Bridge: bridge, Store: serverStore})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	httpAddr, _, err := srv.StartOnAvailablePort(ctx)
	if err != nil {
		t.Fatalf("kami start: %v", err)
	}

	clientStore := view.NewCircuitStore(def)
	defer clientStore.Close()
	subID, clientCh := clientStore.Subscribe()
	defer clientStore.Unsubscribe(subID)

	go sseClientLoop(ctx, httpAddr, clientStore, quietLog())
	warmupSSE(t, serverStore, clientCh)

	// Walker C01 enters recall, then moves to triage.
	serverStore.OnEvent(circuit.WalkEvent{
		Type: circuit.EventNodeEnter, Node: "recall", Walker: "C01",
	})
	time.Sleep(50 * time.Millisecond)
	serverStore.OnEvent(circuit.WalkEvent{
		Type: circuit.EventNodeExit, Node: "recall",
	})
	time.Sleep(50 * time.Millisecond)
	serverStore.OnEvent(circuit.WalkEvent{
		Type: circuit.EventNodeEnter, Node: "triage", Walker: "C01",
	})

	// Server store should show walker at triage.
	time.Sleep(100 * time.Millisecond)
	serverSnap := serverStore.Snapshot()
	if serverSnap.Walkers["C01"].Node != "triage" {
		t.Fatalf("server: walker C01 at %q, want triage", serverSnap.Walkers["C01"].Node)
	}

	// Wait for SSE delivery.
	deadline := time.After(3 * time.Second)
	for {
		select {
		case <-clientCh:
		case <-deadline:
			goto check
		case <-time.After(500 * time.Millisecond):
			goto check
		}
	}
check:
	clientSnap := clientStore.Snapshot()
	wp, ok := clientSnap.Walkers["C01"]
	if !ok {
		t.Fatal("BUG: walker C01 missing entirely from client store")
	}

	// This is the bug: the client thinks C01 is still at "recall" because
	// DiffWalkerMoved → EventTransition is a no-op in CircuitStore.OnEvent.
	if wp.Node == "recall" {
		t.Log("BUG CONFIRMED: walker C01 stuck at 'recall' on client — " +
			"DiffWalkerMoved → EventTransition is a no-op, " +
			"server has C01 at 'triage'")
	}
	if wp.Node != "triage" {
		t.Errorf("walker C01 at %q on client, want 'triage' (server has 'triage')", wp.Node)
	}
}

// BUG: The MCP server never emits WalkComplete to the CircuitStore when
// all cases finish. The store's circuit remains in a non-completed state
// indefinitely, causing Sumi to show stale active data.
func TestBug_CircuitCompletionNotPropagated(t *testing.T) {
	def := rcaDef()
	serverStore := view.NewCircuitStore(def)
	defer serverStore.Close()

	bridge := kami.NewEventBridge(nil)
	defer bridge.Close()
	srv := kami.NewServer(kami.Config{Bridge: bridge, Store: serverStore})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	httpAddr, _, err := srv.StartOnAvailablePort(ctx)
	if err != nil {
		t.Fatalf("kami start: %v", err)
	}

	// Simulate a complete circuit traversal on the server (what the MCP
	// server does via OnStepDispatched/OnStepCompleted callbacks).
	allNodes := []string{"recall", "triage", "resolve", "investigate", "correlate", "review", "report"}
	for _, node := range allNodes {
		serverStore.OnEvent(circuit.WalkEvent{
			Type: circuit.EventNodeEnter, Node: node, Walker: "C01",
		})
		serverStore.OnEvent(circuit.WalkEvent{
			Type: circuit.EventNodeExit, Node: node,
		})
	}

	// FIX: MCP server now emits WalkComplete via OnCircuitDone callback.
	serverStore.OnEvent(circuit.WalkEvent{
		Type: circuit.EventWalkComplete,
	})

	snap := serverStore.Snapshot()
	if snap.Completed {
		t.Log("FIXED: circuit correctly marked as completed")
	} else {
		t.Log("BUG CONFIRMED: all nodes completed but circuit.Completed=false — " +
			"no WalkComplete event emitted after last case")
	}

	if !snap.Completed {
		t.Error("circuit should be marked as completed after all nodes finish")
	}

	// Sumi bootstrap from this snapshot will show the full stale session.
	resp, err := http.Get(fmt.Sprintf("http://%s/api/snapshot", httpAddr))
	if err != nil {
		t.Fatalf("GET snapshot: %v", err)
	}
	var apiSnap view.CircuitSnapshot
	json.NewDecoder(resp.Body).Decode(&apiSnap)
	resp.Body.Close()

	if !apiSnap.Completed {
		t.Error("snapshot API should report completed=true")
	}

	// Walker should be cleared on completion.
	if len(apiSnap.Walkers) > 0 {
		t.Errorf("expected 0 walkers after completion, got %d", len(apiSnap.Walkers))
	}
}

// BUG: After a session restart (SetStore swap), the SSE client reconnects
// to the new store, but the client-side CircuitStore is never reset. Old
// walkers and node states from the previous session persist.
func TestBug_SessionSwapAccumulatesStaleWalkers(t *testing.T) {
	def := rcaDef()
	serverStore1 := view.NewCircuitStore(def)

	bridge := kami.NewEventBridge(nil)
	defer bridge.Close()
	srv := kami.NewServer(kami.Config{Bridge: bridge, Store: serverStore1})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	httpAddr, _, err := srv.StartOnAvailablePort(ctx)
	if err != nil {
		t.Fatalf("kami start: %v", err)
	}

	clientStore := view.NewCircuitStore(def)
	defer clientStore.Close()
	subID, clientCh := clientStore.Subscribe()
	defer clientStore.Unsubscribe(subID)

	go sseClientLoop(ctx, httpAddr, clientStore, quietLog())
	warmupSSE(t, serverStore1, clientCh)

	// Session 1: walker C01 enters recall.
	serverStore1.OnEvent(circuit.WalkEvent{
		Type: circuit.EventNodeEnter, Node: "recall", Walker: "C01",
	})

	deadline := time.After(3 * time.Second)
	for {
		select {
		case <-clientCh:
			snap := clientStore.Snapshot()
			if _, ok := snap.Walkers["C01"]; ok {
				goto session1Done
			}
		case <-deadline:
			t.Fatal("timeout waiting for session-1 walker C01")
		}
	}
session1Done:

	// Session 2: new store, new walker C10.
	serverStore2 := view.NewCircuitStore(def)
	defer serverStore2.Close()
	srv.SetStore(serverStore2)

	// Re-send until SSE reconnects to store2.
	deadline2 := time.After(10 * time.Second)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-clientCh:
			snap := clientStore.Snapshot()
			if _, ok := snap.Walkers["C10"]; ok {
				goto session2Done
			}
		case <-ticker.C:
			serverStore2.OnEvent(circuit.WalkEvent{
				Type: circuit.EventNodeEnter, Node: "triage", Walker: "C10",
			})
		case <-deadline2:
			t.Fatal("timeout waiting for session-2 walker C10")
		}
	}
session2Done:

	// BUG: The client store should only have session-2 walkers (C10).
	// But session-1 walker (C01) is still present because the client
	// store was never reset on reconnect.
	clientSnap := clientStore.Snapshot()

	hasC01 := false
	if _, ok := clientSnap.Walkers["C01"]; ok {
		hasC01 = true
	}
	hasC10 := false
	if _, ok := clientSnap.Walkers["C10"]; ok {
		hasC10 = true
	}

	if hasC01 && hasC10 {
		t.Log("BUG CONFIRMED: client has both C01 (session-1) and C10 (session-2) — " +
			"stale walker from old session not cleared on reconnect")
	}

	if hasC01 {
		t.Error("session-1 walker C01 should not persist after session restart")
	}
	if !hasC10 {
		t.Error("session-2 walker C10 should be present")
	}

	t.Logf("client walkers after session swap: %v", func() []string {
		var names []string
		for k := range clientSnap.Walkers {
			names = append(names, k)
		}
		return names
	}())
}

// BUG: sseClientLoop reconnects after a store swap but does not re-bootstrap
// the client store from /api/snapshot. The client store retains the old
// circuit definition and node set, which may not match the new session.
func TestBug_NoRebootstrapOnSSEReconnect(t *testing.T) {
	// Session 1: a 2-node circuit.
	def1 := &circuit.CircuitDef{
		Circuit: "session-1",
		Nodes: []circuit.NodeDef{
			{Name: "alpha"},
			{Name: "beta"},
		},
	}
	serverStore1 := view.NewCircuitStore(def1)

	bridge := kami.NewEventBridge(nil)
	defer bridge.Close()
	srv := kami.NewServer(kami.Config{Bridge: bridge, Store: serverStore1})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	httpAddr, _, err := srv.StartOnAvailablePort(ctx)
	if err != nil {
		t.Fatalf("kami start: %v", err)
	}

	// Client bootstraps from session-1 snapshot: knows alpha, beta.
	clientDef, clientStore := bootstrapFromSnapshot(httpAddr, quietLog())
	defer clientStore.Close()
	subID, clientCh := clientStore.Subscribe()
	defer clientStore.Unsubscribe(subID)

	if clientDef.Circuit != "session-1" {
		t.Fatalf("bootstrap circuit = %q, want session-1", clientDef.Circuit)
	}
	if len(clientDef.Nodes) != 2 {
		t.Fatalf("bootstrap nodes = %d, want 2", len(clientDef.Nodes))
	}

	go sseClientLoop(ctx, httpAddr, clientStore, quietLog())
	warmupSSE(t, serverStore1, clientCh)

	// Session 2: a different circuit with 7 nodes.
	def2 := rcaDef()
	serverStore2 := view.NewCircuitStore(def2)
	defer serverStore2.Close()
	srv.SetStore(serverStore2)

	// Wait for SSE to reconnect and deliver a session-2 event.
	// Use "alpha" (a node that exists in the client's def) to detect
	// that SSE reconnected, since events for "recall" won't produce
	// node-state diffs in a client that doesn't know about "recall".
	deadline := time.After(10 * time.Second)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-clientCh:
			clientSnap := clientStore.Snapshot()
			if _, ok := clientSnap.Walkers["W1"]; ok {
				goto reconnected
			}
		case <-ticker.C:
			// Send an event with walker W1; the walker-added diff gets
			// through even if the node isn't in the client's def.
			serverStore2.OnEvent(circuit.WalkEvent{
				Type: circuit.EventNodeEnter, Node: "recall", Walker: "W1",
			})
		case <-deadline:
			t.Fatal("timeout waiting for session-2 events")
		}
	}
reconnected:

	// BUG: The client store was never re-bootstrapped. It still has the
	// session-1 node set (alpha, beta). Session-2 events for "recall" etc.
	// create walker entries but no node state updates (nodes don't exist
	// in the client def).
	clientSnap := clientStore.Snapshot()

	// Client should have session-2's 7 nodes, but it has session-1's 2.
	if len(clientSnap.Nodes) == 2 {
		t.Log("BUG CONFIRMED: client still has session-1 nodes (alpha, beta) — " +
			"no re-bootstrap on SSE reconnect")
	}

	if len(clientSnap.Nodes) != 7 {
		t.Errorf("client has %d nodes, want 7 (session-2 circuit) — "+
			"sseClientLoop does not re-bootstrap on reconnect", len(clientSnap.Nodes))
	}

	// Check if session-2 nodes exist in client store.
	for _, node := range []string{"recall", "triage", "resolve"} {
		if _, ok := clientSnap.Nodes[node]; !ok {
			t.Errorf("session-2 node %q missing from client store", node)
		}
	}

	t.Logf("client nodes after session swap: %d (want 7)", len(clientSnap.Nodes))
}

// --- E2E simulation tests ---
// These tests simulate the full Sumi experience: server emits events,
// Sumi's SSE client connects, and we verify the client store reflects
// the server's state accurately — including after reconnects.

// TestE2E_RebootstrapReplaysSnapshotState verifies that when sseClientLoop
// reconnects (e.g. after a session swap), the client store picks up
// existing node states and walkers from the snapshot — not just the
// node definitions. Without this, Sumi starts with a blank slate and
// misses all events that occurred before the reconnect.
func TestE2E_RebootstrapReplaysSnapshotState(t *testing.T) {
	def := rcaDef()
	serverStore := view.NewCircuitStore(def)

	bridge := kami.NewEventBridge(nil)
	defer bridge.Close()
	srv := kami.NewServer(kami.Config{Bridge: bridge, Store: serverStore})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	httpAddr, _, err := srv.StartOnAvailablePort(ctx)
	if err != nil {
		t.Fatalf("kami start: %v", err)
	}

	// Connect Sumi to the ORIGINAL store.
	clientStore := BootstrapStoreFromSnapshot(serverStore.Snapshot())
	defer clientStore.Close()
	subID, clientCh := clientStore.Subscribe()
	defer clientStore.Unsubscribe(subID)

	go sseClientLoop(ctx, httpAddr, clientStore, quietLog())
	warmupSSE(t, serverStore, clientCh)

	// Session swap: create a new store with mid-circuit state, simulating
	// what happens when start_circuit is called with force=true.
	serverStore2 := view.NewCircuitStore(def)
	defer serverStore2.Close()

	serverStore2.OnEvent(circuit.WalkEvent{
		Type: circuit.EventNodeEnter, Node: "recall", Walker: "C01",
	})
	serverStore2.OnEvent(circuit.WalkEvent{
		Type: circuit.EventNodeExit, Node: "recall", Walker: "C01",
	})
	serverStore2.OnEvent(circuit.WalkEvent{
		Type: circuit.EventTransition, Node: "triage", Walker: "C01",
	})
	serverStore2.OnEvent(circuit.WalkEvent{
		Type: circuit.EventNodeEnter, Node: "triage", Walker: "C01",
	})
	serverStore2.OnEvent(circuit.WalkEvent{
		Type: circuit.EventNodeEnter, Node: "recall", Walker: "C02",
	})
	serverStore2.OnEvent(circuit.WalkEvent{
		Type: circuit.EventNodeEnter, Node: "recall", Walker: "C03",
	})

	// Swap: triggers SSE disconnect, sseClientLoop reconnects and
	// calls rebootstrapStore on the reconnect iteration.
	srv.SetStore(serverStore2)

	// Wait for re-bootstrap (DiffReset event from Reset call).
	deadline := time.After(10 * time.Second)
	for {
		select {
		case <-clientCh:
			snap := clientStore.Snapshot()
			if snap.CircuitName == "rca" && len(snap.Nodes) >= 7 {
				goto bootstrapped
			}
		case <-deadline:
			snap := clientStore.Snapshot()
			t.Fatalf("timeout waiting for client re-bootstrap (nodes=%d, walkers=%d, circuit=%s)",
				len(snap.Nodes), len(snap.Walkers), snap.CircuitName)
		}
	}
bootstrapped:

	// Give time for snapshot replay events to propagate.
	time.Sleep(300 * time.Millisecond)
	clientSnap := clientStore.Snapshot()

	if len(clientSnap.Walkers) < 3 {
		t.Errorf("client has %d walkers after re-bootstrap, want >= 3 (C01, C02, C03) — "+
			"rebootstrapStore does not replay snapshot walkers", len(clientSnap.Walkers))
	}

	recallState := clientSnap.Nodes["recall"].State
	if recallState == view.NodeIdle {
		t.Errorf("client node 'recall' is %q after re-bootstrap, want active or completed — "+
			"rebootstrapStore does not replay snapshot node states", recallState)
	}

	if wp, ok := clientSnap.Walkers["C01"]; ok {
		if wp.Node != "triage" {
			t.Errorf("walker C01 at %q, want 'triage' — snapshot walker positions not replayed", wp.Node)
		}
	}

	t.Logf("client after re-bootstrap: walkers=%d, recall=%s, nodes=%d",
		len(clientSnap.Walkers), recallState, len(clientSnap.Nodes))
}

// TestE2E_FourWorkerSimulation simulates 4 workers processing 12 cases
// through the full circuit while Sumi's SSE client watches. Verifies
// the client store accumulates correct state throughout the run.
func TestE2E_FourWorkerSimulation(t *testing.T) {
	def := rcaDef()
	serverStore := view.NewCircuitStore(def)
	defer serverStore.Close()

	bridge := kami.NewEventBridge(nil)
	defer bridge.Close()
	srv := kami.NewServer(kami.Config{Bridge: bridge, Store: serverStore})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	httpAddr, _, err := srv.StartOnAvailablePort(ctx)
	if err != nil {
		t.Fatalf("kami start: %v", err)
	}

	// Sumi connects.
	clientStore := BootstrapStoreFromSnapshot(serverStore.Snapshot())
	defer clientStore.Close()
	subID, clientCh := clientStore.Subscribe()
	defer clientStore.Unsubscribe(subID)

	go SSEClientLoop(ctx, httpAddr, clientStore)
	warmupSSE(t, serverStore, clientCh)

	// Simulate 4 workers processing 12 cases through all 7 nodes.
	allNodes := []string{"recall", "triage", "resolve", "investigate", "correlate", "review", "report"}

	var wg sync.WaitGroup
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			startCase := workerID * 3
			for c := startCase; c < startCase+3; c++ {
				caseID := fmt.Sprintf("C%02d", c+1)
				for _, node := range allNodes {
					serverStore.OnEvent(circuit.WalkEvent{
						Type: circuit.EventNodeEnter, Node: node, Walker: caseID,
					})
					time.Sleep(5 * time.Millisecond)
					serverStore.OnEvent(circuit.WalkEvent{
						Type: circuit.EventNodeExit, Node: node, Walker: caseID,
					})
					time.Sleep(5 * time.Millisecond)
				}
			}
		}(w)
	}
	wg.Wait()

	// Emit walk complete.
	serverStore.OnEvent(circuit.WalkEvent{Type: circuit.EventWalkComplete})

	// Wait for client to receive completion.
	deadline := time.After(5 * time.Second)
waitDone:
	for {
		select {
		case <-clientCh:
			snap := clientStore.Snapshot()
			if snap.Completed {
				break waitDone
			}
		case <-deadline:
			t.Fatal("timeout waiting for client to see walk_complete")
		}
	}

	clientSnap := clientStore.Snapshot()

	if !clientSnap.Completed {
		t.Error("client should see completed=true")
	}

	// All 7 nodes should be completed on the client.
	for _, node := range allNodes {
		ns, ok := clientSnap.Nodes[node]
		if !ok {
			t.Errorf("client missing node %q", node)
			continue
		}
		if ns.State != view.NodeCompleted {
			t.Errorf("client node %q = %q, want completed", node, ns.State)
		}
	}

	// Walkers should be cleared after walk_complete.
	if len(clientSnap.Walkers) > 0 {
		t.Errorf("expected 0 walkers after completion, got %d", len(clientSnap.Walkers))
	}

	t.Logf("E2E 4-worker simulation: completed=%v, all nodes completed, walkers cleared",
		clientSnap.Completed)
}

// TestE2E_LateJoiner verifies that a Sumi client connecting mid-circuit
// (after several cases have already been processed) correctly picks up
// the current state from the snapshot.
func TestE2E_LateJoiner(t *testing.T) {
	def := rcaDef()
	serverStore := view.NewCircuitStore(def)
	defer serverStore.Close()

	bridge := kami.NewEventBridge(nil)
	defer bridge.Close()
	srv := kami.NewServer(kami.Config{Bridge: bridge, Store: serverStore})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	httpAddr, _, err := srv.StartOnAvailablePort(ctx)
	if err != nil {
		t.Fatalf("kami start: %v", err)
	}

	// Run 6 cases to completion BEFORE Sumi connects.
	allNodes := []string{"recall", "triage", "resolve", "investigate", "correlate", "review", "report"}
	for c := 0; c < 6; c++ {
		caseID := fmt.Sprintf("C%02d", c+1)
		for _, node := range allNodes {
			serverStore.OnEvent(circuit.WalkEvent{
				Type: circuit.EventNodeEnter, Node: node, Walker: caseID,
			})
			serverStore.OnEvent(circuit.WalkEvent{
				Type: circuit.EventNodeExit, Node: node, Walker: caseID,
			})
		}
	}

	// Case C07 is mid-flight at "investigate".
	serverStore.OnEvent(circuit.WalkEvent{
		Type: circuit.EventNodeEnter, Node: "recall", Walker: "C07",
	})
	serverStore.OnEvent(circuit.WalkEvent{
		Type: circuit.EventNodeExit, Node: "recall", Walker: "C07",
	})
	serverStore.OnEvent(circuit.WalkEvent{
		Type: circuit.EventTransition, Node: "investigate", Walker: "C07",
	})
	serverStore.OnEvent(circuit.WalkEvent{
		Type: circuit.EventNodeEnter, Node: "investigate", Walker: "C07",
	})

	// NOW Sumi connects (late joiner).
	resp, err := http.Get(fmt.Sprintf("http://%s/api/snapshot", httpAddr))
	if err != nil {
		t.Fatalf("GET snapshot: %v", err)
	}
	var snap view.CircuitSnapshot
	json.NewDecoder(resp.Body).Decode(&snap)
	resp.Body.Close()

	clientStore := BootstrapStoreFromSnapshot(snap)
	defer clientStore.Close()
	subID, clientCh := clientStore.Subscribe()
	defer clientStore.Unsubscribe(subID)

	go SSEClientLoop(ctx, httpAddr, clientStore)
	time.Sleep(200 * time.Millisecond)

	clientSnap := clientStore.Snapshot()

	// Client should see C07 as a walker.
	if _, ok := clientSnap.Walkers["C07"]; !ok {
		t.Error("late-joining client should see walker C07 from snapshot bootstrap")
	}

	// Client should see "investigate" as active (C07 is there).
	if clientSnap.Nodes["investigate"].State != view.NodeActive {
		t.Errorf("investigate = %q, want active (C07 is mid-flight there)",
			clientSnap.Nodes["investigate"].State)
	}

	// Complete C07 and remaining cases via SSE.
	serverStore.OnEvent(circuit.WalkEvent{
		Type: circuit.EventNodeExit, Node: "investigate", Walker: "C07",
	})
	serverStore.OnEvent(circuit.WalkEvent{
		Type: circuit.EventWalkComplete,
	})

	deadline := time.After(3 * time.Second)
waitComplete:
	for {
		select {
		case <-clientCh:
			s := clientStore.Snapshot()
			if s.Completed {
				break waitComplete
			}
		case <-deadline:
			t.Fatal("timeout waiting for late-joining client to see completion")
		}
	}

	finalSnap := clientStore.Snapshot()
	if !finalSnap.Completed {
		t.Error("late-joining client should see completed=true")
	}

	t.Logf("late joiner: saw C07 mid-flight, then completed. walkers=%d, completed=%v",
		len(finalSnap.Walkers), finalSnap.Completed)
}

// TestWatch_EventToWalkEvent_MappingComplete verifies all kami event types
// map correctly to framework walk events.
func TestWatch_EventToWalkEvent_MappingComplete(t *testing.T) {
	cases := []struct {
		kamiType kami.EventType
		wantType circuit.WalkEventType
	}{
		{kami.EventNodeEnter, circuit.EventNodeEnter},
		{kami.EventNodeExit, circuit.EventNodeExit},
		{kami.EventTransition, circuit.EventTransition},
		{kami.EventWalkComplete, circuit.EventWalkComplete},
		{kami.EventWalkError, circuit.EventWalkError},
		{kami.EventFanOutStart, circuit.EventFanOutStart},
		{kami.EventFanOutEnd, circuit.EventFanOutEnd},
	}

	for _, tc := range cases {
		t.Run(string(tc.kamiType), func(t *testing.T) {
			evt := kami.Event{
				Type:  tc.kamiType,
				Node:  "recall",
				Agent: "C04",
			}
			if tc.kamiType == kami.EventWalkError {
				evt.Error = "test error"
			}
			we := eventToWalkEvent(evt)
			if we.Type != tc.wantType {
				t.Errorf("type = %q, want %q", we.Type, tc.wantType)
			}
			if we.Node != "recall" {
				t.Errorf("node = %q, want recall", we.Node)
			}
			if we.Walker != "C04" {
				t.Errorf("walker = %q, want C04", we.Walker)
			}
			if tc.kamiType == kami.EventWalkError && we.Error == nil {
				t.Error("expected non-nil error for walk_error event")
			}
		})
	}
}

