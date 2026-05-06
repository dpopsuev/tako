package userstory

import (
	"context"
	"testing"

	"github.com/dpopsuev/tako/testkit/rehearsal"
)

func TestUserStory_FixCompileError(t *testing.T) {
	SkipWithoutLLM(t)

	dir := rehearsal.SetupWorkspace(t,
		rehearsal.WithExtraFiles(map[string]string{
			"main.go": `package main

import "fmt"

func main( {
	fmt.Println("hello world")
}
`,
		}),
		rehearsal.WithGitRepo(),
	)

	agent := NewRealAgent(t, dir)
	result := RunAgent(t, agent, "There is a compile error in main.go. Fix it so the project builds successfully.")

	referee := GoReferee{WorkDir: dir}
	check := referee.Check(context.Background())

	if !check.Compiles {
		t.Fatalf("agent should have fixed the compile error.\nResult: %s\nErrors: %v", result, check.Errors)
	}

	t.Logf("PASS: agent fixed compile error in %d turns", agent.Result().Turns())
	t.Logf("Result: %s", result)
}
