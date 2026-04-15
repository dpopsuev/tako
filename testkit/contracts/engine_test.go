package contracts_test

import (
	"context"
	"testing"

	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami/testkit/contracts"
	"github.com/dpopsuev/origami/testkit/stubs"
)

func TestBatchWalkContract_Real(t *testing.T) {
	contracts.RunBatchWalkContract(t, engine.BatchWalk, engine.BuildGraph)
}

func TestBatchWalkContract_Mock(t *testing.T) {
	// Mock that respects the config's case count — Liskov substitution.
	batchWalk := func(_ context.Context, cfg engine.BatchWalkConfig) []engine.BatchWalkResult {
		results := make([]engine.BatchWalkResult, len(cfg.Cases))
		for i, c := range cfg.Cases {
			results[i] = engine.BatchWalkResult{CaseID: c.ID}
		}
		return results
	}

	contracts.RunBatchWalkContract(t, batchWalk, engine.BuildGraph)
}

func TestTuneContract_Real(t *testing.T) {
	contracts.RunTuneContract(t, engine.TuneAll)
}

func TestTuneContract_Mock(t *testing.T) {
	mock := &stubs.MockTuner{}
	contracts.RunTuneContract(t, mock.TuneAll)
}
