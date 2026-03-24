package dispatch

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/dpopsuev/bugle/signal"
)

func startTestServer(t *testing.T, mux *MuxDispatcher, opts ...NetworkServerOption) (string, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())

	srv := NewNetworkServer(mux, "127.0.0.1:0", opts...)
	ready := make(chan string, 1)

	go func() {
		// Wait for server to bind, then emit address
		go func() {
			for {
				addr := srv.Addr()
				if addr != "127.0.0.1:0" && addr != "" {
					ready <- addr
					return
				}
				time.Sleep(time.Millisecond)
			}
		}()
		if err := srv.Serve(ctx); err != nil {
			t.Logf("server stopped: %v", err)
		}
	}()

	select {
	case addr := <-ready:
		return addr, cancel
	case <-time.After(3 * time.Second):
		cancel()
		t.Fatal("server did not start in time")
		return "", cancel
	}
}

func TestNetworkDispatch_SingleAgent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mux := NewMuxDispatcher(ctx)
	addr, stopServer := startTestServer(t, mux)
	defer stopServer()

	client := NewNetworkClient("http://" + addr)

	// Runner sends a dispatch in a goroutine
	var dispatchResult []byte
	var dispatchErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		dispatchResult, dispatchErr = mux.Dispatch(context.Background(), DispatchContext{
			CaseID:       "C1",
			Step:         "F0_RECALL",
			PromptPath:   "/tmp/prompt.json",
			ArtifactPath: "/tmp/artifact.json",
		})
	}()

	// Agent pulls work over network
	dc, err := client.GetNextStep(ctx)
	if err != nil {
		t.Fatalf("GetNextStep: %v", err)
	}
	if dc.CaseID != "C1" || dc.Step != "F0_RECALL" {
		t.Errorf("got case=%q step=%q, want C1/F0_RECALL", dc.CaseID, dc.Step)
	}

	// Agent submits artifact over network
	artifact := []byte(`{"analysis":"root cause found"}`)
	if err := client.SubmitArtifact(ctx, dc.DispatchID, artifact); err != nil {
		t.Fatalf("SubmitArtifact: %v", err)
	}

	<-done
	if dispatchErr != nil {
		t.Fatalf("Dispatch: %v", dispatchErr)
	}
	if string(dispatchResult) != string(artifact) {
		t.Errorf("dispatch result = %q, want %q", dispatchResult, artifact)
	}
}

func TestNetworkDispatch_TwoAgentsConcurrent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mux := NewMuxDispatcher(ctx)
	addr, stopServer := startTestServer(t, mux)
	defer stopServer()

	var wg sync.WaitGroup

	// Two agents connect
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := NewNetworkClient("http://" + addr)
			dc, err := client.GetNextStep(ctx)
			if err != nil {
				t.Errorf("agent %d GetNextStep: %v", i, err)
				return
			}
			artifact := []byte(fmt.Sprintf(`{"agent":%d}`, i))
			if err := client.SubmitArtifact(ctx, dc.DispatchID, artifact); err != nil {
				t.Errorf("agent %d SubmitArtifact: %v", i, err)
			}
		}()
	}

	// Runner dispatches two tasks
	results := make([][]byte, 2)
	errs := make([]error, 2)
	var dispatchWg sync.WaitGroup
	for i := 0; i < 2; i++ {
		dispatchWg.Add(1)
		go func() {
			defer dispatchWg.Done()
			data, err := mux.Dispatch(context.Background(), DispatchContext{
				CaseID: fmt.Sprintf("C%d", i),
				Step:   "F0",
			})
			results[i] = data
			errs[i] = err
		}()
	}

	dispatchWg.Wait()
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("dispatch %d: %v", i, err)
		}
		if len(results[i]) == 0 {
			t.Errorf("dispatch %d: empty result", i)
		}
	}
}

