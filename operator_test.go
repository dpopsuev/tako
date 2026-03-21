package framework

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestEvalAction_Constants(t *testing.T) {
	if ActionContinue != "continue" {
		t.Errorf("ActionContinue = %q", ActionContinue)
	}
	if ActionEscalate != "escalate" {
		t.Errorf("ActionEscalate = %q", ActionEscalate)
	}
	if ActionDone != "done" {
		t.Errorf("ActionDone = %q", ActionDone)
	}
}

func TestGoal_ZeroValue(t *testing.T) {
	var g Goal
	if g.Description != "" {
		t.Errorf("zero Description = %q", g.Description)
	}
	if g.Constraints != nil {
		t.Error("zero Constraints should be nil")
	}
	if g.Timeout != 0 {
		t.Errorf("zero Timeout = %v", g.Timeout)
	}
}

func TestGoal_Construction(t *testing.T) {
	g := Goal{
		Description: "find root cause",
		Constraints: map[string]any{"max_cost": 100},
		Timeout:     30 * time.Second,
	}
	if g.Description != "find root cause" {
		t.Errorf("Description = %q", g.Description)
	}
	if g.Constraints["max_cost"] != 100 {
		t.Errorf("Constraints[max_cost] = %v", g.Constraints["max_cost"])
	}
}

func TestSystemState_ZeroValue(t *testing.T) {
	var s SystemState
	if s.Iteration != 0 {
		t.Errorf("zero Iteration = %d", s.Iteration)
	}
	if s.Artifacts != nil {
		t.Error("zero Artifacts should be nil")
	}
}

func TestEvaluation_Met(t *testing.T) {
	e := Evaluation{Met: true, Progress: 1.0, Action: ActionDone}
	if !e.Met {
		t.Error("Met should be true")
	}
	if e.Progress != 1.0 {
		t.Errorf("Progress = %f", e.Progress)
	}
	if e.Action != ActionDone {
		t.Errorf("Action = %q", e.Action)
	}
}

func TestWalkResult_ZeroValue(t *testing.T) {
	var r WalkResult
	if r.Artifacts != nil {
		t.Error("zero Artifacts should be nil")
	}
	if r.Error != nil {
		t.Error("zero Error should be nil")
	}
}

// ---------------------------------------------------------------------------
// Mock operator for P2 tests
// ---------------------------------------------------------------------------

type mockOperator struct {
	observeCount   int
	reconcileCount int
	evaluateCount  int
	targetIter     int
	escalateAt     int
	reconcileErr   error
}

func (m *mockOperator) Observe(_ context.Context) (SystemState, error) {
	m.observeCount++
	return SystemState{Iteration: m.observeCount}, nil
}

func (m *mockOperator) Reconcile(_ context.Context, _ Goal, _ SystemState) (*CircuitDef, error) {
	m.reconcileCount++
	if m.reconcileErr != nil {
		return nil, m.reconcileErr
	}
	return testCircuitDef(), nil
}

func (m *mockOperator) Evaluate(_ context.Context, _ Goal, _ WalkResult) (Evaluation, error) {
	m.evaluateCount++
	if m.escalateAt > 0 && m.evaluateCount >= m.escalateAt {
		return Evaluation{Action: ActionEscalate, Reason: "stuck"}, nil
	}
	if m.evaluateCount >= m.targetIter {
		return Evaluation{Met: true, Progress: 1.0, Action: ActionDone}, nil
	}
	return Evaluation{Progress: float64(m.evaluateCount) / float64(m.targetIter), Action: ActionContinue}, nil
}

// testCircuitDef returns a minimal 2-node circuit: A -> _done.
func testCircuitDef() *CircuitDef {
	return &CircuitDef{
		Circuit:     "test",
		HandlerType: "transformer",
		Start:       "A",
		Done:        "_done",
		Nodes: []NodeDef{
			{Name: "A", Handler: "passthrough"},
		},
		Edges: []EdgeDef{
			{ID: "a-done", From: "A", To: "_done"},
		},
	}
}

func testRegistries() GraphRegistries {
	return GraphRegistries{
		Transformers: TransformerRegistry{"passthrough": &passthroughTransformer{}},
	}
}

// ---------------------------------------------------------------------------
// P2: RunOperator tests
// ---------------------------------------------------------------------------

func TestRunOperator_ConvergesAfter3(t *testing.T) {
	op := &mockOperator{targetIter: 3}
	err := RunOperator(context.Background(), op, Goal{Description: "test"}, testRegistries())
	if err != nil {
		t.Fatalf("RunOperator() = %v, want nil", err)
	}
	// Iteration 1: observe, evaluate (not met), reconcile, walk, then
	// Iteration 2: observe, evaluate (not met), reconcile, walk, then
	// Iteration 3: observe, evaluate (met) -> done.
	// So evaluateCount should be targetIter (3), and reconcileCount should be targetIter-1 (2).
	if op.evaluateCount != 3 {
		t.Errorf("evaluateCount = %d, want 3", op.evaluateCount)
	}
	if op.reconcileCount != 2 {
		t.Errorf("reconcileCount = %d, want 2", op.reconcileCount)
	}
}

