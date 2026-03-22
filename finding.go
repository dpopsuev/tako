package framework

// Category: Processing & Support — aliases to core/ and finding/ packages.

import (
	"github.com/dpopsuev/origami/core"
	"github.com/dpopsuev/origami/finding"
)

type FindingSeverity = core.FindingSeverity

const (
	FindingInfo    = core.FindingInfo
	FindingWarning = core.FindingWarning
	FindingError   = core.FindingError
)

type Finding = core.Finding
type FindingCollector = core.FindingCollector

const FindingCollectorKey = core.FindingCollectorKey

func SeverityAtOrAbove(have, threshold FindingSeverity) bool {
	return core.SeverityAtOrAbove(have, threshold)
}

type InMemoryFindingCollector = finding.InMemoryFindingCollector
