package kanban

import (
	"sync"

	"github.com/dpopsuev/tako/fab"
)

// StubBoard is a static Kanban projection from a Fab Assembly.
type StubBoard struct {
	mu       sync.Mutex
	stations map[string]*StationStatus
}

var _ Board = (*StubBoard)(nil)

// NewStubBoard creates a Kanban board from a Fab Assembly.
func NewStubBoard(assembly fab.Assembly) *StubBoard {
	stations := make(map[string]*StationStatus, len(assembly.Stations))
	for name, s := range assembly.Stations {
		stations[name] = &StationStatus{
			Name:      s.Name,
			Claimable: !s.Terminus,
		}
	}
	return &StubBoard{stations: stations}
}

func (b *StubBoard) Stations() []StationStatus {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]StationStatus, 0, len(b.stations))
	for _, s := range b.stations {
		out = append(out, *s)
	}
	return out
}

func (b *StubBoard) Claim(station, agentID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	s, ok := b.stations[station]
	if !ok {
		return fab.ErrStationNotFound
	}
	if !s.Claimable {
		return ErrStationBusy
	}
	s.Claimable = false
	s.ClaimedBy = agentID
	return nil
}

func (b *StubBoard) Release(station, _ string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	s, ok := b.stations[station]
	if !ok {
		return fab.ErrStationNotFound
	}
	s.Claimable = true
	s.ClaimedBy = ""
	return nil
}
