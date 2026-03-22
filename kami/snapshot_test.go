package kami

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/dpopsuev/origami/circuit"
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

func TestSSE_InitialSnapshot_ContainsCircuitNodes(t *testing.T) {
	def := rcaDef()
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

	snap := store.Snapshot()
	if len(snap.Nodes) != 7 {
		t.Fatalf("store snapshot has %d nodes, want 7", len(snap.Nodes))
	}
	if snap.CircuitName != "rca" {
		t.Errorf("circuit name = %q, want %q", snap.CircuitName, "rca")
	}

	resp, err := http.Get(fmt.Sprintf("http://%s/api/snapshot", httpAddr))
	if err != nil {
		t.Fatalf("GET /api/snapshot: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("snapshot status %d, want 200", resp.StatusCode)
	}

	var apiSnap view.CircuitSnapshot
	if err := json.NewDecoder(resp.Body).Decode(&apiSnap); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}

	if apiSnap.CircuitName != "rca" {
		t.Errorf("API circuit name = %q, want %q", apiSnap.CircuitName, "rca")
	}
	if len(apiSnap.Nodes) != 7 {
		t.Errorf("API snapshot has %d nodes, want 7", len(apiSnap.Nodes))
	}

	for _, name := range []string{"recall", "triage", "resolve", "investigate", "correlate", "review", "report"} {
		ns, ok := apiSnap.Nodes[name]
		if !ok {
			t.Errorf("node %q missing from snapshot", name)
			continue
		}
		if ns.State != view.NodeIdle {
			t.Errorf("node %q state = %q, want %q", name, ns.State, view.NodeIdle)
		}
	}
}

func TestSSE_NodeEnterUpdatesNodeState(t *testing.T) {
	def := rcaDef()
	store := view.NewCircuitStore(def)
	defer store.Close()

	bridge := NewEventBridge(nil)
	defer bridge.Close()

	srv := NewServer(Config{Bridge: bridge, Store: store})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv.StartOnAvailablePort(ctx)

	store.OnEvent(circuit.WalkEvent{
		Type:   circuit.EventNodeEnter,
		Node:   "recall",
		Walker: "C08",
	})

	snap := store.Snapshot()

	ns, ok := snap.Nodes["recall"]
	if !ok {
		t.Fatal("recall node missing from snapshot")
	}
	if ns.State != view.NodeActive {
		t.Errorf("recall state = %q, want %q", ns.State, view.NodeActive)
	}

	wp, ok := snap.Walkers["C08"]
	if !ok {
		t.Fatal("walker C08 missing from snapshot")
	}
	if wp.Node != "recall" {
		t.Errorf("walker C08 at %q, want %q", wp.Node, "recall")
	}
}

func TestSSE_EmptyDefDropsNodeEvents(t *testing.T) {
	emptyDef := &circuit.CircuitDef{Circuit: "watch"}
	store := view.NewCircuitStore(emptyDef)
	defer store.Close()

	store.OnEvent(circuit.WalkEvent{
		Type:   circuit.EventNodeEnter,
		Node:   "recall",
		Walker: "C08",
	})

	snap := store.Snapshot()

	if len(snap.Nodes) != 0 {
		t.Errorf("expected 0 nodes in empty def store, got %d", len(snap.Nodes))
	}

	_, nodeExists := snap.Nodes["recall"]
	if nodeExists {
		t.Error("recall node should NOT exist in empty def store")
	}

	wp, walkerExists := snap.Walkers["C08"]
	if !walkerExists {
		t.Error("walker C08 should still be added (walkers are dynamic)")
	} else if wp.Node != "recall" {
		t.Errorf("walker at %q, want recall", wp.Node)
	}

	t.Log("BUG CONFIRMED: empty CircuitDef silently drops node state updates, " +
		"but walkers are tracked — this is why Sumi shows '(empty circuit)' with a walker position")
}

