package reactivity

// Catalyst is the structured Need — the Kanban card.
// Carries completion criteria that sensors can verify.
type Catalyst struct {
	Need     string
	Criteria map[string]any
}
