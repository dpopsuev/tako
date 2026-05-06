package e2e

import (
	"context"
	"testing"

	"github.com/dpopsuev/tako/testkit/rehearsal"
	"github.com/dpopsuev/tako/testkit"
)

func TestUserStory_AddFunction(t *testing.T) {
	testkit.SkipWithoutLLM(t)

	dir := rehearsal.SetupWorkspace(t,
		rehearsal.WithExtraFiles(map[string]string{
			"math.go": `package main
`,
		}),
		rehearsal.WithGitRepo(),
	)

	agent := testkit.NewRealAgent(t, dir)
	result := testkit.RunAgent(t, agent, "Add a function Add(a, b int) int to math.go that returns the sum of a and b. Also add a test in the same package that verifies Add(2,3) == 5. Make sure go test ./... passes.")

	referee := testkit.GoReferee{WorkDir: dir}
	check := referee.Check(context.Background())

	if !check.Compiles {
		t.Fatalf("should compile.\nErrors: %v", check.Errors)
	}
	if !check.TestsPass {
		t.Fatalf("tests should pass.\nErrors: %v", check.Errors)
	}

	t.Logf("PASS: agent added function in %d turns", agent.Result().Turns())
	t.Logf("Result: %s", result)
}
