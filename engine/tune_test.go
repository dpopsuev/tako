package engine

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/circuit/def"
)

func tuneManifest(name, tuneArgs string) *circuit.InstrumentManifest {
	return &circuit.InstrumentManifest{
		Kind:      circuit.KindInstrument,
		Name:      name,
		Namespace: "test",
		Dispatch:  circuit.DispatchCLI,
		Binary:    "bash",
		Tune:      tuneArgs,
		Actions: map[string]def.ActionDef{
			"default": {Command: "echo ok"},
		},
	}
}

func tuneManifestWithChecksum(name, checksum string) *circuit.InstrumentManifest {
	m := tuneManifest(name, "--version")
	m.Checksum = checksum
	return m
}

// bashChecksum returns sha256:<hex> of the bash binary for test assertions.
func bashChecksum(t *testing.T) string {
	t.Helper()
	m := tuneManifest("bash", "--version")
	cs, err := ComputeChecksum(m)
	if err != nil {
		t.Fatalf("ComputeChecksum(bash): %v", err)
	}
	return cs
}

func TestTuneAll_AllPass(t *testing.T) {
	reg := InstrumentRegistry{
		"alpha": tuneManifest("alpha", "--version"),
		"beta":  tuneManifest("beta", "--version"),
	}

	if err := TuneAll(t.Context(), reg, ""); err != nil {
		t.Fatalf("TuneAll: unexpected error: %v", err)
	}
}

func failManifest(name string) *circuit.InstrumentManifest {
	m := tuneManifest(name, "--version")
	m.Binary = "false"
	return m
}

func TestTuneAll_OneFails(t *testing.T) {
	reg := InstrumentRegistry{
		"good": tuneManifest("good", "--version"),
		"bad":  failManifest("bad"),
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
		"slow": tuneManifest("slow", "-c 'sleep 10'"),
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
		"noisy": tuneManifest("noisy", "-c 'echo boom >&2 && false'"),
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
		"aaa": failManifest("aaa"),
		"zzz": tuneManifest("zzz", "--version"),
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

func TestTuneAll_ChecksumPass(t *testing.T) {
	cs := bashChecksum(t)
	reg := InstrumentRegistry{
		"pinned": tuneManifestWithChecksum("pinned", cs),
	}
	if err := TuneAll(t.Context(), reg, ""); err != nil {
		t.Fatalf("TuneAll: unexpected error: %v", err)
	}
}

func TestTuneAll_ChecksumMismatch(t *testing.T) {
	reg := InstrumentRegistry{
		"tampered": tuneManifestWithChecksum("tampered", "sha256:0000000000000000000000000000000000000000000000000000000000000000"),
	}
	err := TuneAll(t.Context(), reg, "")
	if err == nil {
		t.Fatal("TuneAll: expected error on checksum mismatch")
	}
	if !errors.Is(err, ErrChecksumMismatch) {
		t.Errorf("error = %v, want ErrChecksumMismatch", err)
	}
}

func TestTuneAll_ChecksumAbsent_Skipped(t *testing.T) {
	reg := InstrumentRegistry{
		"unpinned": tuneManifest("unpinned", "--version"), // no checksum → skip verification
	}
	if err := TuneAll(t.Context(), reg, ""); err != nil {
		t.Fatalf("TuneAll: unexpected error: %v", err)
	}
}

func TestTuneAll_ChecksumBadFormat(t *testing.T) {
	reg := InstrumentRegistry{
		"bad": tuneManifestWithChecksum("bad", "md5:abc123"),
	}
	err := TuneAll(t.Context(), reg, "")
	if err == nil {
		t.Fatal("TuneAll: expected error on bad checksum format")
	}
	if !errors.Is(err, ErrChecksumMismatch) {
		t.Errorf("error = %v, want ErrChecksumMismatch", err)
	}
}

func TestComputeChecksum(t *testing.T) {
	m := tuneManifest("test", "--version")
	got, err := ComputeChecksum(m)
	if err != nil {
		t.Fatalf("ComputeChecksum: %v", err)
	}
	if !strings.HasPrefix(got, "sha256:") {
		t.Errorf("ComputeChecksum = %q, want sha256:... prefix", got)
	}
	if len(got) != 71 { // "sha256:" + 64 hex chars
		t.Errorf("ComputeChecksum length = %d, want 71", len(got))
	}
}

func TestTuneAll_ChecksumSingleBitFlip(t *testing.T) {
	cs := bashChecksum(t)
	// Flip last character.
	flipped := cs[:len(cs)-1] + "0"
	if flipped == cs {
		flipped = cs[:len(cs)-1] + "1"
	}
	reg := InstrumentRegistry{
		"flipped": tuneManifestWithChecksum("flipped", flipped),
	}
	err := TuneAll(t.Context(), reg, "")
	if err == nil {
		t.Fatal("single bit flip in checksum should fail")
	}
	if !errors.Is(err, ErrChecksumMismatch) {
		t.Errorf("error = %v, want ErrChecksumMismatch", err)
	}
}

func TestHashBinary(t *testing.T) {
	got, err := hashBinary("bash")
	if err != nil {
		t.Fatalf("hashBinary: %v", err)
	}
	if !strings.HasPrefix(got, "sha256:") {
		t.Errorf("hashBinary = %q, want sha256:... prefix", got)
	}
	// Same binary should produce same hash.
	got2, _ := hashBinary("bash")
	if got != got2 {
		t.Errorf("hashBinary not deterministic: %q != %q", got, got2)
	}
}

func TestHashBinary_NotFound(t *testing.T) {
	_, err := hashBinary("nonexistent-binary-xyz-123")
	if err == nil {
		t.Error("expected error for nonexistent binary")
	}
}
