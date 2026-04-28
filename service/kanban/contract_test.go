package kanban

import (
	"errors"
	"testing"

	"github.com/dpopsuev/tako/fab"
)

func TestStubBoardClaimAndRelease(t *testing.T) {
	assembly := fab.StubAssembly()
	board := NewStubBoard(assembly)

	if err := board.Claim("intake", "agent-1"); err != nil {
		t.Fatalf("Claim failed: %v", err)
	}

	stations := board.Stations()
	for _, s := range stations {
		if s.Name == "intake" {
			if s.Claimable {
				t.Error("intake should not be claimable after claim")
			}
			if s.ClaimedBy != "agent-1" {
				t.Errorf("expected ClaimedBy agent-1, got %q", s.ClaimedBy)
			}
		}
	}

	if err := board.Release("intake", "agent-1"); err != nil {
		t.Fatalf("Release failed: %v", err)
	}
}

func TestStubBoardClaimNotFound(t *testing.T) {
	assembly := fab.StubAssembly()
	board := NewStubBoard(assembly)
	err := board.Claim("nonexistent", "agent-1")
	if !errors.Is(err, fab.ErrStationNotFound) {
		t.Errorf("expected ErrStationNotFound, got %v", err)
	}
}
