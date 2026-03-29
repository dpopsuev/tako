package engine

import (
	"fmt"

	"github.com/dpopsuev/origami/circuit"
)

// HITLResult is returned after a HITL walk step completes. It indicates
// whether the walk was interrupted (awaiting human input) or completed.
type HITLResult struct {
	PromptPath  string
	CurrentStep string
	IsDone      bool
	Explanation string
}

// CheckpointInspection contains structured metadata about a checkpointed walker
// for HITL inspection — current node, status, interrupt data, and history.
type CheckpointInspection struct {
	WalkerID      string               `json:"walker_id"`
	CurrentNode   string               `json:"current_node"`
	Status        string               `json:"status"`
	InterruptData map[string]any       `json:"interrupt_data,omitempty"`
	History       []circuit.StepRecord `json:"history"`
	LoopCounts    map[string]int       `json:"loop_counts,omitempty"`
}

// InspectCheckpoint loads a checkpoint and returns structured metadata
// for HITL inspection. Returns an error if the checkpoint does not exist.
func InspectCheckpoint(cp circuit.Checkpointer, walkerID string) (*CheckpointInspection, error) {
	state, err := cp.Load(walkerID)
	if err != nil {
		return nil, fmt.Errorf("load checkpoint %s: %w", walkerID, err)
	}
	if state == nil {
		return nil, fmt.Errorf("checkpoint %s: %w", walkerID, ErrWalkerNotFound)
	}

	var interruptData map[string]any
	if data, ok := state.Context["interrupt_data"].(map[string]any); ok {
		interruptData = data
	}

	return &CheckpointInspection{
		WalkerID:      state.ID,
		CurrentNode:   state.CurrentNode,
		Status:        state.Status,
		InterruptData: interruptData,
		History:       state.History,
		LoopCounts:    state.LoopCounts,
	}, nil
}

// LoadCheckpointState loads a WalkerState from a JSON checkpoint directory.
// Returns nil, nil if no checkpoint exists for the given walker ID.
func LoadCheckpointState(checkpointDir, walkerID string) (*circuit.WalkerState, error) {
	cp, err := NewJSONCheckpointer(checkpointDir)
	if err != nil {
		return nil, err
	}
	state, err := cp.Load(walkerID)
	if err != nil {
		return nil, nil
	}
	return state, nil
}

// BuildHITLResult interprets a walker's state after a walk to produce an
// HITLResult. If the walk was interrupted, it extracts the prompt path and
// step from the interrupt data. If the walk completed, it returns IsDone.
func BuildHITLResult(walker circuit.Walker, walkErr error) (*HITLResult, error) {
	state := walker.State()

	if state.Status == walkStatusInterrupted {
		step := state.CurrentNode
		promptPath := ""
		if data, ok := state.Context["interrupt_data"].(map[string]any); ok {
			promptPath, _ = data["prompt_path"].(string)
			if s, ok := data["step"].(string); ok {
				step = s
			}
		}
		return &HITLResult{
			PromptPath:  promptPath,
			CurrentStep: step,
			Explanation: fmt.Sprintf("generated prompt for %s", step),
		}, nil
	}

	if walkErr != nil {
		return nil, fmt.Errorf("walk: %w", walkErr)
	}

	return &HITLResult{
		IsDone:      true,
		CurrentStep: "DONE",
		Explanation: "circuit complete",
	}, nil
}

// RestoreWalkerState applies a previously checkpointed state onto a new
// walker. Returns the node to resume from ("" if no checkpoint).
func RestoreWalkerState(walker circuit.Walker, loaded *circuit.WalkerState) string {
	if loaded == nil {
		return ""
	}
	state := walker.State()
	for k, v := range loaded.LoopCounts {
		state.LoopCounts[k] = v
	}
	state.Status = loaded.Status
	state.CurrentNode = loaded.CurrentNode
	state.History = loaded.History
	state.Outputs = loaded.Outputs
	return loaded.CurrentNode
}
