package reactivity

import (
	"testing"
)

// --- Spark node ---

func TestNode_Spark_ProducesDesire(t *testing.T) {
	c := NewCircuit()
	m := NewMolecule("test")
	result, _ := c.Add(m, mkAtom("desire-eat", IntentAtom, "intent.desire.eat", Fresh))
	if result != Pass {
		t.Fatalf("Spark should accept desire atom, got %s", result)
	}
	if m.Mass(IntentAtom) != 1 {
		t.Errorf("expected 1 intent atom, got %d", m.Mass(IntentAtom))
	}
}

func TestNode_Spark_MultipleDesires(t *testing.T) {
	c := NewCircuit()
	m := NewMolecule("test")
	c.Add(m, mkAtom("desire-eat", IntentAtom, "intent.desire.eat", Fresh))
	c.Add(m, mkAtom("desire-rest", IntentAtom, "intent.desire.rest", Fresh))
	if m.Mass(IntentAtom) != 2 {
		t.Errorf("expected 2 intent atoms, got %d", m.Mass(IntentAtom))
	}
}

func TestNode_Spark_SealsOnExhaustion(t *testing.T) {
	c := NewCircuit()
	m := NewMolecule("test")
	c.Add(m, mkAtom("desire-eat", IntentAtom, "intent.desire.eat", Fresh))
	if m.Phase() != AssessmentAtom {
		t.Errorf("after intent, should advance to assessment phase, got %s", m.Phase())
	}
}

// --- Assess node ---

func TestNode_Assess_ProducesFindings(t *testing.T) {
	c := NewCircuit()
	m := NewMolecule("test")
	c.Add(m, mkAtom("desire", IntentAtom, "intent.desire.eat", Fresh))
	result, _ := c.Add(m, mkAtom("finding-fridge", AssessmentAtom, "assessment.availability.fridge", Fresh))
	if result != Pass {
		t.Fatalf("Assess should accept finding atom, got %s", result)
	}
	if m.Mass(AssessmentAtom) != 1 {
		t.Errorf("expected 1 assessment atom, got %d", m.Mass(AssessmentAtom))
	}
}

func TestNode_Assess_AcceptedInAnyPhase(t *testing.T) {
	c := NewCircuit()
	m := NewMolecule("test")
	c.Add(m, mkAtom("desire", IntentAtom, "intent.desire.eat", Fresh))
	c.Add(m, mkAtom("finding", AssessmentAtom, "assessment.state.kitchen", Fresh))
	c.Add(m, mkAtom("task", PlanAtom, "plan.option.cook", Fresh))

	result, _ := c.Add(m, mkAtom("late-finding", AssessmentAtom, "assessment.state.window", Fresh))
	if result == Incompatible {
		t.Error("Assessment atoms should be accepted in any phase")
	}
}

func TestNode_Assess_RecollectedSource(t *testing.T) {
	c := NewCircuit()
	m := NewMolecule("test")
	c.Add(m, mkAtom("desire", IntentAtom, "intent.desire.investigate", Fresh))
	c.Add(m, mkAtom("recollect-logs", AssessmentAtom, "assessment.state.logs", Recollected))

	if m.SourceMass(Recollected) != 1 {
		t.Errorf("expected 1 recollected atom, got %d", m.SourceMass(Recollected))
	}
}

// --- Expand node ---

func TestNode_Expand_ProducesOptions(t *testing.T) {
	c := NewCircuit()
	m := NewMolecule("test")
	c.Add(m, mkAtom("desire", IntentAtom, "intent.desire.eat", Fresh))
	c.Add(m, mkAtom("finding", AssessmentAtom, "assessment.availability.fridge", Fresh))

	result, _ := c.Add(m, mkAtom("option-rice", PlanAtom, "plan.option.rice", Fresh))
	if result != Pass {
		t.Fatalf("Expand should accept option atom, got %s", result)
	}
}

func TestNode_Expand_MultipleOptions(t *testing.T) {
	c := NewCircuit()
	m := NewMolecule("test")
	c.Add(m, mkAtom("desire", IntentAtom, "intent.desire.eat", Fresh))
	c.Add(m, mkAtom("finding", AssessmentAtom, "assessment.availability.fridge", Fresh))

	c.Add(m, mkAtom("opt-rice", PlanAtom, "plan.option.rice", Fresh))
	c.Add(m, mkAtom("opt-eggs", PlanAtom, "plan.option.eggs", Fresh))
	c.Add(m, mkAtom("opt-salad", PlanAtom, "plan.option.salad", Fresh))

	if m.Mass(PlanAtom) != 3 {
		t.Errorf("expected 3 plan atoms, got %d", m.Mass(PlanAtom))
	}
}

// --- Drive node ---

func TestNode_Drive_ProducesResults(t *testing.T) {
	c := NewCircuit()
	m := NewMolecule("test")
	c.Add(m, mkAtom("desire", IntentAtom, "intent.desire.clean", Fresh))
	c.Add(m, mkAtom("finding", AssessmentAtom, "assessment.state.dirty", Fresh))
	c.Add(m, mkAtom("task", PlanAtom, "plan.task.sweep", Fresh))

	result, _ := c.Add(m, mkAtom("done", ExecutionAtom, "execution.result.swept", Fresh, "task"))
	if result != Pass {
		t.Fatalf("Drive should accept execution atom, got %s", result)
	}
}

func TestNode_Drive_RejectsWithoutPlan(t *testing.T) {
	c := NewCircuit()
	m := NewMolecule("test")
	c.Add(m, mkAtom("desire", IntentAtom, "intent.desire.clean", Fresh))

	result, _ := c.Add(m, mkAtom("premature", ExecutionAtom, "execution.result.swept", Fresh))
	if result != Incompatible {
		t.Errorf("Drive should reject execution without plan, got %s", result)
	}
}

// --- Reflect node ---

func TestNode_Reflect_ProducesWish(t *testing.T) {
	c := NewCircuit()
	m := NewMolecule("test")
	c.Add(m, mkAtom("desire", IntentAtom, "intent.desire.clean", Fresh))
	c.Add(m, mkAtom("finding", AssessmentAtom, "assessment.state.dirty", Fresh))
	c.Add(m, mkAtom("task", PlanAtom, "plan.task.sweep", Fresh))
	c.Add(m, mkAtom("done", ExecutionAtom, "execution.result.swept", Fresh))
	c.Add(m, mkAtom("learning", RetrospectionAtom, "retrospection.learning.done", Fresh))

	c.Seal(m, mkAtom("wish", RetrospectionAtom, "retrospection.wish.be-cleaner", Fresh))

	if !m.Sealed() {
		t.Error("Circuit should be sealed after Wish")
	}
}

// --- Contradiction ---

func TestNode_Contradict_CrossType(t *testing.T) {
	c := NewCircuit()
	m := NewMolecule("test")
	c.Add(m, mkAtom("desire", IntentAtom, "intent.desire.clean", Fresh))
	c.Add(m, mkAtom("assess-floor", AssessmentAtom, "assessment.state.floor", Fresh))
	c.Add(m, mkAtom("task-floor", PlanAtom, "plan.task.floor", Fresh, "assess-floor"))
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

// --- Taxonomy lookup ---

func TestNode_TaxonomyLookup(t *testing.T) {
	c := NewCircuit()
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
