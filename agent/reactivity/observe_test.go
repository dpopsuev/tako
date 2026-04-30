package reactivity

import (
	"context"
	"testing"

	"github.com/dpopsuev/tako/ergograph"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestObserve_AddEmitsRecordAndSpan(t *testing.T) {
	pool := &ergograph.StubPool{}
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	c := NewReactor(WithPool(pool), WithTracer(tp.Tracer("test")))
	m := NewMolecule("observed")
	c.Add(m, mkAtom("desire", IntentAtom, "intent.desire.eat", Fresh))

	if pool.Len() < 1 {
		t.Errorf("expected at least 1 ergograph record, got %d", pool.Len())
	}

	recs := pool.Records()
	found := false
	for _, r := range recs {
		if r.Action == "reactor.react" {
			found = true
			if r.Labels[labelType] != "intent" {
				t.Errorf("expected label type=intent, got %q", r.Labels[labelType])
			}
		}
	}
	if !found {
		t.Error("expected record with action 'reactor.react' action")
	}

	spans := exporter.GetSpans()
	spanFound := false
	for _, s := range spans {
		if s.Name == "reactor.react" {
			spanFound = true
		}
	}
	if !spanFound {
		t.Error("expected OTel span named 'reactor.react' action")
	}
}

func TestObserve_TriadSealEmitsRecord(t *testing.T) {
	pool := &ergograph.StubPool{}
	c := NewReactor(WithPool(pool))
	m := NewMolecule("observed")

	c.Add(m, mkAtom("desire", IntentAtom, "intent.desire.eat", Fresh))
	c.Add(m, mkAtom("finding", AssessmentAtom, "assessment.availability.fridge", Fresh))

	found := false
	for _, r := range pool.Records() {
		if r.Action == "reactor.react" && r.Labels[labelTriad] == "reason" {
			found = true
		}
	}
	if !found {
		t.Error("expected record for Reason triad seal")
	}
}

func TestObserve_UnsealEmitsRecord(t *testing.T) {
	pool := &ergograph.StubPool{}
	c := NewReactor(WithPool(pool))
	m := NewMolecule("observed")

	c.Add(m, mkAtom("desire", IntentAtom, "intent.desire.clean", Fresh))
	c.Add(m, mkAtom("finding", AssessmentAtom, "assessment.state.dirty", Fresh))
	c.Add(m, mkAtom("task", PlanAtom, "plan.task.sweep", Fresh))

	c.UnsealTriad(m, PlanTriad)

	found := false
	for _, r := range pool.Records() {
		if r.Action == "reactor.triad.unseal" && r.Labels[labelTriad] == "plan" {
			found = true
		}
	}
	if !found {
		t.Error("expected record for Plan triad unseal")
	}
}

func TestObserve_SealEmitsRecord(t *testing.T) {
	pool := &ergograph.StubPool{}
	c := NewReactor(WithPool(pool))
	m := NewMolecule("observed")

	c.Add(m, mkAtom("desire", IntentAtom, "intent.desire.eat", Fresh))
	c.Seal(m, mkAtom("wish", RetrospectionAtom, "retrospection.wish.done", Fresh))

	found := false
	for _, r := range pool.Records() {
		if r.Action == "reactor.seal" {
			found = true
			if r.Labels[labelWish] != "wish" {
				t.Errorf("expected wish label, got %q", r.Labels[labelWish])
			}
		}
	}
	if !found {
		t.Error("expected record for circuit seal")
	}
}

func TestObserve_NoPoolNoError(t *testing.T) {
	c := NewReactor()
	m := NewMolecule("no-pool")
	c.Add(m, mkAtom("desire", IntentAtom, "intent.desire.eat", Fresh))
	c.Seal(m, mkAtom("wish", RetrospectionAtom, "retrospection.wish.done", Fresh))
}
