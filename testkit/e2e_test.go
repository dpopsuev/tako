package testkit_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/dpopsuev/origami/agentport"
	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami/calibrate"
	"github.com/dpopsuev/origami/dispatch"
	"github.com/dpopsuev/origami/mcp"
	"github.com/dpopsuev/origami/testkit/builders"
	"github.com/dpopsuev/origami/testkit/stubs"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMain(m *testing.M) {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError,
	})))
	os.Exit(m.Run())
}

// --- TSK-231: All-stubs calibration E2E ---

// stubCaseCollector counts cases and returns a simple metric.
type stubCaseCollector struct {
	casesProcessed int
}

func (c *stubCaseCollector) Collect(_ context.Context, results []engine.BatchWalkResult) (
	values map[string]float64, details map[string]string, err error,
) {
	c.casesProcessed = len(results)
	return map[string]float64{
		"M1": float64(len(results)),
	}, map[string]string{
		"M1": fmt.Sprintf("%d cases processed", len(results)),
	}, nil
}

func TestE2E_Calibration_AllStubs(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Step 1: Build a 3-node circuit (A -> B -> done) using the CircuitDefBuilder.
	def := builders.NewCircuitDef("e2e-test").
		HandlerType("transformer").
		AddNode("A", "stub").
		AddNode("B", "stub").
		AddEdge("A", "B", "true").
		AddEdge("B", "done", "true").
		Start("A").
		Done("done").
		Build()

	// Step 2: Create a StubTransformer with canned artifacts for each node.
	stubTx := stubs.NewStubTransformer("stub", nil)

	// Step 3: Create a Component that registers the stub transformer.
	comp := &engine.Component{
		Namespace: "testkit",
		Transformers: engine.TransformerRegistry{
			"stub": stubTx,
		},
	}

	// Step 4: Create a GenericScenario with 3 cases.
	scenario := &calibrate.GenericScenario{
		Name: "e2e-test-scenario",
		Cases: []calibrate.GenericCase{
			{ID: "C01", Input: map[string]any{"key": "val1"}},
			{ID: "C02", Input: map[string]any{"key": "val2"}},
			{ID: "C03", Input: map[string]any{"key": "val3"}},
		},
	}

	// Step 5: Build HarnessConfig.
	collector := &stubCaseCollector{}
	scoreCard := &calibrate.ScoreCard{
		Name: "e2e-scorecard",
		MetricDefs: []calibrate.MetricDef{
			{
				ID:        "M1",
				Name:      "cases_count",
				Tier:      calibrate.TierOutcome,
				Direction: calibrate.HigherIsBetter,
				Threshold: 1.0,
				Weight:    1.0,
			},
		},
	}

	cfg := calibrate.HarnessConfig{
		Loader:      &calibrate.GenericScenarioLoader{Scenario: scenario},
		Collector:   collector,
		CircuitDef:  def,
		ScoreCard:   scoreCard,
		Components:  []*engine.Component{comp},
		Parallel:    4,
		Runs:        1,
		Scenario:    "e2e-all-stubs",
		Transformer: "stub",
	}

	// Step 6: Call calibrate.Run().
	report, err := calibrate.Run(ctx, &cfg)

	// Step 7: Verify.
	if err != nil {
		t.Fatalf("calibrate.Run failed: %v", err)
	}
	if report == nil {
		t.Fatal("report is nil")
	}
	if report.Scenario != "e2e-all-stubs" {
		t.Errorf("Scenario = %q, want %q", report.Scenario, "e2e-all-stubs")
	}
	if report.Runs != 1 {
		t.Errorf("Runs = %d, want 1", report.Runs)
	}
	if collector.casesProcessed != 3 {
		t.Errorf("casesProcessed = %d, want 3", collector.casesProcessed)
	}

	passed, total := report.Metrics.PassCount()
	if total == 0 {
		t.Error("no metrics evaluated")
	}
	if passed != total {
		t.Errorf("PassCount = %d/%d, expected all to pass", passed, total)
	}

	// Verify the stub transformer was actually called.
	if stubTx.CallCount() == 0 {
		t.Error("StubTransformer was never called — circuit did not walk nodes")
	}
	t.Logf("StubTransformer called %d times, nodes: %v", stubTx.CallCount(), stubTx.Calls())
}

// --- TSK-232: MCP protocol E2E ---

