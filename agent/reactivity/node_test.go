package reactivity

import (
	"testing"
)

func TestNode_Intent_ProducesDesire(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("test")
	result, _ := c.Add(m, mkAtom("desire-eat", IntentAtom, "intent.desire.eat", Fresh))
	if result != Pass {
		t.Fatalf("should accept intent atom, got %s", result)
	}
	if m.Mass(IntentAtom) != 1 {
		t.Errorf("expected 1 intent atom, got %d", m.Mass(IntentAtom))
	}
}

func TestNode_Intent_MultipleDesires(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("test")
	c.Add(m, mkAtom("desire-eat", IntentAtom, "intent.desire.eat", Fresh))
	c.Add(m, mkAtom("desire-rest", IntentAtom, "intent.desire.rest", Fresh))
	if m.Mass(IntentAtom) != 2 {
		t.Errorf("expected 2 intent atoms, got %d", m.Mass(IntentAtom))
	}
}

func TestNode_Intent_AdvancesToAssessment(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("test")
	c.Add(m, mkAtom("desire-eat", IntentAtom, "intent.desire.eat", Fresh))
	if m.Phase() != AssessmentAtom {
		t.Errorf("after intent, should advance to assessment phase, got %s", m.Phase())
	}
}

func TestNode_Assessment_ProducesFindings(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("test")
	c.Add(m, mkAtom("desire", IntentAtom, "intent.desire.eat", Fresh))
	result, _ := c.Add(m, mkAtom("finding-fridge", AssessmentAtom, "assessment.availability.fridge", Fresh))
	if result != Pass {
		t.Fatalf("should accept assessment atom, got %s", result)
	}
	if m.Mass(AssessmentAtom) != 1 {
		t.Errorf("expected 1 assessment atom, got %d", m.Mass(AssessmentAtom))
	}
}

func TestNode_Assessment_AdvancesToUnderstanding(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("test")
	c.Add(m, mkAtom("desire", IntentAtom, "intent.desire.eat", Fresh))
	c.Add(m, mkAtom("finding", AssessmentAtom, "assessment.state.kitchen", Fresh))
	if m.Phase() != KnowledgeAtom {
		t.Errorf("after assessment, should advance to understanding phase, got %s", m.Phase())
	}
}

func TestNode_Assessment_AcceptedInAnyPhase(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("test")
	addReasonAtoms(c, m, "eat")
	addFormationAtoms(c, m, "eat")

	result, _ := c.Add(m, mkAtom("late-finding", AssessmentAtom, "assessment.state.window", Fresh))
	if result == Incompatible {
		t.Error("assessment atoms should be accepted in any phase")
	}
}

func TestNode_Assessment_RecollectedSource(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("test")
	c.Add(m, mkAtom("desire", IntentAtom, "intent.desire.investigate", Fresh))
	c.Add(m, mkAtom("recollect-logs", AssessmentAtom, "assessment.state.logs", Recollected))

	if m.SourceMass(Recollected) != 1 {
		t.Errorf("expected 1 recollected atom, got %d", m.SourceMass(Recollected))
	}
}

func TestNode_Understanding_SealsThinkTriad(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("test")
	c.Add(m, mkAtom("desire", IntentAtom, "intent.desire.eat", Fresh))
	c.Add(m, mkAtom("finding", AssessmentAtom, "assessment.availability.fridge", Fresh))
	c.Add(m, mkAtom("synth", KnowledgeAtom, "understanding.synth.eat", Fresh))

	if !m.TriadSealed(ThinkTriad) {
		t.Error("reason triad should seal after understanding atom")
	}
	if m.Phase() != ExpansionAtom {
		t.Errorf("after understanding, should advance to plan phase, got %s", m.Phase())
	}
}

func TestNode_Plan_ProducesOptions(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("test")
	addReasonAtoms(c, m, "eat")

	result, _ := c.Add(m, mkAtom("option-rice", ExpansionAtom, "plan.option.rice", Fresh))
	if result != Pass {
		t.Fatalf("should accept plan atom, got %s", result)
	}
}

func TestNode_Plan_MultipleOptions(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("test")
	addReasonAtoms(c, m, "eat")

	c.Add(m, mkAtom("opt-rice", ExpansionAtom, "plan.option.rice", Fresh))
	c.Add(m, mkAtom("opt-eggs", ExpansionAtom, "plan.option.eggs", Fresh))
	c.Add(m, mkAtom("opt-salad", ExpansionAtom, "plan.option.salad", Fresh))

	if m.Mass(ExpansionAtom) != 3 {
		t.Errorf("expected 3 plan atoms, got %d", m.Mass(ExpansionAtom))
	}
}

