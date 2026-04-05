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

// Event types — definitions live in troupe root.
type (
	Event       = troupe.Event
	EventKind   = troupe.EventKind
	EventDetail = troupe.EventDetail
)

// Event kind constants.
const (
	EventStarted    = troupe.Started
	EventCompleted  = troupe.Completed
	EventFailed     = troupe.Failed
	EventTransition = troupe.Transition
	EventDone       = troupe.Done
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

// Hook types — definitions live in troupe root.
type (
	Hook        = troupe.Hook
	SpawnHook   = troupe.SpawnHook
	PerformHook = troupe.PerformHook
	Meter       = troupe.Meter
	Usage       = troupe.Usage
	UsageDetail = troupe.UsageDetail
)

// Broker options.
type BrokerOption = troupe.BrokerOption

var (
	WithHook  = troupe.WithHook
	WithMeter = troupe.WithMeter
)

// Facade constructors.
var (
	NewBroker       = troupe.NewBroker
	NewInMemoryMeter = troupe.NewInMemoryMeter
	SpawnCollective = collective.SpawnCollective
)
