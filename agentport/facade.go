package agentport

import (
	"github.com/dpopsuev/bugle/collective"
	"github.com/dpopsuev/bugle/facade"
	"github.com/dpopsuev/bugle/pool"
)

// Facade type aliases — definitions live in bugle/facade.
type (
	Staff        = facade.Staff
	AgentHandle  = facade.AgentHandle
	LaunchConfig = pool.LaunchConfig
)

// Collective type aliases — definitions live in bugle/collective.
type (
	AgentCollective    = collective.AgentCollective
	CollectiveConfig   = collective.CollectiveConfig
	CollectiveStrategy = collective.CollectiveStrategy
	Dialectic          = collective.Dialectic
)

// Facade constructors.
var (
	NewStaff        = facade.NewStaff
	SpawnCollective = collective.SpawnCollective
)
