package cerebrum

import (
	"testing"
	"time"

	"github.com/dpopsuev/tako/agent/reactivity"
)

func TestTriage_Inception_NewDomain(t *testing.T) {
	store := reactivity.NewMoleculeStore()
	reactor := reactivity.NewReactor()

	store.Receive(reactivity.Atom{
		ID:        "a1",
		Type:      reactivity.IntentAtom,
		Taxonomy:  "intent.goal.ptp",
		Content:   []byte("investigate PTP"),
		CreatedAt: time.Now(),
	})

	results := Route(store, reactor)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Verdict != Inception {
		t.Errorf("expected inception, got %s", results[0].Verdict)
	}
}

func TestTriage_Continuation_SameDomain(t *testing.T) {
	store := reactivity.NewMoleculeStore()
	reactor := reactivity.NewReactor()

	m := store.Focus("mol-ptp")
	reactor.Add(m, reactivity.Atom{
		ID:        "existing",
		Type:      reactivity.IntentAtom,
		Taxonomy:  "intent.goal.ptp",
		Content:   []byte("existing intent"),
		CreatedAt: time.Now(),
	})
	store.Park()

	store.Receive(reactivity.Atom{
		ID:        "a2",
		Type:      reactivity.AssessmentAtom,
		Taxonomy:  "assessment.eval.ptp",
		Content:   []byte("assess PTP"),
		CreatedAt: time.Now(),
	})

	results := Route(store, reactor)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Verdict != Continuation {
		t.Errorf("expected continuation, got %s", results[0].Verdict)
	}
	if results[0].MoleculeID != "mol-ptp" {
		t.Errorf("expected mol-ptp, got %s", results[0].MoleculeID)
	}
}

func TestTriage_Contradiction_ConflictingAtom(t *testing.T) {
	store := reactivity.NewMoleculeStore()
	reactor := reactivity.NewReactor()

	m := store.Focus("mol-test")
	reactor.Add(m, reactivity.Atom{
		ID:        "intent-a",
		Type:      reactivity.IntentAtom,
		Taxonomy:  "intent.goal.deploy",
		Content:   []byte("deploy v1"),
		CreatedAt: time.Now(),
	})
	store.Park()

	store.Receive(reactivity.Atom{
		ID:        "intent-b",
		Type:      reactivity.IntentAtom,
		Taxonomy:  "intent.revised.deploy",
		Content:   []byte("deploy v2 instead"),
		CreatedAt: time.Now(),
	})

	results := Route(store, reactor)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Verdict != Contradiction {
		t.Errorf("expected contradiction, got %s", results[0].Verdict)
	}
	if results[0].Conflict == nil {
		t.Error("expected conflict atom")
	}
}

func TestTriage_MultiplAtoms_MixedVerdicts(t *testing.T) {
	store := reactivity.NewMoleculeStore()
	reactor := reactivity.NewReactor()

	store.Receive(reactivity.Atom{
		ID: "a1", Type: reactivity.IntentAtom,
		Taxonomy: "intent.goal.alpha", Content: []byte("alpha"), CreatedAt: time.Now(),
	})
	store.Receive(reactivity.Atom{
		ID: "a2", Type: reactivity.IntentAtom,
		Taxonomy: "intent.goal.beta", Content: []byte("beta"), CreatedAt: time.Now(),
	})

	results := Route(store, reactor)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	for _, r := range results {
		if r.Verdict != Inception {
			t.Errorf("expected inception for new domain, got %s", r.Verdict)
		}
	}
}

func TestTriage_DrainsUnsorted(t *testing.T) {
	store := reactivity.NewMoleculeStore()
	reactor := reactivity.NewReactor()

	store.Receive(reactivity.Atom{
		ID: "a1", Type: reactivity.IntentAtom,
		Taxonomy: "intent.goal.test", Content: []byte("t"), CreatedAt: time.Now(),
	})

	Route(store, reactor)

	if len(store.Unsorted()) != 0 {
		t.Error("unsorted should be empty after triage")
	}
}

func TestRouteVerdict_String(t *testing.T) {
	cases := []struct {
		v    RouteVerdict
		want string
	}{
		{Inception, "inception"},
		{Continuation, "continuation"},
		{Contradiction, "contradiction"},
		{RouteVerdict(99), "unknown"},
	}
	for _, tc := range cases {
		if got := tc.v.String(); got != tc.want {
			t.Errorf("RouteVerdict(%d).String() = %q, want %q", tc.v, got, tc.want)
		}
	}
}
