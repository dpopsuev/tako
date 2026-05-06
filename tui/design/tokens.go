// tokens.go — semantic color tokens bridging palette → purpose.
//
// TokenSet maps visual purpose to adaptive colors. All styles should read
// from a TokenSet, never from raw hex. Themes remap tokens via TokensFromTheme().
//
// This package defines the types and pure conversion functions.
// The mutable state (ActiveTokens, ApplyTokens) lives in the tui/ package.
package design

import "github.com/charmbracelet/lipgloss"

// TokenSet maps visual purpose to color. Every styled element reads from here.
type TokenSet struct {
	// Identity — who is speaking
	UserFg      lipgloss.AdaptiveColor
	AssistantFg lipgloss.AdaptiveColor

	// Tool status
	ToolNameFg lipgloss.AdaptiveColor
	ToolArgFg  lipgloss.AdaptiveColor

	// State — what is happening
	SuccessFg lipgloss.AdaptiveColor
	ErrorFg   lipgloss.AdaptiveColor
	WarningFg lipgloss.AdaptiveColor

	// Brand
	AccentFg   lipgloss.AdaptiveColor
	FocusDimFg lipgloss.AdaptiveColor

	// Diff
	DiffAddFg    lipgloss.AdaptiveColor
	DiffDelFg    lipgloss.AdaptiveColor
	DiffHeaderFg lipgloss.AdaptiveColor

	// Health / coherence zones (thermal gradient)
	HealthGreenFg  lipgloss.AdaptiveColor
	HealthYellowFg lipgloss.AdaptiveColor
	HealthRedFg    lipgloss.AdaptiveColor

	// Coherence — extended zones beyond basic health
	ZoneColdFg    lipgloss.AdaptiveColor // blue — fresh context
	ZoneFocusedFg lipgloss.AdaptiveColor // dark green — deep focus
}

// TokensFromTheme maps a Theme to a full TokenSet.
// Theme covers 8 semantic colors; tokens extend with diff, health, and zone colors.
func TokensFromTheme(t Theme) TokenSet { //nolint:gocritic // Theme is a value type, copy is cheap
	return TokenSet{
		// Direct from Theme
		UserFg:      t.User,
		AssistantFg: t.Assistant,
		ToolNameFg:  t.ToolName,
		ToolArgFg:   t.ToolArg,
		SuccessFg:   t.Success,
		ErrorFg:     t.Error,
		AccentFg:    t.Accent,
		FocusDimFg:  t.FocusDim,

		// Extended — derived from Theme semantics
		WarningFg:    t.ToolName,
		DiffAddFg:    t.Success,
		DiffDelFg:    t.Error,
		DiffHeaderFg: lipgloss.AdaptiveColor{Light: Teal40, Dark: Teal20},

		// Health uses the traffic light triad
		HealthGreenFg:  t.Success,
		HealthYellowFg: t.ToolName,
		HealthRedFg:    t.Error,

		// Coherence thermal gradient
		ZoneColdFg:    t.Assistant,
		ZoneFocusedFg: lipgloss.AdaptiveColor{Light: Green40, Dark: Green30},
	}
}

// DefaultTokens returns tokens derived from DefaultTheme.
func DefaultTokens() TokenSet {
	return TokensFromTheme(DefaultTheme)
}
