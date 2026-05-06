// styles.go — StyleSet holds all computed styles from a TokenSet (GOL-42).
//
// BuildStyles is a pure function: TokenSet → StyleSet. ActiveStyles is the
// global mutable state, set by tui.ApplyTokens(). Sub-packages (elements/,
// icons/) read from ActiveStyles to avoid importing tui/ (circular dep).
package design

import "github.com/charmbracelet/lipgloss"

// StyleSet holds all computed styles from a TokenSet.
// Created by BuildStyles(), consumed by tui.ApplyTokens().
type StyleSet struct {
	// Core styles — re-exported by tui/styles.go
	User        lipgloss.Style
	Assistant   lipgloss.Style
	ToolName    lipgloss.Style
	ToolArg     lipgloss.Style
	ToolSuccess lipgloss.Style
	Error       lipgloss.Style
	Dim         lipgloss.Style
	Logo        lipgloss.Style

	// Diff
	DiffAdd    lipgloss.Style
	DiffDel    lipgloss.Style
	DiffHeader lipgloss.Style

	// Health
	HealthGreen  lipgloss.Style
	HealthYellow lipgloss.Style
	HealthRed    lipgloss.Style

	// Budget
	BudgetOK   lipgloss.Style
	BudgetWarn lipgloss.Style
	BudgetOver lipgloss.Style

	// Coherence zones
	ZoneCold    lipgloss.Style
	ZoneWarm    lipgloss.Style
	ZoneFocused lipgloss.Style
	ZoneHot     lipgloss.Style
	ZoneRedline lipgloss.Style

	// Drift
	DriftGood lipgloss.Style
	DriftMid  lipgloss.Style
	DriftBad  lipgloss.Style

	// Dashboard mode indicators
	ModeInsert   lipgloss.Style
	ModeStream   lipgloss.Style
	ModeApproval lipgloss.Style

	// Focus borders
	FocusBorder     lipgloss.Style
	UnfocusedBorder lipgloss.Style

	// Turn envelope
	TurnBorder lipgloss.Style

	// Separator
	SepFocus lipgloss.Style

	// Brand color (for direct use)
	AccentFg lipgloss.AdaptiveColor
}

// BuildStyles computes a full StyleSet from a TokenSet.
// Pure function — no side effects. Call from ApplyTokens().
func BuildStyles(ts TokenSet) StyleSet { //nolint:gocritic // TokenSet is cheap
	return StyleSet{
		// Core
		User:        lipgloss.NewStyle().Foreground(ts.UserFg).Bold(true),
		Assistant:   lipgloss.NewStyle().Foreground(ts.AssistantFg).Bold(true),
		ToolName:    lipgloss.NewStyle().Foreground(ts.ToolNameFg),
		ToolArg:     lipgloss.NewStyle().Foreground(ts.ToolArgFg),
		ToolSuccess: lipgloss.NewStyle().Foreground(ts.SuccessFg),
		Error:       lipgloss.NewStyle().Foreground(ts.ErrorFg),
		Dim:         lipgloss.NewStyle().Faint(true),
		Logo:        lipgloss.NewStyle().Foreground(ts.AccentFg).Bold(true),

		// Diff
		DiffAdd:    lipgloss.NewStyle().Foreground(ts.DiffAddFg),
		DiffDel:    lipgloss.NewStyle().Foreground(ts.DiffDelFg),
		DiffHeader: lipgloss.NewStyle().Foreground(ts.DiffHeaderFg),

		// Health
		HealthGreen:  lipgloss.NewStyle().Foreground(ts.HealthGreenFg),
		HealthYellow: lipgloss.NewStyle().Foreground(ts.HealthYellowFg),
		HealthRed:    lipgloss.NewStyle().Foreground(ts.HealthRedFg),

		// Budget (uses health colors)
		BudgetOK:   lipgloss.NewStyle().Foreground(ts.HealthGreenFg),
		BudgetWarn: lipgloss.NewStyle().Foreground(ts.HealthYellowFg),
		BudgetOver: lipgloss.NewStyle().Foreground(ts.HealthRedFg),

		// Coherence zones
		ZoneCold:    lipgloss.NewStyle().Foreground(ts.ZoneColdFg),
		ZoneWarm:    lipgloss.NewStyle().Foreground(ts.SuccessFg),
		ZoneFocused: lipgloss.NewStyle().Foreground(ts.ZoneFocusedFg),
		ZoneHot:     lipgloss.NewStyle().Foreground(ts.HealthYellowFg),
		ZoneRedline: lipgloss.NewStyle().Foreground(ts.HealthRedFg),

		// Drift
		DriftGood: lipgloss.NewStyle().Foreground(ts.SuccessFg),
		DriftMid:  lipgloss.NewStyle().Foreground(ts.WarningFg),
		DriftBad:  lipgloss.NewStyle().Foreground(ts.ErrorFg),

		// Dashboard mode indicators
		ModeInsert:   lipgloss.NewStyle().Bold(true).Foreground(ts.UserFg),
		ModeStream:   lipgloss.NewStyle().Bold(true).Foreground(ts.AssistantFg),
		ModeApproval: lipgloss.NewStyle().Bold(true).Foreground(ts.WarningFg),

		// Focus borders
		FocusBorder:     lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ts.AccentFg),
		UnfocusedBorder: lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ts.FocusDimFg),

		// Turn envelope
		TurnBorder: lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ts.FocusDimFg),

		// Separator
		SepFocus: lipgloss.NewStyle().Foreground(ts.AssistantFg),

		// Brand
		AccentFg: ts.AccentFg,
	}
}

// ActiveStyles is the live computed StyleSet. Set by tui.ApplyTokens().
// Sub-packages read from this to avoid importing tui/ directly.
var ActiveStyles StyleSet

func init() {
	ActiveStyles = BuildStyles(DefaultTokens())
}
