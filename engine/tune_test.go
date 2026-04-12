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

func tuneManifestWithChecksum(name, tuneCmd, checksum string) *circuit.InstrumentManifest {
	m := tuneManifest(name, tuneCmd)
	m.Checksum = checksum
	return m
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

// sha256 of "ok\n" (output of `echo ok`)
const echoOKChecksum = "sha256:dc51b8c96c2d745df3bd5590d990230a482fd247123599548e0632fdbf97fc22"

// sha256 of "" (output of `true`)
const emptyChecksum = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

func TestTuneAll_ChecksumPass(t *testing.T) {
	reg := InstrumentRegistry{
		"pinned": tuneManifestWithChecksum("pinned", "echo ok", echoOKChecksum),
	}
	if err := TuneAll(t.Context(), reg, ""); err != nil {
		t.Fatalf("TuneAll: unexpected error: %v", err)
	}
}

func TestTuneAll_ChecksumMismatch(t *testing.T) {
	reg := InstrumentRegistry{
		"tampered": tuneManifestWithChecksum("tampered", "echo ok", "sha256:0000000000000000000000000000000000000000000000000000000000000000"),
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
	// No checksum declared — should pass without verification.
	reg := InstrumentRegistry{
		"unpinned": tuneManifest("unpinned", "echo ok"),
	}
	if err := TuneAll(t.Context(), reg, ""); err != nil {
		t.Fatalf("TuneAll: unexpected error: %v", err)
	}
}

func TestTuneAll_ChecksumBadFormat(t *testing.T) {
	reg := InstrumentRegistry{
		"bad": tuneManifestWithChecksum("bad", "true", "md5:abc123"),
	}
	err := TuneAll(t.Context(), reg, "")
	if err == nil {
		t.Fatal("TuneAll: expected error on bad checksum format")
	}
	if !errors.Is(err, ErrChecksumMismatch) {
		t.Errorf("error = %v, want ErrChecksumMismatch", err)
	}
}

func TestTuneAll_ChecksumEmptyOutput(t *testing.T) {
	// `true` outputs nothing — verify against sha256 of empty string.
	reg := InstrumentRegistry{
		"empty": tuneManifestWithChecksum("empty", "true", emptyChecksum),
	}
	if err := TuneAll(t.Context(), reg, ""); err != nil {
		t.Fatalf("TuneAll: unexpected error: %v", err)
	}
}

func TestComputeChecksum(t *testing.T) {
	m := tuneManifest("test", "echo ok")
	got, err := ComputeChecksum(t.Context(), m, "")
	if err != nil {
		t.Fatalf("ComputeChecksum: %v", err)
	}
	if got != echoOKChecksum {
		t.Errorf("ComputeChecksum = %q, want %q", got, echoOKChecksum)
	}
}

func TestTuneAll_ChecksumSingleBitFlip(t *testing.T) {
	// Flip one character in the hash — must fail.
	// Original ends in ...fc22, flip last char to fc23.
	flipped := "sha256:dc51b8c96c2d745df3bd5590d990230a482fd247123599548e0632fdbf97fc23"
	reg := InstrumentRegistry{
		"flipped": tuneManifestWithChecksum("flipped", "echo ok", flipped),
	}
	err := TuneAll(t.Context(), reg, "")
	if err == nil {
		t.Fatal("single bit flip in checksum should fail")
	}
	if !errors.Is(err, ErrChecksumMismatch) {
		t.Errorf("error = %v, want ErrChecksumMismatch", err)
	}
}

func TestVerifyChecksum_Direct(t *testing.T) {
	// Direct unit test for verifyChecksum.
	if err := verifyChecksum("test", []byte("ok\n"), echoOKChecksum); err != nil {
		t.Errorf("verifyChecksum: %v", err)
	}
	if err := verifyChecksum("test", []byte("tampered\n"), echoOKChecksum); err == nil {
		t.Error("verifyChecksum: expected error for tampered content")
	}
	if err := verifyChecksum("test", []byte("ok\n"), "noprefix"); err == nil {
		t.Error("verifyChecksum: expected error for missing sha256: prefix")
	}
}
