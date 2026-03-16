package framework

import (
	"context"
	"testing"
)

type execTestNode struct {
	name     string
	artifact Artifact
}

func (n execTestNode) Name() string              { return n.name }
func (n execTestNode) ElementAffinity() Element   { return "" }
func (n execTestNode) Process(_ context.Context, _ NodeContext) (Artifact, error) {
	return n.artifact, nil
}

type execTestArtifact struct{ val string }

func (a execTestArtifact) Type() string       { return "test" }
func (a execTestArtifact) Confidence() float64 { return 1.0 }
func (a execTestArtifact) Raw() any            { return a.val }

func TestInProcessExecutor(t *testing.T) {
	node := execTestNode{name: "test", artifact: execTestArtifact{val: "hello"}}
	exec := InProcessExecutor{}

	art, err := exec.Execute(context.Background(), node, NodeContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if art.Raw() != "hello" {
		t.Errorf("got %v, want hello", art.Raw())
	}
}

func TestExecutorFunc(t *testing.T) {
	called := false
	exec := ExecutorFunc(func(_ context.Context, node Node, _ NodeContext) (Artifact, error) {
		called = true
		return node.Process(context.Background(), NodeContext{})
	})

	node := execTestNode{name: "test", artifact: execTestArtifact{val: "world"}}
	art, err := exec.Execute(context.Background(), node, NodeContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("ExecutorFunc was not called")
	}
	if art.Raw() != "world" {
		t.Errorf("got %v, want world", art.Raw())
	}
}
