// view.go — ViewMode enum and View interface for the Djinn TUI.
//
// Two orthogonal axes: ViewMode selects WHICH panels are visible,
// LayoutEngine determines HOW they're arranged. Views consume state
// from Terminal.Viewer, not domain types directly.
//
// GOL-181, TSK-1163
package core

// ViewMode identifies which view is active.
type ViewMode int

const (
	ViewConversation ViewMode = iota // default — chat + output + input
	ViewPlan                         // plan inspection, task tree, fast-travel
	ViewAgents                       // agent roster, per-agent output
	ViewDebug                        // event log, trace, debug panel
	ViewDashboard                    // system health, metrics, drift
	ViewObserver                     // agent introspection — drift, scratch, symbols
	ViewForum                        // threaded conversation with topic sidebar
)

// ViewModeCount is the total number of defined view modes.
const ViewModeCount = 7

func (m ViewMode) String() string {
	switch m {
	case ViewConversation:
		return "conversation"
	case ViewPlan:
		return "plan"
	case ViewAgents:
		return "agents"
	case ViewDebug:
		return "debug"
	case ViewDashboard:
		return "dashboard"
	case ViewObserver:
		return "observer"
	case ViewForum:
		return "forum"
	default:
		return "unknown"
	}
}

// ParseViewMode parses a string into a ViewMode. Returns false if invalid.
func ParseViewMode(s string) (ViewMode, bool) {
	switch s {
	case "conversation", "chat":
		return ViewConversation, true
	case "plan":
		return ViewPlan, true
	case "agents":
		return ViewAgents, true
	case "debug":
		return ViewDebug, true
	case "dashboard", "dash":
		return ViewDashboard, true
	case "observer", "observe":
		return ViewObserver, true
	case "forum":
		return ViewForum, true
	default:
		return 0, false
	}
}

// View provides panels and layout hints for a specific ViewMode.
// Each View composes panels from tui/widgets and exposes them as
// Slots — the LayoutEngine translates Slots into PanelSlots for rendering.
type View interface {
	// ID returns the view identifier (matches ViewMode.String()).
	ID() string

	// Slots returns the panel slots for this view.
	// Called by ViewRouter when this view becomes active.
	Slots() Slots
}

// StubView implements View for testing.
type StubView struct {
	ViewID    string
	ViewSlots Slots
}

var _ View = (*StubView)(nil)

func (v *StubView) ID() string   { return v.ViewID }
func (v *StubView) Slots() Slots { return v.ViewSlots }
