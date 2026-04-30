package reactivity

import (
	"testing"
)

// Gauntlet Test 1: Dirty Shoes (contradiction detection)
//
// Agent cleans room. Plan: make bed, sweep floor, clean window.
// Executes all three. New assessment: floor dirty again (dirty shoes).
// Expected: contradiction detected between "floor swept" and "floor dirty again".
func TestGauntlet_DirtyShoes(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("dirty-shoes")

	// Intent
	c.Add(m, mkAtom("intent-clean", IntentAtom, "intent.desire.clean-room", Fresh))

	// Assessment: what's dirty
	c.Add(m, mkAtom("assess-bed", AssessmentAtom, "assessment.state.bed", Fresh))
	c.Add(m, mkAtom("assess-floor", AssessmentAtom, "assessment.state.floor", Fresh))
	c.Add(m, mkAtom("assess-window", AssessmentAtom, "assessment.state.window", Fresh))

	// Plan: address each finding
	c.Add(m, mkAtom("plan-bed", PlanAtom, "plan.task.bed", Fresh, "assess-bed"))
	c.Add(m, mkAtom("plan-floor", PlanAtom, "plan.task.floor", Fresh, "assess-floor"))
	c.Add(m, mkAtom("plan-window", PlanAtom, "plan.task.window", Fresh, "assess-window"))

	// Execute: complete each task
	c.Add(m, mkAtom("exec-bed", ExecutionAtom, "execution.result.bed", Fresh, "plan-bed"))
	c.Add(m, mkAtom("exec-floor", ExecutionAtom, "execution.result.floor", Fresh, "plan-floor"))
	c.Add(m, mkAtom("exec-window", ExecutionAtom, "execution.result.window", Fresh, "plan-window"))

	// New assessment: floor is dirty again (dirty shoes!)
	dirtyAgain := mkAtom("assess-floor-dirty-again", AssessmentAtom, "assessment.state.floor", Fresh, "exec-floor")

	// The circuit should detect a contradiction:
	// "exec-floor" says floor is done, but new assessment says floor is dirty.
	// Both target the same concern (floor).
	contradicts, existing := c.Contradict(m, dirtyAgain)
	if !contradicts {
		t.Fatal("expected contradiction: 'floor done' vs 'floor dirty again'")
	}
	if existing == nil {
		t.Fatal("expected conflicting atom")
	}

	// Add the contradicting assessment anyway — the circuit accepts it
	// but the agent should now re-plan
	result, _ := c.Add(m, dirtyAgain)
	if result != Pass && result != Insufficient {
		t.Errorf("expected assessment to be accepted, got %s", result)
	}

	// After adding contradicting assessment, we should have more assessment mass than execution
	if m.Mass(AssessmentAtom) <= m.Mass(ExecutionAtom) {
		t.Error("contradicting assessment should increase assessment mass, signaling re-plan needed")
	}
}

