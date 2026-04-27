package memory

import "time"

// Tier represents the DIKUW knowledge level.
type Tier int

const (
	Knowledge     Tier = iota // raw facts
	Understanding             // connected facts
	Wisdom                    // actionable insight
)

// KnowledgeNode is an atom in the Memory Mesh.
type KnowledgeNode struct {
	ID        string
	Content   string
	Tier      Tier
	CreatedAt time.Time
}

// Edge connects two KnowledgeNodes in the Mesh.
type Edge struct {
	From      string
	To        string
	Relation  string
	Weight    float64
	CreatedAt time.Time
}
