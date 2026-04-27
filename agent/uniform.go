package agent

// Uniform is a role — the RBAC token an agent wears inside a Fab.
type Uniform int

const (
	Worker   Uniform = iota // execution: has AXI, station-bound
	Foreman                 // tactical: no AXI, observes all stations
	Director                // strategic: no AXI, sees fabs not stations
	Avatar                  // human proxy: no AXI, canvas access
)

func (u Uniform) String() string {
	switch u {
	case Worker:
		return "worker"
	case Foreman:
		return "foreman"
	case Director:
		return "director"
	case Avatar:
		return "avatar"
	default:
		return "unknown"
	}
}

// HasAXI returns true if this uniform grants Agent Execution Interface access.
func (u Uniform) HasAXI() bool {
	return u == Worker
}
