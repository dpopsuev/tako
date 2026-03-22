package circuit

// Category: Core Primitives

import "github.com/dpopsuev/origami/element"

// Type aliases — definitions live in element/ sub-package.
type (
	Approach      = element.Approach
	Element       = element.Element
	SpeedClass    = element.SpeedClass
	ElementTraits = element.ElementTraits
)

// Approach constants.
const (
	ApproachRapid      = element.ApproachRapid
	ApproachAggressive = element.ApproachAggressive
	ApproachMethodical = element.ApproachMethodical
	ApproachRigorous   = element.ApproachRigorous
	ApproachAnalytical = element.ApproachAnalytical
	ApproachHolistic   = element.ApproachHolistic
)

// Element constants.
const (
	ElementFire      = element.ElementFire
	ElementLightning = element.ElementLightning
	ElementEarth     = element.ElementEarth
	ElementDiamond   = element.ElementDiamond
	ElementWater     = element.ElementWater
	ElementAir       = element.ElementAir
)

// SpeedClass constants.
const (
	SpeedFastest  = element.SpeedFastest
	SpeedFast     = element.SpeedFast
	SpeedSteady   = element.SpeedSteady
	SpeedPrecise  = element.SpeedPrecise
	SpeedDeep     = element.SpeedDeep
	SpeedHolistic = element.SpeedHolistic
)

// DefaultTraits returns the canonical trait set for a given element.
func DefaultTraits(e Element) ElementTraits { return element.DefaultTraits(e) }

// AllElements returns the six core elements.
func AllElements() []Element { return element.AllElements() }

// ResolveApproach maps a user-facing approach name to an internal Element.
func ResolveApproach(name string) (Element, bool) { return element.ResolveApproach(name) }

// ApproachForElement returns the user-facing approach name for an element.
func ApproachForElement(e Element) Approach { return element.ApproachForElement(e) }

// ApproachEmoji returns the emoji for an approach.
func ApproachEmoji(a Approach) string { return element.ApproachEmoji(a) }

// ApproachTraits returns the ElementTraits for an approach.
func ApproachTraits(a Approach) ElementTraits { return element.ApproachTraits(a) }

// ApproachTraitsSummary returns a formatted multi-line summary for LSP hover.
func ApproachTraitsSummary(a Approach) string { return element.ApproachTraitsSummary(a) }

// AllApproaches returns the six core approaches.
func AllApproaches() []Approach { return element.AllApproaches() }