func TestSSE_WalkerMovesAcrossNodes(t *testing.T) {
	def := rcaDef()
	store := view.NewCircuitStore(def)
	defer store.Close()

	steps := []string{"recall", "triage", "resolve", "investigate"}
	for _, node := range steps {
		store.OnEvent(circuit.WalkEvent{
			Type:   circuit.EventNodeEnter,
			Node:   node,
			Walker: "C04",
		})
		store.OnEvent(circuit.WalkEvent{
			Type: circuit.EventNodeExit,
			Node: node,
		})
	}

	snap := store.Snapshot()

	wp := snap.Walkers["C04"]
	if wp.Node != "investigate" {
		t.Errorf("walker C04 at %q, want %q", wp.Node, "investigate")
	}

	for _, name := range []string{"recall", "triage", "resolve", "investigate"} {
		ns := snap.Nodes[name]
		if ns.State != view.NodeCompleted {
			t.Errorf("node %q state = %q, want %q", name, ns.State, view.NodeCompleted)
		}
	}

	for _, name := range []string{"correlate", "review", "report"} {
		ns := snap.Nodes[name]
		if ns.State != view.NodeIdle {
			t.Errorf("unvisited node %q state = %q, want %q", name, ns.State, view.NodeIdle)
		}
	}
}

func TestSSE_MultipleWalkersAtDifferentNodes(t *testing.T) {
	def := rcaDef()
	store := view.NewCircuitStore(def)
	defer store.Close()

	store.OnEvent(circuit.WalkEvent{Type: circuit.EventNodeEnter, Node: "recall", Walker: "C04"})
	store.OnEvent(circuit.WalkEvent{Type: circuit.EventNodeEnter, Node: "triage", Walker: "C05"})
	store.OnEvent(circuit.WalkEvent{Type: circuit.EventNodeEnter, Node: "investigate", Walker: "C08"})

	snap := store.Snapshot()
	if len(snap.Walkers) != 3 {
		t.Fatalf("expected 3 walkers, got %d", len(snap.Walkers))
	}

	cases := map[string]string{"C04": "recall", "C05": "triage", "C08": "investigate"}
	for walkerID, expectedNode := range cases {
		wp, ok := snap.Walkers[walkerID]
		if !ok {
			t.Errorf("walker %s missing", walkerID)
			continue
		}
		if wp.Node != expectedNode {
			t.Errorf("walker %s at %q, want %q", walkerID, wp.Node, expectedNode)
		}
	}
}

func TestSnapshot_NoStore_Returns503(t *testing.T) {
	bridge := NewEventBridge(nil)
	defer bridge.Close()

	srv := NewServer(Config{Bridge: bridge})
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
	resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503 when no store is set", resp.StatusCode)
	}
}

func TestSnapshot_AfterSetStore_ReturnsNewCircuit(t *testing.T) {
	bridge := NewEventBridge(nil)
	defer bridge.Close()

	srv := NewServer(Config{Bridge: bridge})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	httpAddr, _, err := srv.StartOnAvailablePort(ctx)
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	resp, _ := http.Get(fmt.Sprintf("http://%s/api/snapshot", httpAddr))
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 before SetStore, got %d", resp.StatusCode)
	}

	store := view.NewCircuitStore(rcaDef())
	defer store.Close()
	srv.SetStore(store)

	resp, err = http.Get(fmt.Sprintf("http://%s/api/snapshot", httpAddr))
	if err != nil {
		t.Fatalf("GET /api/snapshot: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 after SetStore, got %d", resp.StatusCode)
	}

	var snap view.CircuitSnapshot
	json.NewDecoder(resp.Body).Decode(&snap)
	if snap.CircuitName != "rca" {
		t.Errorf("circuit name = %q, want %q", snap.CircuitName, "rca")
	}
	if len(snap.Nodes) != 7 {
		t.Errorf("nodes = %d, want 7", len(snap.Nodes))
	}
}

func TestSSE_EventsMatchStoreSnapshot(t *testing.T) {
	def := rcaDef()
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

	store.OnEvent(circuit.WalkEvent{Type: circuit.EventNodeEnter, Node: "recall", Walker: "C04"})
	store.OnEvent(circuit.WalkEvent{Type: circuit.EventNodeExit, Node: "recall"})
	store.OnEvent(circuit.WalkEvent{Type: circuit.EventNodeEnter, Node: "triage", Walker: "C04"})

	time.Sleep(50 * time.Millisecond)

	resp, err := http.Get(fmt.Sprintf("http://%s/api/snapshot", httpAddr))
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	var snap view.CircuitSnapshot
	json.NewDecoder(resp.Body).Decode(&snap)

	if snap.Nodes["recall"].State != view.NodeCompleted {
		t.Errorf("recall = %q, want completed", snap.Nodes["recall"].State)
	}
	if snap.Nodes["triage"].State != view.NodeActive {
		t.Errorf("triage = %q, want active", snap.Nodes["triage"].State)
	}
	if snap.Walkers["C04"].Node != "triage" {
		t.Errorf("C04 at %q, want triage", snap.Walkers["C04"].Node)
	}
}
