package framework

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// collector gathers narration lines for assertion.
type collector struct {
	mu    sync.Mutex
	lines []string
}

func (c *collector) sink(line string) {
	c.mu.Lock()
	c.lines = append(c.lines, line)
	c.mu.Unlock()
}

func (c *collector) all() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(c.lines))
	copy(out, c.lines)
	return out
}

func TestNarrationObserver_ZeroConfig(t *testing.T) {
	obs := newNarrationObserver()
	if obs == nil {
		t.Fatal("newNarrationObserver() returned nil")
	}
	p := obs.Progress()
	if p.NodesVisited != 0 {
		t.Errorf("fresh observer: NodesVisited = %d, want 0", p.NodesVisited)
	}
}

func TestNarrationObserver_NodeEnterExit(t *testing.T) {
	c := &collector{}
	vocab := NewMapVocabulary().Register("F0", "Recall").Register("F1", "Triage")
	obs := newNarrationObserver(
		withVocabulary(vocab),
		withSink(c.sink),
		withMilestoneInterval(0),
	)

	obs.OnEvent(WalkEvent{Type: EventNodeEnter, Node: "F0"})
	obs.OnEvent(WalkEvent{Type: EventNodeExit, Node: "F0", Elapsed: 150 * time.Millisecond})
	obs.OnEvent(WalkEvent{Type: EventNodeEnter, Node: "F1"})
	obs.OnEvent(WalkEvent{Type: EventNodeExit, Node: "F1", Elapsed: 2 * time.Second})

	lines := c.all()
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d: %v", len(lines), lines)
	}

	if !strings.Contains(lines[0], "Entering Recall") {
		t.Errorf("line 0: %q, want 'Entering Recall'", lines[0])
	}
	if !strings.Contains(lines[1], "Completed Recall") {
		t.Errorf("line 1: %q, want 'Completed Recall'", lines[1])
	}
	if !strings.Contains(lines[1], "150ms") {
		t.Errorf("line 1: %q, want duration '150ms'", lines[1])
	}
	if !strings.Contains(lines[3], "2.0s") {
		t.Errorf("line 3: %q, want duration '2.0s'", lines[3])
	}

	p := obs.Progress()
	if p.NodesVisited != 2 {
		t.Errorf("NodesVisited = %d, want 2", p.NodesVisited)
	}
}

func TestNarrationObserver_WalkerSwitch(t *testing.T) {
	c := &collector{}
	obs := newNarrationObserver(withSink(c.sink), withMilestoneInterval(0))

	obs.OnEvent(WalkEvent{Type: EventWalkerSwitch, Walker: "Ember", Node: "F2"})
	lines := c.all()
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "Handing off to Ember") {
		t.Errorf("line: %q, want 'Handing off to Ember'", lines[0])
	}
}

func TestNarrationObserver_WalkComplete(t *testing.T) {
	c := &collector{}
	obs := newNarrationObserver(withSink(c.sink), withMilestoneInterval(0))

	obs.OnEvent(WalkEvent{Type: EventNodeEnter, Node: "start"})
	obs.OnEvent(WalkEvent{Type: EventNodeExit, Node: "start", Elapsed: time.Millisecond})
	obs.OnEvent(WalkEvent{Type: EventWalkComplete})

	lines := c.all()
	last := lines[len(lines)-1]
	if !strings.Contains(last, "Walk complete") {
		t.Errorf("last line: %q, want 'Walk complete'", last)
	}
	if !strings.Contains(last, "1 nodes visited") {
		t.Errorf("last line: %q, want '1 nodes visited'", last)
	}
}

func TestNarrationObserver_WalkError(t *testing.T) {
	c := &collector{}
	obs := newNarrationObserver(withSink(c.sink), withMilestoneInterval(0))

	obs.OnEvent(WalkEvent{Type: EventWalkError, Node: "F3", Error: errors.New("timeout")})
	lines := c.all()
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "Walk failed at F3") {
		t.Errorf("line: %q, want 'Walk failed at F3'", lines[0])
	}
	if !strings.Contains(lines[0], "timeout") {
		t.Errorf("line: %q, want 'timeout'", lines[0])
	}
}

func TestNarrationObserver_Milestone(t *testing.T) {
	c := &collector{}
	obs := newNarrationObserver(
		withSink(c.sink),
		withMilestoneInterval(3),
		withETA(true),
	)

	for i := 0; i < 6; i++ {
		node := "N"
		obs.OnEvent(WalkEvent{Type: EventNodeEnter, Node: node})
		obs.OnEvent(WalkEvent{Type: EventNodeExit, Node: node, Elapsed: time.Millisecond})
	}

	lines := c.all()
	milestones := 0
	for _, l := range lines {
		if strings.Contains(l, "--- progress:") {
			milestones++
		}
	}
	if milestones != 2 {
		t.Errorf("expected 2 milestones (at 3 and 6), got %d", milestones)
	}
}

func TestNarrationObserver_WithWalkerTag(t *testing.T) {
	c := &collector{}
	obs := newNarrationObserver(withSink(c.sink), withMilestoneInterval(0))

	obs.OnEvent(WalkEvent{Type: EventNodeEnter, Node: "F0", Walker: "Ember"})
	lines := c.all()
	if !strings.Contains(lines[0], "[Ember]") {
		t.Errorf("line: %q, want '[Ember]' prefix", lines[0])
	}
}

func TestNarrationObserver_ErrorInExit(t *testing.T) {
	c := &collector{}
	obs := newNarrationObserver(withSink(c.sink), withMilestoneInterval(0))

	obs.OnEvent(WalkEvent{
		Type:  EventNodeExit,
		Node:  "F3",
		Error: errors.New("node failed"),
	})

	lines := c.all()
	if !strings.Contains(lines[0], "Failed at F3") {
		t.Errorf("line: %q, want 'Failed at F3'", lines[0])
	}
}

func TestNarrationObserver_SilentEvents(t *testing.T) {
	c := &collector{}
	obs := newNarrationObserver(withSink(c.sink), withMilestoneInterval(0))

	obs.OnEvent(WalkEvent{Type: EventTransition, Node: "F0", Edge: "e1"})
	obs.OnEvent(WalkEvent{Type: EventEdgeEvaluate, Node: "F0", Edge: "e1"})

	if len(c.all()) != 0 {
		t.Errorf("transition and edge_evaluate should be silent, got %d lines", len(c.all()))
	}
}

func TestFmtNarrateDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{50 * time.Millisecond, "50ms"},
		{1500 * time.Millisecond, "1.5s"},
		{65 * time.Second, "1m5s"},
	}
	for _, tt := range tests {
		if got := fmtNarrateDuration(tt.d); got != tt.want {
			t.Errorf("fmtNarrateDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}
