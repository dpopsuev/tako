package framework

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

func TestFanOut_BasicParallelExecution(t *testing.T) {
	nodeA := &stubNode{name: "A", artifact: &stubArtifact{typ: "a", confidence: 1.0}}
	nodeB1 := &stubNode{name: "B1", artifact: &stubArtifact{typ: "b1", confidence: 0.9}}
	nodeB2 := &stubNode{name: "B2", artifact: &stubArtifact{typ: "b2", confidence: 0.8}}
	nodeB3 := &stubNode{name: "B3", artifact: &stubArtifact{typ: "b3", confidence: 0.7}}
	nodeC := &stubNode{name: "C", artifact: &stubArtifact{typ: "c", confidence: 1.0}}

	edges := []Edge{
		&stubEdge{id: "A-B1", from: "A", to: "B1", parallel: true},
		&stubEdge{id: "A-B2", from: "A", to: "B2", parallel: true},
		&stubEdge{id: "A-B3", from: "A", to: "B3", parallel: true},
		&stubEdge{id: "B1-C", from: "B1", to: "C"},
		&stubEdge{id: "B2-C", from: "B2", to: "C"},
		&stubEdge{id: "B3-C", from: "B3", to: "C"},
		&stubEdge{id: "C-done", from: "C", to: "_done"},
	}

	g, err := NewGraph("test-fanout", []Node{nodeA, nodeB1, nodeB2, nodeB3, nodeC}, edges, nil)
	if err != nil {
		t.Fatal(err)
	}

	tc := &TraceCollector{}
	g.SetObserver(tc)
	w := NewProcessWalker("fan-1")

	if err := g.Walk(context.Background(), w, "A"); err != nil {
		t.Fatalf("Walk failed: %v", err)
	}

	if w.State().Status != "done" {
		t.Errorf("status = %q, want done", w.State().Status)
	}

	for _, name := range []string{"B1", "B2", "B3"} {
		if _, ok := w.State().Outputs[name]; !ok {
			t.Errorf("missing output for parallel branch %q", name)
		}
	}
	if _, ok := w.State().Outputs["C"]; !ok {
		t.Error("missing output for merge node C")
	}

	fanStarts := tc.EventsOfType(EventFanOutStart)
	if len(fanStarts) != 1 {
		t.Errorf("expected 1 fan_out_start event, got %d", len(fanStarts))
	}
	fanEnds := tc.EventsOfType(EventFanOutEnd)
	if len(fanEnds) != 1 {
		t.Errorf("expected 1 fan_out_end event, got %d", len(fanEnds))
	}
}

