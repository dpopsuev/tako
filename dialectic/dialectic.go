// Package dialectic implements the adversarial dialectic circuit pattern:
// thesis-antithesis-synthesis debate with evidence gap tracking.
package dialectic

import (
	"fmt"
	"time"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
)

// Config controls the adversarial dialectic circuit activation and limits.
// When the Thesis path's confidence falls below the contradiction threshold,
// the adversarial path activates for thesis-antithesis-synthesis debate.
type Config struct {
	Enabled                bool          `json:"enabled"`
	TTL                    time.Duration `json:"ttl"`
	MaxTurns               int           `json:"max_turns"`
	MaxNegations           int           `json:"max_negations"`
	ContradictionThreshold float64       `json:"contradiction_threshold"`
	GapClosureThreshold    float64       `json:"gap_closure_threshold"`
	CMRREnabled            bool          `json:"cmrr_enabled"`
	ContextGuard           func(map[string]any) map[string]any `json:"-"`
}

// DefaultConfig returns conservative defaults for the dialectic circuit.
func DefaultConfig() Config {
	return Config{
		Enabled:                false,
		TTL:                    10 * time.Minute,
		MaxTurns:               6,
		MaxNegations:           2,
		ContradictionThreshold: 0.85,
		GapClosureThreshold:    0.15,
	}
}

// NeedsAntithesis returns true when a Thesis path confidence falls in the
// uncertain range that triggers adversarial dialectic review.
func (c Config) NeedsAntithesis(confidence float64) bool {
	if !c.Enabled {
		return false
	}
	return confidence >= 0.50 && confidence < c.ContradictionThreshold
}

// SynthesisDecision represents the outcome of the adversarial dialectic.
type SynthesisDecision string

const (
	SynthesisAffirm     SynthesisDecision = "affirm"
	SynthesisAmend      SynthesisDecision = "amend"
	SynthesisAcquit     SynthesisDecision = "acquit"
	SynthesisRemand     SynthesisDecision = "remand"
	SynthesisUnresolved SynthesisDecision = "unresolved"
)

// EvidenceItem is a single piece of evidence with an assigned weight.
type EvidenceItem struct {
	Description string  `json:"description"`
	Source      string  `json:"source"`
	Weight      float64 `json:"weight"`
}

// ThesisChallenge is the D0 thesis-holder artifact: charged defect type with
// itemized evidence and a thesis-holder narrative.
type ThesisChallenge struct {
	ChargedDefectType string         `json:"charged_defect_type"`
	ThesisNarrative   string         `json:"thesis_narrative"`
	Evidence          []EvidenceItem `json:"evidence"`
	ConfidenceScore   float64        `json:"confidence"`
}

func (t *ThesisChallenge) Type() string       { return "thesis_challenge" }
func (t *ThesisChallenge) Confidence() float64 { return t.ConfidenceScore }
func (t *ThesisChallenge) Raw() any            { return t }

// EvidenceChallenge captures a specific challenge to an evidence item.
type EvidenceChallenge struct {
	EvidenceIndex int    `json:"evidence_index"`
	Challenge     string `json:"challenge"`
	Severity      string `json:"severity"`
}

// AntithesisResponse is the D2 antithesis-holder artifact: challenges to evidence,
// alternative hypothesis, and concession flag.
type AntithesisResponse struct {
	Challenges            []EvidenceChallenge `json:"challenges"`
	AlternativeHypothesis string              `json:"alternative_hypothesis,omitempty"`
	Concession            bool                `json:"concession"`
	ConfidenceScore       float64             `json:"confidence"`
}

func (a *AntithesisResponse) Type() string       { return "antithesis_response" }
func (a *AntithesisResponse) Confidence() float64 { return a.ConfidenceScore }
func (a *AntithesisResponse) Raw() any            { return a }

// Round captures one round of thesis argument, antithesis rebuttal, and
// arbiter notes.
type Round struct {
	Round              int    `json:"round"`
	ThesisArgument     string `json:"thesis_argument"`
	AntithesisRebuttal string `json:"antithesis_rebuttal"`
	ArbiterNotes       string `json:"arbiter_notes"`
}

// Record is the D3 dialectic artifact: rounds of structured debate.
type Record struct {
	Rounds     []Round `json:"rounds"`
	MaxRounds  int     `json:"max_rounds"`
	Converged  bool    `json:"converged"`
	GapClosure float64 `json:"gap_closure"`
}

func (d *Record) Type() string       { return "dialectic_record" }
func (d *Record) Confidence() float64 { return 0 }
func (d *Record) Raw() any            { return d }

// Synthesis is the D4 final decision artifact.
type Synthesis struct {
	Decision            SynthesisDecision `json:"decision"`
	FinalClassification string            `json:"final_classification"`
	ConfidenceScore     float64           `json:"confidence"`
	Reasoning           string            `json:"reasoning"`
	NegationFeedback    *NegationFeedback `json:"negation_feedback,omitempty"`
}

