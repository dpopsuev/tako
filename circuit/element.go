package circuit

// Category: Core Primitives

import "github.com/dpopsuev/origami/agentport"

// Type aliases — definitions live in bugle/element, re-exported via agentport.
type (
	Approach      = agentport.Approach
	Element       = agentport.Element
	SpeedClass    = agentport.SpeedClass
	ElementTraits = agentport.ElementTraits
)

// Approach constants.
const (
	ApproachRapid      = agentport.ApproachRapid
	ApproachAggressive = agentport.ApproachAggressive
	ApproachMethodical = agentport.ApproachMethodical
	ApproachRigorous   = agentport.ApproachRigorous
	ApproachAnalytical = agentport.ApproachAnalytical
	ApproachHolistic   = agentport.ApproachHolistic
)

// Element constants.
const (
	ElementFire      = agentport.ElementFire
	ElementLightning = agentport.ElementLightning
	ElementEarth     = agentport.ElementEarth
	ElementDiamond   = agentport.ElementDiamond
	ElementWater     = agentport.ElementWater
	ElementAir       = agentport.ElementAir
)

// SpeedClass constants.
const (
	SpeedFastest  = agentport.SpeedFastest
	SpeedFast     = agentport.SpeedFast
	SpeedSteady   = agentport.SpeedSteady
	SpeedPrecise  = agentport.SpeedPrecise
	SpeedDeep     = agentport.SpeedDeep
	SpeedHolistic = agentport.SpeedHolistic
)

// DefaultTraits returns the canonical trait set for a given element.
func DefaultTraits(e Element) ElementTraits { return agentport.DefaultTraits(e) }

// AllElements returns the six core elements.
func AllElements() []Element { return agentport.AllElements() }

// ResolveApproach maps a user-facing approach name to an internal Element.
func ResolveApproach(name string) (Element, bool) { return agentport.ResolveApproach(name) }

// ApproachForElement returns the user-facing approach name for an element.
func ApproachForElement(e Element) Approach { return agentport.ApproachForElement(e) }

// ApproachEmoji returns the emoji for an approach.
func ApproachEmoji(a Approach) string { return agentport.ApproachEmoji(a) }

// ApproachTraits returns the ElementTraits for an approach.
func ApproachTraits(a Approach) ElementTraits { return agentport.ApproachTraits(a) }

// ApproachTraitsSummary returns a formatted multi-line summary for LSP hover.
func ApproachTraitsSummary(a Approach) string { return agentport.ApproachTraitsSummary(a) }

// AllApproaches returns the six core approaches.
func AllApproaches() []Approach { return agentport.AllApproaches() }
