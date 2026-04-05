package agentport

import "github.com/dpopsuev/troupe/identity"

// Type aliases — definitions live in jericho/identity.
type (
	Persona         = identity.Archetype
	PersonaResolver = identity.ArchetypeResolver
	Alignment       = identity.Alignment
	Position        = identity.Position
	MetaPhase       = identity.MetaPhase
	Role            = identity.Role
	ModelIdentity   = identity.ModelIdentity
	CostProfile     = identity.CostProfile
	Reservation     = identity.Reservation
)

// Backward-compat: AgentIdentity was deleted in v0.2.0.
// Persona is now flat — use Persona directly instead of Persona.Identity.
type AgentIdentity = identity.Archetype // flattened in v0.2.0

// Color is now jericho/identity.Color (was identity.Color + palette.ColorIdentity).
type Color = identity.Color

// Alignment constants.
const (
	AlignmentThesis     = identity.AlignmentThesis
	AlignmentAntithesis = identity.AlignmentAntithesis
)

// Position constants.
const (
	PositionPG = identity.PositionPG
	PositionSG = identity.PositionSG
	PositionPF = identity.PositionPF
	PositionC  = identity.PositionC
)

// MetaPhase constants.
const (
	MetaPhaseBk = identity.MetaPhaseBk
	MetaPhaseFc = identity.MetaPhaseFc
	MetaPhasePt = identity.MetaPhasePt
)

// Role constants.
const (
	RoleWorker   = identity.RoleWorker
	RoleManager  = identity.RoleManager
	RoleEnforcer = identity.RoleEnforcer
	RoleBroker   = identity.RoleBroker
)

// ValidRoles contains all recognized role values for validation.
var ValidRoles = identity.ValidRoles

// HomeZoneFor returns the MetaPhase for a given Position.
var HomeZoneFor = identity.HomeZoneFor

// GetDefaultPersonaResolver returns the currently registered persona resolver.
func GetDefaultPersonaResolver() PersonaResolver {
	return identity.DefaultArchetypeResolver
}

// PersonaAll returns all known archetypes (backward compat for persona name).
func PersonaAll() []Persona {
	return identity.All()
}
