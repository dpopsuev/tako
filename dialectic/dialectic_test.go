package dialectic

import (
	"testing"

	"github.com/dpopsuev/origami/circuit"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Enabled {
		t.Error("default dialectic should be disabled")
	}
	if cfg.MaxNegations != 2 {
		t.Errorf("MaxNegations = %d, want 2", cfg.MaxNegations)
	}
	if cfg.MaxTurns != 6 {
		t.Errorf("MaxTurns = %d, want 6", cfg.MaxTurns)
	}
	if cfg.ContradictionThreshold != 0.85 {
		t.Errorf("ContradictionThreshold = %f, want 0.85", cfg.ContradictionThreshold)
	}
	if cfg.GapClosureThreshold != 0.15 {
		t.Errorf("GapClosureThreshold = %f, want 0.15", cfg.GapClosureThreshold)
	}
}

func TestConfig_NeedsAntithesis(t *testing.T) {
	cfg := Config{Enabled: true, ContradictionThreshold: 0.85}

	cases := []struct {
		confidence float64
		want       bool
	}{
		{0.90, false},
		{0.85, false},
		{0.84, true},
		{0.65, true},
		{0.50, true},
		{0.49, false},
		{0.30, false},
		{1.00, false},
	}
	for _, tc := range cases {
		got := cfg.NeedsAntithesis(tc.confidence)
		if got != tc.want {
			t.Errorf("NeedsAntithesis(%f) = %v, want %v", tc.confidence, got, tc.want)
		}
	}
}

func TestConfig_NeedsAntithesis_Disabled(t *testing.T) {
	cfg := Config{Enabled: false, ContradictionThreshold: 0.85}
	if cfg.NeedsAntithesis(0.65) {
		t.Error("disabled dialectic should never activate")
	}
}

func TestThesisChallenge_ArtifactInterface(t *testing.T) {
	tc := &ThesisChallenge{
		ChargedDefectType: "product_bug",
		ConfidenceScore:   0.8,
		Evidence:          []EvidenceItem{{Description: "test", Source: "log", Weight: 0.9}},
	}
	if tc.Type() != "thesis_challenge" {
		t.Errorf("Type() = %q, want %q", tc.Type(), "thesis_challenge")
	}
	if tc.Raw() != tc {
		t.Error("Raw() should return self")
	}
}

func TestAntithesisResponse_ArtifactInterface(t *testing.T) {
	ar := &AntithesisResponse{
		Challenges:      []EvidenceChallenge{{EvidenceIndex: 0, Challenge: "weak", Severity: "high"}},
		Concession:      false,
		ConfidenceScore: 0.7,
	}
	if ar.Type() != "antithesis_response" {
		t.Errorf("Type() = %q, want %q", ar.Type(), "antithesis_response")
	}
}

func TestRecord_ArtifactInterface(t *testing.T) {
	record := &Record{
		Rounds:    []Round{{Round: 1, ThesisArgument: "t", AntithesisRebuttal: "a", ArbiterNotes: "n"}},
		MaxRounds: 3,
		Converged: false,
	}
	if record.Type() != "dialectic_record" {
		t.Errorf("Type() = %q, want %q", record.Type(), "dialectic_record")
	}
}

func TestSynthesis_ArtifactInterface(t *testing.T) {
	s := &Synthesis{
		Decision:            SynthesisAffirm,
		FinalClassification: "product_bug",
		ConfidenceScore:     0.9,
		Reasoning:           "confirmed",
	}
	if s.Type() != "synthesis" {
		t.Errorf("Type() = %q, want %q", s.Type(), "synthesis")
	}
}

func TestSynthesis_Remand(t *testing.T) {
	s := &Synthesis{
		Decision: SynthesisRemand,
		NegationFeedback: &NegationFeedback{
			ChallengedEvidence: []int{0, 2},
			AlternativeHyp:     "could be flaky",
			SpecificQuestions:   []string{"Was network stable?"},
		},
	}
	if s.Decision != SynthesisRemand {
		t.Errorf("Decision = %q, want remand", s.Decision)
	}
	if s.NegationFeedback == nil {
		t.Fatal("NegationFeedback should not be nil for remand")
	}
	if len(s.NegationFeedback.ChallengedEvidence) != 2 {
		t.Errorf("ChallengedEvidence count = %d, want 2", len(s.NegationFeedback.ChallengedEvidence))
	}
}