func TestRunOperator_Escalation(t *testing.T) {
	op := &mockOperator{targetIter: 100, escalateAt: 2}
	err := RunOperator(context.Background(), op, Goal{Description: "test"}, testRegistries())
	if !errors.Is(err, ErrEscalate) {
		t.Fatalf("RunOperator() = %v, want ErrEscalate", err)
	}
}

func TestRunOperator_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	op := &mockOperator{targetIter: 100}
	err := RunOperator(ctx, op, Goal{Description: "test"}, testRegistries())
	if err == nil {
		t.Fatal("RunOperator() = nil, want context error")
	}
}

func TestRunOperator_MaxIterations(t *testing.T) {
	op := &mockOperator{targetIter: 100}
	err := RunOperator(context.Background(), op, Goal{Description: "test"}, testRegistries(), WithMaxIterations(2))
	if !errors.Is(err, ErrMaxIterations) {
		t.Fatalf("RunOperator() = %v, want ErrMaxIterations", err)
	}
}

func TestRunOperator_GoalTimeout(t *testing.T) {
	op := &mockOperator{targetIter: 100}
	goal := Goal{Description: "test", Timeout: 1 * time.Nanosecond}
	err := RunOperator(context.Background(), op, goal, testRegistries())
	if err == nil {
		t.Fatal("RunOperator() = nil, want timeout error")
	}
}

// recordingObserver captures operator events for testing.
type recordingObserver struct {
	observes    []SystemState
	evaluates   []Evaluation
	reconciles  []*CircuitDef
	walkResults []WalkResult
}

func (r *recordingObserver) OnObserve(s SystemState)      { r.observes = append(r.observes, s) }
func (r *recordingObserver) OnEvaluate(e Evaluation)       { r.evaluates = append(r.evaluates, e) }
func (r *recordingObserver) OnReconcile(d *CircuitDef)     { r.reconciles = append(r.reconciles, d) }
func (r *recordingObserver) OnWalkComplete(w WalkResult)   { r.walkResults = append(r.walkResults, w) }

func TestRunOperator_Observer(t *testing.T) {
	obs := &recordingObserver{}
	op := &mockOperator{targetIter: 2}
	err := RunOperator(context.Background(), op, Goal{Description: "test"}, testRegistries(),
		WithOperatorObserver(obs))
	if err != nil {
		t.Fatalf("RunOperator() = %v", err)
	}
	if len(obs.observes) != 2 {
		t.Errorf("observes = %d, want 2", len(obs.observes))
	}
	if len(obs.evaluates) != 2 {
		t.Errorf("evaluates = %d, want 2", len(obs.evaluates))
	}
	if len(obs.reconciles) != 1 {
		t.Errorf("reconciles = %d, want 1", len(obs.reconciles))
	}
	if len(obs.walkResults) != 1 {
		t.Errorf("walkResults = %d, want 1", len(obs.walkResults))
	}
}

func TestRunOperator_ReconcileError(t *testing.T) {
	op := &mockOperator{targetIter: 100, reconcileErr: fmt.Errorf("broken")}
	err := RunOperator(context.Background(), op, Goal{Description: "test"}, testRegistries())
	if err == nil {
		t.Fatal("RunOperator() = nil, want error")
	}
	if !errors.Is(err, op.reconcileErr) {
		t.Errorf("RunOperator() error chain does not contain reconcileErr")
	}
}

// ---------------------------------------------------------------------------
// P3: CircuitContainer tests
// ---------------------------------------------------------------------------

func TestContainerStatus_Constants(t *testing.T) {
	if StatusPending != "pending" {
		t.Errorf("StatusPending = %q", StatusPending)
	}
	if StatusRunning != "running" {
		t.Errorf("StatusRunning = %q", StatusRunning)
	}
	if StatusSucceeded != "succeeded" {
		t.Errorf("StatusSucceeded = %q", StatusSucceeded)
	}
	if StatusFailed != "failed" {
		t.Errorf("StatusFailed = %q", StatusFailed)
	}
	if StatusAborted != "aborted" {
		t.Errorf("StatusAborted = %q", StatusAborted)
	}
}

func TestInMemoryContainer_Lifecycle(t *testing.T) {
	def := testCircuitDef()
	c := NewInMemoryContainer("test-1", def, nil)

	if c.ID() != "test-1" {
		t.Errorf("ID() = %q, want %q", c.ID(), "test-1")
	}
	if c.Def() != def {
		t.Error("Def() should return the provided CircuitDef")
	}
	if c.Status() != StatusPending {
		t.Errorf("initial Status() = %q, want %q", c.Status(), StatusPending)
	}

	result, err := c.Walk(context.Background(), testRegistries())
	if err != nil {
		t.Fatalf("Walk() = %v", err)
	}
	if result == nil {
		t.Fatal("Walk() result is nil")
	}
	if c.Status() != StatusSucceeded {
		t.Errorf("post-walk Status() = %q, want %q", c.Status(), StatusSucceeded)
	}
	if result.Elapsed == 0 {
		t.Error("Walk() result.Elapsed should be > 0")
	}
}

