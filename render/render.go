package render

// Canvas is the shared UI surface (Blackboard architecture pattern).
// Sub-systems post Panels. The Avatar reads and composes a view.
type Canvas interface {
	Post(panel Panel)
	Retract(id string)
	Panels() []Panel
	Subscribe() <-chan Event
}

// Panel is one knowledge contribution on the Canvas.
type Panel struct {
	ID       string
	Source   string
	Priority int
	Data     []byte
	Children []Panel
}

// Event signals a change on the Canvas.
type Event struct {
	Type    EventType
	PanelID string
}

// EventType describes what changed.
type EventType int

const (
	Posted EventType = iota
	Retracted
	Updated
)

// Renderable projects itself onto a Panel.
type Renderable interface {
	Render() []byte
}
