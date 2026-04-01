package agentport

import "github.com/dpopsuev/jericho/symbol"

// Type aliases — definitions live in jericho/symbol.
type (
	Element       = symbol.Element
	Approach      = symbol.Approach
	SpeedClass    = symbol.SpeedClass
	ElementTraits = symbol.ElementTraits
)

// Element constants.
const (
	ElementFire      = symbol.ElementFire
	ElementLightning = symbol.ElementLightning
	ElementEarth     = symbol.ElementEarth
	ElementDiamond   = symbol.ElementDiamond
	ElementWater     = symbol.ElementWater
	ElementAir       = symbol.ElementAir
)

// Approach constants.
const (
	ApproachRapid      = symbol.ApproachRapid
	ApproachAggressive = symbol.ApproachAggressive
	ApproachMethodical = symbol.ApproachMethodical
	ApproachRigorous   = symbol.ApproachRigorous
	ApproachAnalytical = symbol.ApproachAnalytical
	ApproachHolistic   = symbol.ApproachHolistic
)

// SpeedClass constants.
const (
	SpeedFastest  = symbol.SpeedFastest
	SpeedFast     = symbol.SpeedFast
	SpeedSteady   = symbol.SpeedSteady
	SpeedPrecise  = symbol.SpeedPrecise
	SpeedDeep     = symbol.SpeedDeep
	SpeedHolistic = symbol.SpeedHolistic
)

// DefaultTraits returns the canonical trait set for a given element.
var DefaultTraits = symbol.DefaultTraits

// AllElements returns the six core elements.
var AllElements = symbol.AllElements

// ResolveApproach maps a user-facing approach name to an internal Element.
var ResolveApproach = symbol.ResolveApproach

// ApproachForElement returns the user-facing approach name for an element.
var ApproachForElement = symbol.ApproachForElement

// ApproachEmoji returns the emoji for an approach.
var ApproachEmoji = symbol.ApproachEmoji

// ApproachTraits returns the ElementTraits for an approach.
var ApproachTraits = symbol.ApproachTraits

// ApproachTraitsSummary returns a formatted multi-line summary for LSP hover.
var ApproachTraitsSummary = symbol.ApproachTraitsSummary

// AllApproaches returns the six core approaches.
var AllApproaches = symbol.AllApproaches
