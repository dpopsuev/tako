package mcp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/bugle/signal"
	bd "github.com/dpopsuev/bugle/dispatch"
	"github.com/dpopsuev/origami/dispatch"
	"github.com/dpopsuev/origami/mcp"
)

// --- lifecycle dispatch transformer ---

// lifecycleTransformer bridges circuit node processing to the MuxDispatcher.
// When a node runs, it sends the prompt via Dispatch() and blocks until the
// agent submits an artifact back.
type lifecycleTransformer struct {
	disp bd.Dispatcher
}

func (t *lifecycleTransformer) Name() string { return "dispatch-lifecycle" }
func (t *lifecycleTransformer) Transform(ctx context.Context, tc *engine.TransformerContext) (any, error) {
	prompt := fmt.Sprintf(`{"node":"%s","step":"test"}`, tc.NodeName)
	data, err := t.disp.Dispatch(ctx, bd.Context{
		CaseID:        tc.WalkerState.ID,
		Step:          tc.NodeName,
		PromptContent: prompt,
	})
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("unmarshal dispatch response: %w", err)
	}
	return result, nil
}

// --- circuit YAML fixtures ---

const linearCircuitYAML = `
circuit: lifecycle-test
start: step-a
done: done
handler_type: transformer

nodes:
  - name: step-a
    handler: dispatch-lifecycle
  - name: step-b
    handler: dispatch-lifecycle

edges:
  - id: a-b
    from: step-a
    to: step-b
  - id: b-done
    from: step-b
    to: done
`

const singleNodeCircuitYAML = `
circuit: lifecycle-single
start: step-x
done: done
handler_type: transformer

nodes:
  - name: step-x
    handler: dispatch-lifecycle

edges:
  - id: x-done
    from: step-x
    to: done
`

// --- step schemas for lifecycle tests ---

var lifecycleStepSchemas = []mcp.StepSchema{
	{
		Name: "step-a",
		Defs: []mcp.FieldDef{{Name: "ok", Type: "bool", Required: true}},
	},
	{
		Name: "step-b",
		Defs: []mcp.FieldDef{{Name: "ok", Type: "bool", Required: true}},
	},
	{
		Name: "step-x",
		Defs: []mcp.FieldDef{{Name: "ok", Type: "bool", Required: true}},
	},
}

// lifecycleConfig creates a CircuitConfig that runs engine.BatchWalk with
// a lifecycleTransformer, proving the full path: YAML circuit → graph walk →
// transformer dispatch → MCP handler loop.
func lifecycleConfig(circuitYAML string, nCases int) mcp.CircuitConfig {
	return mcp.CircuitConfig{
		Name:                      "lifecycle-test",
		Version:                   "dev",
		StepSchemas:               lifecycleStepSchemas,
		DefaultGetNextStepTimeout: 3000,
		DefaultSessionTTL:         30000,
		CreateSession: func(ctx context.Context, params mcp.StartParams, disp *dispatch.MuxDispatcher, bus signal.Bus) (mcp.RunFunc, mcp.SessionMeta, error) {
			def, err := circuit.LoadCircuit([]byte(circuitYAML))
			if err != nil {
				return nil, mcp.SessionMeta{}, fmt.Errorf("load circuit: %w", err)
			}

			dt := &lifecycleTransformer{disp: disp}
			reg := engine.GraphRegistries{
				Transformers: engine.TransformerRegistry{
					"dispatch-lifecycle": dt,
				},
			}

			cases := make([]engine.BatchCase, nCases)
			for i := range cases {
				cases[i] = engine.BatchCase{
					ID:      fmt.Sprintf("C%02d", i+1),
					Context: map[string]any{"input": "test"},
				}
			}

			runFn := func(ctx context.Context) (any, error) {
				results := engine.BatchWalk(ctx, engine.BatchWalkConfig{
					Def:      def,
					Shared:   reg,
					Cases:    cases,
					Parallel: params.Parallel,
				})
				for _, r := range results {
					if r.Error != nil {
						return nil, fmt.Errorf("case %s: %w", r.CaseID, r.Error)
					}
				}
				return map[string]any{
					"cases_done": len(results),
				}, nil
			}

			return runFn, mcp.SessionMeta{
				TotalCases: nCases,
				Scenario:   "lifecycle",
			}, nil
		},
	}
}

// --- Tests ---

