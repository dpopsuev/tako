package agentport

import "github.com/dpopsuev/bugle/facade"

// Facade type aliases — definitions live in bugle/facade.
type (
	Staff       = facade.Staff
	AgentHandle = facade.AgentHandle
)

// Facade constructors.
var NewStaff = facade.NewStaff
