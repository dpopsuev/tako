package toolkit

import (
	"fmt"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
)

// HITLResult is returned after a HITL walk step completes. It indicates
// whether the walk was interrupted (awaiting human input) or completed.
type HITLResult struct {
	PromptPath  string
	CurrentStep string
	IsDone      bool
	Explanation string
}

// LoadCheckpointState loads a WalkerState from a JSON checkpoint directory.
// Returns nil, nil if no checkpoint exists for the given walker ID.
func LoadCheckpointState(checkpointDir, walkerID string) (*circuit.WalkerState, error) {
	cp, err := engine.NewJSONCheckpointer(checkpointDir)
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

	if state.Status == "interrupted" {
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
