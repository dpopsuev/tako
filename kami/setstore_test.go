package kami

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/view"
)

func testDef() *circuit.CircuitDef {
	return &circuit.CircuitDef{
		Circuit: "test",
		Nodes: []circuit.NodeDef{
			{Name: "recall"},
			{Name: "triage"},
			{Name: "resolve"},
		},
	}
}

func TestSetStore_NilToStore(t *testing.T) {
	bridge := NewEventBridge(nil)
	defer bridge.Close()

	srv := NewServer(Config{Bridge: bridge})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	httpAddr, _, err := srv.StartOnAvailablePort(ctx)
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	resp, err := http.Get(fmt.Sprintf("http://%s/events/stream", httpAddr))
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("want 503 with nil store, got %d", resp.StatusCode)
	}

	store := view.NewCircuitStore(testDef())
	defer store.Close()
	srv.SetStore(store)

	evtCh := make(chan Event, 1)
	go func() {
		resp, err := http.Get(fmt.Sprintf("http://%s/events/stream", httpAddr))
		if err != nil {
			return
		}
		defer resp.Body.Close()
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			var evt Event
			json.Unmarshal([]byte(line[6:]), &evt)
			evtCh <- evt
			return
		}
	}()

	time.Sleep(50 * time.Millisecond)
	store.OnEvent(circuit.WalkEvent{Type: circuit.EventNodeEnter, Node: "recall", Walker: "w1"})

	select {
	case evt := <-evtCh:
		if evt.Type != EventNodeEnter {
			t.Errorf("Type = %q, want %q", evt.Type, EventNodeEnter)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for SSE event after SetStore")
	}
}

func TestSetStore_StoreToStore(t *testing.T) {
	bridge := NewEventBridge(nil)
	defer bridge.Close()

	store1 := view.NewCircuitStore(testDef())
	srv := NewServer(Config{Bridge: bridge, Store: store1})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	httpAddr, _, err := srv.StartOnAvailablePort(ctx)
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	// Connect an SSE reader to store1.
	store1Closed := make(chan struct{})
	go func() {
		defer close(store1Closed)
		resp, err := http.Get(fmt.Sprintf("http://%s/events/stream", httpAddr))
		if err != nil {
			return
		}
		defer resp.Body.Close()
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			// Read until connection closes.
		}
	}()

	time.Sleep(50 * time.Millisecond)

	store2 := view.NewCircuitStore(testDef())
	defer store2.Close()
	srv.SetStore(store2)

	select {
	case <-store1Closed:
		// SSE reader exited because store1 was closed.
	case <-time.After(3 * time.Second):
		t.Fatal("SSE reader did not exit after store swap")
	}
}

func TestSetStore_OldStoreClosedOnSwap(t *testing.T) {
	store1 := view.NewCircuitStore(testDef())
	id, ch := store1.Subscribe()
	defer store1.Unsubscribe(id)

	bridge := NewEventBridge(nil)
	defer bridge.Close()

	srv := NewServer(Config{Bridge: bridge, Store: store1})

	store2 := view.NewCircuitStore(testDef())
	defer store2.Close()

	srv.SetStore(store2)

	_, ok := <-ch
	if ok {
		t.Error("expected channel to be closed after store swap")
	}
}

func TestSetStore_SSEHandlerCapturesStoreRef(t *testing.T) {
	bridge := NewEventBridge(nil)
	defer bridge.Close()

	store1 := view.NewCircuitStore(testDef())
	srv := NewServer(Config{Bridge: bridge, Store: store1})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	httpAddr, _, err := srv.StartOnAvailablePort(ctx)
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	// Start SSE reader targeting store1.
	sseReady := make(chan struct{})
	sseDone := make(chan struct{})
	go func() {
		defer close(sseDone)
		resp, err := http.Get(fmt.Sprintf("http://%s/events/stream", httpAddr))
		if err != nil {
			return
		}
		defer resp.Body.Close()
		close(sseReady)
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			// drain
		}
	}()

	select {
	case <-sseReady:
	case <-time.After(3 * time.Second):
		t.Fatal("SSE connection not established")
	}

	time.Sleep(50 * time.Millisecond)

	store2 := view.NewCircuitStore(testDef())
	defer store2.Close()
	srv.SetStore(store2)

	select {
	case <-sseDone:
		// Handler exited cleanly: Unsubscribe was called on store1.
	case <-time.After(3 * time.Second):
		t.Fatal("SSE handler did not exit cleanly after swap")
	}
}