func (s *Synthesis) Type() string       { return "synthesis" }
func (s *Synthesis) Confidence() float64 { return s.ConfidenceScore }
func (s *Synthesis) Raw() any            { return s }

// NegationFeedback provides structured feedback when a case is remanded
// back to the Thesis path for reinvestigation.
type NegationFeedback struct {
	ChallengedEvidence []int    `json:"challenged_evidence"`
	AlternativeHyp     string   `json:"alternative_hypothesis"`
	SpecificQuestions  []string `json:"specific_questions"`
}

// DialecticEvidenceGap extends EvidenceGap with dialectic-specific context.
type DialecticEvidenceGap struct {
	EvidenceGap
	DialecticPhase string `json:"dialectic_phase,omitempty"`
}

// CMRRCheck captures shared-assumption detection results between thesis and antithesis.
// When both sides share unexamined premises, the debate may converge on a wrong answer.
// CMRR (Common-Mode Rejection Ratio) flags this: high SuspicionScore means
// shared assumptions need independent verification.
type CMRRCheck struct {
	SharedPremises []string `json:"shared_premises"`
	SuspicionScore float64  `json:"suspicion_score"`
}

func (c *CMRRCheck) Type() string       { return "cmrr_check" }
func (c *CMRRCheck) Confidence() float64 { return 1.0 - c.SuspicionScore }
func (c *CMRRCheck) Raw() any            { return c }

// BuildEdgeFactory returns an EdgeFactory with skeleton dialectic evaluators
// (HD1-HD13) for the adversarial dialectic circuit. Each evaluator checks the
// artifact type and dialectic-specific conditions.
func BuildEdgeFactory(cfg Config) engine.EdgeFactory {
	return engine.EdgeFactory{
		"HD1": dialecticEdgeFactory(func(a circuit.Artifact, _ *circuit.WalkerState) *circuit.Transition {
			tc, ok := unwrapThesisChallenge(a)
			if !ok {
				return nil
			}
			if tc.ConfidenceScore >= 0.95 {
				return &circuit.Transition{NextNode: "defend", Explanation: "fast-track: thesis confidence >= 0.95"}
			}
			return nil
		}),
		"HD2": dialecticEdgeFactory(func(a circuit.Artifact, _ *circuit.WalkerState) *circuit.Transition {
			ar, ok := unwrapAntithesisResponse(a)
			if !ok {
				return nil
			}
			if ar.Concession {
				return &circuit.Transition{NextNode: "verdict", Explanation: "concession: antithesis-holder concedes"}
			}
			return nil
		}),
		"HD3": dialecticEdgeFactory(func(a circuit.Artifact, _ *circuit.WalkerState) *circuit.Transition {
			ar, ok := unwrapAntithesisResponse(a)
			if !ok {
				return nil
			}
			if len(ar.Challenges) > 0 && ar.AlternativeHypothesis == "" {
				return &circuit.Transition{NextNode: "hearing", Explanation: "partial negation: challenges without alternative"}
			}
			return nil
		}),
		"HD4": dialecticEdgeFactory(func(a circuit.Artifact, _ *circuit.WalkerState) *circuit.Transition {
			ar, ok := unwrapAntithesisResponse(a)
			if !ok {
				return nil
			}
			if ar.AlternativeHypothesis != "" {
				return &circuit.Transition{NextNode: "hearing", Explanation: "alternative hypothesis presented"}
			}
			return nil
		}),
		"HD5": dialecticEdgeFactory(func(a circuit.Artifact, _ *circuit.WalkerState) *circuit.Transition {
			rec, ok := unwrapRecord(a)
			if !ok {
				return nil
			}
			if rec.GapClosure > 0 && rec.GapClosure < cfg.GapClosureThreshold {
				return &circuit.Transition{NextNode: "verdict", Explanation: fmt.Sprintf("gap closed (%.2f < %.2f threshold)", rec.GapClosure, cfg.GapClosureThreshold)}
			}
			if rec.Converged || len(rec.Rounds) >= rec.MaxRounds {
				return &circuit.Transition{NextNode: "verdict", Explanation: "dialectic complete"}
			}
			return nil
		}),
		"HD6": dialecticEdgeFactory(func(a circuit.Artifact, _ *circuit.WalkerState) *circuit.Transition {
			s, ok := unwrapSynthesis(a)
			if !ok {
				return nil
			}
			if s.Decision == SynthesisAffirm {
				return &circuit.Transition{NextNode: "_done", Explanation: "synthesis: affirm"}
			}
			return nil
		}),
		"HD7": dialecticEdgeFactory(func(a circuit.Artifact, _ *circuit.WalkerState) *circuit.Transition {
			s, ok := unwrapSynthesis(a)
			if !ok {
				return nil
			}
			if s.Decision == SynthesisAmend {
				return &circuit.Transition{NextNode: "_done", Explanation: "synthesis: amend"}
			}
			return nil
		}),
		"HD8": dialecticEdgeFactory(func(a circuit.Artifact, ws *circuit.WalkerState) *circuit.Transition {
			s, ok := unwrapSynthesis(a)
			if !ok {
				return nil
			}
			if s.Decision == SynthesisRemand && ws.LoopCounts["verdict"] < cfg.MaxNegations {
				return &circuit.Transition{
					NextNode:    "indict",
					Explanation: "synthesis: remand for reinvestigation",
				}
			}
			return nil
		}),
		"HD9": dialecticEdgeFactory(func(a circuit.Artifact, _ *circuit.WalkerState) *circuit.Transition {
			s, ok := unwrapSynthesis(a)
			if !ok {
				return nil
			}
			if s.Decision == SynthesisAcquit {
				return &circuit.Transition{NextNode: "_done", Explanation: "synthesis: acquit (evidence gap brief)"}
			}
			return nil
		}),
		"HD10": dialecticEdgeFactory(func(a circuit.Artifact, ws *circuit.WalkerState) *circuit.Transition {
			totalLoops := ws.LoopCounts["verdict"] + ws.LoopCounts["hearing"]
			if totalLoops > cfg.MaxTurns {
				gc := float64(0)
				if rec, ok := unwrapRecord(a); ok {
					gc = rec.GapClosure
				}
				return &circuit.Transition{NextNode: "_done", Explanation: fmt.Sprintf("safety ceiling reached after %d turns, gap closure was %.2f", totalLoops, gc)}
			}
			return nil
		}),
		"HD12": dialecticEdgeFactory(func(a circuit.Artifact, _ *circuit.WalkerState) *circuit.Transition {
			s, ok := unwrapSynthesis(a)
			if !ok {
				return nil
			}
			if s.Decision == SynthesisUnresolved {
				return &circuit.Transition{NextNode: "_done", Explanation: "synthesis: unresolved contradiction declared by arbiter"}
			}
			return nil
		}),
		"HD13": dialecticEdgeFactory(func(a circuit.Artifact, _ *circuit.WalkerState) *circuit.Transition {
			if !cfg.CMRREnabled {
				return nil
			}
			cmrr, ok := unwrapCMRRCheck(a)
			if !ok {
				return nil
			}
			if cmrr.SuspicionScore > 0 {
				return &circuit.Transition{
					NextNode:    "hearing",
					Explanation: fmt.Sprintf("CMRR: shared assumptions detected (suspicion %.2f)", cmrr.SuspicionScore),
					ContextAdditions: map[string]any{
						"cmrr_shared_premises": cmrr.SharedPremises,
						"cmrr_suspicion":       cmrr.SuspicionScore,
					},
				}
			}
			return nil
		}),
	}
}

