//go:build ignore

package calibrate

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dpopsuev/bugle/dispatch"
	"github.com/dpopsuev/origami/agentport"
	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami/transformers"
)

// TestCalibrateWithCLI proves the generic calibration path:
// CLIDispatcher → core.llm transformer → calibrate.Run() → scorecard.
//
// Requires CALIBRATE_CLI_COMMAND env var (e.g., "claude").
// Optional CALIBRATE_CLI_ARGS (e.g., "--print").
//
// This test is backend-agnostic — it works with any CLI tool that
// reads a prompt from stdin and returns JSON on stdout.
//
// Quick smoke test (no LLM needed):
//
//	CALIBRATE_CLI_COMMAND=cat go test -run TestCalibrateWithCLI -v ./calibrate/
//
// With Claude CLI:
//
//	CALIBRATE_CLI_COMMAND=claude CALIBRATE_CLI_ARGS="--print" \
//	  go test -run TestCalibrateWithCLI -v -timeout 10m ./calibrate/
func TestCalibrateWithCLI(t *testing.T) {
	command := os.Getenv("CALIBRATE_CLI_COMMAND")
	if command == "" {
		t.Skip("CALIBRATE_CLI_COMMAND not set — skipping CLI calibration test")
	}

	var args []string
	if a := os.Getenv("CALIBRATE_CLI_ARGS"); a != "" {
		args = strings.Fields(a)
	}

	timeout := 5 * time.Minute
	if d := os.Getenv("CALIBRATE_CLI_TIMEOUT"); d != "" {
		if parsed, err := time.ParseDuration(d); err == nil {
			timeout = parsed
		}
	}

	cliDisp, err := dispatch.NewCLIDispatcher(command,
		agentport.WithCLIArgs(args...),
		agentport.WithCLITimeout(timeout),
	)
	if err != nil {
		t.Skipf("CLI dispatcher unavailable: %v", err)
	}

	// Load test circuit, scorecard, and scenario from testdata/.
	circuitYAML, err := os.ReadFile("testdata/echo-circuit.yaml")
	if err != nil {
		t.Fatalf("read circuit: %v", err)
	}
	circuitDef, err := circuit.LoadCircuit(circuitYAML)
	if err != nil {
		t.Fatalf("parse circuit: %v", err)
	}

	scorecardYAML, err := os.ReadFile("testdata/echo-scorecard.yaml")
	if err != nil {
		t.Fatalf("read scorecard: %v", err)
	}
	sc, err := ParseScoreCard(scorecardYAML)
	if err != nil {
		t.Fatalf("parse scorecard: %v", err)
	}

	scenarioYAML, err := os.ReadFile("testdata/echo-scenario.yaml")
	if err != nil {
		t.Fatalf("read scenario: %v", err)
	}
	scenario, err := LoadGenericScenario(scenarioYAML)
	if err != nil {
		t.Fatalf("parse scenario: %v", err)
	}

	// Wire: CLIDispatcher → CoreComponent(core.llm) → calibrate.Run()
	coreComp := transformers.CoreComponent(cliDisp,
		transformers.WithCoreBaseDir("testdata"),
	)

	contract := ContractFromDef(circuitDef.Calibration)
	loader := &GenericScenarioLoader{Scenario: scenario}
	collector := NewContractCollector(contract, sc, scenario)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	report, err := Run(ctx, &HarnessConfig{
		CircuitDef:  circuitDef,
		ScoreCard:   sc,
		Loader:      loader,
		Collector:   collector,
		Components:  []*engine.Component{coreComp},
		Scenario:    scenario.Name,
		Transformer: command,
		Runs:        1,
		Parallel:    1,
		OnCaseComplete: func(i int, result engine.BatchWalkResult) {
			if result.Error != nil {
				t.Logf("case %d (%s) ERROR: %v", i, result.CaseID, result.Error)
			} else {
				t.Logf("case %d (%s) OK, steps: %d, artifacts: %v",
					i, result.CaseID, len(result.Path),
					artifactKeys(result.StepArtifacts))
			}
		},
	})
	if err != nil {
		t.Fatalf("calibrate.Run: %v", err)
	}

	t.Logf("Calibration report: %d metrics", len(report.Metrics.Metrics))
	for _, m := range report.Metrics.Metrics {
		t.Logf("  %s (%s): %.2f (threshold: %.2f, pass: %v)",
			m.ID, m.Name, m.Value, m.Threshold, m.Pass)
	}

	if len(report.Metrics.Metrics) == 0 {
		t.Error("no metrics in report — scoring pipeline may be broken")
	}
}

func artifactKeys(m map[string]circuit.Artifact) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
