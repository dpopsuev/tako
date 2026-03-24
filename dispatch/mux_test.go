package dispatch_test

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dpopsuev/bugle/signal"
	bd "github.com/dpopsuev/bugle/dispatch"
	"github.com/dpopsuev/origami/dispatch"
)

func TestMux_SingleRoundTrip(t *testing.T) {
	d := dispatch.NewMuxDispatcher(context.Background())
	ctx := context.Background()
	want := []byte(`{"defect_type":"pb001"}`)

	dc := bd.Context{
		CaseID:       "C1",
		Step:         "F0_RECALL",
		PromptPath:   "/tmp/prompt.md",
		ArtifactPath: "/tmp/artifact.json",
	}

	go func() {
		got, err := d.GetNextStep(ctx)
		if err != nil {
			t.Errorf("GetNextStep error: %v", err)
			return
		}
		if got.CaseID != dc.CaseID || got.Step != dc.Step {
			t.Errorf("GetNextStep got case=%s step=%s, want case=%s step=%s",
				got.CaseID, got.Step, dc.CaseID, dc.Step)
		}
		if got.DispatchID == 0 {
			t.Error("expected non-zero DispatchID")
		}
		if err := d.SubmitArtifact(ctx, got.DispatchID, want); err != nil {
			t.Errorf("SubmitArtifact error: %v", err)
		}
	}()

	got, err := d.Dispatch(context.Background(), dc)
	if err != nil {
		t.Fatalf("Dispatch error: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("Dispatch got %s, want %s", got, want)
	}
}

func TestMux_ConcurrentDispatch_CorrectRouting(t *testing.T) {
	d := dispatch.NewMuxDispatcher(context.Background())
	ctx := context.Background()

	type result struct {
		caseID string
		data   []byte
		err    error
	}

	results := make(chan result, 2)

	for _, cid := range []string{"C1", "C2"} {
		cid := cid
		go func() {
			data, err := d.Dispatch(context.Background(), bd.Context{
				CaseID: cid,
				Step:   "F0_RECALL",
			})
			results <- result{caseID: cid, data: data, err: err}
		}()
	}

	// Collect both steps, then submit in REVERSE order
	time.Sleep(50 * time.Millisecond)

	step1, err := d.GetNextStep(ctx)
	if err != nil {
		t.Fatalf("GetNextStep 1: %v", err)
	}
	step2, err := d.GetNextStep(ctx)
	if err != nil {
		t.Fatalf("GetNextStep 2: %v", err)
	}

	// Submit in reverse: step2 first, then step1
	if err := d.SubmitArtifact(ctx, step2.DispatchID, []byte(fmt.Sprintf(`{"case":"%s"}`, step2.CaseID))); err != nil {
		t.Fatalf("SubmitArtifact step2: %v", err)
	}
	if err := d.SubmitArtifact(ctx, step1.DispatchID, []byte(fmt.Sprintf(`{"case":"%s"}`, step1.CaseID))); err != nil {
		t.Fatalf("SubmitArtifact step1: %v", err)
	}

	for i := 0; i < 2; i++ {
		r := <-results
		if r.err != nil {
			t.Fatalf("Dispatch %s error: %v", r.caseID, r.err)
		}
		expected := fmt.Sprintf(`{"case":"%s"}`, r.caseID)
		if string(r.data) != expected {
			t.Errorf("case %s got %s, want %s — artifact routed to wrong dispatcher", r.caseID, r.data, expected)
		}
	}
}

func TestMux_HighParallelism(t *testing.T) {
	d := dispatch.NewMuxDispatcher(context.Background())
	ctx := context.Background()
	n := 10

	type result struct {
		index int
		data  []byte
		err   error
	}

	results := make(chan result, n)

	for i := 0; i < n; i++ {
		i := i
		go func() {
			data, err := d.Dispatch(context.Background(), bd.Context{
				CaseID: fmt.Sprintf("C%d", i),
				Step:   "F0_RECALL",
			})
			results <- result{index: i, data: data, err: err}
		}()
	}

	time.Sleep(50 * time.Millisecond)

	// Collect all steps
	steps := make([]bd.Context, n)
	for i := 0; i < n; i++ {
		s, err := d.GetNextStep(ctx)
		if err != nil {
			t.Fatalf("GetNextStep %d: %v", i, err)
		}
		steps[i] = s
	}

	// Shuffle and submit in random order
	rand.Shuffle(n, func(i, j int) { steps[i], steps[j] = steps[j], steps[i] })

	for _, s := range steps {
		payload := []byte(fmt.Sprintf(`{"case":"%s"}`, s.CaseID))
		if err := d.SubmitArtifact(ctx, s.DispatchID, payload); err != nil {
			t.Fatalf("SubmitArtifact dispatch_id=%d: %v", s.DispatchID, err)
		}
	}

	for i := 0; i < n; i++ {
		r := <-results
		if r.err != nil {
			t.Fatalf("Dispatch C%d error: %v", r.index, r.err)
		}
		expected := fmt.Sprintf(`{"case":"C%d"}`, r.index)
		if string(r.data) != expected {
			t.Errorf("C%d got %s, want %s — artifact misrouted", r.index, r.data, expected)
		}
	}
}

func TestMux_SubmitUnknownDispatchID(t *testing.T) {
	d := dispatch.NewMuxDispatcher(context.Background())
	err := d.SubmitArtifact(context.Background(), 9999, []byte("{}"))
	if err == nil {
		t.Fatal("expected error for unknown dispatch ID")
	}
	t.Logf("got expected error: %v", err)
}

func TestMux_DoubleSubmitSameID(t *testing.T) {
	d := dispatch.NewMuxDispatcher(context.Background())
	ctx := context.Background()

	go func() {
		d.Dispatch(context.Background(), bd.Context{CaseID: "C1", Step: "F0_RECALL"})
	}()

	time.Sleep(50 * time.Millisecond)
	step, err := d.GetNextStep(ctx)
	if err != nil {
		t.Fatalf("GetNextStep: %v", err)
	}

	if err := d.SubmitArtifact(ctx, step.DispatchID, []byte(`{"first":true}`)); err != nil {
		t.Fatalf("first SubmitArtifact: %v", err)
	}

	// Second submit for the same ID should fail
	err = d.SubmitArtifact(ctx, step.DispatchID, []byte(`{"second":true}`))
	if err == nil {
		t.Fatal("expected error for double submit")
	}
	t.Logf("got expected error: %v", err)
}

func TestMux_ContextCancel_OneOfMany(t *testing.T) {
	dispCtx, dispCancel := context.WithCancel(context.Background())
	defer dispCancel()
	d := dispatch.NewMuxDispatcher(dispCtx)
	ctx := context.Background()

	type result struct {
		caseID string
		data   []byte
		err    error
	}
	results := make(chan result, 3)

	// Start 3 dispatches
	for _, cid := range []string{"C1", "C2", "C3"} {
		cid := cid
		go func() {
			data, err := d.Dispatch(context.Background(), bd.Context{CaseID: cid, Step: "F0_RECALL"})
			results <- result{caseID: cid, data: data, err: err}
		}()
	}

	time.Sleep(50 * time.Millisecond)

	// Collect all 3 steps
	steps := make([]bd.Context, 3)
	for i := 0; i < 3; i++ {
		s, err := d.GetNextStep(ctx)
		if err != nil {
			t.Fatalf("GetNextStep %d: %v", i, err)
		}
		steps[i] = s
	}

	// Submit only 2 of 3 (skip the first one to simulate one dispatch never completing)
	for i := 1; i < 3; i++ {
		payload := []byte(fmt.Sprintf(`{"case":"%s"}`, steps[i].CaseID))
		if err := d.SubmitArtifact(ctx, steps[i].DispatchID, payload); err != nil {
			t.Fatalf("SubmitArtifact %d: %v", i, err)
		}
	}

	// 2 should complete successfully
	var successes int
	timeout := time.After(2 * time.Second)
	for i := 0; i < 2; i++ {
		select {
		case r := <-results:
			if r.err != nil {
				t.Errorf("dispatch %s error: %v", r.caseID, r.err)
			} else {
				successes++
			}
		case <-timeout:
			t.Fatal("timed out waiting for results")
		}
	}

	if successes != 2 {
		t.Errorf("expected 2 successes, got %d", successes)
	}

	// Cancel dispatcher context to unblock the orphaned dispatch
	dispCancel()
	select {
	case r := <-results:
		if r.err == nil {
			t.Error("expected error for unfulfilled dispatch after cancel")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("orphaned dispatch did not unblock after context cancel")
	}
}

func TestMux_DispatcherContextCancel(t *testing.T) {
	dispCtx, dispCancel := context.WithCancel(context.Background())
	d := dispatch.NewMuxDispatcher(dispCtx)

	errCh := make(chan error, 3)
	for i := 0; i < 3; i++ {
		i := i
		go func() {
			_, err := d.Dispatch(context.Background(), bd.Context{
				CaseID: fmt.Sprintf("C%d", i),
				Step:   "F0_RECALL",
			})
			errCh <- err
		}()
	}

	time.Sleep(50 * time.Millisecond)
	dispCancel()

	for i := 0; i < 3; i++ {
		select {
		case err := <-errCh:
			if err == nil {
				t.Error("expected error from cancelled dispatcher context")
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Dispatch did not unblock after dispatcher context cancel")
		}
	}
}

func TestMux_Abort(t *testing.T) {
	d := dispatch.NewMuxDispatcher(context.Background())

	var wg sync.WaitGroup
	errCh := make(chan error, 3)

	for i := 0; i < 3; i++ {
		wg.Add(1)
		i := i
		go func() {
			defer wg.Done()
			_, err := d.Dispatch(context.Background(), bd.Context{
				CaseID: fmt.Sprintf("C%d", i),
				Step:   "F0_RECALL",
			})
			errCh <- err
		}()
	}

	time.Sleep(50 * time.Millisecond)
	d.Abort(fmt.Errorf("test abort"))

	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err == nil {
			t.Error("expected error from Abort")
		}
	}
}

func TestMux_GetNextStep_BlocksUntilDispatch(t *testing.T) {
	d := dispatch.NewMuxDispatcher(context.Background())

	got := make(chan bd.Context, 1)
	go func() {
		dc, err := d.GetNextStep(context.Background())
		if err != nil {
			t.Errorf("GetNextStep error: %v", err)
			return
		}
		got <- dc
	}()

	// Should not have a result yet
	select {
	case <-got:
		t.Fatal("GetNextStep returned before any Dispatch call")
	case <-time.After(100 * time.Millisecond):
	}

	// Now dispatch
	go func() {
		d.Dispatch(context.Background(), bd.Context{CaseID: "C1", Step: "F0_RECALL"})
	}()

	select {
	case dc := <-got:
		if dc.CaseID != "C1" {
			t.Errorf("got case %s, want C1", dc.CaseID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("GetNextStep did not unblock after Dispatch")
	}
}

func TestMux_GetNextStep_Cancelled(t *testing.T) {
	d := dispatch.NewMuxDispatcher(context.Background())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := d.GetNextStep(ctx)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestMux_GetNextStepWithHints_ExactCaseMatch(t *testing.T) {
	d := dispatch.NewMuxDispatcher(context.Background())
	ctx := context.Background()

	go func() {
		d.Dispatch(context.Background(), bd.Context{CaseID: "C1", Step: "F0", Provider: "zone-a"})
	}()
	go func() {
		time.Sleep(10 * time.Millisecond)
		d.Dispatch(context.Background(), bd.Context{CaseID: "C2", Step: "F0", Provider: "zone-b"})
	}()
	go func() {
		time.Sleep(20 * time.Millisecond)
		d.Dispatch(context.Background(), bd.Context{CaseID: "C3", Step: "F0", Provider: "zone-a"})
	}()

	time.Sleep(50 * time.Millisecond)

	// Worker wants C3 specifically
	dc, err := d.GetNextStepWithHints(ctx, bd.PullHints{PreferredCaseID: "C3"})
	if err != nil {
		t.Fatalf("GetNextStepWithHints error: %v", err)
	}
	if dc.CaseID != "C3" {
		t.Errorf("expected C3, got %s", dc.CaseID)
	}

	// Clean up remaining dispatches
	for i := 0; i < 2; i++ {
		dc, _ := d.GetNextStep(ctx)
		d.SubmitArtifact(ctx, dc.DispatchID, []byte("{}"))
	}
	d.SubmitArtifact(ctx, dc.DispatchID, []byte("{}"))
}

func TestMux_GetNextStepWithHints_ZoneMatch(t *testing.T) {
	d := dispatch.NewMuxDispatcher(context.Background())
	ctx := context.Background()

	go func() {
		d.Dispatch(context.Background(), bd.Context{CaseID: "C1", Step: "F0", Provider: "zone-a"})
	}()
	go func() {
		time.Sleep(10 * time.Millisecond)
		d.Dispatch(context.Background(), bd.Context{CaseID: "C2", Step: "F0", Provider: "zone-b"})
	}()

	time.Sleep(50 * time.Millisecond)

	// Worker prefers zone-b
	dc, err := d.GetNextStepWithHints(ctx, bd.PullHints{PreferredZone: "zone-b"})
	if err != nil {
		t.Fatalf("GetNextStepWithHints error: %v", err)
	}
	if dc.Provider != "zone-b" {
		t.Errorf("expected zone-b, got provider=%s case=%s", dc.Provider, dc.CaseID)
	}

	// Clean up
	dc2, _ := d.GetNextStep(ctx)
	d.SubmitArtifact(ctx, dc.DispatchID, []byte("{}"))
	d.SubmitArtifact(ctx, dc2.DispatchID, []byte("{}"))
}

func TestMux_GetNextStepWithHints_Stickiness0_FallbackImmediate(t *testing.T) {
	d := dispatch.NewMuxDispatcher(context.Background())
	ctx := context.Background()

	go func() {
		d.Dispatch(context.Background(), bd.Context{CaseID: "C1", Step: "F0", Provider: "zone-a"})
	}()

	time.Sleep(50 * time.Millisecond)

	// Worker wants zone-b but stickiness=0 means take anything
	dc, err := d.GetNextStepWithHints(ctx, bd.PullHints{
		PreferredZone: "zone-b",
		Stickiness:    0,
	})
	if err != nil {
		t.Fatalf("GetNextStepWithHints error: %v", err)
	}
	if dc.CaseID != "C1" {
		t.Errorf("expected C1 as fallback, got %s", dc.CaseID)
	}

	d.SubmitArtifact(ctx, dc.DispatchID, []byte("{}"))
}

func TestMux_GetNextStepWithHints_Stickiness3_WaitsForMatch(t *testing.T) {
	d := dispatch.NewMuxDispatcher(context.Background())

	go func() {
		d.Dispatch(context.Background(), bd.Context{CaseID: "C1", Step: "F0", Provider: "zone-a"})
	}()

	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Worker wants zone-b with stickiness=3 (exclusive) — should time out since only zone-a is available
	_, err := d.GetNextStepWithHints(ctx, bd.PullHints{
		PreferredZone: "zone-b",
		Stickiness:    3,
	})
	if err == nil {
		t.Fatal("expected timeout error for stickiness=3 with no matching zone")
	}

	// Clean up: the non-matching item was put back in queue; drain it
	d.Abort(fmt.Errorf("cleanup"))
}

func TestMux_GetNextStepWithHints_Stickiness3_MatchArrives(t *testing.T) {
	d := dispatch.NewMuxDispatcher(context.Background())
	ctx := context.Background()

	// First dispatch zone-a (non-matching)
	go func() {
		d.Dispatch(context.Background(), bd.Context{CaseID: "C1", Step: "F0", Provider: "zone-a"})
	}()

	// After 100ms, dispatch zone-b (matching)
	go func() {
		time.Sleep(100 * time.Millisecond)
		d.Dispatch(context.Background(), bd.Context{CaseID: "C2", Step: "F0", Provider: "zone-b"})
	}()

	// Worker wants zone-b with stickiness=3 — should skip C1 and wait for C2
	dc, err := d.GetNextStepWithHints(ctx, bd.PullHints{
		PreferredZone: "zone-b",
		Stickiness:    3,
	})
	if err != nil {
		t.Fatalf("GetNextStepWithHints error: %v", err)
	}
	if dc.CaseID != "C2" {
		t.Errorf("expected C2 (zone-b), got %s (provider=%s)", dc.CaseID, dc.Provider)
	}

	// Clean up C1 still in queue
	dc2, _ := d.GetNextStep(ctx)
	d.SubmitArtifact(ctx, dc.DispatchID, []byte("{}"))
	d.SubmitArtifact(ctx, dc2.DispatchID, []byte("{}"))
}

func TestMux_GetNextStepWithHints_BackwardCompat(t *testing.T) {
	d := dispatch.NewMuxDispatcher(context.Background())
	ctx := context.Background()

	go func() {
		d.Dispatch(context.Background(), bd.Context{CaseID: "C1", Step: "F0"})
	}()

	time.Sleep(50 * time.Millisecond)

	// GetNextStep (no hints) should work exactly as before
	dc, err := d.GetNextStep(ctx)
	if err != nil {
		t.Fatalf("GetNextStep error: %v", err)
	}
	if dc.CaseID != "C1" {
		t.Errorf("expected C1, got %s", dc.CaseID)
	}

	d.SubmitArtifact(ctx, dc.DispatchID, []byte("{}"))
}

func TestMux_GetNextStepWithHints_QueueDrain(t *testing.T) {
	d := dispatch.NewMuxDispatcher(context.Background())
	ctx := context.Background()

	// Dispatch 3 items
	for _, cid := range []string{"C1", "C2", "C3"} {
		cid := cid
		go func() {
			d.Dispatch(context.Background(), bd.Context{CaseID: cid, Step: "F0", Provider: "zone-a"})
		}()
	}
	time.Sleep(100 * time.Millisecond)

	// Worker with stickiness=3 wants zone-b — will drain all 3 into queue without matching
	timeoutCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	_, err := d.GetNextStepWithHints(timeoutCtx, bd.PullHints{
		PreferredZone: "zone-b",
		Stickiness:    3,
	})
	if err == nil {
		t.Fatal("expected timeout")
	}

	// Now a FIFO worker should get items from the queue
	dc, err := d.GetNextStep(ctx)
	if err != nil {
		t.Fatalf("GetNextStep after queue drain: %v", err)
	}
	// Should be the first enqueued item (C1)
	t.Logf("got %s from queue (expected one of C1/C2/C3)", dc.CaseID)
	d.SubmitArtifact(ctx, dc.DispatchID, []byte("{}"))

	// Clean up remaining
	for i := 0; i < 2; i++ {
		dc, _ := d.GetNextStep(ctx)
		d.SubmitArtifact(ctx, dc.DispatchID, []byte("{}"))
	}
}

func TestMux_WorkStealing_Stickiness1_StealsAfterOneMiss(t *testing.T) {
	bus := signal.NewMemBus()
	d := dispatch.NewMuxDispatcher(context.Background(), dispatch.WithMuxSignalBus(bus))
	ctx := context.Background()

	go func() {
		d.Dispatch(context.Background(), bd.Context{CaseID: "C1", Step: "F0", Provider: "zone-a"})
	}()

	time.Sleep(50 * time.Millisecond)

	// stickiness=1, ConsecutiveMisses=0 → should NOT steal (need >= 1 miss)
	timeoutCtx, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
	defer cancel()
	_, err := d.GetNextStepWithHints(timeoutCtx, bd.PullHints{
		PreferredZone:     "zone-b",
		Stickiness:        1,
		ConsecutiveMisses: 0,
	})
	if err == nil {
		t.Fatal("stickiness=1 with 0 misses should not steal")
	}

	// stickiness=1, ConsecutiveMisses=1 → should steal
	dc, err := d.GetNextStepWithHints(ctx, bd.PullHints{
		PreferredZone:     "zone-b",
		Stickiness:        1,
		ConsecutiveMisses: 1,
	})
	if err != nil {
		t.Fatalf("GetNextStepWithHints error: %v", err)
	}
	if dc.CaseID != "C1" {
		t.Errorf("expected C1 from steal, got %s", dc.CaseID)
	}

	d.SubmitArtifact(ctx, dc.DispatchID, []byte("{}"))

	// Verify zone_shift signal was emitted
	sigs := bus.Since(0)
	var found bool
	for _, s := range sigs {
		if s.Event == "zone_shift" {
			found = true
			if s.Meta["from_zone"] != "zone-b" || s.Meta["to_zone"] != "zone-a" {
				t.Errorf("zone_shift meta: from=%s to=%s, want from=zone-b to=zone-a",
					s.Meta["from_zone"], s.Meta["to_zone"])
			}
		}
	}
	if !found {
		t.Error("zone_shift signal not emitted")
	}
}

func TestMux_WorkStealing_Stickiness2_StealsAfterThreeMisses(t *testing.T) {
	d := dispatch.NewMuxDispatcher(context.Background())
	ctx := context.Background()

	go func() {
		d.Dispatch(context.Background(), bd.Context{CaseID: "C1", Step: "F0", Provider: "zone-a"})
	}()

	time.Sleep(50 * time.Millisecond)

	// stickiness=2, ConsecutiveMisses=2 → should NOT steal (need >= 3)
	timeoutCtx, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
	defer cancel()
	_, err := d.GetNextStepWithHints(timeoutCtx, bd.PullHints{
		PreferredZone:     "zone-b",
		Stickiness:        2,
		ConsecutiveMisses: 2,
	})
	if err == nil {
		t.Fatal("stickiness=2 with 2 misses should not steal")
	}

	// stickiness=2, ConsecutiveMisses=3 → should steal
	dc, err := d.GetNextStepWithHints(ctx, bd.PullHints{
		PreferredZone:     "zone-b",
		Stickiness:        2,
		ConsecutiveMisses: 3,
	})
	if err != nil {
		t.Fatalf("GetNextStepWithHints error: %v", err)
	}
	if dc.CaseID != "C1" {
		t.Errorf("expected C1 from steal, got %s", dc.CaseID)
	}

	d.SubmitArtifact(ctx, dc.DispatchID, []byte("{}"))
}

func TestMux_WorkStealing_Stickiness3_NeverSteals(t *testing.T) {
	d := dispatch.NewMuxDispatcher(context.Background())

	go func() {
		d.Dispatch(context.Background(), bd.Context{CaseID: "C1", Step: "F0", Provider: "zone-a"})
	}()

	time.Sleep(50 * time.Millisecond)

	// stickiness=3 with high misses → should never steal
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, err := d.GetNextStepWithHints(timeoutCtx, bd.PullHints{
		PreferredZone:     "zone-b",
		Stickiness:        3,
		ConsecutiveMisses: 100,
	})
	if err == nil {
		t.Fatal("stickiness=3 should never steal regardless of miss count")
	}

	d.Abort(fmt.Errorf("cleanup"))
}

func TestMux_WorkStealing_ZoneShiftSignal_NotEmittedOnMatch(t *testing.T) {
	bus := signal.NewMemBus()
	d := dispatch.NewMuxDispatcher(context.Background(), dispatch.WithMuxSignalBus(bus))
	ctx := context.Background()

	go func() {
		d.Dispatch(context.Background(), bd.Context{CaseID: "C1", Step: "F0", Provider: "zone-a"})
	}()

	time.Sleep(50 * time.Millisecond)

	// Worker matches zone-a → no steal, no zone_shift
	dc, err := d.GetNextStepWithHints(ctx, bd.PullHints{PreferredZone: "zone-a"})
	if err != nil {
		t.Fatalf("GetNextStepWithHints error: %v", err)
	}
	if dc.CaseID != "C1" {
		t.Errorf("expected C1, got %s", dc.CaseID)
	}

	d.SubmitArtifact(ctx, dc.DispatchID, []byte("{}"))

	sigs := bus.Since(0)
	for _, s := range sigs {
		if s.Event == "zone_shift" {
			t.Error("zone_shift should not be emitted on exact match")
		}
	}
}

func TestMux_PerDispatchTimeout(t *testing.T) {
	d := dispatch.NewMuxDispatcher(context.Background())
	ctx := context.Background()

	// Drain prompts but never submit artifacts
	go func() {
		for {
			if _, err := d.GetNextStep(ctx); err != nil {
				return
			}
		}
	}()

	start := time.Now()
	_, err := d.Dispatch(context.Background(), bd.Context{
		CaseID:  "C1",
		Step:    "F0_RECALL",
		Timeout: 50 * time.Millisecond,
	})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "dispatch timeout") {
		t.Errorf("expected 'dispatch timeout' in error, got: %v", err)
	}
	if elapsed > 300*time.Millisecond {
		t.Errorf("dispatch took %v, expected ~50ms", elapsed)
	}
	t.Logf("dispatch timed out in %v (timeout=50ms)", elapsed)
}

func TestMux_PerDispatchTimeout_ZeroIsNoLimit(t *testing.T) {
	dispCtx, dispCancel := context.WithCancel(context.Background())
	d := dispatch.NewMuxDispatcher(dispCtx)

	// Drain prompts but never submit
	go func() {
		for {
			if _, err := d.GetNextStep(dispCtx); err != nil {
				return
			}
		}
	}()

	errCh := make(chan error, 1)
	go func() {
		_, err := d.Dispatch(context.Background(), bd.Context{
			CaseID:  "C1",
			Step:    "F0_RECALL",
			Timeout: 0, // zero = no per-dispatch timeout
		})
		errCh <- err
	}()

	time.Sleep(50 * time.Millisecond)
	dispCancel()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected error after context cancel")
		}
		if strings.Contains(err.Error(), "dispatch timeout") {
			t.Errorf("Timeout=0 should not trigger dispatch timeout, got: %v", err)
		}
		if !strings.Contains(err.Error(), "cancelled") {
			t.Errorf("expected 'cancelled' in error, got: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("dispatch did not unblock after context cancel")
	}
}

func TestMux_PerDispatchTimeout_SubmitBeforeDeadline(t *testing.T) {
	d := dispatch.NewMuxDispatcher(context.Background())
	ctx := context.Background()
	want := []byte(`{"ok":true}`)

	go func() {
		time.Sleep(10 * time.Millisecond)
		dc, err := d.GetNextStep(ctx)
		if err != nil {
			t.Errorf("GetNextStep: %v", err)
			return
		}
		if err := d.SubmitArtifact(ctx, dc.DispatchID, want); err != nil {
			t.Errorf("SubmitArtifact: %v", err)
		}
	}()

	got, err := d.Dispatch(context.Background(), bd.Context{
		CaseID:  "C1",
		Step:    "F0_RECALL",
		Timeout: 500 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestMux_MultipleSequentialRoundTrips(t *testing.T) {
	d := dispatch.NewMuxDispatcher(context.Background())
	ctx := context.Background()

	go func() {
		for i := 0; i < 3; i++ {
			dc, err := d.GetNextStep(ctx)
			if err != nil {
				t.Errorf("round %d GetNextStep error: %v", i, err)
				return
			}
			artifact := []byte(fmt.Sprintf(`{"round":"%s"}`, dc.CaseID))
			if err := d.SubmitArtifact(ctx, dc.DispatchID, artifact); err != nil {
				t.Errorf("round %d SubmitArtifact error: %v", i, err)
				return
			}
		}
	}()

	for i := 0; i < 3; i++ {
		caseID := string(rune('0' + i))
		got, err := d.Dispatch(context.Background(), bd.Context{CaseID: caseID, Step: "F0_RECALL"})
		if err != nil {
			t.Fatalf("round %d Dispatch error: %v", i, err)
		}
		want := fmt.Sprintf(`{"round":"%s"}`, caseID)
		if string(got) != want {
			t.Errorf("round %d got %s, want %s", i, got, want)
		}
	}
}

func TestMux_LateConsumer_BlocksUntilReady(t *testing.T) {
	d := dispatch.NewMuxDispatcher(context.Background())
	want := []byte(`{"result":"late_ok"}`)

	type dispatchResult struct {
		data []byte
		err  error
	}
	resultCh := make(chan dispatchResult, 1)

	go func() {
		data, err := d.Dispatch(context.Background(), bd.Context{
			CaseID: "C1", Step: "F0_RECALL",
		})
		resultCh <- dispatchResult{data, err}
	}()

	// Simulate worker startup latency — consumer arrives 500ms after producer.
	time.Sleep(500 * time.Millisecond)

	dc, err := d.GetNextStep(context.Background())
	if err != nil {
		t.Fatalf("GetNextStep after 500ms delay: %v", err)
	}
	if err := d.SubmitArtifact(context.Background(), dc.DispatchID, want); err != nil {
		t.Fatalf("SubmitArtifact: %v", err)
	}

	select {
	case r := <-resultCh:
		if r.err != nil {
			t.Fatalf("Dispatch failed with late consumer: %v", r.err)
		}
		if string(r.data) != string(want) {
			t.Errorf("got %s, want %s", r.data, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Dispatch did not complete within 5s")
	}
}