func TestNode_Risk_AdvancesToStrategy(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("test")
	addReasonAtoms(c, m, "eat")
	c.Add(m, mkAtom("task", ExpansionAtom, "plan.task.cook", Fresh))
	c.Add(m, mkAtom("risk", ReductionAtom, "risk.eval.burn", Fresh))

	if m.Phase() != SelectionAtom {
		t.Errorf("after risk, should advance to strategy phase, got %s", m.Phase())
	}
}

func TestNode_Strategy_SealsComposeTriad(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("test")
	addReasonAtoms(c, m, "eat")
	addFormationAtoms(c, m, "eat")

	if !m.TriadSealed(ComposeTriad) {
		t.Error("plan triad should seal after strategy atom")
	}
	if m.Phase() != ExecutionAtom {
		t.Errorf("after strategy, should advance to execution phase, got %s", m.Phase())
	}
}

func TestNode_Execution_ProducesResults(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("test")
	addReasonAtoms(c, m, "clean")
	addFormationAtoms(c, m, "clean")

	result, _ := c.Add(m, mkAtom("done", ExecutionAtom, "execution.result.swept", Fresh))
	if result != Pass {
		t.Fatalf("should accept execution atom, got %s", result)
	}
}

func TestNode_Execution_RejectsWithoutFormation(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("test")
	c.Add(m, mkAtom("desire", IntentAtom, "intent.desire.clean", Fresh))

	result, _ := c.Add(m, mkAtom("premature", ExecutionAtom, "execution.result.swept", Fresh))
	if result != Incompatible {
		t.Errorf("should reject execution without formation, got %s", result)
	}
}

func TestNode_Observation_AdvancesToAdaptation(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("test")
	addReasonAtoms(c, m, "clean")
	addFormationAtoms(c, m, "clean")
	c.Add(m, mkAtom("exec", ExecutionAtom, "execution.result.swept", Fresh))
	c.Add(m, mkAtom("obs", AcclimationAtom, "observation.eval.swept", Fresh))

	if m.Phase() != RefinementAtom {
		t.Errorf("after observation, should advance to adaptation phase, got %s", m.Phase())
	}
}

func TestNode_Adaptation_SealsActionTriad(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("test")
	addReasonAtoms(c, m, "clean")
	addFormationAtoms(c, m, "clean")
	addActionAtoms(c, m, "clean")

	if !m.TriadSealed(ActionTriad) {
		t.Error("act triad should seal after adaptation atom")
	}
	if m.Phase() != RetrospectionAtom {
		t.Errorf("after adaptation, should advance to retrospection phase, got %s", m.Phase())
	}
}

func TestNode_Retrospection_ProducesLearning(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("test")
	addReasonAtoms(c, m, "clean")
	addFormationAtoms(c, m, "clean")
	addActionAtoms(c, m, "clean")
	c.Add(m, mkAtom("learning", RetrospectionAtom, "retrospection.learning.done", Fresh))

	c.Seal(m, mkAtom("wish", RetrospectionAtom, "retrospection.wish.be-cleaner", Fresh))

	if !m.Sealed() {
		t.Error("should be sealed after wish")
	}
}

func TestNode_Contradict_CrossType(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("test")
	c.Add(m, mkAtom("desire", IntentAtom, "intent.desire.clean", Fresh))
	c.Add(m, mkAtom("assess-floor", AssessmentAtom, "assessment.state.floor", Fresh))
	c.Add(m, mkAtom("understand", KnowledgeAtom, "understanding.synth.clean", Fresh))
	c.Add(m, mkAtom("task-floor", ExpansionAtom, "plan.task.floor", Fresh, "assess-floor"))
	c.Add(m, mkAtom("risk-floor", ReductionAtom, "risk.eval.floor", Fresh))
	c.Add(m, mkAtom("strat-floor", SelectionAtom, "strategy.synth.floor", Fresh))
	c.Add(m, mkAtom("exec-floor", ExecutionAtom, "execution.result.floor", Fresh, "task-floor"))

	dirty := mkAtom("floor-dirty-again", AssessmentAtom, "assessment.state.floor", Fresh, "exec-floor")
	contradicts, existing := c.Contradict(m, dirty)

	if !contradicts {
		t.Error("expected contradiction between execution.result.floor and assessment.state.floor")
	}
	if existing == nil {
		t.Error("expected conflicting atom")
	}
}

func TestNode_TaxonomyLookup(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("test")
	c.Add(m, mkAtom("desire", IntentAtom, "intent.desire.eat", Fresh))
	c.Add(m, mkAtom("f1", AssessmentAtom, "assessment.availability.chicken", Fresh))
	c.Add(m, mkAtom("f2", AssessmentAtom, "assessment.availability.rice", Fresh))
	c.Add(m, mkAtom("f3", AssessmentAtom, "assessment.state.time", Fresh))

	avail := m.ByTaxonomy("assessment.availability")
	if len(avail) != 2 {
		t.Errorf("expected 2 availability atoms, got %d", len(avail))
	}
}
