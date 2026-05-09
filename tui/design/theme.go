// theme.go — semantic color palette definitions and registry.
//
// Theme defines semantic color slots mapped to Red Hat brand colors.
// Each slot is a lipgloss.AdaptiveColor that picks light/dark automatically.
//
//nolint:dupl // each theme has unique colors, structural similarity is inherent
package design

import (
	"github.com/charmbracelet/lipgloss"
)

// Theme defines the semantic color palette.
type Theme struct {
	// Content colors
	User      lipgloss.AdaptiveColor // user input text
	Assistant lipgloss.AdaptiveColor // assistant output
	Muted     lipgloss.AdaptiveColor // secondary/dim text
	// Status colors
	Success lipgloss.AdaptiveColor // success indicators
	Error   lipgloss.AdaptiveColor // error messages
	Warning lipgloss.AdaptiveColor // warnings
	// UI chrome
	Accent   lipgloss.AdaptiveColor // brand accent (borders, logo, title)
	Border   lipgloss.AdaptiveColor // active border
	FocusDim lipgloss.AdaptiveColor // unfocused/pillar border
	// Phase colors — spinner and status
	Thinking  lipgloss.AdaptiveColor // contemplation phase
	Executing lipgloss.AdaptiveColor // action/work phase
	Reading   lipgloss.AdaptiveColor // research/intake phase
	// Surface colors — depth layers
	Surface0 lipgloss.AdaptiveColor // deepest background
	Surface1 lipgloss.AdaptiveColor // pillar/raised background
	Surface2 lipgloss.AdaptiveColor // card/panel background
}

// RedHatTheme — official Red Hat brand palette.
//
// Psychology:
//   Red (#EE0000): attention — accent only, never flood
//   Teal (#37a3a3): focus & calm — output, active state
//   Orange (#f5921b): warm highlight — tool calls, progress
//   Purple (#876fd4): contemplation — thinking phase
//   Gray gradient: depth — surface layers via lightness steps
var RedHatTheme = Theme{
	User:      lipgloss.AdaptiveColor{Light: "#004d4d", Dark: "#63bdbd"},  // teal-70/40 — calm input
	Assistant: lipgloss.AdaptiveColor{Light: "#147878", Dark: "#9ad8d8"},  // teal-60/30 — trusted output
	Muted:     lipgloss.AdaptiveColor{Light: "#707070", Dark: "#8c8c8c"},  // gray-50/45 — recedes
	Success:   lipgloss.AdaptiveColor{Light: "#004d4d", Dark: "#63bdbd"},  // teal — growth
	Error:     lipgloss.AdaptiveColor{Light: "#a60000", Dark: "#ee0000"},  // red-60/50 — demands attention
	Warning:   lipgloss.AdaptiveColor{Light: "#96640f", Dark: "#ffcc17"},  // yellow-60/30 — caution
	Accent:    lipgloss.AdaptiveColor{Light: "#a60000", Dark: "#ee0000"},  // red — brand core
	Border:    lipgloss.AdaptiveColor{Light: "#383838", Dark: "#707070"},  // gray-70/50 — structure
	FocusDim:  lipgloss.AdaptiveColor{Light: "#c7c7c7", Dark: "#4d4d4d"}, // gray-30/60 — breathes
	Thinking:  lipgloss.AdaptiveColor{Light: "#3d2785", Dark: "#876fd4"},  // purple-60/40 — contemplation
	Executing: lipgloss.AdaptiveColor{Light: "#9e4a06", Dark: "#f5921b"},  // orange-60/40 — warm action
	Reading:   lipgloss.AdaptiveColor{Light: "#147878", Dark: "#37a3a3"},  // teal-60/50 — absorption
	Surface0:  lipgloss.AdaptiveColor{Light: "#ffffff", Dark: "#151515"},  // deepest layer
	Surface1:  lipgloss.AdaptiveColor{Light: "#f2f2f2", Dark: "#1f1f1f"},  // pillar depth
	Surface2:  lipgloss.AdaptiveColor{Light: "#e0e0e0", Dark: "#292929"},  // panel depth
}

// DefaultTheme is the Djinn default palette — Red Hat Red accent.
var DefaultTheme = RedHatTheme

