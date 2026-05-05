package reactivity

// Catalyst is the navigation vector — Current State → Desired State.
// Steps are an unordered set of sub-catalysts. The executor resolves
// execution order from state dependencies: if step X's Current references
// a dimension that step Y's Desired modifies, X waits for Y.
// Parallel execution falls out naturally from independent state dimensions.
type Catalyst struct {
	Need    string         // human-readable task description (prompt text)
	Current map[string]any // observed initial state (absolute 0)
	Desired map[string]any // goal state (absolute 1)
	Trust   float64        // 0.0 = full HITL, 1.0 = full auto
	Steps   []*Catalyst    // unordered set — executor resolves order from state deps
}

// Ready returns Steps whose Current preconditions are satisfied by the actual state.
func (c *Catalyst) Ready(actual map[string]any) []*Catalyst {
	var ready []*Catalyst
	for _, step := range c.Steps {
		if step.preconditionsMet(actual) {
			ready = append(ready, step)
		}
	}
	return ready
}

// Completed returns true if the actual state satisfies all Desired dimensions.
func (c *Catalyst) Completed(actual map[string]any) bool {
	for k, expected := range c.Desired {
		if actual[k] != expected {
			return false
		}
	}
	return len(c.Desired) > 0
}

func (c *Catalyst) preconditionsMet(actual map[string]any) bool {
	for k, expected := range c.Current {
		if actual[k] != expected {
			return false
		}
	}
	return true
}

// DependsOn returns true if this step's Current references any dimension
// that other's Desired modifies. State overlap IS the dependency edge.
func (c *Catalyst) DependsOn(other *Catalyst) bool {
	for k := range c.Current {
		if _, modifies := other.Desired[k]; modifies {
			return true
		}
	}
	return false
}
