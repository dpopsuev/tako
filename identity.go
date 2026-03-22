package framework

// Category: Processing & Support — aliases to core/ package.

import "github.com/dpopsuev/origami/core"

type Persona = core.Persona
type PersonaResolver = core.PersonaResolver
type Color = core.Color
type Alignment = core.Alignment
type Position = core.Position
type MetaPhase = core.MetaPhase
type Role = core.Role
type CostProfile = core.CostProfile
type AgentIdentity = core.AgentIdentity
type ModelIdentity = core.ModelIdentity

const (
	AlignmentThesis     = core.AlignmentThesis
	AlignmentAntithesis = core.AlignmentAntithesis
)

const (
	PositionPG = core.PositionPG
	PositionSG = core.PositionSG
	PositionPF = core.PositionPF
	PositionC  = core.PositionC
)

const (
	MetaPhaseBk = core.MetaPhaseBk
	MetaPhaseFc = core.MetaPhaseFc
	MetaPhasePt = core.MetaPhasePt
)

const (
	RoleWorker   = core.RoleWorker
	RoleManager  = core.RoleManager
	RoleEnforcer = core.RoleEnforcer
	RoleBroker   = core.RoleBroker
)

var ValidRoles = core.ValidRoles
var DefaultPersonaResolver = core.DefaultPersonaResolver

func HomeZoneFor(p Position) MetaPhase { return core.HomeZoneFor(p) }
