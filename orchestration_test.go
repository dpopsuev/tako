package framework

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

// --- Failing transformer for error propagation tests ---

type failingTransformer struct {
	failAt string // node name to fail at
	err    error
}

func (t *failingTransformer) Name() string { return "failing" }
func (ft *failingTransformer) Transform(ctx context.Context, tc *TransformerContext) (any, error) {
	if tc.NodeName == ft.failAt {
		return nil, ft.err
	}
	return map[string]any{"ok": true}, nil
}

// --- Error Propagation Tests ---

func TestErrorPropagation_TransformerFailure(t *testing.T) {
	def := &CircuitDef{
		Circuit: "error-test", Start: "step-a", Done: "done",
		HandlerType: "transformer",
		Nodes: []NodeDef{
			{Name: "step-a", HandlerType: "transformer", Handler: "test-handler"},
			{Name: "step-b", HandlerType: "transformer", Handler: "test-handler"},
		},
		Edges: []EdgeDef{
			{ID: "a-b", From: "step-a", To: "step-b"},
			{ID: "b-done", From: "step-b", To: "done"},
		},
	}

	ft := &failingTransformer{failAt: "step-b", err: fmt.Errorf("connection refused")}
	reg := GraphRegistries{
		Transformers: TransformerRegistry{
			"test-handler": ft,
		},
	}

	result := walkCircuit(t, def, reg, map[string]any{"input": "test"})
	if result.Error == nil {
		t.Fatal("expected error from failing transformer")
	}

	errMsg := result.Error.Error()
	if !strings.Contains(errMsg, "step-b") {
		t.Errorf("error should contain node name 'step-b', got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "connection refused") {
		t.Errorf("error should contain root cause 'connection refused', got: %s", errMsg)
	}
	t.Logf("error (correctly includes node name): %s", errMsg)
}

func TestErrorPropagation_SubCircuitFailure(t *testing.T) {
	child := &CircuitDef{
		Circuit: "child", Start: "process", Done: "child-done",
		HandlerType: "transformer",
		Nodes:       []NodeDef{{Name: "process", HandlerType: "transformer", Handler: "failing"}},
		Edges:       []EdgeDef{{ID: "process-done", From: "process", To: "child-done"}},
	}
	parent := &CircuitDef{
		Circuit: "parent", Start: "main", Done: "done",
		HandlerType: "transformer",
		Nodes:       []NodeDef{{Name: "main", HandlerType: "circuit", Handler: "child"}},
		Edges:       []EdgeDef{{ID: "main-done", From: "main", To: "done"}},
	}

	ft := &failingTransformer{failAt: "process", err: fmt.Errorf("child failed")}
	reg := GraphRegistries{
		Transformers: TransformerRegistry{"failing": ft},
		Circuits:     map[string]*CircuitDef{"child": child},
	}

	result := walkCircuit(t, parent, reg, map[string]any{})
	if result.Error == nil {
		t.Fatal("expected error from sub-circuit failure")
	}

	errMsg := result.Error.Error()
	// Error should propagate through delegate: "node main: delegate main: ... node process: child failed"
	if !strings.Contains(errMsg, "main") {
		t.Errorf("error should contain parent node name 'main', got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "process") {
		t.Errorf("error should contain child node name 'process', got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "child failed") {
		t.Errorf("error should contain root cause 'child failed', got: %s", errMsg)
	}
	t.Logf("error chain: %s", errMsg)
}

func TestErrorPropagation_PartialBatch(t *testing.T) {
	def := &CircuitDef{
		Circuit: "partial-fail", Start: "step-a", Done: "done",
		HandlerType: "transformer",
		Nodes: []NodeDef{
			{Name: "step-a", HandlerType: "transformer", Handler: "conditional"},
			{Name: "step-b", HandlerType: "transformer", Handler: "conditional"},
		},
		Edges: []EdgeDef{
			{ID: "a-b", From: "step-a", To: "step-b"},
			{ID: "b-done", From: "step-b", To: "done"},
		},
	}

	// Transformer that fails only for case "C2".
	conditionalFail := &conditionalFailTransformer{failCaseID: "C2"}
	reg := GraphRegistries{
		Transformers: TransformerRegistry{"conditional": conditionalFail},
	}

	results := BatchWalk(context.Background(), BatchWalkConfig{
		Def:      def,
		Shared:   reg,
		Cases:    []BatchCase{{ID: "C1"}, {ID: "C2"}, {ID: "C3"}},
		Parallel: 1,
	})

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// C1 and C3 should succeed.
	if results[0].Error != nil {
		t.Errorf("C1 should succeed, got: %v", results[0].Error)
	}
	if results[2].Error != nil {
		t.Errorf("C3 should succeed, got: %v", results[2].Error)
	}

	// C2 should fail with identifiable error.
	if results[1].Error == nil {
		t.Fatal("C2 should fail")
	}
	if !strings.Contains(results[1].Error.Error(), "intentional failure") {
		t.Errorf("C2 error = %v, want 'intentional failure'", results[1].Error)
	}

	t.Logf("C1: %v, C2: %v, C3: %v", results[0].Error, results[1].Error, results[2].Error)
}

type conditionalFailTransformer struct {
	failCaseID string
}

func (cf *conditionalFailTransformer) Name() string { return "conditional-fail" }
func (cf *conditionalFailTransformer) Transform(ctx context.Context, tc *TransformerContext) (any, error) {
	if tc.WalkerState != nil && tc.WalkerState.ID == cf.failCaseID {
		return nil, errors.New("intentional failure for " + cf.failCaseID)
	}
	return map[string]any{"ok": true}, nil
}

// --- Log Assertion Tests ---

// logEntry represents a parsed JSON slog entry.
type logEntry struct {
	Msg       string `json:"msg"`
	Component string `json:"component"`
	Node      string `json:"node"`
	From      string `json:"from"`
	To        string `json:"to"`
	Edge      string `json:"edge"`
	Loop      bool   `json:"loop"`
	Shortcut  bool   `json:"shortcut"`
	Count     int    `json:"count"`
	Circuit   string `json:"circuit"`
}

// captureLogs replaces slog.Default with a JSON handler that writes to buf.
// Returns a restore function for t.Cleanup.
func captureLogs(buf *bytes.Buffer) func() {
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	return func() { slog.SetDefault(old) }
}

// parseLogs parses newline-delimited JSON log entries.
func parseLogs(buf *bytes.Buffer) []logEntry {
	var entries []logEntry
	for _, line := range strings.Split(buf.String(), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var e logEntry
		if err := json.Unmarshal([]byte(line), &e); err == nil {
			entries = append(entries, e)
		}
	}
	return entries
}

func findLog(entries []logEntry, msg string) *logEntry {
	for i := range entries {
		if entries[i].Msg == msg {
			return &entries[i]
		}
	}
	return nil
}

func countLogs(entries []logEntry, msg string) int {
	n := 0
	for _, e := range entries {
		if e.Msg == msg {
			n++
		}
	}
	return n
}

func TestLogInstrumentation_NodeEnterExit(t *testing.T) {
	var buf bytes.Buffer
	t.Cleanup(captureLogs(&buf))

	def, err := LoadCircuit(loadTestCircuit(t, "linear.yaml"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	walkCircuit(t, def, passthroughRegistry(), map[string]any{})

	entries := parseLogs(&buf)
	enterCount := countLogs(entries, LogNodeEnter)
	exitCount := countLogs(entries, LogNodeExit)

	if enterCount < 2 {
		t.Errorf("expected >= 2 node enter logs, got %d", enterCount)
	}
	if exitCount < 2 {
		t.Errorf("expected >= 2 node exit logs, got %d", exitCount)
	}
	t.Logf("log entries: %d total, %d enters, %d exits", len(entries), enterCount, exitCount)
}

func TestLogInstrumentation_EdgeTaken(t *testing.T) {
	var buf bytes.Buffer
	t.Cleanup(captureLogs(&buf))

	def, err := LoadCircuit(loadTestCircuit(t, "branching.yaml"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	reg := passthroughRegistry()
	reg.Transformers["context-echo"] = &contextTransformer{}
	walkCircuit(t, def, reg, map[string]any{"confidence": 0.9})

	entries := parseLogs(&buf)
	edgeCount := countLogs(entries, LogEdgeTaken)
	if edgeCount < 1 {
		t.Errorf("expected >= 1 edge taken logs, got %d", edgeCount)
	}

	// Find the shortcut edge log.
	for _, e := range entries {
		if e.Msg == LogEdgeTaken && e.Shortcut {
			t.Logf("shortcut edge: from=%s to=%s edge=%s", e.From, e.To, e.Edge)
			return
		}
	}
	t.Error("expected a shortcut edge log entry for high-confidence branch")
}

func TestLogInstrumentation_LoopCount(t *testing.T) {
	var buf bytes.Buffer
	t.Cleanup(captureLogs(&buf))

	def, err := LoadCircuit(loadTestCircuit(t, "looping.yaml"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	reg := passthroughRegistry()
	reg.Transformers["context-echo"] = &contextTransformer{}
	walkCircuit(t, def, reg, map[string]any{"convergence": 0.3})

	entries := parseLogs(&buf)
	loopLogs := countLogs(entries, LogLoopIncremented)
	if loopLogs < 1 {
		t.Errorf("expected >= 1 loop incremented logs, got %d", loopLogs)
	}
	t.Logf("loop increment logs: %d", loopLogs)
}

func TestLogInstrumentation_DelegateStart(t *testing.T) {
	var buf bytes.Buffer
	t.Cleanup(captureLogs(&buf))

	parentData := loadTestCircuit(t, "subcircuit.yaml")
	childData := loadTestCircuit(t, "child.yaml")

	parentDef, _ := LoadCircuit(parentData)
	childDef, _ := LoadCircuit(childData)

	reg := passthroughRegistry()
	reg.Circuits = map[string]*CircuitDef{"child": childDef}

	walkCircuit(t, parentDef, reg, map[string]any{})

	entries := parseLogs(&buf)
	delegateStart := findLog(entries, LogDelegateStart)
	if delegateStart == nil {
		t.Error("expected delegate start log entry")
	} else {
		t.Logf("delegate start: node=%s circuit=%s", delegateStart.Node, delegateStart.Circuit)
	}
}

// --- Concurrency Tests ---

func TestBatchWalk_ConcurrencyParity(t *testing.T) {
	def, err := LoadCircuit(loadTestCircuit(t, "linear.yaml"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	cases := make([]BatchCase, 10)
	for i := range cases {
		cases[i] = BatchCase{ID: fmt.Sprintf("C%d", i+1), Context: map[string]any{"i": i}}
	}

	// Serial
	serial := BatchWalk(context.Background(), BatchWalkConfig{
		Def: def, Shared: passthroughRegistry(), Cases: cases, Parallel: 1,
	})

	// Parallel
	parallel := BatchWalk(context.Background(), BatchWalkConfig{
		Def: def, Shared: passthroughRegistry(), Cases: cases, Parallel: 4,
	})

	if len(serial) != len(parallel) {
		t.Fatalf("result count: serial=%d parallel=%d", len(serial), len(parallel))
	}

	for i := range serial {
		if serial[i].CaseID != parallel[i].CaseID {
			t.Errorf("case %d: serial=%s parallel=%s", i, serial[i].CaseID, parallel[i].CaseID)
		}
		if serial[i].Error != nil || parallel[i].Error != nil {
			t.Errorf("case %d: serial_err=%v parallel_err=%v", i, serial[i].Error, parallel[i].Error)
			continue
		}
		if len(serial[i].Path) != len(parallel[i].Path) {
			t.Errorf("case %d path length: serial=%d parallel=%d", i, len(serial[i].Path), len(parallel[i].Path))
		}
	}
	t.Logf("10 cases: serial and parallel produce identical results")
}
