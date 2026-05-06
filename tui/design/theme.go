// theme.go — semantic color palette definitions and registry.
//
// Theme defines 8 semantic color slots. Each slot is a lipgloss.AdaptiveColor
// that picks light/dark automatically. 4 built-in presets: djinn, claude, gemini, codex.
//
//nolint:dupl // each theme has unique colors, structural similarity is inherent
package design

import (
	"github.com/charmbracelet/lipgloss"
)

// Theme defines the semantic color palette.
type Theme struct {
	User      lipgloss.AdaptiveColor // user input text
	Assistant lipgloss.AdaptiveColor // assistant label
	ToolName  lipgloss.AdaptiveColor // tool call name
	ToolArg   lipgloss.AdaptiveColor // tool call arguments
	Success   lipgloss.AdaptiveColor // success indicators
	Error     lipgloss.AdaptiveColor // error messages
	Accent    lipgloss.AdaptiveColor // brand accent (borders, logo)
	FocusDim  lipgloss.AdaptiveColor // unfocused border color
}

// DefaultTheme is the Djinn default palette — Red Hat Red accent.
var DefaultTheme = Theme{
	User:      lipgloss.AdaptiveColor{Light: "#22c55e", Dark: "#4ade80"},
	Assistant: lipgloss.AdaptiveColor{Light: "#3b82f6", Dark: "#60a5fa"},
	ToolName:  lipgloss.AdaptiveColor{Light: "#eab308", Dark: "#facc15"},
	ToolArg:   lipgloss.AdaptiveColor{Light: "#a855f7", Dark: "#c084fc"},
	Success:   lipgloss.AdaptiveColor{Light: "#22c55e", Dark: "#4ade80"},
	Error:     lipgloss.AdaptiveColor{Light: "#ef4444", Dark: "#f87171"},
	Accent:    lipgloss.AdaptiveColor{Light: "#CC0000", Dark: "#EE0000"},
	FocusDim:  lipgloss.AdaptiveColor{Light: "#808080", Dark: "#505050"},
}

// ClaudeTheme — warm orange/amber tones.
var ClaudeTheme = Theme{
	User:      lipgloss.AdaptiveColor{Light: "#d97706", Dark: "#f59e0b"},
	Assistant: lipgloss.AdaptiveColor{Light: "#7c3aed", Dark: "#a78bfa"},
	ToolName:  lipgloss.AdaptiveColor{Light: "#059669", Dark: "#34d399"},
	ToolArg:   lipgloss.AdaptiveColor{Light: "#6b7280", Dark: "#9ca3af"},
	Success:   lipgloss.AdaptiveColor{Light: "#059669", Dark: "#34d399"},
	Error:     lipgloss.AdaptiveColor{Light: "#dc2626", Dark: "#f87171"},
	Accent:    lipgloss.AdaptiveColor{Light: "#d97706", Dark: "#f59e0b"},
	FocusDim:  lipgloss.AdaptiveColor{Light: "#9ca3af", Dark: "#6b7280"},
}

// GeminiTheme — cool blue tones.
var GeminiTheme = Theme{
	User:      lipgloss.AdaptiveColor{Light: "#2563eb", Dark: "#60a5fa"},
	Assistant: lipgloss.AdaptiveColor{Light: "#7c3aed", Dark: "#a78bfa"},
	ToolName:  lipgloss.AdaptiveColor{Light: "#0891b2", Dark: "#22d3ee"},
	ToolArg:   lipgloss.AdaptiveColor{Light: "#6b7280", Dark: "#9ca3af"},
	Success:   lipgloss.AdaptiveColor{Light: "#16a34a", Dark: "#4ade80"},
	Error:     lipgloss.AdaptiveColor{Light: "#dc2626", Dark: "#f87171"},
	Accent:    lipgloss.AdaptiveColor{Light: "#2563eb", Dark: "#60a5fa"},
	FocusDim:  lipgloss.AdaptiveColor{Light: "#94a3b8", Dark: "#64748b"},
}

// CodexTheme — green monochrome.
var CodexTheme = Theme{
	User:      lipgloss.AdaptiveColor{Light: "#16a34a", Dark: "#4ade80"},
	Assistant: lipgloss.AdaptiveColor{Light: "#16a34a", Dark: "#86efac"},
	ToolName:  lipgloss.AdaptiveColor{Light: "#059669", Dark: "#34d399"},
	ToolArg:   lipgloss.AdaptiveColor{Light: "#6b7280", Dark: "#9ca3af"},
	Success:   lipgloss.AdaptiveColor{Light: "#16a34a", Dark: "#4ade80"},
	Error:     lipgloss.AdaptiveColor{Light: "#dc2626", Dark: "#f87171"},
	Accent:    lipgloss.AdaptiveColor{Light: "#16a34a", Dark: "#4ade80"},
	FocusDim:  lipgloss.AdaptiveColor{Light: "#6b7280", Dark: "#4b5563"},
}

// Registry holds named themes.
var registry = map[string]Theme{
	"djinn":  DefaultTheme,
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