func TestSynthesisDecision_Constants(t *testing.T) {
	decisions := []SynthesisDecision{SynthesisAffirm, SynthesisAmend, SynthesisAcquit, SynthesisRemand, SynthesisUnresolved}
	if len(decisions) != 5 {
		t.Errorf("expected 5 synthesis decisions, got %d", len(decisions))
	}
	seen := make(map[SynthesisDecision]bool)
	for _, d := range decisions {
		if seen[d] {
			t.Errorf("duplicate decision: %s", d)
		}
		seen[d] = true
	}
}

func TestDialecticEvidenceGap(t *testing.T) {
	gap := DialecticEvidenceGap{
		EvidenceGap: EvidenceGap{
			Description:     "missing network metrics during failure window",
			Source:          "infrastructure_telemetry",
			Severity:        GapSeverityHigh,
			SuggestedAction: "collect pod-level network stats from prometheus",
		},
		DialecticPhase: "D3",
	}
	if gap.Description == "" {
		t.Error("Description should not be empty")
	}
	if gap.SuggestedAction == "" {
		t.Error("SuggestedAction should not be empty")
	}
	if gap.DialecticPhase != "D3" {
		t.Errorf("DialecticPhase = %q, want D3", gap.DialecticPhase)
	}
}

func TestBuildEdgeFactory_AllKeysPresent(t *testing.T) {
	cfg := DefaultConfig()
	factory := BuildEdgeFactory(cfg)
	expectedKeys := []string{
		"HD1", "HD2", "HD3", "HD4", "HD5", "HD6",
		"HD7", "HD8", "HD9", "HD10", "HD12", "HD13",
	}
	for _, k := range expectedKeys {
		if _, ok := factory[k]; !ok {
			t.Errorf("missing edge factory key %q", k)
		}
	}
	if len(factory) != len(expectedKeys) {
		t.Errorf("factory has %d keys, want %d", len(factory), len(expectedKeys))
	}
}

func TestBuildEdgeFactory_EdgesImplementInterface(t *testing.T) {
	cfg := DefaultConfig()
	factory := BuildEdgeFactory(cfg)
	for id, fn := range factory {
		edge := fn(circuit.EdgeDef{ID: id, From: "a", To: "b"})
		if edge.ID() != id {
			t.Errorf("edge %s: ID() = %q, want %q", id, edge.ID(), id)
		}
		if edge.From() != "a" {
			t.Errorf("edge %s: From() = %q, want %q", id, edge.From(), "a")
		}
	}
}

func TestDialecticEdge_HD1_FastTrack(t *testing.T) {
	cfg := DefaultConfig()
	factory := BuildEdgeFactory(cfg)
	edge := factory["HD1"](circuit.EdgeDef{ID: "HD1", From: "indict", To: "defend"})

	high := &ThesisChallenge{ConfidenceScore: 0.96}
	tr := edge.Evaluate(high, &circuit.WalkerState{})
	if tr == nil {
		t.Fatal("HD1 should trigger for confidence >= 0.95")
	}
	if tr.NextNode != "defend" {
		t.Errorf("NextNode = %q, want defend", tr.NextNode)
	}

	low := &ThesisChallenge{ConfidenceScore: 0.80}
	tr = edge.Evaluate(low, &circuit.WalkerState{})
	if tr != nil {
		t.Error("HD1 should not trigger for confidence < 0.95")
	}
}

func TestDialecticEdge_HD2_Concession(t *testing.T) {
	cfg := DefaultConfig()
	factory := BuildEdgeFactory(cfg)
	edge := factory["HD2"](circuit.EdgeDef{ID: "HD2", From: "defend", To: "verdict"})

	concede := &AntithesisResponse{Concession: true, ConfidenceScore: 0.5}
	tr := edge.Evaluate(concede, &circuit.WalkerState{})
	if tr == nil {
		t.Fatal("HD2 should trigger on concession")
	}

	noConcede := &AntithesisResponse{Concession: false, ConfidenceScore: 0.5}
	tr = edge.Evaluate(noConcede, &circuit.WalkerState{})
	if tr != nil {
		t.Error("HD2 should not trigger without concession")
	}
}

