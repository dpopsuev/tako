package toolkit

import (
	"errors"
	"testing"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
)

func TestLoadCheckpointState_NoCheckpoint(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	state, err := LoadCheckpointState(dir, "nonexistent-walker")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if state != nil {
		t.Errorf("expected nil state for missing checkpoint, got %+v", state)
	}
}

func TestLoadCheckpointState_WithCheckpoint(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	cp, err := engine.NewJSONCheckpointer(dir)
	if err != nil {
		t.Fatalf("create checkpointer: %v", err)
	}
	walker := circuit.NewProcessWalker("walker-1")
	walker.State().CurrentNode = "triage"
	walker.State().Status = "interrupted"
	if err := cp.Save(walker.State()); err != nil {
		t.Fatalf("save checkpoint: %v", err)
	}

	loaded, err := LoadCheckpointState(dir, "walker-1")
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected non-nil state")
	}
	if loaded.CurrentNode != "triage" {
		t.Errorf("CurrentNode = %q, want triage", loaded.CurrentNode)
	}
	if loaded.Status != "interrupted" {
		t.Errorf("Status = %q, want interrupted", loaded.Status)
	}
}

func TestBuildHITLResult_Completed(t *testing.T) {
	t.Parallel()
	walker := circuit.NewProcessWalker("w")
	walker.State().Status = "done"

	result, err := BuildHITLResult(walker, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !result.IsDone {
		t.Error("expected IsDone=true")
	}
	if result.CurrentStep != "DONE" {
		t.Errorf("CurrentStep = %q, want DONE", result.CurrentStep)
	}
}

func TestBuildHITLResult_WalkError(t *testing.T) {
	t.Parallel()
	walker := circuit.NewProcessWalker("w")
	walker.State().Status = "running"

	_, err := BuildHITLResult(walker, errors.New("timeout"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildHITLResult_Interrupted(t *testing.T) {
	t.Parallel()
	walker := circuit.NewProcessWalker("w")
	walker.State().Status = "interrupted"
	walker.State().CurrentNode = "resolve"
	walker.State().Context["interrupt_data"] = map[string]any{
		"prompt_path": "/tmp/prompt.md",
		"step":        "resolve",
	}

	result, err := BuildHITLResult(walker, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result.IsDone {
		t.Error("expected IsDone=false for interrupted walk")
	}
	if result.PromptPath != "/tmp/prompt.md" {
		t.Errorf("PromptPath = %q, want /tmp/prompt.md", result.PromptPath)
	}
	if result.CurrentStep != "resolve" {
		t.Errorf("CurrentStep = %q, want resolve", result.CurrentStep)
	}
}

func TestBuildHITLResult_InterruptedNoData(t *testing.T) {
	t.Parallel()
	walker := circuit.NewProcessWalker("w")
	walker.State().Status = "interrupted"
	walker.State().CurrentNode = "triage"

	result, err := BuildHITLResult(walker, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result.PromptPath != "" {
		t.Errorf("PromptPath = %q, want empty", result.PromptPath)
	}
	if result.CurrentStep != "triage" {
		t.Errorf("CurrentStep = %q, want triage (fallback to CurrentNode)", result.CurrentStep)
	}
}

func TestRestoreWalkerState_NilLoaded(t *testing.T) {
	t.Parallel()
	walker := circuit.NewProcessWalker("w")
	resumeNode := RestoreWalkerState(walker, nil)
	if resumeNode != "" {
		t.Errorf("expected empty resume node for nil loaded, got %q", resumeNode)
	}
}

func TestRestoreWalkerState_WithCheckpoint(t *testing.T) {
	t.Parallel()
	walker := circuit.NewProcessWalker("w")

	loaded := &circuit.WalkerState{
		ID:          "w",
		Status:      "interrupted",
		CurrentNode: "correlate",
		LoopCounts:  map[string]int{"investigate": 2},
		History:     []circuit.StepRecord{{Node: "recall"}},
	}

	resumeNode := RestoreWalkerState(walker, loaded)
	if resumeNode != "correlate" {
		t.Errorf("resumeNode = %q, want correlate", resumeNode)
	}
	if walker.State().Status != "interrupted" {
		t.Errorf("Status = %q, want interrupted", walker.State().Status)
	}
	if walker.State().LoopCounts["investigate"] != 2 {
		t.Errorf("LoopCounts[investigate] = %d, want 2", walker.State().LoopCounts["investigate"])
	}
	if len(walker.State().History) != 1 {
		t.Errorf("History len = %d, want 1", len(walker.State().History))
	}
}
