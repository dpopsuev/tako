package fab

import (
	"testing"

	"github.com/dpopsuev/tako/artifact"
)

func TestPredicate_SimpleCondition(t *testing.T) {
	p, err := NewPredicate(`output.ticker <= 0`)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	env := artifact.NewEnvelope("test", []byte(`{"ticker": 0, "alive": true}`))
	ok, err := p.Evaluate(Contract{}, env)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if !ok {
		t.Error("ticker=0 should pass 'ticker <= 0'")
	}
}

func TestPredicate_FailsWhenNotMet(t *testing.T) {
	p, err := NewPredicate(`output.ticker <= 0`)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	env := artifact.NewEnvelope("test", []byte(`{"ticker": 5}`))
	ok, err := p.Evaluate(Contract{}, env)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if ok {
		t.Error("ticker=5 should NOT pass 'ticker <= 0'")
	}
}

func TestPredicate_CompoundCondition(t *testing.T) {
	p, err := NewPredicate(`output.severity == "critical" && output.confidence > 0.7`)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	env := artifact.NewEnvelope("test", []byte(`{"severity": "critical", "confidence": 0.85}`))
	ok, err := p.Evaluate(Contract{}, env)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if !ok {
		t.Error("should pass compound condition")
	}
}

func TestPredicate_Labels(t *testing.T) {
	p, err := NewPredicate(`labels.stage == "kraken"`)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	env := artifact.NewEnvelope("test", []byte(`{}`))
	env.Labels["stage"] = "kraken"
	ok, err := p.Evaluate(Contract{}, env)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if !ok {
		t.Error("label stage=kraken should match")
	}
}

func TestPredicate_InvalidExpression(t *testing.T) {
	_, err := NewPredicate(`output.{{invalid`)
	if err == nil {
		t.Error("invalid expression should fail compilation")
	}
}

func TestPredicate_MustPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustPredicate should panic on invalid expression")
		}
	}()
	MustPredicate(`output.{{invalid`)
}

func TestPredicate_MinitakoDayContract(t *testing.T) {
	p := MustPredicate(`output.ActionTicker <= 0`)

	day := artifact.NewEnvelope("arcade", []byte(`{"ActionTicker": 14, "Alive": true}`))
	ok, _ := p.Evaluate(Contract{}, day)
	if ok {
		t.Error("14 ticks remaining should NOT pass")
	}

	night := artifact.NewEnvelope("arcade", []byte(`{"ActionTicker": 0, "Alive": true}`))
	ok, _ = p.Evaluate(Contract{}, night)
	if !ok {
		t.Error("0 ticks should pass")
	}
}
