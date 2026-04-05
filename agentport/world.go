package agentport

import (
	"github.com/dpopsuev/troupe/identity"
	"github.com/dpopsuev/troupe/world"
)

// Type aliases — definitions live in jericho/world.
type (
	World         = world.World
	EntityID      = world.EntityID
	Component     = world.Component
	ComponentType = world.ComponentType
	DiffKind      = world.DiffKind
	DiffHook      = world.DiffHook
	Alive         = world.Alive
	AliveState    = world.AliveState
	Ready         = world.Ready
	ReadyReason   = world.ReadyReason
	Hierarchy     = world.Hierarchy
	Budget        = world.Budget
	Progress      = world.Progress
	Display       = world.Display

	IdentityStrategy = world.IdentityStrategy
)

// Backward-compat aliases for pre-v0.2.0 code.
type (
	Health     = world.Alive      // renamed in v0.2.0
	AgentState = world.AliveState // renamed in v0.2.0
)

// DiffKind constants.
const (
	DiffAttached = world.DiffAttached
	DiffDetached = world.DiffDetached
	DiffUpdated  = world.DiffUpdated
)

// AliveState constants.
const (
	AliveRunning    = world.AliveRunning
	AliveTerminated = world.AliveTerminated
)

// Backward-compat: old AgentState constants.
const (
	Active = world.AliveRunning
	Done   = world.AliveTerminated
)

// ReadyReason constants.
const (
	ReasonIdle        = world.ReasonIdle
	ReasonStale       = world.ReasonStale
	ReasonErrored     = world.ReasonErrored
	ReasonTerminating = world.ReasonTerminating
	ReasonTerminated  = world.ReasonTerminated
)

// Component type constants.
const (
	AliveType     = world.AliveType
	ReadyType     = world.ReadyType
	HierarchyType = world.HierarchyType
	BudgetType    = world.BudgetType
	ProgressType  = world.ProgressType
	DisplayType   = world.DisplayType
)

// Backward-compat.
const HealthType = world.AliveType // renamed in v0.2.0

// NewWorld creates an empty ECS world.
var NewWorld = world.NewWorld

// Generic function wrappers — Go generic functions cannot be aliased via var.

// Attach adds a component to an entity. Replaces if already present.
func Attach[T world.Component](w *world.World, id world.EntityID, c T) {
	world.Attach(w, id, c)
}

// TryAttach adds a component if the entity exists. Returns false if dead.
func TryAttach[T world.Component](w *world.World, id world.EntityID, c T) bool {
	return world.TryAttach(w, id, c)
}

// Get retrieves a component. Panics if the entity or component is not present.
func Get[T world.Component](w *world.World, id world.EntityID) T {
	return world.Get[T](w, id)
}

// TryGet retrieves a component, returns (zero, false) if absent.
func TryGet[T world.Component](w *world.World, id world.EntityID) (T, bool) {
	return world.TryGet[T](w, id)
}

// Detach removes a component from an entity. No-op if not present.
func Detach[T world.Component](w *world.World, id world.EntityID) {
	world.Detach[T](w, id)
}

// Query returns all entity IDs that have the specified component type.
func Query[T world.Component](w *world.World) []world.EntityID {
	return world.Query[T](w)
}

// --- Symbol re-exports (was palette) ---

// Type aliases — definitions live in jericho/identity.
type (
	ColorIdentity   = identity.Color // renamed in v0.2.0
	SymbolColor     = identity.Color
	Registry        = identity.Registry
	Shade           = identity.Shade
	PaletteColor    = identity.PaletteColor
	DefaultStrategy = identity.DefaultStrategy
)

// ColorType is the ComponentType for Color (was ColorIdentityType).
const ColorType = identity.ColorType

// Backward-compat.
const ColorIdentityType = identity.ColorType // renamed in v0.2.0

// Palette is the full 12x8 color palette.
var Palette = identity.Palette

// Constructors and lookups.
var (
	NewRegistry        = identity.NewRegistry
	NewDefaultStrategy = identity.NewDefaultStrategy
	LookupShade        = identity.LookupShade
	LookupColor        = identity.LookupColor
)