func TestE2E_MCP_AllStubs(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Define step schemas matching a simple 2-step circuit.
	stepSchemas := []mcp.StepSchema{
		{
			Name: "STEP_A",
			Defs: []mcp.FieldDef{
				{Name: "value", Type: "string", Required: true},
			},
		},
		{
			Name: "STEP_B",
			Defs: []mcp.FieldDef{
				{Name: "result", Type: "string", Required: true},
			},
		},
	}

	// Create a StubTransformer-based config.
	nCases := 2
	nSteps := 2
	steps := []string{"STEP_A", "STEP_B"}

	cfg := mcp.CircuitConfig{
		Name:        "testkit-e2e",
		Version:     "dev",
		StepSchemas: stepSchemas,
		DefaultGetNextStepTimeout: 2000,
		DefaultSessionTTL:         30000,
		CreateSession: func(ctx context.Context, _ mcp.StartParams, disp *dispatch.MuxDispatcher, bus agentport.Bus) (mcp.RunFunc, mcp.SessionMeta, error) {
			runFn := func(ctx context.Context) (any, error) {
				for c := 0; c < nCases; c++ {
					caseID := fmt.Sprintf("C%02d", c+1)
					for s := 0; s < nSteps; s++ {
						dc := agentport.Context{
							CaseID:       caseID,
							Step:         steps[s],
							ArtifactPath: fmt.Sprintf("/tmp/testkit_%s_%s.json", caseID, steps[s]),
						}
						if _, err := disp.Dispatch(ctx, dc); err != nil {
							return nil, err
						}
					}
				}
				return map[string]any{"cases": nCases, "steps": nCases * nSteps}, nil
			}
			return runFn, mcp.SessionMeta{TotalCases: nCases, Scenario: "testkit-stub"}, nil
		},
		FormatReport: func(result any) (string, any, error) {
			return fmt.Sprintf("testkit E2E report: %v", result), result, nil
		},
	}

	// Create server and connect in-memory.
	srv := mcp.NewCircuitServer(&cfg)
	t.Cleanup(srv.Shutdown)

	t1, t2 := sdkmcp.NewInMemoryTransports()
	serverSession, err := srv.MCPServer.Connect(ctx, t1, nil)
	if err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	t.Cleanup(func() { serverSession.Close() })

	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "testkit-client", Version: "v0.0.1"}, nil)
	session, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	defer session.Close()

	// Call circuit(action=start).
	startResult := callTool(t, ctx, session, "circuit", map[string]any{"action": "start"})
	sessionID, ok := startResult["session_id"].(string)
	if !ok || sessionID == "" {
		t.Fatalf("circuit/start: expected non-empty session_id, got %v", startResult)
	}
	totalCases, _ := startResult["total_cases"].(float64)
	if int(totalCases) != nCases {
		t.Fatalf("circuit/start: total_cases = %v, want %d", totalCases, nCases)
	}

	// Loop: circuit(action=step) / circuit(action=submit) until done.
	stepsProcessed := 0
	for {
		stepResult := callTool(t, ctx, session, "circuit", map[string]any{
			"action":     "step",
			"session_id": sessionID,
			"timeout_ms": 2000,
		})
		if done, _ := stepResult["done"].(bool); done {
			break
		}
		if avail, _ := stepResult["available"].(bool); !avail {
			continue
		}

		step, _ := stepResult["step"].(string)
		dispatchID, _ := stepResult["dispatch_id"].(float64)

		fields := map[string]any{}
		switch step {
		case "STEP_A":
			fields["value"] = "testkit-e2e"
		case "STEP_B":
			fields["result"] = "pass"
		default:
			fields["data"] = step
		}

		callTool(t, ctx, session, "circuit", map[string]any{
			"action":      "submit",
			"session_id":  sessionID,
			"dispatch_id": int64(dispatchID),
			"step":        step,
			"fields":      fields,
		})
		stepsProcessed++
	}

	if stepsProcessed != nCases*nSteps {
		t.Errorf("processed %d steps, want %d", stepsProcessed, nCases*nSteps)
	}

	// Get report.
	reportResult := callTool(t, ctx, session, "circuit", map[string]any{
		"action":     "report",
		"session_id": sessionID,
	})
	status, _ := reportResult["status"].(string)
	if status != "done" {
		t.Fatalf("circuit/report: status = %q, want %q", status, "done")
	}
	reportText, _ := reportResult["report"].(string)
	if reportText == "" {
		t.Error("circuit/report: report text is empty")
	}
	t.Logf("MCP E2E report: %s", reportText)
}

// --- Helper: callTool (local copy for testkit_test package) ---

func callTool(t *testing.T, ctx context.Context, session *sdkmcp.ClientSession, name string, args map[string]any) map[string]any {
	t.Helper()
	if args == nil {
		args = map[string]any{}
	}
	res, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("CallTool(%s): %v", name, err)
	}
	if res.IsError {
		for _, c := range res.Content {
			if tc, ok := c.(*sdkmcp.TextContent); ok {
				t.Fatalf("CallTool(%s) returned error: %s", name, tc.Text)
			}
		}
		t.Fatalf("CallTool(%s) returned error", name)
	}
	result := make(map[string]any)
	for _, c := range res.Content {
		if tc, ok := c.(*sdkmcp.TextContent); ok {
			if err := json.Unmarshal([]byte(tc.Text), &result); err != nil {
				t.Fatalf("unmarshal %s result: %v (text: %s)", name, err, tc.Text)
			}
			return result
		}
	}
	t.Fatalf("no text content in %s result", name)
	return nil
}
