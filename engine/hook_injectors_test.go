package engine

import (
	"context"
	"errors"
	"testing"

	"github.com/dpopsuev/tako/circuit"
)

// stubArtifact defined in testhelpers_test.go

func TestNewContextInjector_InjectsData(t *testing.T) {
	t.Parallel()

	called := false
	hook := NewContextInjector("inject.test", func(walkerCtx map[string]any) {
		walkerCtx["injected"] = "hello"
		called = true
	})

	if hook.Name() != "inject.test" {
		t.Errorf("Name() = %q, want %q", hook.Name(), "inject.test")
	}

	walker := circuit.NewProcessWalker("test-walker")
	ctx := WithWalkerState(context.Background(), walker.State())

	err := hook.Run(ctx, "node", &stubArtifact{})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if !called {
		t.Error("injector function was not called")
	}
	if walker.State().Context["injected"] != "hello" {
		t.Errorf("context[injected] = %v, want hello", walker.State().Context["injected"])
	}
}

func TestNewContextInjector_NilWalkerState(t *testing.T) {
	t.Parallel()

	called := false
	hook := NewContextInjector("inject.noop", func(_ map[string]any) {
		called = true
	})

	err := hook.Run(context.Background(), "node", &stubArtifact{})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if called {
		t.Error("injector should not be called when walker state is nil")
	}
}

func TestNewContextInjectorErr_ReturnsError(t *testing.T) {
	t.Parallel()

	want := errors.New("inject failed")
	hook := NewContextInjectorErr("inject.err", func(_ context.Context, _ map[string]any) error {
		return want
	})

	walker := circuit.NewProcessWalker("test-walker")
	ctx := WithWalkerState(context.Background(), walker.State())

	err := hook.Run(ctx, "node", &stubArtifact{})
	if !errors.Is(err, want) {
		t.Errorf("Run error = %v, want %v", err, want)
	}
}

func TestNewContextInjectorErr_NilWalkerState(t *testing.T) {
	t.Parallel()

	hook := NewContextInjectorErr("inject.err-noop", func(_ context.Context, _ map[string]any) error {
		return errors.New("should not reach")
	})

	err := hook.Run(context.Background(), "node", &stubArtifact{})
	if err != nil {
		t.Fatalf("should be no-op with nil walker state: %v", err)
	}
}

func TestNewContextInjectorErr_Success(t *testing.T) {
	t.Parallel()

	hook := NewContextInjectorErr("inject.ok", func(_ context.Context, walkerCtx map[string]any) error {
		walkerCtx["data"] = 42
		return nil
	})

	walker := circuit.NewProcessWalker("test")
	ctx := WithWalkerState(context.Background(), walker.State())

	err := hook.Run(ctx, "node", &stubArtifact{})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if walker.State().Context["data"] != 42 {
		t.Errorf("context[data] = %v, want 42", walker.State().Context["data"])
	}
}