func TestFanOut_ConcurrentExecution(t *testing.T) {
	var running atomic.Int32
	var maxConcurrent atomic.Int32

	makeNode := func(name string) *stubNode {
		return &stubNode{
			name: name,
			artifact: &stubArtifact{typ: name, confidence: 1.0},
		}
	}

	nodeA := makeNode("A")
	nodeC := makeNode("C")

	slowProcess := func(ctx context.Context, nc NodeContext) (Artifact, error) {
		cur := running.Add(1)
		for {
			old := maxConcurrent.Load()
			if cur > old {
				if maxConcurrent.CompareAndSwap(old, cur) {
					break
				}
			} else {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
		running.Add(-1)
		return &stubArtifact{typ: "done", confidence: 1.0}, nil
	}

	nodeB1 := &stubNode{name: "B1", artifact: &stubArtifact{typ: "b1"}}
	nodeB1.artifact = nil
	nodeB2 := &stubNode{name: "B2", artifact: &stubArtifact{typ: "b2"}}
	nodeB2.artifact = nil

	concurrentNodes := make([]Node, 2)
	for i := range concurrentNodes {
		name := fmt.Sprintf("B%d", i+1)
		concurrentNodes[i] = &concurrentStubNode{name: name, processFn: slowProcess}
	}

	edges := []Edge{
		&stubEdge{id: "A-B1", from: "A", to: "B1", parallel: true},
		&stubEdge{id: "A-B2", from: "A", to: "B2", parallel: true},
		&stubEdge{id: "B1-C", from: "B1", to: "C"},
		&stubEdge{id: "B2-C", from: "B2", to: "C"},
		&stubEdge{id: "C-done", from: "C", to: "_done"},
	}

	allNodes := append([]Node{nodeA}, concurrentNodes...)
	allNodes = append(allNodes, nodeC)

	g, err := NewGraph("test-concurrent", allNodes, edges, nil)
	if err != nil {
		t.Fatal(err)
	}

	w := NewProcessWalker("conc-1")
	if err := g.Walk(context.Background(), w, "A"); err != nil {
		t.Fatalf("Walk failed: %v", err)
	}

	if maxConcurrent.Load() < 2 {
		t.Errorf("max concurrent = %d, want >= 2 (branches should run in parallel)", maxConcurrent.Load())
	}
}

func TestFanOut_ErrorCancelsSiblings(t *testing.T) {
	nodeA := &stubNode{name: "A", artifact: &stubArtifact{typ: "a"}}
	nodeC := &stubNode{name: "C", artifact: &stubArtifact{typ: "c"}}

	failNode := &concurrentStubNode{
		name: "B1",
		processFn: func(ctx context.Context, nc NodeContext) (Artifact, error) {
			return nil, fmt.Errorf("B1 failed")
		},
	}

	slowNode := &concurrentStubNode{
		name: "B2",
		processFn: func(ctx context.Context, nc NodeContext) (Artifact, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(5 * time.Second):
				return &stubArtifact{typ: "b2"}, nil
			}
		},
	}

	edges := []Edge{
		&stubEdge{id: "A-B1", from: "A", to: "B1", parallel: true},
		&stubEdge{id: "A-B2", from: "A", to: "B2", parallel: true},
		&stubEdge{id: "B1-C", from: "B1", to: "C"},
		&stubEdge{id: "B2-C", from: "B2", to: "C"},
		&stubEdge{id: "C-done", from: "C", to: "_done"},
	}

	g, err := NewGraph("test-error", []Node{nodeA, failNode, slowNode, nodeC}, edges, nil)
	if err != nil {
		t.Fatal(err)
	}

	w := NewProcessWalker("err-1")
	err = g.Walk(context.Background(), w, "A")
	if err == nil {
		t.Fatal("expected error from failed branch")
	}
	if w.State().Status != "error" {
		t.Errorf("status = %q, want error", w.State().Status)
	}
}

func TestFanOut_ContextDeadline(t *testing.T) {
	nodeA := &stubNode{name: "A", artifact: &stubArtifact{typ: "a"}}
	nodeC := &stubNode{name: "C", artifact: &stubArtifact{typ: "c"}}

	hangNode := func(name string) Node {
		return &concurrentStubNode{
			name: name,
			processFn: func(ctx context.Context, nc NodeContext) (Artifact, error) {
				<-ctx.Done()
				return nil, ctx.Err()
			},
		}
	}

	edges := []Edge{
		&stubEdge{id: "A-B1", from: "A", to: "B1", parallel: true},
		&stubEdge{id: "A-B2", from: "A", to: "B2", parallel: true},
		&stubEdge{id: "B1-C", from: "B1", to: "C"},
		&stubEdge{id: "B2-C", from: "B2", to: "C"},
		&stubEdge{id: "C-done", from: "C", to: "_done"},
	}

	g, err := NewGraph("test-timeout", []Node{nodeA, hangNode("B1"), hangNode("B2"), nodeC}, edges, nil)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	w := NewProcessWalker("timeout-1")
	err = g.Walk(ctx, w, "A")
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestFanOut_MergeBranchesDisagree(t *testing.T) {
	nodeA := &stubNode{name: "A", artifact: &stubArtifact{typ: "a"}}
	nodeB1 := &stubNode{name: "B1", artifact: &stubArtifact{typ: "b1"}}
	nodeB2 := &stubNode{name: "B2", artifact: &stubArtifact{typ: "b2"}}
	nodeC := &stubNode{name: "C", artifact: &stubArtifact{typ: "c"}}
	nodeD := &stubNode{name: "D", artifact: &stubArtifact{typ: "d"}}

	edges := []Edge{
		&stubEdge{id: "A-B1", from: "A", to: "B1", parallel: true},
		&stubEdge{id: "A-B2", from: "A", to: "B2", parallel: true},
		&stubEdge{id: "B1-C", from: "B1", to: "C"},
		&stubEdge{id: "B2-D", from: "B2", to: "D"},
		&stubEdge{id: "C-done", from: "C", to: "_done"},
		&stubEdge{id: "D-done", from: "D", to: "_done"},
	}

	g, err := NewGraph("test-disagree", []Node{nodeA, nodeB1, nodeB2, nodeC, nodeD}, edges, nil)
	if err != nil {
		t.Fatal(err)
	}

	w := NewProcessWalker("dis-1")
	err = g.Walk(context.Background(), w, "A")
	if !errors.Is(err, ErrFanOutMerge) {
		t.Errorf("expected ErrFanOutMerge, got %v", err)
	}
}

func TestFanOut_MergeToDone(t *testing.T) {
	nodeA := &stubNode{name: "A", artifact: &stubArtifact{typ: "a"}}
	nodeB1 := &stubNode{name: "B1", artifact: &stubArtifact{typ: "b1"}}
	nodeB2 := &stubNode{name: "B2", artifact: &stubArtifact{typ: "b2"}}

	edges := []Edge{
		&stubEdge{id: "A-B1", from: "A", to: "B1", parallel: true},
		&stubEdge{id: "A-B2", from: "A", to: "B2", parallel: true},
		&stubEdge{id: "B1-done", from: "B1", to: "_done"},
		&stubEdge{id: "B2-done", from: "B2", to: "_done"},
	}

	g, err := NewGraph("test-merge-done", []Node{nodeA, nodeB1, nodeB2}, edges, nil)
	if err != nil {
		t.Fatal(err)
	}

	w := NewProcessWalker("done-1")
	if err := g.Walk(context.Background(), w, "A"); err != nil {
		t.Fatalf("Walk failed: %v", err)
	}

	if w.State().Status != "done" {
		t.Errorf("status = %q, want done", w.State().Status)
	}
}

func TestFanOut_SequentialBeforeAndAfter(t *testing.T) {
	nodeStart := &stubNode{name: "start", artifact: &stubArtifact{typ: "s"}}
	nodeA := &stubNode{name: "A", artifact: &stubArtifact{typ: "a"}}
	nodeB1 := &stubNode{name: "B1", artifact: &stubArtifact{typ: "b1"}}
	nodeB2 := &stubNode{name: "B2", artifact: &stubArtifact{typ: "b2"}}
	nodeC := &stubNode{name: "C", artifact: &stubArtifact{typ: "c"}}
	nodeEnd := &stubNode{name: "end_node", artifact: &stubArtifact{typ: "e"}}

	edges := []Edge{
		&stubEdge{id: "start-A", from: "start", to: "A"},
		&stubEdge{id: "A-B1", from: "A", to: "B1", parallel: true},
		&stubEdge{id: "A-B2", from: "A", to: "B2", parallel: true},
		&stubEdge{id: "B1-C", from: "B1", to: "C"},
		&stubEdge{id: "B2-C", from: "B2", to: "C"},
		&stubEdge{id: "C-end", from: "C", to: "end_node"},
		&stubEdge{id: "end-done", from: "end_node", to: "_done"},
	}

	g, err := NewGraph("test-mixed",
		[]Node{nodeStart, nodeA, nodeB1, nodeB2, nodeC, nodeEnd}, edges, nil)
	if err != nil {
		t.Fatal(err)
	}

	w := NewProcessWalker("mix-1")
	if err := g.Walk(context.Background(), w, "start"); err != nil {
		t.Fatalf("Walk failed: %v", err)
	}

	if w.State().Status != "done" {
		t.Errorf("status = %q, want done", w.State().Status)
	}

	for _, name := range []string{"start", "A", "B1", "B2", "C", "end_node"} {
		if _, ok := w.State().Outputs[name]; !ok {
			t.Errorf("missing output for node %q", name)
		}
	}
}

func TestFanOut_SingleParallelEdgeFallsBackToSequential(t *testing.T) {
	nodeA := &stubNode{name: "A", artifact: &stubArtifact{typ: "a"}}
	nodeB := &stubNode{name: "B", artifact: &stubArtifact{typ: "b"}}

	edges := []Edge{
		&stubEdge{id: "A-B", from: "A", to: "B", parallel: true},
		&stubEdge{id: "B-done", from: "B", to: "_done"},
	}

	g, err := NewGraph("test-single", []Node{nodeA, nodeB}, edges, nil)
	if err != nil {
		t.Fatal(err)
	}

	w := &stubWalker{state: NewWalkerState("single-1")}
	if err := g.Walk(context.Background(), w, "A"); err != nil {
		t.Fatalf("Walk failed: %v", err)
	}
	if w.state.Status != "done" {
		t.Errorf("status = %q, want done", w.state.Status)
	}
}

func TestFanOut_BackwardCompatNoParallel(t *testing.T) {
	nodeA := &stubNode{name: "A", artifact: &stubArtifact{typ: "a"}}
	nodeB := &stubNode{name: "B", artifact: &stubArtifact{typ: "b"}}
	nodeC := &stubNode{name: "C", artifact: &stubArtifact{typ: "c"}}

	edges := []Edge{
		&stubEdge{id: "A-B", from: "A", to: "B"},
		&stubEdge{id: "B-C", from: "B", to: "C"},
		&stubEdge{id: "C-done", from: "C", to: "_done"},
	}

	g, err := NewGraph("test-compat", []Node{nodeA, nodeB, nodeC}, edges, nil)
	if err != nil {
		t.Fatal(err)
	}

	w := &stubWalker{state: NewWalkerState("compat-1")}
	if err := g.Walk(context.Background(), w, "A"); err != nil {
		t.Fatalf("Walk failed: %v", err)
	}

	want := []string{"A", "B", "C"}
	if len(w.visited) != len(want) {
		t.Fatalf("visited = %v, want %v", w.visited, want)
	}
	for i, v := range want {
		if w.visited[i] != v {
			t.Errorf("visited[%d] = %s, want %s", i, w.visited[i], v)
		}
	}
}

// concurrentStubNode allows custom Process functions for concurrency testing.
type concurrentStubNode struct {
	name      string
	processFn func(context.Context, NodeContext) (Artifact, error)
}

func (n *concurrentStubNode) Name() string            { return n.name }
func (n *concurrentStubNode) ElementAffinity() Element { return "" }
func (n *concurrentStubNode) Process(ctx context.Context, nc NodeContext) (Artifact, error) {
	return n.processFn(ctx, nc)
}
