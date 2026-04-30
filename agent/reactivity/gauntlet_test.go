package reactivity

import (
	"testing"
)

func TestGauntlet_DirtyShoes(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("dirty-shoes")

	c.Add(m, mkAtom("intent-clean", IntentAtom, "intent.desire.clean-room", Fresh))

	c.Add(m, mkAtom("assess-bed", AssessmentAtom, "assessment.state.bed", Fresh))
	c.Add(m, mkAtom("assess-floor", AssessmentAtom, "assessment.state.floor", Fresh))
	c.Add(m, mkAtom("assess-window", AssessmentAtom, "assessment.state.window", Fresh))
	c.Add(m, mkAtom("understand-clean", UnderstandingAtom, "understanding.synth.clean-room", Fresh))

	c.Add(m, mkAtom("plan-bed", PlanAtom, "plan.task.bed", Fresh, "assess-bed"))
	c.Add(m, mkAtom("plan-floor", PlanAtom, "plan.task.floor", Fresh, "assess-floor"))
	c.Add(m, mkAtom("plan-window", PlanAtom, "plan.task.window", Fresh, "assess-window"))
	c.Add(m, mkAtom("risk-clean", RiskAtom, "risk.eval.clean-room", Fresh))
	c.Add(m, mkAtom("strat-clean", StrategyAtom, "strategy.synth.clean-room", Fresh))

	c.Add(m, mkAtom("exec-bed", ExecutionAtom, "execution.result.bed", Fresh, "plan-bed"))
	c.Add(m, mkAtom("exec-floor", ExecutionAtom, "execution.result.floor", Fresh, "plan-floor"))
	c.Add(m, mkAtom("exec-window", ExecutionAtom, "execution.result.window", Fresh, "plan-window"))

	dirtyAgain := mkAtom("assess-floor-dirty-again", AssessmentAtom, "assessment.state.floor", Fresh, "exec-floor")

	contradicts, existing := c.Contradict(m, dirtyAgain)
	if !contradicts {
		t.Fatal("expected contradiction: 'floor done' vs 'floor dirty again'")
	}
	if existing == nil {
		t.Fatal("expected conflicting atom")
	}

	result, _ := c.Add(m, dirtyAgain)
	if result != Pass && result != Insufficient {
		t.Errorf("expected assessment to be accepted, got %s", result)
	}

	if m.Mass(AssessmentAtom) <= m.Mass(ExecutionAtom) {
		t.Error("contradicting assessment should increase assessment mass, signaling re-plan needed")
	}
}

