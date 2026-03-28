package contracts_test

import (
	"context"
	"testing"

	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami/calibrate"
	"github.com/dpopsuev/origami/testkit/contracts"
)

// stubCollector is a minimal CaseCollector for contract testing.
type stubCollector struct{}

func (s *stubCollector) Collect(_ context.Context, results []engine.BatchWalkResult) (
	values map[string]float64, details map[string]string, err error,
) {
	values = map[string]float64{"cases_processed": float64(len(results))}
	details = map[string]string{"cases_processed": "count of results"}
	return values, details, nil
}

func TestCaseCollectorContract_StubCollector(t *testing.T) {
	contracts.RunCaseCollectorContract(t, func() calibrate.CaseCollector {
		return &stubCollector{}
	})
}
