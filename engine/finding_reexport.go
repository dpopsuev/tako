package engine

// Re-exports from engine/finding sub-package for backward compatibility.

import "github.com/dpopsuev/origami/engine/finding"

// Type aliases for finding types.
type (
	FindingRouter            = finding.FindingRouter
	FindingHandlers          = finding.FindingHandlers
	RouteRule                = finding.RouteRule
	RouteTarget              = finding.RouteTarget
	VetoHook                 = finding.VetoHook
	VetoArtifact             = finding.VetoArtifact
	InMemoryFindingCollector = finding.InMemoryFindingCollector
)

// Re-exported constructors.
var (
	NewFindingRouter = finding.NewFindingRouter
	NewVetoHook      = finding.NewVetoHook
)

// Re-exported constants.
const (
	TargetManager = finding.TargetManager
	TargetBroker  = finding.TargetBroker
	TargetLog     = finding.TargetLog
)
