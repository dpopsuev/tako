package circuit

// Category: Core Primitives

import "github.com/dpopsuev/troupe/visual"

// Element is the behavioral archetype tag for nodes and zones.
type Element = visual.Element

// AgentIdentity is the local walker identity type. Replaces the deleted
// troupe/identity.Archetype with a minimal struct carrying only the
// fields Origami actually uses.
type AgentIdentity struct {
	Name            string             `json:"name"`
	Role            string             `json:"role,omitempty"`
	Element         Element            `json:"element"`
	Skills          []string           `json:"skills,omitempty"`
	StickinessLevel int                `json:"stickiness_level"`
	StepAffinity    map[string]float64 `json:"step_affinity,omitempty"`
}
