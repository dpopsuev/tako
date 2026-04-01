package circuit

// Category: Core Primitives

import "github.com/dpopsuev/origami/agentport"

// Minimal type aliases — only types required by circuit's own interfaces
// (Node.ElementAffinity, Walker.Identity). All other identity/element types
// live in agentport and should be imported from there.
type (
	Element       = agentport.Element
	AgentIdentity = agentport.AgentIdentity
)
