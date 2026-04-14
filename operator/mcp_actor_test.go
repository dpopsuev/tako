package operator_test

import (
	"testing"
	"time"

	"github.com/dpopsuev/origami/operator"
	"github.com/dpopsuev/origami/simulate/sdlc"
)

func TestMCPActor_RunsCircuitViaMCP(t *testing.T) {
	if testing.Short() {
		t.Skip("-short flag set")
	}

	t.Setenv("SDLC_MODE", "stub")
	t.Setenv("SDLC_REPO_PATH", "../simulate/sdlc")

	actor := operator.NewMCPActor(
		sdlc.SessionFactory(),
		operator.WithMCPTimeout(30*time.Second),
	)

	result, err := actor.Act(operator.DriftResult{
		Drifted: true,
		Reasons: []string{"scan findings detected"},
	})
	if err != nil {
		t.Fatalf("Act: %v", err)
	}
	if !result.Success {
		t.Fatalf("circuit failed: %s", result.Error)
	}
	t.Logf("circuit completed in %s", result.Duration)
}

func TestMCPActor_ImplementsActor(t *testing.T) {
	var _ operator.Actor = (*operator.MCPActor)(nil)
}
