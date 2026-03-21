package contracts_test

import (
	"context"
	"testing"

	framework "github.com/dpopsuev/origami"
	"github.com/dpopsuev/origami/calibrate"
	"github.com/dpopsuev/origami/testkit/contracts"
)

// stubCollector is a minimal CaseCollector for contract testing.
type stubCollector struct{}

func (s *stubCollector) Collect(_ context.Context, results []framework.BatchWalkResult) (
	map[string]float64, map[string]string, error,
) {
	values := map[string]float64{"cases_processed": float64(len(results))}
	details := map[string]string{"cases_processed": "count of results"}
	return values, details, nil
}

func TestCaseCollectorContract_StubCollector(t *testing.T) {
	contracts.RunCaseCollectorContract(t, func() calibrate.CaseCollector {
		return &stubCollector{}
	})
}