type dialecticEvalFunc func(circuit.Artifact, *circuit.WalkerState) *circuit.Transition

func dialecticEdgeFactory(eval dialecticEvalFunc) func(circuit.EdgeDef) circuit.Edge {
	return func(def circuit.EdgeDef) circuit.Edge {
		return &dialecticEdge{def: def, eval: eval}
	}
}

type dialecticEdge struct {
	def  circuit.EdgeDef
	eval dialecticEvalFunc
}

func (e *dialecticEdge) ID() string       { return e.def.ID }
func (e *dialecticEdge) From() string     { return e.def.From }
func (e *dialecticEdge) To() string       { return e.def.To }
func (e *dialecticEdge) IsShortcut() bool { return e.def.Shortcut }
func (e *dialecticEdge) IsLoop() bool     { return e.def.Loop }
func (e *dialecticEdge) Evaluate(a circuit.Artifact, s *circuit.WalkerState) *circuit.Transition {
	return e.eval(a, s)
}

func unwrapThesisChallenge(a circuit.Artifact) (*ThesisChallenge, bool) {
	if a == nil {
		return nil, false
	}
	tc, ok := a.Raw().(*ThesisChallenge)
	return tc, ok
}

func unwrapAntithesisResponse(a circuit.Artifact) (*AntithesisResponse, bool) {
	if a == nil {
		return nil, false
	}
	ar, ok := a.Raw().(*AntithesisResponse)
	return ar, ok
}

func unwrapRecord(a circuit.Artifact) (*Record, bool) {
	if a == nil {
		return nil, false
	}
	rec, ok := a.Raw().(*Record)
	return rec, ok
}

func unwrapSynthesis(a circuit.Artifact) (*Synthesis, bool) {
	if a == nil {
		return nil, false
	}
	s, ok := a.Raw().(*Synthesis)
	return s, ok
}

func unwrapCMRRCheck(a circuit.Artifact) (*CMRRCheck, bool) {
	if a == nil {
		return nil, false
	}
	c, ok := a.Raw().(*CMRRCheck)
	return c, ok
}