func TestDialecticEdge_HD5_DialecticComplete(t *testing.T) {
	cfg := DefaultConfig()
	factory := BuildEdgeFactory(cfg)
	edge := factory["HD5"](circuit.EdgeDef{ID: "HD5", From: "hearing", To: "verdict"})

	converged := &Record{Converged: true, MaxRounds: 3, Rounds: []Round{{Round: 1}}}
	tr := edge.Evaluate(converged, &circuit.WalkerState{})
	if tr == nil {
		t.Fatal("HD5 should trigger when converged")
	}

	maxRounds := &Record{Converged: false, MaxRounds: 2, Rounds: []Round{{Round: 1}, {Round: 2}}}
	tr = edge.Evaluate(maxRounds, &circuit.WalkerState{})
	if tr == nil {
		t.Fatal("HD5 should trigger when max rounds reached")
	}

	inProgress := &Record{Converged: false, MaxRounds: 5, Rounds: []Round{{Round: 1}}}
	tr = edge.Evaluate(inProgress, &circuit.WalkerState{})
	if tr != nil {
		t.Error("HD5 should not trigger mid-dialectic")
	}
}

func TestDialecticEdge_HD6_Affirm(t *testing.T) {
	cfg := DefaultConfig()
	factory := BuildEdgeFactory(cfg)
	edge := factory["HD6"](circuit.EdgeDef{ID: "HD6", From: "verdict", To: "_done"})

	s := &Synthesis{Decision: SynthesisAffirm}
	tr := edge.Evaluate(s, &circuit.WalkerState{})
	if tr == nil || tr.NextNode != "_done" {
		t.Fatal("HD6 should route affirm to _done")
	}

	s2 := &Synthesis{Decision: SynthesisAmend}
	tr = edge.Evaluate(s2, &circuit.WalkerState{})
	if tr != nil {
		t.Error("HD6 should not trigger for amend")
	}
}

func TestDialecticEdge_HD8_Remand_WithLimit(t *testing.T) {
	cfg := Config{MaxNegations: 2}
	factory := BuildEdgeFactory(cfg)
	edge := factory["HD8"](circuit.EdgeDef{ID: "HD8", From: "verdict", To: "indict", Loop: true})

	state := &circuit.WalkerState{LoopCounts: map[string]int{"verdict": 0}}
	s := &Synthesis{Decision: SynthesisRemand}
	tr := edge.Evaluate(s, state)
	if tr == nil {
		t.Fatal("HD8 should allow remand when under limit")
	}
	if tr.NextNode != "indict" {
		t.Errorf("NextNode = %q, want indict", tr.NextNode)
	}

	state.LoopCounts["verdict"] = 2
	tr = edge.Evaluate(s, state)
	if tr != nil {
		t.Error("HD8 should not remand when at limit")
	}
}

func TestDialecticEdge_HD9_Acquit(t *testing.T) {
	cfg := DefaultConfig()
	factory := BuildEdgeFactory(cfg)
	edge := factory["HD9"](circuit.EdgeDef{ID: "HD9", From: "verdict", To: "_done"})

	s := &Synthesis{Decision: SynthesisAcquit}
	tr := edge.Evaluate(s, &circuit.WalkerState{})
	if tr == nil || tr.NextNode != "_done" {
		t.Fatal("HD9 should route acquit to _done")
	}
}

func TestDialecticEdge_HD12_Unresolved(t *testing.T) {
	cfg := DefaultConfig()
	factory := BuildEdgeFactory(cfg)
	edge := factory["HD12"](circuit.EdgeDef{ID: "HD12", From: "verdict", To: "_done"})

	s := &Synthesis{Decision: SynthesisUnresolved}
	tr := edge.Evaluate(s, &circuit.WalkerState{})
	if tr == nil || tr.NextNode != "_done" {
		t.Fatal("HD12 should route unresolved contradiction to _done")
	}
}