func TestInMemoryContainer_Abort_Pending(t *testing.T) {
	c := NewInMemoryContainer("test-abort", testCircuitDef(), nil)
	if err := c.Abort("test"); err != nil {
		t.Fatalf("Abort() = %v", err)
	}
	if c.Status() != StatusAborted {
		t.Errorf("Status() = %q, want %q", c.Status(), StatusAborted)
	}
	_, err := c.Walk(context.Background(), testRegistries())
	if err == nil {
		t.Fatal("Walk() after abort should fail")
	}
}

func TestInMemoryContainer_Abort_Completed(t *testing.T) {
	c := NewInMemoryContainer("test-abort-done", testCircuitDef(), nil)
	_, _ = c.Walk(context.Background(), testRegistries())
	err := c.Abort("too late")
	if err == nil {
		t.Fatal("Abort() on succeeded container should fail")
	}
}

func TestInMemoryContainer_Artifacts(t *testing.T) {
	c := NewInMemoryContainer("test-arts", testCircuitDef(), nil)
	if a := c.Artifacts(); a != nil {
		t.Error("Artifacts() before walk should be nil")
	}
	_, err := c.Walk(context.Background(), testRegistries())
	if err != nil {
		t.Fatalf("Walk() = %v", err)
	}
	arts := c.Artifacts()
	if arts == nil {
		t.Fatal("Artifacts() after walk should not be nil")
	}
}

func TestInMemoryContainer_Interface(t *testing.T) {
	var _ CircuitContainer = (*InMemoryContainer)(nil)
}

// ---------------------------------------------------------------------------
// P6: StubOperator + integration tests
// ---------------------------------------------------------------------------

// stubOperator is a reference implementation that converges after a
// configurable number of iterations. It generates a trivial 2-node
// passthrough circuit on each Reconcile call.
type stubOperator struct {
	target     int
	escalateAt int
	iteration  int
}

func (s *stubOperator) Observe(_ context.Context) (SystemState, error) {
	s.iteration++
	return SystemState{Iteration: s.iteration}, nil
}

func (s *stubOperator) Reconcile(_ context.Context, _ Goal, _ SystemState) (*CircuitDef, error) {
	return testCircuitDef(), nil
}

func (s *stubOperator) Evaluate(_ context.Context, _ Goal, _ WalkResult) (Evaluation, error) {
	if s.escalateAt > 0 && s.iteration >= s.escalateAt {
		return Evaluation{Action: ActionEscalate, Reason: "cannot proceed"}, nil
	}
	if s.iteration >= s.target {
		return Evaluation{Met: true, Progress: 1.0, Action: ActionDone, Reason: "goal met"}, nil
	}
	return Evaluation{
		Progress: float64(s.iteration) / float64(s.target),
		Action:   ActionContinue,
	}, nil
}

func TestOperator_StubIntegration_FullLoop(t *testing.T) {
	obs := &recordingObserver{}
	op := &stubOperator{target: 3}
	err := RunOperator(context.Background(), op, Goal{Description: "integration"},
		testRegistries(), WithOperatorObserver(obs))
	if err != nil {
		t.Fatalf("RunOperator() = %v", err)
	}

	// 3 observe calls, 3 evaluate calls, 2 reconcile+walk calls (met on 3rd eval)
	if len(obs.observes) != 3 {
		t.Errorf("observes = %d, want 3", len(obs.observes))
	}
	if len(obs.evaluates) != 3 {
		t.Errorf("evaluates = %d, want 3", len(obs.evaluates))
	}
	if len(obs.reconciles) != 2 {
		t.Errorf("reconciles = %d, want 2", len(obs.reconciles))
	}
	if len(obs.walkResults) != 2 {
		t.Errorf("walkResults = %d, want 2", len(obs.walkResults))
	}

	// Verify progress increases
	for i := 1; i < len(obs.evaluates); i++ {
		if obs.evaluates[i].Progress < obs.evaluates[i-1].Progress {
			t.Errorf("progress decreased: %f -> %f at eval %d",
				obs.evaluates[i-1].Progress, obs.evaluates[i].Progress, i)
		}
	}

	// Final evaluation should be met
	last := obs.evaluates[len(obs.evaluates)-1]
	if !last.Met {
		t.Error("final evaluation should be Met")
	}
}

func TestOperator_StubIntegration_Escalation(t *testing.T) {
	op := &stubOperator{target: 100, escalateAt: 2}
	err := RunOperator(context.Background(), op, Goal{Description: "escalate"},
		testRegistries())
	if !errors.Is(err, ErrEscalate) {
		t.Fatalf("RunOperator() = %v, want ErrEscalate", err)
	}
}

func TestOperator_StubIntegration_MaxIterations(t *testing.T) {
	op := &stubOperator{target: 10}
	err := RunOperator(context.Background(), op, Goal{Description: "capped"},
		testRegistries(), WithMaxIterations(2))
	if !errors.Is(err, ErrMaxIterations) {
		t.Fatalf("RunOperator() = %v, want ErrMaxIterations", err)
	}
}