// TestSessionLifecycle_StartGetSubmitReport proves the full MCP session
// lifecycle: start_circuit → get_next_step → submit_step (repeat) → get_report.
// Uses a 2-node linear circuit with 1 case, driven by engine.BatchWalk.
func TestSessionLifecycle_StartGetSubmitReport(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg := lifecycleConfig(linearCircuitYAML, 1)
	srv := newTestServer(t, cfg)
	session := connectInMemory(t, ctx, srv)

	// 1. Start circuit
	startOut := callTool(t, ctx, session, "start_circuit", nil)
	sessionID, ok := startOut["session_id"].(string)
	if !ok || sessionID == "" {
		t.Fatalf("expected session_id, got %v", startOut)
	}
	if int(startOut["total_cases"].(float64)) != 1 {
		t.Fatalf("expected total_cases=1, got %v", startOut["total_cases"])
	}

	// 2. Loop: get_next_step → submit_step until done
	var steps []string
	for i := 0; i < 10; i++ { // safety bound
		out := callTool(t, ctx, session, "get_next_step", map[string]any{
			"session_id": sessionID,
		})

		if done, _ := out["done"].(bool); done {
			break
		}

		available, _ := out["available"].(bool)
		if !available {
			t.Fatal("expected available=true")
		}

		step, _ := out["step"].(string)
		dispatchID := out["dispatch_id"].(float64)
		steps = append(steps, step)

		callTool(t, ctx, session, "submit_step", map[string]any{
			"session_id":  sessionID,
			"dispatch_id": int64(dispatchID),
			"step":        step,
			"fields":      map[string]any{"ok": true},
		})
	}

	// 3. Verify steps
	if len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %d: %v", len(steps), steps)
	}
	if steps[0] != "step-a" {
		t.Errorf("first step: want step-a, got %s", steps[0])
	}
	if steps[1] != "step-b" {
		t.Errorf("second step: want step-b, got %s", steps[1])
	}

	// 4. Get report
	report := callTool(t, ctx, session, "get_report", map[string]any{
		"session_id": sessionID,
	})
	if report["status"] != "done" {
		t.Errorf("report status: want done, got %v", report["status"])
	}
	if structured, ok := report["structured"].(map[string]any); ok {
		if int(structured["cases_done"].(float64)) != 1 {
			t.Errorf("cases_done: want 1, got %v", structured["cases_done"])
		}
	}
}

// TestSessionLifecycle_MultiCase proves 3 cases through a 1-node circuit.
// All 3 prompts must appear and all 3 must complete.
func TestSessionLifecycle_MultiCase(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const nCases = 3
	cfg := lifecycleConfig(singleNodeCircuitYAML, nCases)
	srv := newTestServer(t, cfg)
	session := connectInMemory(t, ctx, srv)

	// Start circuit
	startOut := callTool(t, ctx, session, "start_circuit", nil)
	sessionID := startOut["session_id"].(string)
	if int(startOut["total_cases"].(float64)) != nCases {
		t.Fatalf("expected total_cases=%d, got %v", nCases, startOut["total_cases"])
	}

	// Drain all steps
	seen := map[string]bool{}
	for i := 0; i < 20; i++ { // safety bound
		out := callTool(t, ctx, session, "get_next_step", map[string]any{
			"session_id": sessionID,
		})

		if done, _ := out["done"].(bool); done {
			break
		}

		available, _ := out["available"].(bool)
		if !available {
			continue
		}

		caseID, _ := out["case_id"].(string)
		step, _ := out["step"].(string)
		dispatchID := out["dispatch_id"].(float64)
		seen[caseID+":"+step] = true

		callTool(t, ctx, session, "submit_step", map[string]any{
			"session_id":  sessionID,
			"dispatch_id": int64(dispatchID),
			"step":        step,
			"fields":      map[string]any{"ok": true},
		})
	}

	// Verify all 3 cases completed
	if len(seen) != nCases {
		t.Fatalf("expected %d case:step pairs, got %d: %v", nCases, len(seen), seen)
	}
	for i := 1; i <= nCases; i++ {
		key := fmt.Sprintf("C%02d:step-x", i)
		if !seen[key] {
			t.Errorf("missing %s", key)
		}
	}

	// Get report
	report := callTool(t, ctx, session, "get_report", map[string]any{
		"session_id": sessionID,
	})
	if report["status"] != "done" {
		t.Errorf("report status: want done, got %v", report["status"])
	}
}
