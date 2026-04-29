package reactivity

import (
	"testing"
	"time"
)

func mkAtom(id string, t AtomType, taxonomy string, source AtomSource, targets ...string) Atom {
	return Atom{
		ID:        id,
		Type:      t,
		Source:    source,
		Taxonomy:  taxonomy,
		Content:   []byte(id),
		Targets:   targets,
		CreatedAt: time.Now(),
	}
}

// --- Spark node ---

func TestNode_Spark_ProducesDesire(t *testing.T) {
	c := NewCircuit("test")
	result, _ := c.Add(mkAtom("desire-eat", IntentAtom, "intent.desire.eat", Fresh))
	if result != Pass {
		t.Fatalf("Spark should accept desire atom, got %s", result)
	}
	if c.Mass(IntentAtom) != 1 {
		t.Errorf("expected 1 intent atom, got %d", c.Mass(IntentAtom))
	}
}

func TestNode_Spark_MultipleDesires(t *testing.T) {
	c := NewCircuit("test")
	c.Add(mkAtom("desire-eat", IntentAtom, "intent.desire.eat", Fresh))
	c.Add(mkAtom("desire-rest", IntentAtom, "intent.desire.rest", Fresh))
	if c.Mass(IntentAtom) != 2 {
		t.Errorf("expected 2 intent atoms, got %d", c.Mass(IntentAtom))
	}
}

func TestNode_Spark_SealsOnExhaustion(t *testing.T) {
	c := NewCircuit("test")
	c.Add(mkAtom("desire-eat", IntentAtom, "intent.desire.eat", Fresh))
	if c.Phase() != AssessmentAtom {
		t.Errorf("after intent, should advance to assessment phase, got %s", c.Phase())
	}
}

// --- Assess node ---

func TestNode_Assess_ProducesFindings(t *testing.T) {
	c := NewCircuit("test")
	c.Add(mkAtom("desire", IntentAtom, "intent.desire.eat", Fresh))
	result, _ := c.Add(mkAtom("finding-fridge", AssessmentAtom, "assessment.availability.fridge", Fresh))
	if result != Pass {
		t.Fatalf("Assess should accept finding atom, got %s", result)
	}
	if c.Mass(AssessmentAtom) != 1 {
		t.Errorf("expected 1 assessment atom, got %d", c.Mass(AssessmentAtom))
	}
}

func TestNode_Assess_AcceptedInAnyPhase(t *testing.T) {
	c := NewCircuit("test")
	c.Add(mkAtom("desire", IntentAtom, "intent.desire.eat", Fresh))
	c.Add(mkAtom("finding", AssessmentAtom, "assessment.state.kitchen", Fresh))
	c.Add(mkAtom("task", PlanAtom, "plan.option.cook", Fresh))

	result, _ := c.Add(mkAtom("late-finding", AssessmentAtom, "assessment.state.window", Fresh))
	if result == Incompatible {
		t.Error("Assessment atoms should be accepted in any phase")
	}
}

func TestNode_Assess_RecollectedSource(t *testing.T) {
	c := NewCircuit("test")
	c.Add(mkAtom("desire", IntentAtom, "intent.desire.investigate", Fresh))
	c.Add(mkAtom("recollect-logs", AssessmentAtom, "assessment.state.logs", Recollected))

	if c.SourceMass(Recollected) != 1 {
		t.Errorf("expected 1 recollected atom, got %d", c.SourceMass(Recollected))
	}
}

// --- Expand node ---

func TestNode_Expand_ProducesOptions(t *testing.T) {
	c := NewCircuit("test")
	c.Add(mkAtom("desire", IntentAtom, "intent.desire.eat", Fresh))
	c.Add(mkAtom("finding", AssessmentAtom, "assessment.availability.fridge", Fresh))

	result, _ := c.Add(mkAtom("option-rice", PlanAtom, "plan.option.rice", Fresh))
	if result != Pass {
		t.Fatalf("Expand should accept option atom, got %s", result)
	}
}

