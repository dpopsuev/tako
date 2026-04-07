package circuit

// Category: Core Primitives

import "github.com/dpopsuev/troupe/identity"

// Minimal type aliases — only types required by circuit's own interfaces
// (Node.ElementAffinity, Walker.Identity). All other identity/element types
// live in roster and should be imported from there.
type (
	Element       = identity.Element
	AgentIdentity = identity.Archetype
)
