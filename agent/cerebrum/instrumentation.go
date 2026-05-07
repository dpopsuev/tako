package cerebrum

import (
	"fmt"

	"github.com/dpopsuev/tako/agent/reactivity"
)

type Gear string

const (
	GearNovel     Gear = "novel"
	GearFamiliar  Gear = "familiar"
	GearIntuition Gear = "intuition"
	GearReflex    Gear = "reflex"
)

type TurnRecord struct {
	MoleculeID        string
	Turn              int
	Phase             string
	Gear              Gear
	Domain            string
	ModelName         string
	TokensIn          int
	TokensOut         int
	ToolCalls         int
	Distance          float64
	DeltaDistance      float64
	Momentum          float64
	UnmetCount        int
	QueueDrained      int
	ReflexHits        int
	ElapsedMs         int64
	NavigatorDecision string
	RegulatorDepth    string
}

func (r TurnRecord) Labels() map[string]string {
	return map[string]string{
		"molecule":           r.MoleculeID,
		"turn":               fmt.Sprintf("%d", r.Turn),
		"phase":              r.Phase,
		"gear":               string(r.Gear),
		"domain":             r.Domain,
		"model":              r.ModelName,
		"tokens_in":          fmt.Sprintf("%d", r.TokensIn),
		"tokens_out":         fmt.Sprintf("%d", r.TokensOut),
		"tool_calls":         fmt.Sprintf("%d", r.ToolCalls),
		"distance":           fmt.Sprintf("%.3f", r.Distance),
		"delta":              fmt.Sprintf("%.3f", r.DeltaDistance),
		"momentum":           fmt.Sprintf("%.3f", r.Momentum),
		"unmet":              fmt.Sprintf("%d", r.UnmetCount),
		"queue_drained":      fmt.Sprintf("%d", r.QueueDrained),
		"reflex_hits":        fmt.Sprintf("%d", r.ReflexHits),
		"elapsed_ms":         fmt.Sprintf("%d", r.ElapsedMs),
		"navigator_decision": r.NavigatorDecision,
		"regulator_depth":    r.RegulatorDepth,
	}
}

type SessionSummary struct {
	MoleculeID      string
	TotalTurns      int
	TotalTokensIn   int
	TotalTokensOut  int
	TotalToolCalls  int
	OAE               float64
	GearNovelPct      float64
	GearFamiliarPct   float64
	GearIntuitionPct  float64
	GearReflexPct     float64
	ReflexHits      int
	ReflexCoverage  float64
	LLMCalls        int
	ReflexFires     int
	AvgTurnMs       int64
	Sealed          bool
	FinalDistance    float64
}

func (s SessionSummary) Labels() map[string]string {
	return map[string]string{
		"molecule":          s.MoleculeID,
		"total_turns":       fmt.Sprintf("%d", s.TotalTurns),
		"total_tokens_in":   fmt.Sprintf("%d", s.TotalTokensIn),
		"total_tokens_out":  fmt.Sprintf("%d", s.TotalTokensOut),
		"total_tool_calls":  fmt.Sprintf("%d", s.TotalToolCalls),
		"oae":               fmt.Sprintf("%.3f", s.OAE),
		"gear_novel_pct":     fmt.Sprintf("%.1f", s.GearNovelPct),
		"gear_familiar_pct": fmt.Sprintf("%.1f", s.GearFamiliarPct),
		"gear_intuition_pct": fmt.Sprintf("%.1f", s.GearIntuitionPct),
		"gear_reflex_pct":   fmt.Sprintf("%.1f", s.GearReflexPct),
		"reflex_hits":       fmt.Sprintf("%d", s.ReflexHits),
		"reflex_coverage":   fmt.Sprintf("%.3f", s.ReflexCoverage),
		"llm_calls":         fmt.Sprintf("%d", s.LLMCalls),
		"reflex_fires":      fmt.Sprintf("%d", s.ReflexFires),
		"avg_turn_ms":       fmt.Sprintf("%d", s.AvgTurnMs),
		"sealed":            fmt.Sprintf("%v", s.Sealed),
		"final_distance":    fmt.Sprintf("%.3f", s.FinalDistance),
	}
}

func computeSessionSummary(moleculeID string, turns []TurnRecord, m *reactivity.Molecule) SessionSummary {
	s := SessionSummary{
		MoleculeID:   moleculeID,
		TotalTurns:   len(turns),
		Sealed:       m.Sealed(),
		FinalDistance: m.Distance(),
	}

	var totalMs int64
	var novel, familiar, intuition, reflex int
	for _, t := range turns {
		s.TotalTokensIn += t.TokensIn
		s.TotalTokensOut += t.TokensOut
		s.TotalToolCalls += t.ToolCalls
		s.ReflexHits += t.ReflexHits
		totalMs += t.ElapsedMs
		switch t.Gear {
		case GearNovel:
			novel++
		case GearFamiliar:
			familiar++
		case GearIntuition:
			intuition++
		case GearReflex:
			reflex++
		}
	}

	s.LLMCalls = novel + familiar + intuition
	s.ReflexFires = reflex
	if s.TotalTurns > 0 {
		s.AvgTurnMs = totalMs / int64(s.TotalTurns)
		total := float64(s.TotalTurns)
		s.GearNovelPct = float64(novel) / total * 100
		s.GearFamiliarPct = float64(familiar) / total * 100
		s.GearIntuitionPct = float64(intuition) / total * 100
		s.GearReflexPct = float64(reflex) / total * 100
		s.ReflexCoverage = float64(reflex) / total
	}

	if s.TotalTurns > 0 {
		optimalTurns := max(1, s.TotalTurns/2)
		s.OAE = float64(optimalTurns) / float64(s.TotalTurns)
		if s.OAE > 1 {
			s.OAE = 1
		}
	}

	return s
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
