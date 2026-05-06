package e2e

import (
	"context"
	"testing"

	"github.com/dpopsuev/tako/testkit/rehearsal"
	"github.com/dpopsuev/tako/testkit"
)

func TestUserStory_FixFailingTest(t *testing.T) {
	testkit.SkipWithoutLLM(t)

	dir := rehearsal.SetupWorkspace(t,
		rehearsal.WithExtraFiles(map[string]string{
			"calc.go": `package main

func Multiply(a, b int) int {
	return a * b
}
`,
			"calc_test.go": `package main

import "testing"

func TestMultiply(t *testing.T) {
	got := Multiply(3, 4)
	if got != 11 {
		t.Errorf("Multiply(3,4) = %d, want 11", got)
	}
}
`,
		}),
		rehearsal.WithGitRepo(),
	)

	agent := testkit.NewRealAgent(t, dir)
	result := testkit.RunAgent(t, agent, "The test in calc_test.go is failing. Fix it so that go test passes.")

	referee := testkit.GoReferee{WorkDir: dir}
	check := referee.Check(context.Background())

	if !check.TestsPass {
		t.Fatalf("tests should pass after fix.\nResult: %s\nErrors: %v", result, check.Errors)
	}

	t.Logf("PASS: agent fixed failing test in %d turns", agent.Result().Turns())
	t.Logf("Result: %s", result)
}
