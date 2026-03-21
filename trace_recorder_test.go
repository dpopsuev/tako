package framework

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestTraceRecorder_WalkEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trace.jsonl")
	rec, err := NewTraceRecorder(path)
	if err != nil {
		t.Fatal(err)
	}
	// Walker name IS the case ID (BatchWalk uses NewProcessWalker(caseID)).
	rec.OnEvent(WalkEvent{Type: EventNodeEnter, Node: "recall", Walker: "C04"})
	rec.OnEvent(WalkEvent{Type: EventNodeExit, Node: "recall", Walker: "C04", Elapsed: 5 * time.Second})
	rec.OnEvent(WalkEvent{Type: EventEdgeEvaluate, Node: "recall", Edge: "recall-triage", Walker: "C04"})
	rec.OnEvent(WalkEvent{Type: EventTransition, Node: "recall", Edge: "recall-triage", Walker: "C04"})

	if err := rec.Close(); err != nil {
		t.Fatal(err)
	}

	events := readTraceEvents(t, path)
	if len(events) != 4 {
		t.Fatalf("got %d events, want 4", len(events))
	}

	// All walker events should be debug level.
	for _, e := range events {
		if e.Level != LevelDebug {
			t.Errorf("event %s: level = %s, want debug", e.Event, e.Level)
		}
		if e.CaseID != "C04" {
			t.Errorf("event %s: case_id = %s, want C04", e.Event, e.CaseID)
		}
	}

	if events[0].Event != "node_enter" {
		t.Errorf("events[0].Event = %s, want node_enter", events[0].Event)
	}
	if events[1].ElapsedMs != 5000 {
		t.Errorf("events[1].ElapsedMs = %d, want 5000", events[1].ElapsedMs)
	}
}

func TestTraceRecorder_ArtifactGetsTraceLevel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trace.jsonl")
	rec, err := NewTraceRecorder(path)
	if err != nil {
		t.Fatal(err)
	}

	rec.OnEvent(WalkEvent{Type: EventNodeExit, Node: "recall", Artifact: &testRecArtifact{}})
	rec.Close()

	events := readTraceEvents(t, path)
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	if events[0].Level != LevelTrace {
		t.Errorf("node_exit with artifact: level = %s, want trace", events[0].Level)
	}
}

func TestTraceRecorder_HandleSignal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trace.jsonl")
	rec, err := NewTraceRecorder(path)
	if err != nil {
		t.Fatal(err)
	}

	rec.HandleSignal(
		time.Now().UTC().Format(time.RFC3339),
		"session_started", "server", "", "",
		map[string]string{"scenario": "ptp", "total_cases": "18"},
	)
	rec.HandleSignal(
		time.Now().UTC().Format(time.RFC3339),
		"step_ready", "server", "C04", "recall",
		nil,
	)
	rec.Close()

	events := readTraceEvents(t, path)
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}

	for _, e := range events {
		if e.Level != LevelInfo {
			t.Errorf("event %s: level = %s, want info", e.Event, e.Level)
		}
	}
	if events[0].Event != "session_started" {
		t.Errorf("events[0].Event = %s, want session_started", events[0].Event)
	}
	if events[1].CaseID != "C04" {
		t.Errorf("events[1].CaseID = %s, want C04", events[1].CaseID)
	}
}

func TestTraceRecorder_Concurrent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trace.jsonl")
	rec, err := NewTraceRecorder(path)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				rec.OnEvent(WalkEvent{Type: EventNodeEnter, Node: "test"})
				rec.HandleSignal(
					time.Now().UTC().Format(time.RFC3339),
					"test", "agent", "", "", nil,
				)
			}
		}()
	}
	wg.Wait()
	rec.Close()

	events := readTraceEvents(t, path)
	if len(events) != 400 { // 4 goroutines × 50 × 2 events
		t.Errorf("got %d events, want 400", len(events))
	}
}

func readTraceEvents(t *testing.T, path string) []TraceEvent {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	var events []TraceEvent
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var te TraceEvent
		if err := json.Unmarshal(scanner.Bytes(), &te); err != nil {
			t.Fatalf("invalid JSONL line: %v\nline: %s", err, scanner.Text())
		}
		events = append(events, te)
	}
	return events
}

type testRecArtifact struct{}

func (a *testRecArtifact) Type() string       { return "test" }
func (a *testRecArtifact) Confidence() float64 { return 1.0 }
func (a *testRecArtifact) Raw() any            { return nil }
