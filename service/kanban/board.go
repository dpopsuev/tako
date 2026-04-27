package kanban

import "errors"

var ErrStationBusy = errors.New("kanban: station already claimed")

// StationStatus is the read-only projection of a station on the Kanban board.
type StationStatus struct {
	Name      string
	Claimable bool
	ClaimedBy string
}

// Board is the stigmergic read-only projection of the Fab graph.
type Board interface {
	Stations() []StationStatus
	Claim(station string, agentID string) error
	Release(station string, agentID string) error
}
