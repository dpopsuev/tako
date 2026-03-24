package agentport

import "github.com/dpopsuev/bugle/identity"

// Type aliases — definitions live in bugle/identity.
type (
	AgentIdentity   = identity.AgentIdentity
	Color           = identity.Color
	Alignment       = identity.Alignment
	Position        = identity.Position
	MetaPhase       = identity.MetaPhase
	Role            = identity.Role
	CostProfile     = identity.CostProfile
	ModelIdentity   = identity.ModelIdentity
	Persona         = identity.Persona
	PersonaResolver = identity.PersonaResolver
)

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
// It reads identity.DefaultPersonaResolver at call time, so it picks up
// init()-registered resolvers correctly.
func GetDefaultPersonaResolver() PersonaResolver {
	return identity.DefaultPersonaResolver
}
