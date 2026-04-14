package trace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadTrace_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trace.jsonl")

	// Write a trace.
	rec, err := NewTraceRecorder(path)
	if err != nil {
		t.Fatalf("NewTraceRecorder: %v", err)
	}

	rec.write(&TraceEvent{Timestamp: "2026-01-01T00:00:00Z", Event: "node_enter", Node: "scan"})
	rec.write(&TraceEvent{Timestamp: "2026-01-01T00:00:01Z", Event: "node_exit", Node: "scan", ElapsedMs: 1000})
	rec.write(&TraceEvent{Timestamp: "2026-01-01T00:00:01Z", Event: "transition", Edge: "e1"})
	rec.write(&TraceEvent{Timestamp: "2026-01-01T00:00:01Z", Event: "node_enter", Node: "build"})
	rec.write(&TraceEvent{Timestamp: "2026-01-01T00:00:02Z", Event: "node_exit", Node: "build", ElapsedMs: 500})
	rec.write(&TraceEvent{Timestamp: "2026-01-01T00:00:02Z", Event: "walk_complete"})
	rec.Close()

	// Read it back.
	events, err := LoadTrace(path)
	if err != nil {
		t.Fatalf("LoadTrace: %v", err)
	}
	if len(events) != 6 {
		t.Fatalf("events = %d, want 6", len(events))
	}
	if events[0].Node != "scan" {
		t.Errorf("first event node = %q, want scan", events[0].Node)
	}
}

func TestLoadTrace_FileNotFound(t *testing.T) {
	_, err := LoadTrace("/nonexistent/trace.jsonl")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadTrace_MalformedLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trace.jsonl")
	os.WriteFile(path, []byte("not json\n{\"event\":\"ok\"}\ngarbage\n"), 0o600)

	events, err := LoadTrace(path)
	if err != nil {
		t.Fatalf("LoadTrace: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("events = %d, want 1 (skip malformed)", len(events))
	}
}

func TestSummarize(t *testing.T) {
	events := []TraceEvent{
		{Timestamp: "2026-01-01T00:00:00Z", Event: "node_enter", Node: "scan"},
		{Timestamp: "2026-01-01T00:00:01Z", Event: "node_exit", Node: "scan", ElapsedMs: 1000},
		{Timestamp: "2026-01-01T00:00:01Z", Event: "node_enter", Node: "build"},
		{Timestamp: "2026-01-01T00:00:02Z", Event: "node_exit", Node: "build", ElapsedMs: 500},
		{Timestamp: "2026-01-01T00:00:02Z", Event: "walk_complete"},
		{Timestamp: "2026-01-01T00:00:03Z", Event: "node_enter", Node: "fail", Error: "boom"},
	}

	s := Summarize(events)

	if s.TotalEvents != 6 {
		t.Errorf("total = %d, want 6", s.TotalEvents)
	}
	if len(s.Nodes) != 3 {
		t.Errorf("nodes = %d, want 3", len(s.Nodes))
	}
	if len(s.Errors) != 1 {
		t.Errorf("errors = %d, want 1", len(s.Errors))
	}
	if s.EventCounts["node_enter"] != 3 {
		t.Errorf("node_enter count = %d, want 3", s.EventCounts["node_enter"])
	}
	if s.Duration.Seconds() != 3 {
		t.Errorf("duration = %v, want 3s", s.Duration)
	}

	// Nodes sorted alphabetically.
	if s.Nodes[0].Name != "build" {
		t.Errorf("first node = %q, want build", s.Nodes[0].Name)
	}
	if s.Nodes[0].ElapsedMs != 500 {
		t.Errorf("build elapsed = %d, want 500", s.Nodes[0].ElapsedMs)
	}
}
