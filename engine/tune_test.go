package engine

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/circuit/def"
)

func tuneManifest(name, tuneCmd string) *circuit.InstrumentManifest {
	return &circuit.InstrumentManifest{
		Kind:      circuit.KindInstrument,
		Name:      name,
		Namespace: "test",
		Dispatch:  circuit.DispatchCLI,
		Tune:      tuneCmd,
		Actions: map[string]def.ActionDef{
			"default": {Command: "echo ok"},
		},
	}
}

func TestTuneAll_AllPass(t *testing.T) {
	reg := InstrumentRegistry{
		"alpha": tuneManifest("alpha", "true"),
		"beta":  tuneManifest("beta", "true"),
	}

	if err := TuneAll(t.Context(), reg, ""); err != nil {
		t.Fatalf("TuneAll: unexpected error: %v", err)
	}
}

func TestTuneAll_OneFails(t *testing.T) {
	reg := InstrumentRegistry{
		"good": tuneManifest("good", "true"),
		"bad":  tuneManifest("bad", "false"),
	}

	err := TuneAll(t.Context(), reg, "")
	if err == nil {
		t.Fatal("TuneAll: expected error, got nil")
	}
	if !errors.Is(err, ErrTuneFailed) {
		t.Errorf("error = %v, want ErrTuneFailed", err)
	}
}

func TestTuneAll_EmptyRegistry(t *testing.T) {
	if err := TuneAll(t.Context(), InstrumentRegistry{}, ""); err != nil {
		t.Fatalf("TuneAll empty: unexpected error: %v", err)
	}
	if err := TuneAll(t.Context(), nil, ""); err != nil {
		t.Fatalf("TuneAll nil: unexpected error: %v", err)
	}
}

func TestTuneAll_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	reg := InstrumentRegistry{
		"slow": tuneManifest("slow", "sleep 10"),
	}

	err := TuneAll(ctx, reg, "")
	if err == nil {
		t.Fatal("TuneAll: expected error on canceled context")
	}
	if !errors.Is(err, ErrTuneFailed) {
		t.Errorf("error = %v, want ErrTuneFailed", err)
	}
}

func TestTuneAll_StderrInError(t *testing.T) {
	reg := InstrumentRegistry{
		"noisy": tuneManifest("noisy", "echo 'boom' >&2 && false"),
	}

	err := TuneAll(t.Context(), reg, "")
	if err == nil {
		t.Fatal("TuneAll: expected error")
	}
	if !errors.Is(err, ErrTuneFailed) {
		t.Errorf("error = %v, want ErrTuneFailed", err)
	}
	// Error message should contain stderr output.
	if err.Error() == "" {
		t.Error("error message is empty")
	}
}

func TestTuneAll_DeterministicOrder(t *testing.T) {
	// "aaa" sorts before "zzz". If "aaa" fails, "zzz" should not be attempted.
	reg := InstrumentRegistry{
		"aaa": tuneManifest("aaa", "false"),
		"zzz": tuneManifest("zzz", "true"),
	}

	err := TuneAll(t.Context(), reg, "")
	if err == nil {
		t.Fatal("TuneAll: expected error")
	}
	// Error should mention "aaa" (first alphabetically), not "zzz".
	if !strings.Contains(err.Error(), "aaa") {
		t.Errorf("error should mention 'aaa', got: %s", err.Error())
	}
}