// ClaudeTheme — warm orange/amber tones.
var ClaudeTheme = Theme{
	User:      lipgloss.AdaptiveColor{Light: "#d97706", Dark: "#f59e0b"},
	Assistant: lipgloss.AdaptiveColor{Light: "#7c3aed", Dark: "#a78bfa"},
	Muted:     lipgloss.AdaptiveColor{Light: "#6b7280", Dark: "#9ca3af"},
	Success:   lipgloss.AdaptiveColor{Light: "#059669", Dark: "#34d399"},
	Error:     lipgloss.AdaptiveColor{Light: "#dc2626", Dark: "#f87171"},
	Warning:   lipgloss.AdaptiveColor{Light: "#d97706", Dark: "#fbbf24"},
	Accent:    lipgloss.AdaptiveColor{Light: "#d97706", Dark: "#f59e0b"},
	Border:    lipgloss.AdaptiveColor{Light: "#374151", Dark: "#6b7280"},
	FocusDim:  lipgloss.AdaptiveColor{Light: "#9ca3af", Dark: "#4b5563"},
	Thinking:  lipgloss.AdaptiveColor{Light: "#7c3aed", Dark: "#a78bfa"},
	Executing: lipgloss.AdaptiveColor{Light: "#d97706", Dark: "#f59e0b"},
	Reading:   lipgloss.AdaptiveColor{Light: "#059669", Dark: "#34d399"},
	Surface0:  lipgloss.AdaptiveColor{Light: "#ffffff", Dark: "#111827"},
	Surface1:  lipgloss.AdaptiveColor{Light: "#f9fafb", Dark: "#1f2937"},
	Surface2:  lipgloss.AdaptiveColor{Light: "#f3f4f6", Dark: "#374151"},
}

// GeminiTheme — cool blue tones.
var GeminiTheme = Theme{
	User:      lipgloss.AdaptiveColor{Light: "#2563eb", Dark: "#60a5fa"},
	Assistant: lipgloss.AdaptiveColor{Light: "#7c3aed", Dark: "#a78bfa"},
	Muted:     lipgloss.AdaptiveColor{Light: "#6b7280", Dark: "#9ca3af"},
	Success:   lipgloss.AdaptiveColor{Light: "#16a34a", Dark: "#4ade80"},
	Error:     lipgloss.AdaptiveColor{Light: "#dc2626", Dark: "#f87171"},
	Warning:   lipgloss.AdaptiveColor{Light: "#ca8a04", Dark: "#facc15"},
	Accent:    lipgloss.AdaptiveColor{Light: "#2563eb", Dark: "#60a5fa"},
	Border:    lipgloss.AdaptiveColor{Light: "#334155", Dark: "#64748b"},
	FocusDim:  lipgloss.AdaptiveColor{Light: "#94a3b8", Dark: "#475569"},
	Thinking:  lipgloss.AdaptiveColor{Light: "#7c3aed", Dark: "#a78bfa"},
	Executing: lipgloss.AdaptiveColor{Light: "#2563eb", Dark: "#60a5fa"},
	Reading:   lipgloss.AdaptiveColor{Light: "#0891b2", Dark: "#22d3ee"},
	Surface0:  lipgloss.AdaptiveColor{Light: "#ffffff", Dark: "#0f172a"},
	Surface1:  lipgloss.AdaptiveColor{Light: "#f8fafc", Dark: "#1e293b"},
	Surface2:  lipgloss.AdaptiveColor{Light: "#f1f5f9", Dark: "#334155"},
}

// CodexTheme — green monochrome.
var CodexTheme = Theme{
	User:      lipgloss.AdaptiveColor{Light: "#16a34a", Dark: "#4ade80"},
	Assistant: lipgloss.AdaptiveColor{Light: "#16a34a", Dark: "#86efac"},
	Muted:     lipgloss.AdaptiveColor{Light: "#6b7280", Dark: "#9ca3af"},
	Success:   lipgloss.AdaptiveColor{Light: "#16a34a", Dark: "#4ade80"},
	Error:     lipgloss.AdaptiveColor{Light: "#dc2626", Dark: "#f87171"},
	Warning:   lipgloss.AdaptiveColor{Light: "#ca8a04", Dark: "#facc15"},
	Accent:    lipgloss.AdaptiveColor{Light: "#16a34a", Dark: "#4ade80"},
	Border:    lipgloss.AdaptiveColor{Light: "#374151", Dark: "#4b5563"},
	FocusDim:  lipgloss.AdaptiveColor{Light: "#6b7280", Dark: "#374151"},
	Thinking:  lipgloss.AdaptiveColor{Light: "#059669", Dark: "#34d399"},
	Executing: lipgloss.AdaptiveColor{Light: "#16a34a", Dark: "#4ade80"},
	Reading:   lipgloss.AdaptiveColor{Light: "#059669", Dark: "#6ee7b7"},
	Surface0:  lipgloss.AdaptiveColor{Light: "#ffffff", Dark: "#0a0a0a"},
	Surface1:  lipgloss.AdaptiveColor{Light: "#f5f5f5", Dark: "#171717"},
	Surface2:  lipgloss.AdaptiveColor{Light: "#e5e5e5", Dark: "#262626"},
}

// Registry holds named themes.
var registry = map[string]Theme{
	"redhat": RedHatTheme,
	"claude": ClaudeTheme,
	"gemini": GeminiTheme,
	"codex":  CodexTheme,
}

// RegisterTheme adds or replaces a named theme.
func RegisterTheme(name string, t Theme) { //nolint:gocritic // Theme stored by value
	registry[name] = t
}

// ThemeByName returns a theme by name. Returns DefaultTheme if not found.
func ThemeByName(name string) Theme {
	if t, ok := registry[name]; ok {
		return t
	}
	return DefaultTheme
}

// ThemeNames returns all registered theme names.
func ThemeNames() []string {
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	return names
}
