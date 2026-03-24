package agentport

import "github.com/dpopsuev/bugle/element"

// Type aliases — definitions live in bugle/element.
type (
	Element       = element.Element
	Approach      = element.Approach
	SpeedClass    = element.SpeedClass
	ElementTraits = element.ElementTraits
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

// Approach constants.
const (
	ApproachRapid      = element.ApproachRapid
	ApproachAggressive = element.ApproachAggressive
	ApproachMethodical = element.ApproachMethodical
	ApproachRigorous   = element.ApproachRigorous
	ApproachAnalytical = element.ApproachAnalytical
	ApproachHolistic   = element.ApproachHolistic
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
var DefaultTraits = element.DefaultTraits

// AllElements returns the six core elements.
var AllElements = element.AllElements

// ResolveApproach maps a user-facing approach name to an internal Element.
var ResolveApproach = element.ResolveApproach

// ApproachForElement returns the user-facing approach name for an element.
var ApproachForElement = element.ApproachForElement

// ApproachEmoji returns the emoji for an approach.
var ApproachEmoji = element.ApproachEmoji

// ApproachTraits returns the ElementTraits for an approach.
var ApproachTraits = element.ApproachTraits

// ApproachTraitsSummary returns a formatted multi-line summary for LSP hover.
var ApproachTraitsSummary = element.ApproachTraitsSummary

// AllApproaches returns the six core approaches.
var AllApproaches = element.AllApproaches
