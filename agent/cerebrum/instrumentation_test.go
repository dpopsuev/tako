package cerebrum

import (
	"testing"

	"github.com/dpopsuev/tako/agent/reactivity"
)

func TestTurnRecord_Labels(t *testing.T) {
	tr := TurnRecord{
		MoleculeID: "mol-1",
		Turn:       3,
		Phase:      "execution",
		Gear:       GearFamiliar,
		Domain:     "complicated",
		TokensIn:   500,
		TokensOut:  100,
		ToolCalls:  2,
		Distance:   0.25,
		ElapsedMs:  1500,
	}

	labels := tr.Labels()
	if labels["gear"] != "familiar" {
		t.Errorf("gear: got %s, want familiar", labels["gear"])
	}
	if labels["tokens_in"] != "500" {
		t.Errorf("tokens_in: got %s, want 500", labels["tokens_in"])
	}
	if labels["tool_calls"] != "2" {
		t.Errorf("tool_calls: got %s, want 2", labels["tool_calls"])
	}
}

func TestSessionSummary_FromTurnRecords(t *testing.T) {
	turns := []TurnRecord{
		{Gear: GearNovel, TokensIn: 1000, TokensOut: 200, ToolCalls: 1, ElapsedMs: 5000},
		{Gear: GearNovel, TokensIn: 800, TokensOut: 150, ToolCalls: 2, ElapsedMs: 3000},
		{Gear: GearFamiliar, TokensIn: 400, TokensOut: 80, ToolCalls: 1, ElapsedMs: 1000},
		{Gear: GearFamiliar, TokensIn: 300, TokensOut: 60, ToolCalls: 0, ElapsedMs: 800},
	}

	m := reactivity.NewMoleculeWithCatalyst("test", reactivity.Catalyst{
		Need:    "test",
		Desired: map[string]any{"done": true},
	})
	m.ReportSensor("done", true)

	summary := computeSessionSummary("test", turns, m)

	if summary.TotalTurns != 4 {
		t.Errorf("turns: got %d, want 4", summary.TotalTurns)
	}
	if summary.TotalTokensIn != 2500 {
		t.Errorf("tokens_in: got %d, want 2500", summary.TotalTokensIn)
	}
	if summary.TotalTokensOut != 490 {
		t.Errorf("tokens_out: got %d, want 490", summary.TotalTokensOut)
	}
	if summary.GearNovelPct != 50 {
		t.Errorf("novel_pct: got %.1f, want 50.0", summary.GearNovelPct)
	}
	if summary.GearFamiliarPct != 50 {
		t.Errorf("familiar_pct: got %.1f, want 50.0", summary.GearFamiliarPct)
	}
	if summary.AvgTurnMs != 2450 {
		t.Errorf("avg_turn_ms: got %d, want 2450", summary.AvgTurnMs)
	}
	if summary.FinalDistance != 0 {
		t.Errorf("final_distance: got %.3f, want 0", summary.FinalDistance)
	}
}

func TestSessionSummary_Labels(t *testing.T) {
	s := SessionSummary{
		MoleculeID:      "mol-1",
		TotalTurns:      10,
		GearNovelPct:    30,
		GearFamiliarPct: 50,
		GearReflexPct:   20,
		OAE:             0.65,
	}

	labels := s.Labels()
	if labels["oae"] != "0.650" {
		t.Errorf("oae: got %s, want 0.650", labels["oae"])
	}
	if labels["gear_reflex_pct"] != "20.0" {
		t.Errorf("reflex_pct: got %s, want 20.0", labels["gear_reflex_pct"])
	}
}
