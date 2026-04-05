package agentport

import (
	"github.com/dpopsuev/troupe"
	"github.com/dpopsuev/troupe/collective"
)

// Core Troupe types — definitions live in troupe root.
type (
	Broker      = troupe.Broker
	Actor       = troupe.Actor
	ActorConfig = troupe.ActorConfig
	BrokerPrefs = troupe.Preferences // renamed to avoid collision with arsenal.Preferences
)

// Backward-compat aliases for pre-Troupe consumer code.
type (
	Staff        = troupe.Broker      // deprecated: use Broker
	LaunchConfig = troupe.ActorConfig // deprecated: use ActorConfig
)

// Collective type aliases — definitions live in troupe/collective.
type (
	Collective         = collective.Collective
	CollectiveStrategy = collective.CollectiveStrategy
	Dialectic          = collective.Dialectic
)

// Backward-compat alias.
type AgentCollective = collective.Collective // deprecated: use Collective

// Collective strategies.
type (
	RoundRobin    = collective.RoundRobin
	Race          = collective.Race
	Scatter       = collective.Scatter
	DialecticPair = collective.DialecticPair
	Gatekeeper    = collective.Gatekeeper
)

// Facade constructors.
var (
	NewBroker       = troupe.NewBroker
	SpawnCollective = collective.SpawnCollective
)
