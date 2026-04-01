package agentport

import "github.com/dpopsuev/jericho/symbol"

// Type aliases — definitions live in jericho/symbol.
type (
	Persona         = symbol.Persona
	PersonaResolver = symbol.PersonaResolver
	Alignment       = symbol.Alignment
	Position        = symbol.Position
	MetaPhase       = symbol.MetaPhase
	Role            = symbol.Role
	ModelIdentity   = symbol.ModelIdentity
	CostProfile     = symbol.CostProfile
	Reservation     = symbol.Reservation
)

// Backward-compat: AgentIdentity was deleted in v0.2.0.
// Persona is now flat — use Persona directly instead of Persona.Identity.
type AgentIdentity = symbol.Persona // flattened in v0.2.0

// Color is now jericho/symbol.Color (was identity.Color + palette.ColorIdentity).
type Color = symbol.Color

// Alignment constants.
const (
	AlignmentThesis     = symbol.AlignmentThesis
	AlignmentAntithesis = symbol.AlignmentAntithesis
)

// Position constants.
const (
	PositionPG = symbol.PositionPG
	PositionSG = symbol.PositionSG
	PositionPF = symbol.PositionPF
	PositionC  = symbol.PositionC
)

// MetaPhase constants.
const (
	MetaPhaseBk = symbol.MetaPhaseBk
	MetaPhaseFc = symbol.MetaPhaseFc
	MetaPhasePt = symbol.MetaPhasePt
)

// Role constants.
const (
	RoleWorker   = symbol.RoleWorker
	RoleManager  = symbol.RoleManager
	RoleEnforcer = symbol.RoleEnforcer
	RoleBroker   = symbol.RoleBroker
)

// ValidRoles contains all recognized role values for validation.
var ValidRoles = symbol.ValidRoles

// HomeZoneFor returns the MetaPhase for a given Position.
var HomeZoneFor = symbol.HomeZoneFor

// GetDefaultPersonaResolver returns the currently registered persona resolver.
func GetDefaultPersonaResolver() PersonaResolver {
	return symbol.DefaultPersonaResolver
}
