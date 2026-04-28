// Package budget re-exports token billing types from troupe/billing.
package budget

import "github.com/dpopsuev/tangle/billing"

// Type aliases — definitions live in troupe/billing.
type (
	Tracker          = billing.Tracker
	InMemoryTracker  = billing.InMemoryTracker
	TokenRecord      = billing.TokenRecord
	TokenSummary     = billing.TokenSummary
	CaseTokenSummary = billing.CaseTokenSummary
	StepTokenSummary = billing.StepTokenSummary
	CostConfig       = billing.CostConfig
	TokenRecordHook  = billing.TokenRecordHook

	CostBill         = billing.CostBill
	CostBillCaseLine = billing.CostBillCaseLine
	CostBillStepLine = billing.CostBillStepLine
	CostBillOption   = billing.CostBillOption
)

// Constructors.
var (
	NewTracker         = billing.NewTracker
	NewTrackerWithCost = billing.NewTrackerWithCost
	DefaultCostConfig  = billing.DefaultCostConfig
)

// Cost bill functions.
var (
	BuildCostBill  = billing.BuildCostBill
	FormatCostBill = billing.FormatCostBill
)

// Cost bill options.
var (
	WithTitle       = billing.WithTitle
	WithSubtitle    = billing.WithSubtitle
	WithCostConfig  = billing.WithCostConfig
	WithStepOrder   = billing.WithStepOrder
	WithStepNames   = billing.WithStepNames
	WithCaseLabels  = billing.WithCaseLabels
	WithCaseDetails = billing.WithCaseDetails
)

// EstimateTokens converts byte count to estimated token count (bytes / 4).
var EstimateTokens = billing.EstimateTokens
