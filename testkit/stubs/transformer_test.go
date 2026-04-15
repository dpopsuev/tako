package stubs_test

import (
	"context"
	"errors"
	"testing"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami/testkit/stubs"
)

func TestStubTransformer_CannedArtifact(t *testing.T) {
	art := &testArt{val: "hello"}
	st := stubs.NewStubTransformer("test", map[string]circuit.Artifact{
		"recall": art,
	})

	result, err := st.Transform(context.Background(), &engine.InstrumentContext{NodeName: "recall"})
	if err != nil {
		t.Fatal(err)
	}
	if result != art {
		t.Errorf("got %v, want canned artifact", result)
	}
}

func TestStubTransformer_DefaultArtifact(t *testing.T) {
	st := stubs.NewStubTransformer("test", nil)

	result, err := st.Transform(context.Background(), &engine.InstrumentContext{NodeName: "recall"})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Error("expected default artifact, got nil")
	}
}

func TestStubTransformer_ErrorInjection(t *testing.T) {
	st := stubs.NewStubTransformer("test", nil)
	injected := errors.New("boom")
	st.SetError("recall", injected)

	_, err := st.Transform(context.Background(), &engine.InstrumentContext{NodeName: "recall"})
	if !errors.Is(err, injected) {
		t.Errorf("got %v, want injected error", err)
	}
}

func TestStubTransformer_CallTracking(t *testing.T) {
	st := stubs.NewStubTransformer("test", nil)
	st.Transform(context.Background(), &engine.InstrumentContext{NodeName: "recall"})
	st.Transform(context.Background(), &engine.InstrumentContext{NodeName: "triage"})
	st.Transform(context.Background(), &engine.InstrumentContext{NodeName: "recall"})

	calls := st.Calls()
	if len(calls) != 3 {
		t.Fatalf("got %d calls, want 3", len(calls))
	}
	if calls[0] != "recall" || calls[1] != "triage" || calls[2] != "recall" {
		t.Errorf("calls = %v, want [recall triage recall]", calls)
	}
	if st.CallCount() != 3 {
		t.Errorf("CallCount = %d, want 3", st.CallCount())
	}
}

func TestStubTransformer_Reset(t *testing.T) {
	st := stubs.NewStubTransformer("test", nil)
	st.SetError("recall", errors.New("e"))
	st.Transform(context.Background(), &engine.InstrumentContext{NodeName: "other"})
	st.Reset()

	if st.CallCount() != 0 {
		t.Error("calls not cleared after Reset")
	}
	_, err := st.Transform(context.Background(), &engine.InstrumentContext{NodeName: "recall"})
	if err != nil {
		t.Error("error not cleared after Reset")
	}
}

type testArt struct{ val string }

func (a *testArt) Type() string        { return "test" }
func (a *testArt) Confidence() float64 { return 1.0 }
func (a *testArt) Raw() any            { return a.val }
