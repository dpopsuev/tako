package framework

// Category: Core Primitives — aliases to core/ package.

import "github.com/dpopsuev/origami/core"

// Type aliases — definitions live in core/ (which re-exports element/).
type (
	Approach      = core.Approach
	Element       = core.Element
	SpeedClass    = core.SpeedClass
	ElementTraits = core.ElementTraits
)

// Approach constants.
const (
	ApproachRapid      = core.ApproachRapid
	ApproachAggressive = core.ApproachAggressive
	ApproachMethodical = core.ApproachMethodical
	ApproachRigorous   = core.ApproachRigorous
	ApproachAnalytical = core.ApproachAnalytical
	ApproachHolistic   = core.ApproachHolistic
)

// Element constants.
const (
	ElementFire      = core.ElementFire
	ElementLightning = core.ElementLightning
	ElementEarth     = core.ElementEarth
	ElementDiamond   = core.ElementDiamond
	ElementWater     = core.ElementWater
	ElementAir       = core.ElementAir
)

// SpeedClass constants.
const (
	SpeedFastest  = core.SpeedFastest
	SpeedFast     = core.SpeedFast
	SpeedSteady   = core.SpeedSteady
	SpeedPrecise  = core.SpeedPrecise
	SpeedDeep     = core.SpeedDeep
	SpeedHolistic = core.SpeedHolistic
)

// DefaultTraits returns the canonical trait set for a given element.
func DefaultTraits(e Element) ElementTraits { return core.DefaultTraits(e) }

// AllElements returns the six core elements.
func AllElements() []Element { return core.AllElements() }

// ResolveApproach maps a user-facing approach name to an internal Element.
func ResolveApproach(name string) (Element, bool) { return core.ResolveApproach(name) }

// ApproachForElement returns the user-facing approach name for an element.
func ApproachForElement(e Element) Approach { return core.ApproachForElement(e) }

// ApproachEmoji returns the emoji for an approach.
func ApproachEmoji(a Approach) string { return core.ApproachEmoji(a) }

// ApproachTraits returns the ElementTraits for an approach.
func ApproachTraits(a Approach) ElementTraits { return core.ApproachTraits(a) }

// ApproachTraitsSummary returns a formatted multi-line summary for LSP hover.
func ApproachTraitsSummary(a Approach) string { return core.ApproachTraitsSummary(a) }

// AllApproaches returns the six core approaches.
func AllApproaches() []Approach { return core.AllApproaches() }
