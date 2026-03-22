package framework

// Category: DSL & Build — aliases to circuit/ package.

import "github.com/dpopsuev/origami/circuit"

type ReportTemplate = circuit.ReportTemplate
type ReportSectionDef = circuit.ReportSectionDef

var (
	LoadReportTemplate   = circuit.LoadReportTemplate
	MergeReportTemplates = circuit.MergeReportTemplates
)
