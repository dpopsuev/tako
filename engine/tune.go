package engine

// Category: Execution — instrument preflight verification.
// Implementation moved to engine/tune sub-package.

import (
	"context"

	"github.com/dpopsuev/tako/circuit"
	"github.com/dpopsuev/tako/engine/tune"
)

// TuneAll is re-exported from engine/tune for backward compatibility.
// All callers reference engine.TuneAll — this alias ensures they compile.
func TuneAll(ctx context.Context, instruments ManifestRegistry, workDir string) error {
	return tune.All(ctx, tune.Registry(instruments), workDir)
}

// ComputeChecksum is re-exported from engine/tune.
func ComputeChecksum(manifest *circuit.InstrumentManifest) (string, error) {
	return tune.ComputeChecksum(manifest)
}
