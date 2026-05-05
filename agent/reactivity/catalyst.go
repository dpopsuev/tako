package reactivity

// Catalyst is the navigation vector — Current State → Desired State.
// Recursive: decomposes into Children during Compose, each a smaller Catalyst.
type Catalyst struct {
	Need     string         // human-readable task description (prompt text)
	Current  map[string]any // observed initial state (absolute 0)
	Desired  map[string]any // goal state (absolute 1)
	Trust    float64        // 0.0 = full HITL, 1.0 = full auto
	Children []*Catalyst    // decomposed sub-catalysts (populated during Compose)
}
