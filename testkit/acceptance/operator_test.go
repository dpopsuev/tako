package acceptance

// Feature: Operator Convergence
//   As an agentic system builder
//   I want to run reconciliation loops until a goal is met
//   So that circuits can self-correct and converge to desired states

import (
	"context"
	"testing"

	"github.com/dpopsuev/origami/engine"
)

// stubConvergingOperator is a test operator that converges after a set number of iterations.
type stubConvergingOperator struct {
	targetIterations int
	currentIteration int
}

func (s *stubConvergingOperator) Observe(_ context.Context) (engine.SystemState, error) {
	s.currentIteration++
	return engine.SystemState{Iteration: s.currentIteration}, nil
}

func (s *stubConvergingOperator) Reconcile(_ context.Context, _ engine.Goal, _ engine.SystemState) (*engine.CircuitDef, error) {
	// Return a minimal passthrough circuit
	return &engine.CircuitDef{
		Circuit:     "convergence-test",
		HandlerType: "transformer",
		Start:       "step",
		Done:        "_done",
		Nodes: []engine.NodeDef{
			{Name: "step", Handler: "echo"},
		},
		Edges: []engine.EdgeDef{
			{ID: "step-done", From: "step", To: "_done", When: "true"},
		},
	}, nil
}

func (s *stubConvergingOperator) Evaluate(_ context.Context, _ engine.Goal, _ engine.WalkResult) (engine.Evaluation, error) {
	if s.currentIteration >= s.targetIterations {
		return engine.Evaluation{
			Met:      true,
			Progress: 1.0,
			Action:   engine.ActionDone,
			Reason:   "converged after iterations",
		}, nil
	}
	return engine.Evaluation{
		Met:      false,
		Progress: float64(s.currentIteration) / float64(s.targetIterations),
		Action:   engine.ActionContinue,
		Reason:   "still iterating",
	}, nil
}

func TestOperator_ConvergesAfterIterations(t *testing.T) {
	// Scenario: Operator converges after fixed number of iterations
	//   Given a stub operator that converges after 3 iterations
	//   When I run RunOperator
	//   Then it completes successfully with ActionDone
	//   And no error is returned

	op := &stubConvergingOperator{targetIterations: 3}
	goal := engine.Goal{Description: "test convergence"}

	err := engine.RunOperator(context.Background(), op, goal, standardRegistries())
	if err != nil {
		t.Fatalf("RunOperator() = %v, want nil (converged)", err)
	}

	// Verify it actually went through 3 iterations
	if op.currentIteration != 3 {
		t.Errorf("currentIteration = %d, want 3", op.currentIteration)
	}
}

func TestOperator_EscalatesOnMaxIterations(t *testing.T) {
	// Scenario: Operator escalates when max iterations is reached
	//   Given a stub operator that would converge after 10 iterations
	//   And MaxIterations is set to 2
	//   When I run RunOperator
	//   Then it returns ErrMaxIterations

	op := &stubConvergingOperator{targetIterations: 10}
	goal := engine.Goal{Description: "capped iterations"}

	err := engine.RunOperator(
		context.Background(),
		op,
		goal,
		standardRegistries(),
		engine.WithMaxIterations(2),
	)

	if err == nil {
		t.Fatal("RunOperator() = nil, want ErrMaxIterations")
	}

	// Check for max iterations error
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}

	// Verify it stopped at max iterations
	if op.currentIteration > 2 {
		t.Errorf("currentIteration = %d, should not exceed 2", op.currentIteration)
	}
}

// stubEscalatingOperator returns ActionEscalate to test escalation path.
type stubEscalatingOperator struct {
	iteration int
}

func (s *stubEscalatingOperator) Observe(_ context.Context) (engine.SystemState, error) {
	s.iteration++
	return engine.SystemState{Iteration: s.iteration}, nil
}

func (s *stubEscalatingOperator) Reconcile(_ context.Context, _ engine.Goal, _ engine.SystemState) (*engine.CircuitDef, error) {
	return &engine.CircuitDef{
		Circuit:     "escalate-test",
		HandlerType: "transformer",
		Start:       "step",
		Done:        "_done",
		Nodes: []engine.NodeDef{
			{Name: "step", Handler: "echo"},
		},
		Edges: []engine.EdgeDef{
			{ID: "step-done", From: "step", To: "_done", When: "true"},
		},
	}, nil
}

func (s *stubEscalatingOperator) Evaluate(_ context.Context, _ engine.Goal, _ engine.WalkResult) (engine.Evaluation, error) {
	// Escalate on second iteration
	if s.iteration >= 2 {
		return engine.Evaluation{
			Met:      false,
			Progress: 0.5,
			Action:   engine.ActionEscalate,
			Reason:   "stuck, cannot proceed",
		}, nil
	}
	return engine.Evaluation{
		Met:      false,
		Progress: 0.3,
		Action:   engine.ActionContinue,
	}, nil
}

func TestOperator_ReturnsEscalateAction(t *testing.T) {
	// Scenario: Operator returns ActionEscalate explicitly
	//   Given an operator that returns ActionEscalate
	//   When I run RunOperator
	//   Then it returns ErrEscalate

	op := &stubEscalatingOperator{}
	goal := engine.Goal{Description: "escalation test"}

	err := engine.RunOperator(context.Background(), op, goal, standardRegistries())
	if err == nil {
		t.Fatal("RunOperator() = nil, want ErrEscalate")
	}

	// Verify error contains escalation
	if err.Error() == "" {
		t.Error("expected non-empty error message for escalation")
	}
}