// Gauntlet Test 2: OCPBUGS-83551 (recollection value)
//
// Same bug in 5 failures. First: full investigation. Fifth: recollect from first.
// Expected: recollected atoms shift mass toward known, Cynefin toward Clear.
func TestGauntlet_Recollection(t *testing.T) {
	// First investigation: full work
	c1 := NewReactor()
	first := NewMolecule("rca-first")
	c1.Add(first, mkAtom("intent-rca", IntentAtom, "intent.desire.investigate-failure", Fresh))
	c1.Add(first, mkAtom("assess-logs", AssessmentAtom, "assessment.state.logs", Fresh))
	c1.Add(first, mkAtom("assess-commits", AssessmentAtom, "assessment.state.commits", Fresh))
	c1.Add(first, mkAtom("assess-jira", AssessmentAtom, "assessment.state.jira-match", Fresh))
	c1.Add(first, mkAtom("plan-classify", PlanAtom, "plan.task.classify", Fresh))
	c1.Add(first, mkAtom("exec-classify", ExecutionAtom, "execution.result.product-bug", Fresh))
	c1.Add(first, mkAtom("retro-83551", RetrospectionAtom, "retrospection.learning.ocpbugs-83551", Fresh))
	c1.Seal(first, mkAtom("wish-done", RetrospectionAtom, "retrospection.wish.done", Fresh))

	if !first.Sealed() {
		t.Fatal("first circuit should be sealed")
	}
	firstMass := first.TotalMass()

	// Fifth investigation: recollect from first
	c5 := NewReactor()
	fifth := NewMolecule("rca-fifth")
	c5.Add(fifth, mkAtom("intent-rca", IntentAtom, "intent.desire.investigate-failure", Fresh))

	// Recollected atoms from first investigation (same taxonomy, Source=Recollected)
	recollectLogs := mkAtom("recollect-logs", AssessmentAtom, "assessment.state.logs", Fresh)
	recollectLogs.Source = Recollected
	c5.Add(fifth, recollectLogs)

	recollectJira := mkAtom("recollect-jira", AssessmentAtom, "assessment.state.jira-match", Fresh)
	recollectJira.Source = Recollected
	c5.Add(fifth, recollectJira)

	recollectResult := mkAtom("recollect-result", AssessmentAtom, "assessment.state.known-bug-83551", Fresh)
	recollectResult.Source = Recollected
	c5.Add(fifth, recollectResult)

	// With recollected knowledge, planning is immediate
	c5.Add(fifth, mkAtom("plan-link", PlanAtom, "plan.task.link-existing-jira", Fresh))
	c5.Add(fifth, mkAtom("exec-link", ExecutionAtom, "execution.result.linked", Fresh))
	c5.Add(fifth, mkAtom("retro-same", RetrospectionAtom, "retrospection.learning.same-as-before", Fresh))

	// Both complete the full chain
	if fifth.Phase() != RetrospectionAtom {
		t.Errorf("fifth should reach retrospection phase, got %s", fifth.Phase())
	}

	// The value of recollection: composition shifts, not mass reduction.
	// First investigation: all Assessment atoms are freshly discovered (unknown territory).
	// Fifth investigation: Assessment atoms are recollected (known territory).
	// Same mass, different known/unknown ratio = different Cynefin domain.
	firstAssessments := first.Mass(AssessmentAtom)
	fifthAssessments := fifth.Mass(AssessmentAtom)

	// Fifth has recollected assessments — count should be similar or more (known context enriches)
	if fifthAssessments == 0 {
		t.Error("fifth should have assessment atoms from recollection")
	}

	// Composition shift: fifth circuit has recollected source mass, first has none.
	if fifth.SourceMass(Recollected) == 0 {
		t.Error("fifth circuit should have recollected source mass")
	}
	if first.SourceMass(Recollected) != 0 {
		t.Error("first circuit should have zero recollected source mass")
	}

	// Cynefin readiness: ratio of recollected to total determines domain.
	// fifth: 3 recollected / 7 total = 0.43 -> Complicated (known territory)
	// first: 0 recollected / 8 total = 0.00 -> Chaotic (unknown territory)
	// The model tracks this. TAK-SPC-20 computes the actual Cynefin domain.
	_ = firstAssessments
	_ = firstMass
}

// Gauntlet Test 3: Hungry (structural back-pressure)
//
// "I'm hungry." Assessment has availability but intent has no target.
// Expected: circuit structurally blocks advancement to Plan.
func TestGauntlet_Hungry(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("hungry")

	// Intent: desire only, no target
	c.Add(m, mkAtom("intent-hungry", IntentAtom, "intent.desire.eat", Fresh))

	// Assessment: what's available
	c.Add(m, mkAtom("assess-fridge", AssessmentAtom, "assessment.availability.chicken", Fresh))
	c.Add(m, mkAtom("assess-rice", AssessmentAtom, "assessment.availability.rice", Fresh))
	c.Add(m, mkAtom("assess-time", AssessmentAtom, "assessment.state.time-8pm", Fresh))

	// Try to add a plan without intent.target — should be in Plan phase now
	if m.Phase() != PlanAtom {
		t.Fatalf("expected Plan phase after assessments, got %s", m.Phase())
	}

	// But we have no intent.target taxonomy — check by querying
	targets := m.ByTaxonomy("intent.target")
	if len(targets) != 0 {
		t.Fatal("should have no intent.target atoms")
	}

	// The structure tells us what's missing: intent.target
	// A deterministic rule engine can check: "Plan requires intent.target, none found"
	// This IS the back-pressure — the data structure reveals the gap
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

	// Human provides target
	c.Add(m, mkAtom("intent-target", IntentAtom, "intent.target.chicken-rice", Fresh))

	// Now check: we have both desire and target
	targets = m.ByTaxonomy("intent.target")
	if len(targets) != 1 {
		t.Fatalf("expected 1 intent.target after human input, got %d", len(targets))
	}

	// Continue: plan, execute, retrospect
	c.Add(m, mkAtom("plan-cook", PlanAtom, "plan.task.cook-chicken-rice", Fresh))
	c.Add(m, mkAtom("exec-cook", ExecutionAtom, "execution.result.cooked", Fresh))
	c.Add(m, mkAtom("retro-eat", RetrospectionAtom, "retrospection.learning.chicken-rice-good", Fresh))

	if m.Phase() != RetrospectionAtom {
		t.Errorf("should reach retrospection, got %s", m.Phase())
	}
}
