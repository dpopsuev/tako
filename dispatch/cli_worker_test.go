package dispatch

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dpopsuev/troupe/signal"
)

func TestCLIWorkerDispatcher_ProcessesSteps(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mux := NewMuxDispatcher(ctx)
	bus := signal.NewMemBus()

	d, err := NewCLIWorkerDispatcher(mux, "cat", 2,
		WithCLIWorkerBus(bus),
		WithCLIWorkerTimeout(5*time.Second),
	)
	if err != nil {
		t.Fatalf("NewCLIWorkerDispatcher: %v", err)
	}

	tmpDir := t.TempDir()
	steps := 4

	// Write prompt files
	for i := 0; i < steps; i++ {
		p := filepath.Join(tmpDir, fmt.Sprintf("prompt_%d.json", i))
		os.WriteFile(p, []byte(fmt.Sprintf(`{"case":"C%d"}`, i)), 0o644)
	}

	// Launch dispatches in background
	results := make([][]byte, steps)
	errs := make([]error, steps)
	var wg sync.WaitGroup
	for i := 0; i < steps; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			data, dispErr := mux.Dispatch(context.Background(), Context{
				CaseID:     fmt.Sprintf("C%d", i),
				Step:       "F0",
				PromptPath: filepath.Join(tmpDir, fmt.Sprintf("prompt_%d.json", i)),
			})
			results[i] = data
			errs[i] = dispErr
		}()
	}

	// Run workers (blocks until mux closes)
	workerDone := make(chan error, 1)
	go func() {
		workerDone <- d.Run(ctx)
	}()

	// Wait for all dispatches to complete
	wg.Wait()

	// Cancel context to stop workers
	cancel()
	<-workerDone

	for i := 0; i < steps; i++ {
		if errs[i] != nil {
			t.Errorf("dispatch %d: %v", i, errs[i])
			continue
		}
		expected := fmt.Sprintf(`{"case":"C%d"}`, i)
		if string(results[i]) != expected {
			t.Errorf("dispatch %d: got %q, want %q", i, results[i], expected)
		}
	}

	// Verify signals were emitted
	sigs := bus.Since(0)
	if len(sigs) == 0 {
		t.Fatal("no signals emitted")
	}

	workerStarted := 0
	stepDone := 0
	workerStopped := 0
	for _, s := range sigs {
		switch s.Event {
		case "worker_started":
			workerStarted++
		case "done":
			stepDone++
		case "worker_stopped":
			workerStopped++
		}
	}
	if workerStarted != 2 {
		t.Errorf("worker_started signals: got %d, want 2", workerStarted)
	}
	if stepDone != steps {
		t.Errorf("done signals: got %d, want %d", stepDone, steps)
	}
	if workerStopped != 2 {
		t.Errorf("worker_stopped signals: got %d, want 2", workerStopped)
	}
}

func TestCLIWorkerDispatcher_InvalidCommand(t *testing.T) {
	mux := NewMuxDispatcher(context.Background())
	_, err := NewCLIWorkerDispatcher(mux, "nonexistent-command-xyz", 1)
	if err == nil {
		t.Fatal("expected error for invalid command")
	}
}

func TestCLIWorkerDispatcher_SingleWorker(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	mux := NewMuxDispatcher(ctx)

	d, err := NewCLIWorkerDispatcher(mux, "cat", 1)
	if err != nil {
		t.Fatalf("NewCLIWorkerDispatcher: %v", err)
	}

	tmpDir := t.TempDir()
	promptPath := filepath.Join(tmpDir, "prompt.json")
	os.WriteFile(promptPath, []byte(`{"single":true}`), 0o644)

	done := make(chan struct{})
	var result []byte
	var dispErr error
	go func() {
		defer close(done)
		result, dispErr = mux.Dispatch(context.Background(), Context{
			CaseID:     "C1",
			Step:       "F0",
			PromptPath: promptPath,
		})
	}()

	workerDone := make(chan error, 1)
	go func() {
		workerDone <- d.Run(ctx)
	}()

	<-done
	cancel()
	<-workerDone

	if dispErr != nil {
		t.Fatalf("Dispatch: %v", dispErr)
	}
	if string(result) != `{"single":true}` {
		t.Errorf("result = %q, want {\"single\":true}", result)
	}
}
