package agentport

import (
	"github.com/dpopsuev/jericho/agent"
	"github.com/dpopsuev/jericho/collective"
	"github.com/dpopsuev/jericho/pool"
)

// Facade type aliases — definitions live in jericho/agent.
type (
	Staff  = agent.Staff
	Solo   = agent.Solo
	Config = pool.AgentConfig
)

// Backward-compat aliases for pre-v0.2.0 consumer code.
type (
	AgentHandle  = agent.Solo       // renamed in v0.2.0
	LaunchConfig = pool.AgentConfig // renamed in v0.2.0
)

// Collective type aliases — definitions live in jericho/collective.
type (
	Collective         = collective.Collective
	CollectiveConfig   = collective.CollectiveConfig
	CollectiveStrategy = collective.CollectiveStrategy
	Dialectic          = collective.Dialectic
)

// Backward-compat alias.
type AgentCollective = collective.Collective // renamed in v0.2.0

// Facade constructors.
var (
	NewStaff        = agent.NewStaff
	SpawnCollective = collective.SpawnCollective
)
