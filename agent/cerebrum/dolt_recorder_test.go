package cerebrum

import (
	"testing"
	"time"
)

func TestDoltTurnRecorder_AppendAndQuery(t *testing.T) {
	db := openTestDoltDB(t)
	recorder := NewDoltTurnRecorder(db.DB)

	record := Record{
		Action:    "cerebrum.turn",
		Timestamp: time.Now(),
		Labels: map[string]string{
			"molecule":           "mol-test-123",
			"turn":               "0",
			"phase":              "assessment",
			"gear":               "novel",
			"domain":             "complicated",
			"model":              "claude-sonnet",
			"tokens_in":          "100",
			"tokens_out":         "50",
			"tool_calls":         "2",
			"distance":           "0.800",
			"delta":              "-0.100",
			"momentum":           "0.500",
			"unmet":              "1",
			"reflex_hits":        "0",
			"elapsed_ms":         "1234",
			"navigator_decision": "shortcut",
			"regulator_depth":    "full",
		},
	}

	if err := recorder.Append(record); err != nil {
		t.Fatalf("Append: %v", err)
	}

	var count int
	if err := db.Get(&count, "SELECT COUNT(*) FROM turn_records WHERE molecule_id = ?", "mol-test-123"); err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}

	var phase string
	if err := db.Get(&phase, "SELECT phase FROM turn_records WHERE molecule_id = ?", "mol-test-123"); err != nil {
		t.Fatalf("query phase: %v", err)
	}
	if phase != "assessment" {
		t.Errorf("phase = %q, want %q", phase, "assessment")
	}
}

func TestDoltTurnRecorder_SessionSummary(t *testing.T) {
	db := openTestDoltDB(t)
	recorder := NewDoltTurnRecorder(db.DB)

	summary := SessionSummary{
		MoleculeID:      "mol-summary-456",
		TotalTurns:      5,
		TotalTokensIn:   500,
		TotalTokensOut:  250,
		TotalToolCalls:  8,
		OAE:             0.6,
		GearNovelPct:    60,
		GearFamiliarPct: 20,
		GearIntuitionPct: 10,
		GearReflexPct:   10,
		ReflexHits:      1,
		ReflexCoverage:  0.1,
		LLMCalls:        4,
		ReflexFires:     1,
		AvgTurnMs:       800,
		Sealed:          true,
		FinalDistance:    0.3,
	}

	if err := recorder.RecordSession(summary); err != nil {
		t.Fatalf("RecordSession: %v", err)
	}

	var oae float64
	if err := db.Get(&oae, "SELECT oae FROM session_summaries WHERE molecule_id = ?", "mol-summary-456"); err != nil {
		t.Fatalf("query oae: %v", err)
	}
	if oae != 0.6 {
		t.Errorf("oae = %f, want 0.6", oae)
	}

	var sealed bool
	if err := db.Get(&sealed, "SELECT sealed FROM session_summaries WHERE molecule_id = ?", "mol-summary-456"); err != nil {
		t.Fatalf("query sealed: %v", err)
	}
	if !sealed {
		t.Error("sealed should be true")
	}
}
