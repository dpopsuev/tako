package circuit

// Category: Processing & Support
//
// Identity types are defined in github.com/dpopsuev/jericho/identity.
// This file re-exports them via agentport as the circuit package's published API.

import "github.com/dpopsuev/origami/agentport"

// Identity types — definitions live in bugle/identity, re-exported via agentport.
type (
	Persona         = agentport.Persona
	PersonaResolver = agentport.PersonaResolver
	Color           = agentport.Color
	Alignment       = agentport.Alignment
	Position        = agentport.Position
	MetaPhase       = agentport.MetaPhase
	Role            = agentport.Role
	CostProfile     = agentport.CostProfile
	AgentIdentity   = agentport.AgentIdentity
	ModelIdentity   = agentport.ModelIdentity
	Reservation     = agentport.Reservation
)

// Alignment constants.
const (
	AlignmentThesis     = agentport.AlignmentThesis
	AlignmentAntithesis = agentport.AlignmentAntithesis
)

// Position constants.
const (
	PositionPG = agentport.PositionPG
	PositionSG = agentport.PositionSG
	PositionPF = agentport.PositionPF
	PositionC  = agentport.PositionC
)

// MetaPhase constants.
const (
	MetaPhaseBk = agentport.MetaPhaseBk
	MetaPhaseFc = agentport.MetaPhaseFc
	MetaPhasePt = agentport.MetaPhasePt
)

// Role constants.
const (
	RoleWorker   = agentport.RoleWorker
	RoleManager  = agentport.RoleManager
	RoleEnforcer = agentport.RoleEnforcer
	RoleBroker   = agentport.RoleBroker
)

// ValidRoles contains all recognized role values for validation.
var ValidRoles = agentport.ValidRoles

// HomeZoneFor returns the MetaPhase for a given Position.
var HomeZoneFor = agentport.HomeZoneFor
