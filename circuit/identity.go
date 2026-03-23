package circuit

// Category: Processing & Support
//
// Identity types are defined in github.com/dpopsuev/bugle.
// This file re-exports them as the circuit package's published API.

import "github.com/dpopsuev/bugle"

// Identity types — definitions live in bugle.
type (
	Persona         = bugle.Persona
	PersonaResolver = bugle.PersonaResolver
	Color           = bugle.Color
	Alignment       = bugle.Alignment
	Position        = bugle.Position
	MetaPhase       = bugle.MetaPhase
	Role            = bugle.Role
	CostProfile     = bugle.CostProfile
	AgentIdentity   = bugle.AgentIdentity
	ModelIdentity   = bugle.ModelIdentity
)

// Alignment constants.
const (
	AlignmentThesis     = bugle.AlignmentThesis
	AlignmentAntithesis = bugle.AlignmentAntithesis
)

// Position constants.
const (
	PositionPG = bugle.PositionPG
	PositionSG = bugle.PositionSG
	PositionPF = bugle.PositionPF
	PositionC  = bugle.PositionC
)

// MetaPhase constants.
const (
	MetaPhaseBk = bugle.MetaPhaseBk
	MetaPhaseFc = bugle.MetaPhaseFc
	MetaPhasePt = bugle.MetaPhasePt
)

// Role constants.
const (
	RoleWorker   = bugle.RoleWorker
	RoleManager  = bugle.RoleManager
	RoleEnforcer = bugle.RoleEnforcer
	RoleBroker   = bugle.RoleBroker
)

// ValidRoles contains all recognized role values for validation.
var ValidRoles = bugle.ValidRoles

// HomeZoneFor returns the MetaPhase for a given Position.
var HomeZoneFor = bugle.HomeZoneFor
