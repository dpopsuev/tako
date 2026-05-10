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
	Order     []string              `json:"order"`
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
		Status:    StepRunning,
		Steps:     make(map[string]*StepState),
		StartedAt: time.Now(),
	}

	stepMap := make(map[string]PipeStep)
	for _, step := range pipe.Steps {
		state.Steps[step.ID] = &StepState{ID: step.ID, Status: StepPending}
		stepMap[step.ID] = step
		state.Order = append(state.Order, step.ID)
	}

	for _, step := range pipe.Steps {
		if len(step.DependsOn) == 0 {
			state.Steps[step.ID].Status = StepReady
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
		return nil, nil, fmt.Errorf("%w: %s", ErrRunNotFound, runID)
	}

	for _, id := range run.Order {
		ss := run.Steps[id]
		if ss.Status == StepReady {
			ss.Status = StepRunning
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
		return nil, fmt.Errorf("%w: %s", ErrRunNotFound, runID)
	}

	ss, ok := run.Steps[stepID]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrStepNotFound, stepID)
	}

	if stepErr != "" {
		ss.Status = StepFailed
		ss.Error = stepErr
		for id, step := range steps {
			for _, dep := range step.DependsOn {
				if dep == stepID {
					run.Steps[id].Status = StepSkipped
					run.Steps[id].Error = fmt.Sprintf("dependency %s failed", stepID)
				}
			}
		}
	} else {
		ss.Status = StepComplete
		ss.Output = output
		for id, step := range steps {
			if run.Steps[id].Status != StepPending {
				continue
			}
			allDone := true
			for _, dep := range step.DependsOn {
				if run.Steps[dep].Status != StepComplete {
					allDone = false
					break
				}
			}
			if allDone {
				run.Steps[id].Status = StepReady
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
		return nil, fmt.Errorf("%w: %s", ErrRunNotFound, runID)
	}
	return run, nil
}

func (e *PipeExecutor) updateRunState(run *RunState) {
	allDone := true
	anyFailed := false
	for _, ss := range run.Steps {
		switch ss.Status {
		case StepPending, StepReady, StepRunning:
			allDone = false
		case StepFailed:
			anyFailed = true
		}
	}

	if allDone {
		if anyFailed {
			run.Status = StepFailed
		} else {
			run.Status = StepComplete
		}
	}
}
