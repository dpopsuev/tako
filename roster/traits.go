// Package roster re-exports agent identity and trait types from troupe sub-packages.
// No other Origami package should import troupe/identity or troupe/world directly.
package roster

import "github.com/dpopsuev/troupe/identity"

// Type aliases — definitions live in troupe/identity.
type (
	Element       = identity.Element
	Approach      = identity.Approach
	SpeedClass    = identity.SpeedClass
	ElementTraits = identity.ElementTraits
)

// Element constants.
const (
	ElementFire      = identity.ElementFire
	ElementLightning = identity.ElementLightning
	ElementEarth     = identity.ElementEarth
	ElementDiamond   = identity.ElementDiamond
	ElementWater     = identity.ElementWater
	ElementAir       = identity.ElementAir
)

// Approach constants.
const (
	ApproachRapid      = identity.ApproachRapid
	ApproachAggressive = identity.ApproachAggressive
	ApproachMethodical = identity.ApproachMethodical
	ApproachRigorous   = identity.ApproachRigorous
	ApproachAnalytical = identity.ApproachAnalytical
	ApproachHolistic   = identity.ApproachHolistic
)

// SpeedClass constants.
const (
	SpeedFastest  = identity.SpeedFastest
	SpeedFast     = identity.SpeedFast
	SpeedSteady   = identity.SpeedSteady
	SpeedPrecise  = identity.SpeedPrecise
	SpeedDeep     = identity.SpeedDeep
	SpeedHolistic = identity.SpeedHolistic
)

// DefaultTraits returns the canonical trait set for a given element.
var DefaultTraits = identity.DefaultTraits

// AllElements returns the six core elements.
var AllElements = identity.AllElements

// ResolveApproach maps a user-facing approach name to an internal Element.
var ResolveApproach = identity.ResolveApproach

// ApproachForElement returns the user-facing approach name for an element.
var ApproachForElement = identity.ApproachForElement

// ApproachEmoji returns the emoji for an approach.
var ApproachEmoji = identity.ApproachEmoji

// ApproachTraits returns the ElementTraits for an approach.
var ApproachTraits = identity.ApproachTraits

// ApproachTraitsSummary returns a formatted multi-line summary for LSP hover.
var ApproachTraitsSummary = identity.ApproachTraitsSummary

// AllApproaches returns the six core approaches.
var AllApproaches = identity.AllApproaches
