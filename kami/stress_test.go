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

func TestStress_200SessionsWithConcurrentSSE(t *testing.T) {
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

	const (
		sessions       = 200
		persistReaders = 5
		eventsPerStore = 3
	)

	var totalEvents atomic.Int64

	// Persistent SSE readers that reconnect on close.
	readerCtx, readerCancel := context.WithCancel(ctx)
	var readerWG sync.WaitGroup
	readerWG.Add(persistReaders)
	for i := 0; i < persistReaders; i++ {
		go func() {
			defer readerWG.Done()
			for {
				if readerCtx.Err() != nil {
					return
				}
				req, _ := http.NewRequestWithContext(readerCtx, "GET",
					fmt.Sprintf("http://%s/events/stream", httpAddr), nil)
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					if readerCtx.Err() != nil {
						return
					}
					time.Sleep(5 * time.Millisecond)
					continue
				}
				scanner := bufio.NewScanner(resp.Body)
				for scanner.Scan() {
					line := scanner.Text()
					if strings.HasPrefix(line, "data: ") {
						totalEvents.Add(1)
					}
				}
				resp.Body.Close()
			}
		}()
	}

	for i := 0; i < sessions; i++ {
		def := &circuit.CircuitDef{
			Circuit: fmt.Sprintf("stress-%d", i),
			Nodes: []circuit.NodeDef{
				{Name: "recall"},
				{Name: "triage"},
				{Name: "resolve"},
			},
		}
		store := view.NewCircuitStore(def)
		srv.SetStore(store)

		time.Sleep(2 * time.Millisecond)

		store.OnEvent(circuit.WalkEvent{Type: circuit.EventNodeEnter, Node: "recall", Walker: "w1"})
		store.OnEvent(circuit.WalkEvent{Type: circuit.EventNodeExit, Node: "recall"})
		store.OnEvent(circuit.WalkEvent{Type: circuit.EventNodeEnter, Node: "triage", Walker: "w1"})
	}

	// Close last store.
	srv.SetStore(nil)

	// Stop persistent readers.
	readerCancel()

	done := make(chan struct{})
	go func() {
		readerWG.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("persistent SSE readers did not exit")
	}

	runtime.GC()
	time.Sleep(200 * time.Millisecond)

	final := runtime.NumGoroutine()
	delta := final - baseline
	if delta > 10 {
		t.Errorf("goroutine leak: baseline=%d final=%d delta=%d (max 10 allowed)", baseline, final, delta)
	}

	evts := totalEvents.Load()
	if evts == 0 {
		t.Error("expected at least some SSE events across 200 sessions")
	}

	t.Logf("stress test complete: %d sessions, %d SSE events received by %d persistent readers, goroutine delta=%d",
		sessions, evts, persistReaders, delta)

	// Verify SSE now returns 503 (nil store).
	resp, err := http.Get(fmt.Sprintf("http://%s/events/stream", httpAddr))
	if err != nil {
		t.Fatalf("GET after cleanup: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503 after nil store, got %d", resp.StatusCode)
	}

	// Verify we can set a new store and it works.
	freshStore := view.NewCircuitStore(testDef())
	defer freshStore.Close()
	srv.SetStore(freshStore)

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
	freshStore.OnEvent(circuit.WalkEvent{Type: circuit.EventNodeEnter, Node: "recall", Walker: "w1"})

	select {
	case evt := <-evtCh:
		if evt.Type != EventNodeEnter {
			t.Errorf("post-stress event type = %q, want node_enter", evt.Type)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no SSE event received after stress test recovery")
	}
}