func TestNode_Expand_MultipleOptions(t *testing.T) {
	c := NewCircuit("test")
	c.Add(mkAtom("desire", IntentAtom, "intent.desire.eat", Fresh))
	c.Add(mkAtom("finding", AssessmentAtom, "assessment.availability.fridge", Fresh))

	c.Add(mkAtom("opt-rice", PlanAtom, "plan.option.rice", Fresh))
	c.Add(mkAtom("opt-eggs", PlanAtom, "plan.option.eggs", Fresh))
	c.Add(mkAtom("opt-salad", PlanAtom, "plan.option.salad", Fresh))

	if c.Mass(PlanAtom) != 3 {
		t.Errorf("expected 3 plan atoms, got %d", c.Mass(PlanAtom))
	}
}

// --- Drive node ---

func TestNode_Drive_ProducesResults(t *testing.T) {
	c := NewCircuit("test")
	c.Add(mkAtom("desire", IntentAtom, "intent.desire.clean", Fresh))
	c.Add(mkAtom("finding", AssessmentAtom, "assessment.state.dirty", Fresh))
	c.Add(mkAtom("task", PlanAtom, "plan.task.sweep", Fresh))

	result, _ := c.Add(mkAtom("done", ExecutionAtom, "execution.result.swept", Fresh, "task"))
	if result != Pass {
		t.Fatalf("Drive should accept execution atom, got %s", result)
	}
}

func TestNode_Drive_RejectsWithoutPlan(t *testing.T) {
	c := NewCircuit("test")
	c.Add(mkAtom("desire", IntentAtom, "intent.desire.clean", Fresh))

	result, _ := c.Add(mkAtom("premature", ExecutionAtom, "execution.result.swept", Fresh))
	if result != Incompatible {
		t.Errorf("Drive should reject execution without plan, got %s", result)
	}
}

// --- Reflect node ---

func TestNode_Reflect_ProducesWish(t *testing.T) {
	c := NewCircuit("test")
	c.Add(mkAtom("desire", IntentAtom, "intent.desire.clean", Fresh))
	c.Add(mkAtom("finding", AssessmentAtom, "assessment.state.dirty", Fresh))
	c.Add(mkAtom("task", PlanAtom, "plan.task.sweep", Fresh))
	c.Add(mkAtom("done", ExecutionAtom, "execution.result.swept", Fresh))
	c.Add(mkAtom("learning", RetrospectionAtom, "retrospection.learning.done", Fresh))

	c.Seal(mkAtom("wish", RetrospectionAtom, "retrospection.wish.be-cleaner", Fresh))

	if !c.Sealed() {
		t.Error("Circuit should be sealed after Wish")
	}
}

// --- Contradiction ---

func TestNode_Contradict_CrossType(t *testing.T) {
	c := NewCircuit("test")
	c.Add(mkAtom("desire", IntentAtom, "intent.desire.clean", Fresh))
	c.Add(mkAtom("assess-floor", AssessmentAtom, "assessment.state.floor", Fresh))
	c.Add(mkAtom("task-floor", PlanAtom, "plan.task.floor", Fresh, "assess-floor"))
	c.Add(mkAtom("exec-floor", ExecutionAtom, "execution.result.floor", Fresh, "task-floor"))

	dirty := mkAtom("floor-dirty-again", AssessmentAtom, "assessment.state.floor", Fresh, "exec-floor")
	contradicts, existing := c.Contradict(dirty)

	if !contradicts {
		t.Error("expected contradiction between execution.result.floor and assessment.state.floor")
	}
	if existing == nil {
		t.Error("expected conflicting atom")
	}
}

// --- Taxonomy lookup ---

func TestNode_TaxonomyLookup(t *testing.T) {
	c := NewCircuit("test")
	c.Add(mkAtom("desire", IntentAtom, "intent.desire.eat", Fresh))
	c.Add(mkAtom("f1", AssessmentAtom, "assessment.availability.chicken", Fresh))
	c.Add(mkAtom("f2", AssessmentAtom, "assessment.availability.rice", Fresh))
	c.Add(mkAtom("f3", AssessmentAtom, "assessment.state.time", Fresh))

	avail := c.ByTaxonomy("assessment.availability")
	if len(avail) != 2 {
		t.Errorf("expected 2 availability atoms, got %d", len(avail))
	}
}
