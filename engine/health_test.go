package engine

import (
	"context"
	"errors"
	"testing"
)

type healthyComponent struct{}

func (h *healthyComponent) HealthCheck(context.Context) error { return nil }

type unhealthyComponent struct{ err error }

func (u *unhealthyComponent) HealthCheck(_ context.Context) error { return u.err }

func TestCheckComponentHealth_AllHealthy(t *testing.T) {
	comps := []*Component{
		{Name: "rp", Health: &healthyComponent{}},
		{Name: "beta", Health: &healthyComponent{}},
	}
	if err := CheckComponentHealth(context.Background(), comps); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestCheckComponentHealth_OneUnhealthy(t *testing.T) {
	comps := []*Component{
		{Name: "rp", Health: &healthyComponent{}},
		{Name: "beta", Health: &unhealthyComponent{err: errors.New("connection refused")}},
	}
	err := CheckComponentHealth(context.Background(), comps)
	if err == nil {
		t.Fatal("expected error for unhealthy component")
	}
	if !errors.Is(err, ErrComponentUnhealthy) {
		t.Errorf("want ErrComponentUnhealthy, got: %v", err)
	}
}

func TestCheckComponentHealth_NoHealthChecker_Skipped(t *testing.T) {
	comps := []*Component{
		{Name: "rp"}, // nil Health — skipped
	}
	if err := CheckComponentHealth(context.Background(), comps); err != nil {
		t.Fatalf("expected no error for component without health checker, got: %v", err)
	}
}

func TestCheckComponentHealth_Empty(t *testing.T) {
	if err := CheckComponentHealth(context.Background(), nil); err != nil {
		t.Fatalf("expected no error for empty list, got: %v", err)
	}
}
