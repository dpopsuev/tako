package reactivity

// StepResult is the outcome of executing one Catalyst step.
type StepResult struct {
	Step    *Catalyst
	Success bool
	State   map[string]any // actual state after execution
}

// ExecutionPlan is a topologically sorted sequence of step batches.
// Each batch contains steps that can execute in parallel.
// The next batch becomes ready only after the current batch completes.
type ExecutionPlan struct {
	Batches [][]*Catalyst
}

// Plan resolves execution order for a Catalyst's Steps using state dependencies.
// Steps with no dependencies on other steps go first. Steps whose Current
// overlaps another step's Desired are scheduled after that step.
func Plan(c *Catalyst) ExecutionPlan {
	if len(c.Steps) == 0 {
		return ExecutionPlan{}
	}

	remaining := make(map[*Catalyst]bool, len(c.Steps))
	for _, step := range c.Steps {
		remaining[step] = true
	}

	var batches [][]*Catalyst
	for len(remaining) > 0 {
		var batch []*Catalyst
		for step := range remaining {
			blocked := false
			for other := range remaining {
				if other == step {
					continue
				}
				if step.DependsOn(other) {
					blocked = true
					break
				}
			}
			if !blocked {
				batch = append(batch, step)
			}
		}
		if len(batch) == 0 {
			for step := range remaining {
				batch = append(batch, step)
			}
			for _, step := range batch {
				delete(remaining, step)
			}
			batches = append(batches, batch)
			break
		}
		for _, step := range batch {
			delete(remaining, step)
		}
		batches = append(batches, batch)
	}

	return ExecutionPlan{Batches: batches}
}

// PlanFromState resolves execution order dynamically using actual state.
// Returns the current ready set — steps whose preconditions are met.
// Call repeatedly after each execution to get the next ready set.
func PlanFromState(c *Catalyst, actual map[string]any, completed map[*Catalyst]bool) []*Catalyst {
	var ready []*Catalyst
	for _, step := range c.Steps {
		if completed[step] {
			continue
		}
		if step.preconditionsMet(actual) {
			ready = append(ready, step)
		}
	}
	return ready
}
