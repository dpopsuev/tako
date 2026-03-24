package agentport

import (
	"github.com/dpopsuev/bugle/palette"
	"github.com/dpopsuev/bugle/world"
)

// Type aliases — definitions live in bugle/world.
type (
	World         = world.World
	EntityID      = world.EntityID
	Component     = world.Component
	ComponentType = world.ComponentType
	DiffKind      = world.DiffKind
	DiffHook      = world.DiffHook
	AgentState    = world.AgentState
	Health        = world.Health
	Hierarchy     = world.Hierarchy
	Budget        = world.Budget
	Progress      = world.Progress

	IdentityStrategy = world.IdentityStrategy
)

// DiffKind constants.
const (
	DiffAttached = world.DiffAttached
	DiffDetached = world.DiffDetached
	DiffUpdated  = world.DiffUpdated
)

// AgentState constants.
const (
	Active  = world.Active
	Idle    = world.Idle
	Stale   = world.Stale
	Errored = world.Errored
	Done    = world.Done
)

// Component type constants.
const (
	HealthType    = world.HealthType
	HierarchyType = world.HierarchyType
	BudgetType    = world.BudgetType
	ProgressType  = world.ProgressType
)

// NewWorld creates an empty ECS world.
var NewWorld = world.NewWorld

// Generic function wrappers — Go generic functions cannot be aliased via var.

// Attach adds a component to an entity. Replaces if already present.
func Attach[T world.Component](w *world.World, id world.EntityID, c T) {
	world.Attach(w, id, c)
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

// --- Palette re-exports (bugle/palette) ---

// Type aliases — definitions live in bugle/palette.
type (
	ColorIdentity   = palette.ColorIdentity
	Registry        = palette.Registry
	Shade           = palette.Shade
	Colour          = palette.Colour
	DefaultStrategy = palette.DefaultStrategy
)

// ColorIdentityType is the ComponentType for ColorIdentity.
const ColorIdentityType = palette.ColorIdentityType

// Palette is the full 7x8 colour palette.
var Palette = palette.Palette

// Constructors and lookups.
var (
	NewRegistry        = palette.NewRegistry
	NewDefaultStrategy = palette.NewDefaultStrategy
	LookupShade        = palette.LookupShade
	LookupColour       = palette.LookupColour
)
