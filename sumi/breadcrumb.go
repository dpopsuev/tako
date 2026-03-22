package sumi

import (
	"strings"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/view"
	"github.com/charmbracelet/lipgloss"
)

// DrillDownRequest is a message emitted when a user requests to expand
// a composite node (marble) to view its sub-circuit.
type DrillDownRequest struct {
	ParentNode string
	CircuitDef *circuit.CircuitDef
	Layout     view.CircuitLayout
}

// BreadcrumbEntry represents one level in the navigation stack.
type BreadcrumbEntry struct {
	Label      string
	CircuitDef *circuit.CircuitDef
	Layout     view.CircuitLayout
}

// BreadcrumbBar renders a navigation breadcrumb for sub-circuit drill-down.
type BreadcrumbBar struct {
	stack   []BreadcrumbEntry
	noColor bool
}

// NewBreadcrumbBar creates a breadcrumb bar with the root circuit.
func NewBreadcrumbBar(rootLabel string, def *circuit.CircuitDef, layout view.CircuitLayout, noColor bool) *BreadcrumbBar {
	return &BreadcrumbBar{
		stack: []BreadcrumbEntry{
			{Label: rootLabel, CircuitDef: def, Layout: layout},
		},
		noColor: noColor,
	}
}

// Push adds a sub-circuit level to the navigation stack.
func (b *BreadcrumbBar) Push(label string, def *circuit.CircuitDef, layout view.CircuitLayout) {
	b.stack = append(b.stack, BreadcrumbEntry{
		Label:      label,
		CircuitDef: def,
		Layout:     layout,
	})
}

// Pop removes the top entry and returns it. Returns nil if at root.
func (b *BreadcrumbBar) Pop() *BreadcrumbEntry {
	if len(b.stack) <= 1 {
		return nil
	}
	top := b.stack[len(b.stack)-1]
	b.stack = b.stack[:len(b.stack)-1]
	return &top
}

// PopTo pops entries until the stack is at the given depth (0-indexed).
// Returns the new top entry, or nil if depth is invalid.
func (b *BreadcrumbBar) PopTo(depth int) *BreadcrumbEntry {
	if depth < 0 || depth >= len(b.stack) {
		return nil
	}
	b.stack = b.stack[:depth+1]
	return &b.stack[depth]
}

// Current returns the top of the navigation stack.
func (b *BreadcrumbBar) Current() BreadcrumbEntry {
	return b.stack[len(b.stack)-1]
}

// Depth returns the stack depth (1 = root only).
func (b *BreadcrumbBar) Depth() int {
	return len(b.stack)
}

// Visible reports whether the breadcrumb bar should be shown.
// Hidden when at root (depth 1).
func (b *BreadcrumbBar) Visible() bool {
	return len(b.stack) > 1
}

// View renders the breadcrumb bar: "Root > SubCircuit > ..."
func (b *BreadcrumbBar) View(width int) string {
	if !b.Visible() {
		return ""
	}

	sep := " > "
	var parts []string
	for i, entry := range b.stack {
		label := entry.Label
		if i < len(b.stack)-1 {
			if !b.noColor {
				label = styleCrumbLink.Render(label)
			}
		} else {
			if !b.noColor {
				label = styleCrumbCurrent.Render(label)
			}
		}
		parts = append(parts, label)
	}

	result := strings.Join(parts, sep)
	if len([]rune(result)) > width {
		result = string([]rune(result)[:width-1]) + "…"
	}
	return result
}

// CrumbAtX returns the stack depth index for the crumb at the given X position.
// Returns -1 if no crumb was hit.
func (b *BreadcrumbBar) CrumbAtX(x int) int {
	pos := 0
	sep := " > "
	for i, entry := range b.stack {
		end := pos + len([]rune(entry.Label))
		if x >= pos && x < end {
			return i
		}
		pos = end + len(sep)
	}
	return -1
}

var (
	styleCrumbLink    = lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Underline(true)
	styleCrumbCurrent = lipgloss.NewStyle().Bold(true)
)