func TestNetworkDispatch_ProtocolIdenticalToInProcess(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mux := NewMuxDispatcher(ctx)
	addr, stopServer := startTestServer(t, mux)
	defer stopServer()

	client := NewNetworkClient("http://" + addr)

	done := make(chan struct{})
	go func() {
		defer close(done)
		mux.Dispatch(context.Background(), DispatchContext{
			CaseID:       "C1",
			Step:         "TRIAGE",
			PromptPath:   "/p/prompt.json",
			ArtifactPath: "/p/artifact.json",
		})
	}()

	dc, err := client.GetNextStep(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if dc.PromptPath != "/p/prompt.json" {
		t.Errorf("PromptPath = %q, want /p/prompt.json", dc.PromptPath)
	}
	if dc.ArtifactPath != "/p/artifact.json" {
		t.Errorf("ArtifactPath = %q, want /p/artifact.json", dc.ArtifactPath)
	}
	if dc.DispatchID == 0 {
		t.Error("DispatchID should be non-zero")
	}

	client.SubmitArtifact(ctx, dc.DispatchID, []byte(`{}`))
	<-done
}

func TestNetworkDispatch_InProcessModeUnchanged(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	mux := NewMuxDispatcher(ctx)

	done := make(chan struct{})
	go func() {
		defer close(done)
		data, err := mux.Dispatch(context.Background(), DispatchContext{CaseID: "C1", Step: "F0"})
		if err != nil {
			t.Errorf("Dispatch: %v", err)
			return
		}
		if string(data) != `{"ok":true}` {
			t.Errorf("got %q", data)
		}
	}()

	dc, err := mux.GetNextStep(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := mux.SubmitArtifact(ctx, dc.DispatchID, []byte(`{"ok":true}`)); err != nil {
		t.Fatal(err)
	}

	<-done
}

func TestNetworkClient_SubmitBadID(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	mux := NewMuxDispatcher(ctx)
	addr, stopServer := startTestServer(t, mux)
	defer stopServer()

	client := NewNetworkClient("http://" + addr)
	err := client.SubmitArtifact(ctx, 999, []byte(`{}`))
	if err == nil {
		t.Fatal("expected error for unknown dispatch ID")
	}
}

func TestNetworkSignal_EmitAndGet(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	mux := NewMuxDispatcher(ctx)
	bus := signal.NewMemBus()
	addr, stopServer := startTestServer(t, mux, WithSignalBus(bus))
	defer stopServer()

	client := NewNetworkClient("http://" + addr)

	if err := client.EmitSignal(ctx, "worker_started", "worker", "", "", map[string]string{"worker_id": "w0"}); err != nil {
		t.Fatalf("EmitSignal: %v", err)
	}
	if err := client.EmitSignal(ctx, "start", "worker", "C1", "F0", nil); err != nil {
		t.Fatalf("EmitSignal: %v", err)
	}

	sigs, err := client.GetSignals(ctx, 0)
	if err != nil {
		t.Fatalf("GetSignals: %v", err)
	}
	if len(sigs) != 2 {
		t.Fatalf("GetSignals: got %d signals, want 2", len(sigs))
	}
	if sigs[0].Event != "worker_started" {
		t.Errorf("signal[0].Event = %q, want worker_started", sigs[0].Event)
	}
	if sigs[0].Meta["worker_id"] != "w0" {
		t.Errorf("signal[0].Meta[worker_id] = %q, want w0", sigs[0].Meta["worker_id"])
	}
	if sigs[1].CaseID != "C1" || sigs[1].Step != "F0" {
		t.Errorf("signal[1]: case=%q step=%q, want C1/F0", sigs[1].CaseID, sigs[1].Step)
	}

	// Since filtering
	sigs2, err := client.GetSignals(ctx, 1)
	if err != nil {
		t.Fatalf("GetSignals(since=1): %v", err)
	}
	if len(sigs2) != 1 {
		t.Fatalf("GetSignals(since=1): got %d, want 1", len(sigs2))
	}
}

func TestNetworkSignal_NoBusReturns404(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	mux := NewMuxDispatcher(ctx)
	addr, stopServer := startTestServer(t, mux) // no WithSignalBus
	defer stopServer()

	client := NewNetworkClient("http://" + addr)

	err := client.EmitSignal(ctx, "test", "agent", "", "", nil)
	if err == nil {
		t.Fatal("expected error when signal bus not configured")
	}

	_, err = client.GetSignals(ctx, 0)
	if err == nil {
		t.Fatal("expected error when signal bus not configured")
	}
}

func TestNetworkSignal_EmptyEventRejected(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	mux := NewMuxDispatcher(ctx)
	bus := signal.NewMemBus()
	addr, stopServer := startTestServer(t, mux, WithSignalBus(bus))
	defer stopServer()

	client := NewNetworkClient("http://" + addr)

	err := client.EmitSignal(ctx, "", "agent", "", "", nil)
	if err == nil {
		t.Fatal("expected error for empty event")
	}
}