func TestDialecticEdge_HD5_GapClosure(t *testing.T) {
	cfg := DefaultConfig()
	cfg.GapClosureThreshold = 0.15
	factory := BuildEdgeFactory(cfg)
	edge := factory["HD5"](circuit.EdgeDef{ID: "HD5", From: "hearing", To: "verdict"})

	closed := &Record{GapClosure: 0.10, MaxRounds: 10, Rounds: []Round{{Round: 1}}}
	tr := edge.Evaluate(closed, &circuit.WalkerState{})
	if tr == nil {
		t.Fatal("HD5 should trigger when gap closure < threshold")
	}

	open := &Record{GapClosure: 0.50, MaxRounds: 10, Rounds: []Round{{Round: 1}}}
	tr = edge.Evaluate(open, &circuit.WalkerState{})
	if tr != nil {
		t.Error("HD5 should not trigger when gap closure > threshold")
	}

	noGap := &Record{GapClosure: 0, MaxRounds: 10, Rounds: []Round{{Round: 1}}}
	tr = edge.Evaluate(noGap, &circuit.WalkerState{})
	if tr != nil {
		t.Error("HD5 should not trigger when gap closure is zero (not set)")
	}
}

func TestDialecticEdge_HD10_SafetyCeiling(t *testing.T) {
	cfg := Config{MaxTurns: 3}
	factory := BuildEdgeFactory(cfg)
	edge := factory["HD10"](circuit.EdgeDef{ID: "HD10", From: "hearing", To: "_done"})

	state := &circuit.WalkerState{LoopCounts: map[string]int{"verdict": 2, "hearing": 2}}
	tr := edge.Evaluate(&Record{GapClosure: 0.45}, state)
	if tr == nil {
		t.Fatal("HD10 should trigger when combined loops exceed MaxTurns")
	}

	state = &circuit.WalkerState{LoopCounts: map[string]int{"verdict": 1, "hearing": 1}}
	tr = edge.Evaluate(&Record{}, state)
	if tr != nil {
		t.Error("HD10 should not trigger when combined loops are under MaxTurns")
	}
}

func TestDialecticEdge_HD13_CMRR(t *testing.T) {
	cfg := DefaultConfig()
	cfg.CMRREnabled = true
	factory := BuildEdgeFactory(cfg)
	edge := factory["HD13"](circuit.EdgeDef{ID: "HD13", From: "hearing", To: "hearing"})

	cmrr := &CMRRCheck{SharedPremises: []string{"both assume stable network"}, SuspicionScore: 0.7}
	tr := edge.Evaluate(cmrr, &circuit.WalkerState{})
	if tr == nil {
		t.Fatal("HD13 should trigger when CMRR detects shared assumptions")
	}
	if tr.NextNode != "hearing" {
		t.Errorf("NextNode = %q, want hearing", tr.NextNode)
	}

	clean := &CMRRCheck{SharedPremises: nil, SuspicionScore: 0}
	tr = edge.Evaluate(clean, &circuit.WalkerState{})
	if tr != nil {
		t.Error("HD13 should not trigger when suspicion is zero")
	}
}

func TestDialecticEdge_HD13_CMRRDisabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.CMRREnabled = false
	factory := BuildEdgeFactory(cfg)
	edge := factory["HD13"](circuit.EdgeDef{ID: "HD13", From: "hearing", To: "hearing"})

	cmrr := &CMRRCheck{SharedPremises: []string{"test"}, SuspicionScore: 0.9}
	tr := edge.Evaluate(cmrr, &circuit.WalkerState{})
	if tr != nil {
		t.Error("HD13 should not trigger when CMRR is disabled")
	}
}

func TestCMRRCheck_ArtifactInterface(t *testing.T) {
	c := &CMRRCheck{SharedPremises: []string{"p1"}, SuspicionScore: 0.3}
	if c.Type() != "cmrr_check" {
		t.Errorf("Type() = %q, want cmrr_check", c.Type())
	}
	if c.Confidence() != 0.7 {
		t.Errorf("Confidence() = %f, want 0.7", c.Confidence())
	}
}

func TestDialecticEdge_NilArtifact(t *testing.T) {
	cfg := DefaultConfig()
	factory := BuildEdgeFactory(cfg)
	for id, fn := range factory {
		edge := fn(circuit.EdgeDef{ID: id})
		tr := edge.Evaluate(nil, &circuit.WalkerState{LoopCounts: map[string]int{}})
		if tr != nil {
			t.Errorf("edge %s should return nil for nil artifact", id)
		}
	}
}