func TestSetStore_ConcurrentSSEDuringSwap(t *testing.T) {
	bridge := NewEventBridge(nil)
	defer bridge.Close()

	store1 := view.NewCircuitStore(testDef())
	srv := NewServer(Config{Bridge: bridge, Store: store1})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	httpAddr, _, err := srv.StartOnAvailablePort(ctx)
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	const readers = 10
	var wg sync.WaitGroup
	wg.Add(readers)
	for i := 0; i < readers; i++ {
		go func() {
			defer wg.Done()
			resp, err := http.Get(fmt.Sprintf("http://%s/events/stream", httpAddr))
			if err != nil {
				return
			}
			defer resp.Body.Close()
			scanner := bufio.NewScanner(resp.Body)
			for scanner.Scan() {
			}
		}()
	}

	time.Sleep(100 * time.Millisecond)

	store2 := view.NewCircuitStore(testDef())
	defer store2.Close()
	srv.SetStore(store2)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("not all SSE readers exited after swap")
	}
}

func TestSetStore_RapidSwap200Sessions(t *testing.T) {
	if testing.Short() {
		t.Skip("stress test")
	}

	bridge := NewEventBridge(nil)
	defer bridge.Close()

	srv := NewServer(Config{Bridge: bridge})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	httpAddr, _, err := srv.StartOnAvailablePort(ctx)
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	const sessions = 200
	const readersPerSwap = 5

	var totalEvents atomic.Int64
	var allReaders sync.WaitGroup

	for i := 0; i < sessions; i++ {
		store := view.NewCircuitStore(testDef())
		srv.SetStore(store)

		allReaders.Add(readersPerSwap)
		for j := 0; j < readersPerSwap; j++ {
			go func() {
				defer allReaders.Done()
				resp, err := http.Get(fmt.Sprintf("http://%s/events/stream", httpAddr))
				if err != nil {
					return
				}
				defer resp.Body.Close()
				scanner := bufio.NewScanner(resp.Body)
				for scanner.Scan() {
					line := scanner.Text()
					if strings.HasPrefix(line, "data: ") {
						totalEvents.Add(1)
					}
				}
			}()
		}

		time.Sleep(5 * time.Millisecond)

		store.OnEvent(circuit.WalkEvent{Type: circuit.EventNodeEnter, Node: "recall", Walker: "w1"})
		store.OnEvent(circuit.WalkEvent{Type: circuit.EventNodeExit, Node: "recall"})
	}

	// Close the last store so the final batch of readers exits.
	srv.SetStore(nil)

	done := make(chan struct{})
	go func() {
		allReaders.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("not all SSE readers exited after 200 session swaps")
	}

	runtime.GC()
	time.Sleep(200 * time.Millisecond)

	final := runtime.NumGoroutine()
	delta := final - baseline
	if delta > 10 {
		t.Errorf("goroutine leak: baseline=%d final=%d delta=%d (max 10 allowed)", baseline, final, delta)
	}

	if totalEvents.Load() == 0 {
		t.Error("expected at least some SSE events to be received")
	}
	t.Logf("200 sessions complete: %d SSE events received, goroutine delta=%d", totalEvents.Load(), delta)
}

func TestSetStore_NilSwap(t *testing.T) {
	bridge := NewEventBridge(nil)
	defer bridge.Close()

	store := view.NewCircuitStore(testDef())
	srv := NewServer(Config{Bridge: bridge, Store: store})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	httpAddr, _, err := srv.StartOnAvailablePort(ctx)
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	srv.SetStore(nil)

	resp, err := http.Get(fmt.Sprintf("http://%s/events/stream", httpAddr))
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("want 503 after nil swap, got %d", resp.StatusCode)
	}
}
