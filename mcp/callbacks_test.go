package mcp_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dpopsuev/origami/mcp"
)

func TestOnStepDispatched_Called(t *testing.T) {
	var mu sync.Mutex
	var calls []struct{ CaseID, Step string }

	cfg := newTestConfig(1, 1, "")
	cfg.OnStepDispatched = func(caseID, step string) {
		mu.Lock()
		calls = append(calls, struct{ CaseID, Step string }{caseID, step})
		mu.Unlock()
	}

	srv := newTestServer(t, cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session := connectInMemory(ctx, t, srv)
	defer session.Close()

	startResult := callTool(ctx, t, session, "start_circuit", map[string]any{"parallel": 1})
	sessionID := startResult["session_id"].(string)

	step := callTool(ctx, t, session, "get_next_step", map[string]any{
		"session_id": sessionID,
		"timeout_ms": 2000,
	})

	if done, _ := step["done"].(bool); done {
		t.Fatal("expected a step, got done=true")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(calls) != 1 {
		t.Fatalf("expected 1 OnStepDispatched call, got %d", len(calls))
	}
	if calls[0].Step == "" {
		t.Error("step should not be empty")
	}
	t.Logf("OnStepDispatched called with caseID=%q step=%q", calls[0].CaseID, calls[0].Step)
}

func TestOnStepCompleted_Called(t *testing.T) {
	var mu sync.Mutex
	var calls []struct {
		CaseID     string
		Step       string
		DispatchID int64
	}

	cfg := newTestConfig(1, 1, "")
	cfg.OnStepCompleted = func(caseID, step string, dispatchID int64) {
		mu.Lock()
		calls = append(calls, struct {
			CaseID     string
			Step       string
			DispatchID int64
		}{caseID, step, dispatchID})
		mu.Unlock()
	}

	srv := newTestServer(t, cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session := connectInMemory(ctx, t, srv)
	defer session.Close()

	startResult := callTool(ctx, t, session, "start_circuit", map[string]any{"parallel": 1})
	sessionID := startResult["session_id"].(string)

	step := callTool(ctx, t, session, "get_next_step", map[string]any{
		"session_id": sessionID,
		"timeout_ms": 2000,
	})
	if done, _ := step["done"].(bool); done {
		t.Fatal("expected a step, got done=true")
	}

	stepName, _ := step["step"].(string)
	dispatchID, _ := step["dispatch_id"].(float64)

	callTool(ctx, t, session, "submit_step", map[string]any{
		"session_id":  sessionID,
		"dispatch_id": int64(dispatchID),
		"step":        stepName,
		"fields":      testFieldsForStep(stepName),
	})

	mu.Lock()
	defer mu.Unlock()
	if len(calls) != 1 {
		t.Fatalf("expected 1 OnStepCompleted call, got %d", len(calls))
	}
	if calls[0].Step != stepName {
		t.Errorf("step = %q, want %q", calls[0].Step, stepName)
	}
	if calls[0].DispatchID != int64(dispatchID) {
		t.Errorf("dispatchID = %d, want %d", calls[0].DispatchID, int64(dispatchID))
	}
	t.Logf("OnStepCompleted called with step=%q dispatchID=%d", calls[0].Step, calls[0].DispatchID)
}

func TestCallbacks_NilSafe(t *testing.T) {
	cfg := newTestConfig(1, 1, "")
	// Callbacks intentionally nil.

	srv := newTestServer(t, cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session := connectInMemory(ctx, t, srv)
	defer session.Close()

	startResult := callTool(ctx, t, session, "start_circuit", map[string]any{"parallel": 1})
	sessionID := startResult["session_id"].(string)

	step := callTool(ctx, t, session, "get_next_step", map[string]any{
		"session_id": sessionID,
		"timeout_ms": 2000,
	})
	if done, _ := step["done"].(bool); done {
		t.Fatal("expected a step, got done=true")
	}

	stepName, _ := step["step"].(string)
	dispatchID, _ := step["dispatch_id"].(float64)

	callTool(ctx, t, session, "submit_step", map[string]any{
		"session_id":  sessionID,
		"dispatch_id": int64(dispatchID),
		"step":        stepName,
		"fields":      testFieldsForStep(stepName),
	})

	t.Log("nil callbacks: no panic during dispatch and submit")
}

func TestCallbacks_ConcurrentDispatchSubmit(t *testing.T) {
	var dispatched atomic.Int64
	var completed atomic.Int64

	cfg := newTestConfig(4, 2, "")
	cfg.OnStepDispatched = func(caseID, step string) {
		dispatched.Add(1)
	}
	cfg.OnStepCompleted = func(caseID, step string, dispatchID int64) {
		completed.Add(1)
	}

	srv := mcp.NewCircuitServer(cfg)
	defer srv.Shutdown()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	session := connectInMemory(ctx, t, srv)
	defer session.Close()

	startResult := callTool(ctx, t, session, "start_circuit", map[string]any{"parallel": 4})
	sessionID := startResult["session_id"].(string)

	const workers = 4
	var wg sync.WaitGroup
	errCh := make(chan error, workers)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(wid int) {
			defer wg.Done()
			for {
				res, err := callToolE(ctx, session, "get_next_step", map[string]any{
					"session_id": sessionID,
					"timeout_ms": 300,
				})
				if err != nil {
					errCh <- fmt.Errorf("w%d get_next_step: %w", wid, err)
					return
				}
				if done, _ := res["done"].(bool); done {
					return
				}
				if avail, _ := res["available"].(bool); !avail {
					continue
				}

				step, _ := res["step"].(string)
				did, _ := res["dispatch_id"].(float64)

				_, err = callToolE(ctx, session, "submit_step", map[string]any{
					"session_id":  sessionID,
					"dispatch_id": int64(did),
					"step":        step,
					"fields":      testFieldsForStepWithWorker(step, wid),
				})
				if err != nil {
					errCh <- fmt.Errorf("w%d submit: %w", wid, err)
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatalf("worker error: %v", err)
	}

	d := dispatched.Load()
	c := completed.Load()
	if d == 0 {
		t.Error("expected >0 dispatched callbacks")
	}
	if c == 0 {
		t.Error("expected >0 completed callbacks")
	}
	if d != c {
		t.Errorf("dispatched=%d != completed=%d", d, c)
	}
	t.Logf("concurrent callbacks: dispatched=%d completed=%d (no races)", d, c)
}