func TestGauntlet_Recollection(t *testing.T) {
	c1 := NewReactor()
	first := NewMolecule("rca-first")
	c1.Add(first, mkAtom("intent-rca", IntentAtom, "intent.desire.investigate-failure", Fresh))
	c1.Add(first, mkAtom("assess-logs", AssessmentAtom, "assessment.state.logs", Fresh))
	c1.Add(first, mkAtom("assess-commits", AssessmentAtom, "assessment.state.commits", Fresh))
	c1.Add(first, mkAtom("assess-jira", AssessmentAtom, "assessment.state.jira-match", Fresh))
	c1.Add(first, mkAtom("understand-rca", UnderstandingAtom, "understanding.synth.rca", Fresh))
	c1.Add(first, mkAtom("plan-classify", PlanAtom, "plan.task.classify", Fresh))
	c1.Add(first, mkAtom("risk-classify", RiskAtom, "risk.eval.classify", Fresh))
	c1.Add(first, mkAtom("strat-classify", StrategyAtom, "strategy.synth.classify", Fresh))
	c1.Add(first, mkAtom("exec-classify", ExecutionAtom, "execution.result.product-bug", Fresh))
	c1.Add(first, mkAtom("obs-classify", ObservationAtom, "observation.eval.classify", Fresh))
	c1.Add(first, mkAtom("adapt-classify", AdaptationAtom, "adaptation.synth.classify", Fresh))
	c1.Add(first, mkAtom("retro-83551", RetrospectionAtom, "retrospection.learning.ocpbugs-83551", Fresh))
	c1.Seal(first, mkAtom("wish-done", RetrospectionAtom, "retrospection.wish.done", Fresh))

	if !first.Sealed() {
		t.Fatal("first molecule should be sealed")
	}
	firstMass := first.TotalMass()

	c5 := NewReactor()
	fifth := NewMolecule("rca-fifth")
	c5.Add(fifth, mkAtom("intent-rca", IntentAtom, "intent.desire.investigate-failure", Fresh))

	recollectLogs := mkAtom("recollect-logs", AssessmentAtom, "assessment.state.logs", Fresh)
	recollectLogs.Source = Recollected
	c5.Add(fifth, recollectLogs)

	recollectJira := mkAtom("recollect-jira", AssessmentAtom, "assessment.state.jira-match", Fresh)
	recollectJira.Source = Recollected
	c5.Add(fifth, recollectJira)

	recollectResult := mkAtom("recollect-result", AssessmentAtom, "assessment.state.known-bug-83551", Fresh)
	recollectResult.Source = Recollected
	c5.Add(fifth, recollectResult)

	c5.Add(fifth, mkAtom("understand-known", UnderstandingAtom, "understanding.synth.known-bug", Fresh))
	c5.Add(fifth, mkAtom("plan-link", PlanAtom, "plan.task.link-existing-jira", Fresh))
	c5.Add(fifth, mkAtom("risk-link", RiskAtom, "risk.eval.link", Fresh))
	c5.Add(fifth, mkAtom("strat-link", StrategyAtom, "strategy.synth.link", Fresh))
	c5.Add(fifth, mkAtom("exec-link", ExecutionAtom, "execution.result.linked", Fresh))
	c5.Add(fifth, mkAtom("obs-link", ObservationAtom, "observation.eval.linked", Fresh))
	c5.Add(fifth, mkAtom("adapt-link", AdaptationAtom, "adaptation.synth.linked", Fresh))
	c5.Add(fifth, mkAtom("retro-same", RetrospectionAtom, "retrospection.learning.same-as-before", Fresh))

	if fifth.Phase() != RetrospectionAtom {
		t.Errorf("fifth should reach retrospection phase, got %s", fifth.Phase())
	}

	firstAssessments := first.Mass(AssessmentAtom)
	fifthAssessments := fifth.Mass(AssessmentAtom)

	if fifthAssessments == 0 {
		t.Error("fifth should have assessment atoms from recollection")
	}

	if fifth.SourceMass(Recollected) == 0 {
		t.Error("fifth molecule should have recollected source mass")
	}
	if first.SourceMass(Recollected) != 0 {
		t.Error("first molecule should have zero recollected source mass")
	}

	_ = firstAssessments
	_ = firstMass
}

func TestGauntlet_Hungry(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("hungry")

	c.Add(m, mkAtom("intent-hungry", IntentAtom, "intent.desire.eat", Fresh))

	c.Add(m, mkAtom("assess-fridge", AssessmentAtom, "assessment.availability.chicken", Fresh))
	c.Add(m, mkAtom("assess-rice", AssessmentAtom, "assessment.availability.rice", Fresh))
	c.Add(m, mkAtom("assess-time", AssessmentAtom, "assessment.state.time-8pm", Fresh))

	if m.Phase() != UnderstandingAtom {
		t.Fatalf("expected understanding phase after assessments, got %s", m.Phase())
	}

	targets := m.ByTaxonomy("intent.target")
	if len(targets) != 0 {
		t.Fatal("should have no intent.target atoms")
	}

	intents := m.Atoms(IntentAtom)
	hasTarget := false
	for _, a := range intents {
		if a.Taxonomy == "intent.target.food" {
			hasTarget = true
		}
	}
	if hasTarget {
		t.Fatal("should not have target yet")
	}

	c.Add(m, mkAtom("intent-target", IntentAtom, "intent.target.chicken-rice", Fresh))

	targets = m.ByTaxonomy("intent.target")
	if len(targets) != 1 {
		t.Fatalf("expected 1 intent.target after human input, got %d", len(targets))
	}

	c.Add(m, mkAtom("understand-hungry", UnderstandingAtom, "understanding.synth.chicken-rice", Fresh))
	c.Add(m, mkAtom("plan-cook", PlanAtom, "plan.task.cook-chicken-rice", Fresh))
	c.Add(m, mkAtom("risk-cook", RiskAtom, "risk.eval.cook", Fresh))
	c.Add(m, mkAtom("strat-cook", StrategyAtom, "strategy.synth.cook", Fresh))
	c.Add(m, mkAtom("exec-cook", ExecutionAtom, "execution.result.cooked", Fresh))
	c.Add(m, mkAtom("obs-cook", ObservationAtom, "observation.eval.cooked", Fresh))
	c.Add(m, mkAtom("adapt-cook", AdaptationAtom, "adaptation.synth.cooked", Fresh))
	c.Add(m, mkAtom("retro-eat", RetrospectionAtom, "retrospection.learning.chicken-rice-good", Fresh))

	if m.Phase() != RetrospectionAtom {
		t.Errorf("should reach retrospection, got %s", m.Phase())
	}
}
