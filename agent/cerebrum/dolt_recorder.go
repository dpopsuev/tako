package cerebrum

import (
	"time"

	"github.com/jmoiron/sqlx"
)

type DoltTurnRecorder struct {
	db *sqlx.DB
}

var _ Recorder = (*DoltTurnRecorder)(nil)

func NewDoltTurnRecorder(db *sqlx.DB) *DoltTurnRecorder {
	return &DoltTurnRecorder{db: db}
}

func (r *DoltTurnRecorder) Append(record Record) error {
	_, err := r.db.Exec(
		`INSERT INTO turn_records
		 (molecule_id, turn, phase, gear, domain, model_name,
		  tokens_in, tokens_out, tool_calls, distance, delta_distance,
		  momentum, unmet_count, reflex_hits, elapsed_ms,
		  navigator_decision, regulator_depth, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.Labels["molecule"],
		record.Labels["turn"],
		record.Labels["phase"],
		record.Labels["conventionality"],
		record.Labels["domain"],
		record.Labels["model"],
		record.Labels["tokens_in"],
		record.Labels["tokens_out"],
		record.Labels["tool_calls"],
		record.Labels["distance"],
		record.Labels["delta"],
		record.Labels["momentum"],
		record.Labels["unmet"],
		record.Labels["reflex_hits"],
		record.Labels["elapsed_ms"],
		record.Labels["navigator_decision"],
		record.Labels["regulator_depth"],
		record.Timestamp,
	)
	return err
}

func (r *DoltTurnRecorder) RecordSession(s SessionSummary) error {
	_, err := r.db.Exec(
		`INSERT INTO session_summaries
		 (molecule_id, total_turns, total_tokens_in, total_tokens_out,
		  total_tool_calls, oae, gear_novel_pct, gear_familiar_pct,
		  gear_intuition_pct, gear_reflex_pct, reflex_hits,
		  reflex_coverage, llm_calls, reflex_fires, avg_turn_ms,
		  sealed, final_distance, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.MoleculeID, s.TotalTurns, s.TotalTokensIn, s.TotalTokensOut,
		s.TotalToolCalls, s.OAE, s.ChaoticPct, s.ComplexPct,
		s.ComplicatedPct, s.ClearPct, s.ReflexHits,
		s.ReflexCoverage, s.LLMCalls, s.ReflexFires, s.AvgTurnMs,
		s.Sealed, s.FinalDistance, time.Now(),
	)
	return err
}
