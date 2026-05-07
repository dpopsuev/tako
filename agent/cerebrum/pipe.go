package cerebrum

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type Pipe struct {
	Name        string     `yaml:"name" json:"name"`
	Description string     `yaml:"description,omitempty" json:"description,omitempty"`
	Embedding   []float64  `yaml:"-" json:"-"`
	Steps       []PipeStep `yaml:"steps" json:"steps"`
	Replays     int        `yaml:"-"`
	Usage       int        `yaml:"-"`
	LastPlayed  time.Time  `yaml:"-"`
}

func (p *Pipe) Score() float64 {
	return float64(p.Replays+1) / float64(p.Usage+2)
}

type PipeStep struct {
	ID         string         `yaml:"id" json:"id"`
	Call       string         `yaml:"call,omitempty" json:"call,omitempty"`
	Args       map[string]any `yaml:"args,omitempty" json:"args,omitempty"`
	DependsOn  []string       `yaml:"depends_on,omitempty" json:"depends_on,omitempty"`
	Expected   [32]byte       `yaml:"-" json:"-"`
	Confidence float64        `yaml:"-" json:"-"`
}

type RunState struct {
	ID        string                `json:"id"`
	Pipe      string                `json:"pipe"`
	Status    string                `json:"status"`
	Steps     map[string]*StepState `json:"steps"`
	StartedAt time.Time             `json:"started_at"`
}

type StepState struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Output any    `json:"output,omitempty"`
	Error  string `json:"error,omitempty"`
}

type PipeConfig struct {
	Pipes map[string]Pipe `yaml:"pipes"`
}

func HashResult(data []byte) [32]byte {
	return sha256.Sum256(data)
}

func argsToJSON(args map[string]any) json.RawMessage {
	if args == nil {
		return json.RawMessage(`{}`)
	}
	data, err := json.Marshal(args)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return data
}

// PipeExecutor manages active pipe runs.
// Absorbed from github.com/dpopsuev/tubus/executor.
type PipeExecutor struct {
	mu   sync.Mutex
	runs map[string]*RunState
	seq  int
}

func NewPipeExecutor() *PipeExecutor {
	return &PipeExecutor{runs: make(map[string]*RunState)}
}

type pipeRun struct {
	state *RunState
	steps map[string]PipeStep
}

func (e *PipeExecutor) StartWithPipe(pipe Pipe) (string, *pipeRun) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.seq++
	runID := fmt.Sprintf("run-%d", e.seq)

	state := &RunState{
		ID:        runID,
		Pipe:      pipe.Name,
		Status:    "running",
		Steps:     make(map[string]*StepState),
		StartedAt: time.Now(),
	}

	stepMap := make(map[string]PipeStep)
	for _, step := range pipe.Steps {
		state.Steps[step.ID] = &StepState{ID: step.ID, Status: "pending"}
		stepMap[step.ID] = step
	}

	for _, step := range pipe.Steps {
		if len(step.DependsOn) == 0 {
			state.Steps[step.ID].Status = "ready"
		}
	}

	pr := &pipeRun{state: state, steps: stepMap}
	e.runs[runID] = state

	return runID, pr
}

func (e *PipeExecutor) NextStepFromPipe(runID string, steps map[string]PipeStep) (*PipeStep, *RunState, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	run, ok := e.runs[runID]
	if !ok {
		return nil, nil, fmt.Errorf("run %q not found", runID)
	}

	for id, ss := range run.Steps {
		if ss.Status == "ready" {
			ss.Status = "running"
			step := steps[id]
			return &step, run, nil
		}
	}

	return nil, run, nil
}

func (e *PipeExecutor) SubmitAndUnlock(runID, stepID string, output any, stepErr string, steps map[string]PipeStep) (*RunState, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	run, ok := e.runs[runID]
	if !ok {
		return nil, fmt.Errorf("run %q not found", runID)
	}

	ss, ok := run.Steps[stepID]
	if !ok {
		return nil, fmt.Errorf("step %q not found", stepID)
	}

	if stepErr != "" {
		ss.Status = "failed"
		ss.Error = stepErr
		for id, step := range steps {
			for _, dep := range step.DependsOn {
				if dep == stepID {
					run.Steps[id].Status = "skipped"
					run.Steps[id].Error = fmt.Sprintf("dependency %s failed", stepID)
				}
			}
		}
	} else {
		ss.Status = "complete"
		ss.Output = output
		for id, step := range steps {
			if run.Steps[id].Status != "pending" {
				continue
			}
			allDone := true
			for _, dep := range step.DependsOn {
				if run.Steps[dep].Status != "complete" {
					allDone = false
					break
				}
			}
			if allDone {
				run.Steps[id].Status = "ready"
			}
		}
	}

	e.updateRunState(run)
	return run, nil
}

func (e *PipeExecutor) Report(runID string) (*RunState, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	run, ok := e.runs[runID]
	if !ok {
		return nil, fmt.Errorf("run %q not found", runID)
	}
	return run, nil
}

func (e *PipeExecutor) updateRunState(run *RunState) {
	allDone := true
	anyFailed := false
	for _, ss := range run.Steps {
		switch ss.Status {
		case "pending", "ready", "running":
			allDone = false
		case "failed":
			anyFailed = true
		}
	}

	if allDone {
		if anyFailed {
			run.Status = "failed"
		} else {
			run.Status = "complete"
		}
	}
}
