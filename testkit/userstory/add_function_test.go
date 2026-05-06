package userstory

import (
	"context"
	"testing"

	"github.com/dpopsuev/tako/testkit/rehearsal"
)

func TestUserStory_AddFunction(t *testing.T) {
	SkipWithoutLLM(t)

	dir := rehearsal.SetupWorkspace(t,
		rehearsal.WithExtraFiles(map[string]string{
			"math.go": `package main
`,
		}),
		rehearsal.WithGitRepo(),
	)

	agent := NewRealAgent(t, dir)
	result := RunAgent(t, agent, "Add a function Add(a, b int) int to math.go that returns the sum of a and b. Also add a test file math_test.go that verifies Add(2,3) == 5. Make sure go test passes.")

	referee := GoReferee{WorkDir: dir}
	check := referee.Check(context.Background())
	referee.AssertFileContains(&check, "math.go", "func Add")
	referee.AssertFileExists(&check, "math_test.go")

	if !check.Compiles {
		t.Fatalf("should compile.\nErrors: %v", check.Errors)
	}
	if !check.TestsPass {
		t.Fatalf("tests should pass.\nErrors: %v", check.Errors)
	}
	if !check.Pass() {
		t.Fatalf("referee checks failed.\nErrors: %v", check.Errors)
	}

	t.Logf("PASS: agent added function in %d turns", agent.Result().Turns())
	t.Logf("Result: %s", result)
}
