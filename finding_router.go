package framework

// Category: Processing & Support — aliases to finding/ package.

import "github.com/dpopsuev/origami/finding"

type RouteTarget = finding.RouteTarget

const (
	TargetManager = finding.TargetManager
	TargetBroker  = finding.TargetBroker
	TargetLog     = finding.TargetLog
)

type RouteRule = finding.RouteRule
type FindingHandlers = finding.FindingHandlers
type FindingRouter = finding.FindingRouter

func NewFindingRouter(rules []RouteRule, handlers FindingHandlers) *FindingRouter {
	return finding.NewFindingRouter(rules, handlers)
}
