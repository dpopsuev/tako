package cerebrum

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/dpopsuev/tako/agent/shell"
)

func TestComputeOverlap(t *testing.T) {
	tests := []struct {
		name     string
		residual map[string]float64
		pattern  map[string]float64
		want     float64
	}{
		{"empty pattern", map[string]float64{"a": 1}, nil, 0},
		{"exact match", map[string]float64{"a": 1, "b": 2}, map[string]float64{"a": 1, "b": 2}, 1.0},
		{"partial match", map[string]float64{"a": 1, "b": 2}, map[string]float64{"a": 1, "c": 3}, 0.5},
		{"no match", map[string]float64{"a": 1}, map[string]float64{"b": 2}, 0},
		{"value mismatch", map[string]float64{"a": 1}, map[string]float64{"a": 9}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeOverlap(tt.residual, tt.pattern)
			if got != tt.want {
				t.Errorf("computeOverlap() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSelectGear(t *testing.T) {
	tests := []struct {
		overlap float64
		want    Gear
	}{
		{1.0, GearReflex},
		{0.9, GearIntuition},
		{0.7, GearIntuition},
		{0.5, GearFamiliar},
		{0.3, GearFamiliar},
		{0.2, GearNovel},
		{0, GearNovel},
	}

	for _, tt := range tests {
		got := selectGear(tt.overlap)
		if got != tt.want {
			t.Errorf("selectGear(%v) = %v, want %v", tt.overlap, got, tt.want)
		}
	}
}

func TestReflexStoreMatch(t *testing.T) {
	caps := []shell.Capability{
		{Name: "read_file"},
		{Name: "write_file"},
		{Name: "bash"},
	}

	store := NewReflexStore(caps)
	store.AddReflex(
		map[string]float64{"file.exists": 0, "content.unknown": 1},
		[]string{"read_file"},
	)
	store.AddReflex(
		map[string]float64{"file.exists": 1, "content.known": 1},
		[]string{"write_file"},
	)

	t.Run("exact match returns reflex gear", func(t *testing.T) {
		residual := map[string]float64{"file.exists": 0, "content.unknown": 1}
		caps, overlap := store.Match(residual)
		if overlap != 1.0 {
			t.Fatalf("overlap = %v, want 1.0", overlap)
		}
		if len(caps) != 1 || caps[0].Name != "read_file" {
			t.Fatalf("caps = %v, want [read_file]", caps)
		}
	})

	t.Run("nil residual returns zero", func(t *testing.T) {
		caps, overlap := store.Match(nil)
		if overlap != 0 || caps != nil {
			t.Fatalf("expected nil/0, got %v/%v", caps, overlap)
		}
	})

	t.Run("no match returns zero", func(t *testing.T) {
		caps, overlap := store.Match(map[string]float64{"x": 99})
		if overlap != 0 || len(caps) != 0 {
			t.Fatalf("expected empty/0, got %v/%v", caps, overlap)
		}
	})

	t.Run("best match wins", func(t *testing.T) {
		residual := map[string]float64{
			"file.exists":    1,
			"content.known":  1,
			"content.unknown": 1,
		}
		caps, overlap := store.Match(residual)
		if overlap != 1.0 {
			t.Fatalf("overlap = %v, want 1.0", overlap)
		}
		if len(caps) != 1 || caps[0].Name != "write_file" {
			t.Fatalf("caps = %v, want [write_file]", caps)
		}
	})
}

func TestReflexStoreEmpty(t *testing.T) {
	store := NewReflexStore(nil)
	caps, overlap := store.Match(map[string]float64{"a": 1})
	if overlap != 0 || caps != nil {
		t.Fatalf("empty store should return nil/0")
	}
}

func TestFireReflex(t *testing.T) {
	var called atomic.Int32
	caps := []shell.Capability{
		{
			Name: "test_cap",
			Execute: func(_ context.Context, _ json.RawMessage) (shell.Result, error) {
				called.Add(1)
				return shell.TextResult("ok"), nil
			},
		},
		{Name: "nil_execute"},
		{
			Name: "error_cap",
			Execute: func(_ context.Context, _ json.RawMessage) (shell.Result, error) {
				return shell.Result{}, errors.New("boom")
			},
		},
	}

	fireReflex(context.Background(), caps, 0.9)
	if called.Load() != 1 {
		t.Fatalf("expected 1 call, got %d", called.Load())
	}
}

func TestReflexStoreInterface(t *testing.T) {
	var _ ReflexStore = (*InMemoryReflexStore)(nil)
}
