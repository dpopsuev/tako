// router.go — ViewRouter: maps ViewMode → View → panel set.
//
// The router owns which View is active and provides its panels
// to the LayoutEngine. Mode switching is atomic — all panels
// swap at once, no partial state.
//
// GOL-181, TSK-1167
package core

// ViewRouter manages view mode transitions.
type ViewRouter struct {
	views   map[ViewMode]View
	current ViewMode
}

// NewViewRouter creates a router with Conversation as the default mode.
func NewViewRouter() *ViewRouter {
	return &ViewRouter{
		views:   make(map[ViewMode]View),
		current: ViewConversation,
	}
}

// Register adds a view for a mode. Overwrites if already registered.
func (r *ViewRouter) Register(mode ViewMode, view View) {
	r.views[mode] = view
}

// SetMode switches to a new view mode. Returns false if the mode has no view registered.
func (r *ViewRouter) SetMode(mode ViewMode) bool {
	if _, ok := r.views[mode]; !ok {
		return false
	}
	r.current = mode
	return true
}

// Mode returns the currently active ViewMode.
func (r *ViewRouter) Mode() ViewMode {
	return r.current
}

// Slots returns the slots for the active view.
// Returns nil if no view is registered for the current mode.
func (r *ViewRouter) Slots() Slots {
	v, ok := r.views[r.current]
	if !ok {
		return nil
	}
	return v.Slots()
}

// Panels returns just the panels (without layout hints) for the active view.
// Convenience helper for callers that only need the panel references.
func (r *ViewRouter) Panels() Panels {
	slots := r.Slots()
	if slots == nil {
		return nil
	}
	return slots.Panels()
}

// ActiveView returns the current View, or nil if none registered.
func (r *ViewRouter) ActiveView() View {
	return r.views[r.current]
}

// RegisteredModes returns all modes that have views registered.
func (r *ViewRouter) RegisteredModes() []ViewMode {
	modes := make([]ViewMode, 0, len(r.views))
	for m := range r.views {
		modes = append(modes, m)
	}
	return modes
}
