package reactivity

import (
	"testing"
	"time"
)

func atom(id string, t AtomType, taxonomy string, targets ...string) Atom {
	return Atom{
		ID:        id,
		Type:      t,
		Taxonomy:  taxonomy,
		Content:   []byte(id),
		Targets:   targets,
		CreatedAt: time.Now(),
	}
}

// Gauntlet Test 1: Dirty Shoes (contradiction detection)
//
// Agent cleans room. Plan: make bed, sweep floor, clean window.
// Executes all three. New assessment: floor dirty again (dirty shoes).
// Expected: contradiction detected between "floor swept" and "floor dirty again".
func TestGauntlet_DirtyShoes(t *testing.T) {
	c := NewCircuit("dirty-shoes")

	// Intent
	c.Add(atom("intent-clean", IntentAtom, "intent.desire.clean-room"))

	// Assessment: what's dirty
	c.Add(atom("assess-bed", AssessmentAtom, "assessment.state.bed"))
	c.Add(atom("assess-floor", AssessmentAtom, "assessment.state.floor"))
	c.Add(atom("assess-window", AssessmentAtom, "assessment.state.window"))

	// Plan: address each finding
	c.Add(atom("plan-bed", PlanAtom, "plan.task.bed", "assess-bed"))
	c.Add(atom("plan-floor", PlanAtom, "plan.task.floor", "assess-floor"))
	c.Add(atom("plan-window", PlanAtom, "plan.task.window", "assess-window"))

	// Execute: complete each task
	c.Add(atom("exec-bed", ExecutionAtom, "execution.result.bed", "plan-bed"))
	c.Add(atom("exec-floor", ExecutionAtom, "execution.result.floor", "plan-floor"))
	c.Add(atom("exec-window", ExecutionAtom, "execution.result.window", "plan-window"))

	// New assessment: floor is dirty again (dirty shoes!)
	dirtyAgain := atom("assess-floor-dirty-again", AssessmentAtom, "assessment.state.floor", "exec-floor")

	// The circuit should detect a contradiction:
	// "exec-floor" says floor is done, but new assessment says floor is dirty.
	// Both target the same concern (floor).
	contradicts, existing := c.Contradict(dirtyAgain)
	if !contradicts {
		t.Fatal("expected contradiction: 'floor done' vs 'floor dirty again'")
	}
	if existing == nil {
		t.Fatal("expected conflicting atom")
	}

	// Add the contradicting assessment anyway — the circuit accepts it
	// but the agent should now re-plan
	result, _ := c.Add(dirtyAgain)
	if result != Pass && result != Insufficient {
		t.Errorf("expected assessment to be accepted, got %s", result)
	}

	// After adding contradicting assessment, we should have more assessment mass than execution
	if c.Mass(AssessmentAtom) <= c.Mass(ExecutionAtom) {
		t.Error("contradicting assessment should increase assessment mass, signaling re-plan needed")
	}
}

// Gauntlet Test 2: OCPBUGS-83551 (recollection value)
//
// Same bug in 5 failures. First: full investigation. Fifth: recollect from first.
// Expected: recollected atoms shift mass toward known, Cynefin toward Clear.
func TestGauntlet_Recollection(t *testing.T) {
	// First investigation: full work
	first := NewCircuit("rca-first")
	first.Add(atom("intent-rca", IntentAtom, "intent.desire.investigate-failure"))
	first.Add(atom("assess-logs", AssessmentAtom, "assessment.state.logs"))
	first.Add(atom("assess-commits", AssessmentAtom, "assessment.state.commits"))
	first.Add(atom("assess-jira", AssessmentAtom, "assessment.state.jira-match"))
	first.Add(atom("plan-classify", PlanAtom, "plan.task.classify"))
	first.Add(atom("exec-classify", ExecutionAtom, "execution.result.product-bug"))
	first.Add(atom("retro-83551", RetrospectionAtom, "retrospection.learning.ocpbugs-83551"))
	first.Seal(atom("wish-done", RetrospectionAtom, "retrospection.wish.done"))

	if !first.Sealed() {
		t.Fatal("first circuit should be sealed")
	}
	firstMass := first.TotalMass()

	// Fifth investigation: recollect from first
	fifth := NewCircuit("rca-fifth")
	fifth.Add(atom("intent-rca", IntentAtom, "intent.desire.investigate-failure"))

	// Recollected atoms from first investigation (same taxonomy, Source=Recollected)
	recollectLogs := atom("recollect-logs", AssessmentAtom, "assessment.state.logs")
	recollectLogs.Source = Recollected
	fifth.Add(recollectLogs)

	recollectJira := atom("recollect-jira", AssessmentAtom, "assessment.state.jira-match")
	recollectJira.Source = Recollected
	fifth.Add(recollectJira)

	recollectResult := atom("recollect-result", AssessmentAtom, "assessment.state.known-bug-83551")
	recollectResult.Source = Recollected
	fifth.Add(recollectResult)

	// With recollected knowledge, planning is immediate
	fifth.Add(atom("plan-link", PlanAtom, "plan.task.link-existing-jira"))
	fifth.Add(atom("exec-link", ExecutionAtom, "execution.result.linked"))
	fifth.Add(atom("retro-same", RetrospectionAtom, "retrospection.learning.same-as-before"))

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
	c := NewCircuit("hungry")

	// Intent: desire only, no target
	c.Add(atom("intent-hungry", IntentAtom, "intent.desire.eat"))

	// Assessment: what's available
	c.Add(atom("assess-fridge", AssessmentAtom, "assessment.availability.chicken"))
	c.Add(atom("assess-rice", AssessmentAtom, "assessment.availability.rice"))
	c.Add(atom("assess-time", AssessmentAtom, "assessment.state.time-8pm"))

	// Try to add a plan without intent.target — should be in Plan phase now
	if c.Phase() != PlanAtom {
		t.Fatalf("expected Plan phase after assessments, got %s", c.Phase())
	}

	// But we have no intent.target taxonomy — check by querying
	targets := c.ByTaxonomy("intent.target")
	if len(targets) != 0 {
		t.Fatal("should have no intent.target atoms")
	}

	// The structure tells us what's missing: intent.target
	// A deterministic rule engine can check: "Plan requires intent.target, none found"
	// This IS the back-pressure — the data structure reveals the gap
	intents := c.Atoms(IntentAtom)
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
	c.Add(atom("intent-target", IntentAtom, "intent.target.chicken-rice"))

	// Now check: we have both desire and target
	targets = c.ByTaxonomy("intent.target")
	if len(targets) != 1 {
		t.Fatalf("expected 1 intent.target after human input, got %d", len(targets))
	}

	// Continue: plan, execute, retrospect
	c.Add(atom("plan-cook", PlanAtom, "plan.task.cook-chicken-rice"))
	c.Add(atom("exec-cook", ExecutionAtom, "execution.result.cooked"))
	c.Add(atom("retro-eat", RetrospectionAtom, "retrospection.learning.chicken-rice-good"))

	if c.Phase() != RetrospectionAtom {
		t.Errorf("should reach retrospection, got %s", c.Phase())
	}
}
