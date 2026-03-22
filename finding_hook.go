package framework

// Category: Processing & Support — aliases to finding/ package.

import "github.com/dpopsuev/origami/finding"

type VetoHook = finding.VetoHook

func NewVetoHook(collector FindingCollector) *VetoHook {
	return finding.NewVetoHook(collector)
}
