package e2e

import (
	"context"
	"testing"

	"github.com/dpopsuev/tako/testkit/rehearsal"
	"github.com/dpopsuev/tako/testkit/rig"
)

func TestUserStory_FixCompileError(t *testing.T) {
	rig.SkipWithoutLLM(t)

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

	agent := rig.NewRealAgent(t, dir)
	result := rig.RunAgent(t, agent, "There is a compile error in main.go. Fix it so the project builds successfully.")

	referee := rig.GoReferee{WorkDir: dir}
	check := referee.Check(context.Background())

	if !check.Compiles {
		t.Fatalf("agent should have fixed the compile error.\nResult: %s\nErrors: %v", result, check.Errors)
	}

	t.Logf("PASS: agent fixed compile error in %d turns", agent.Result().Turns())
	t.Logf("Result: %s", result)
}
