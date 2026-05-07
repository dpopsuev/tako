// sight.go — Bidirectional TUI state in agent prompt (GOL-62, SPC-85).
//
// Sighted is an optional interface panels implement to report
// what the operator is looking at. On SubmitMsg, the active panel's cell
// sight is injected into the agent prompt so "this" resolves from TUI state.
package cerebrum

import (
	"fmt"
	"strings"
)

// SightField is a typed key-value pair in a CellSight.
// Sensitive fields are excluded from agent prompts unless the operator
// explicitly overrides with :sight reveal.
type SightField struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	Sensitive bool   `json:"sensitive,omitempty"` // excluded from agent prompt unless operator overrides
}

// CellSight describes what the operator is currently focused on in a panel.
type CellSight struct {
	PanelID   string       `json:"panel"`
	CellID    string       `json:"cell,omitempty"`
	CellTitle string       `json:"title,omitempty"`
	Kind      string       `json:"kind,omitempty"`
	Fields    []SightField `json:"fields,omitempty"`
}

// IsEmpty returns true if the context carries no meaningful focus information.
func (fc CellSight) IsEmpty() bool {
	return fc.PanelID == "" && fc.CellID == ""
}

// FormatPrompt renders the cell sight as a structured prompt block.
// Sensitive fields are filtered out; a hint is shown when hidden fields exist.
// Output is under 200 tokens — context, not content.
func (fc CellSight) FormatPrompt() string {
	if fc.IsEmpty() {
		return ""
	}

	var b strings.Builder
	b.WriteString("<cell-sight>\n")
	fmt.Fprintf(&b, "  Panel: %s\n", fc.PanelID)
	if fc.CellID != "" {
		fmt.Fprintf(&b, "  Selected: %s", fc.CellID)
		if fc.CellTitle != "" {
			fmt.Fprintf(&b, " %q", fc.CellTitle)
		}
		b.WriteByte('\n')
	}
	if fc.Kind != "" {
		fmt.Fprintf(&b, "  Kind: %s\n", fc.Kind)
	}
	hidden := 0
	for i := range fc.Fields {
		if fc.Fields[i].Sensitive {
			hidden++
			continue
		}
		fmt.Fprintf(&b, "  %s: %s\n", fc.Fields[i].Key, fc.Fields[i].Value)
	}
	if hidden > 0 {
		fmt.Fprintf(&b, "  [%d fields hidden — :sight reveal panel.field]\n", hidden)
	}
	b.WriteString("</cell-sight>")
	return b.String()
}

// Sighted is an optional interface for panels that can report
// what the operator is focused on. Panels without selectable elements
// (e.g., InputPanel) don't implement this — backward compatible via type assertion.
//
// SightGate is a runtime toggle: false means the panel is invisible to
// the agent and its CellSight will not be injected into the prompt.
type Sighted interface {
	CellSight() CellSight
	SightGate() bool // false = panel invisible to agent
}
