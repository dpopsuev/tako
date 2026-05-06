package e2e

import (
	"context"
	"testing"

	"github.com/dpopsuev/tako/testkit/rehearsal"
	"github.com/dpopsuev/tako/testkit/rig"
)

func TestUserStory_MultistepGrepEditTestCommit(t *testing.T) {
	rig.SkipWithoutLLM(t)

	dir := rehearsal.SetupWorkspace(t,
		rehearsal.WithExtraFiles(map[string]string{
			"util.go": `package main

// TODO: implement Reverse function that reverses a string

func Reverse(s string) string {
	return s
}
`,
			"util_test.go": `package main

import "testing"

func TestReverse(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"hello", "olleh"},
		{"", ""},
		{"a", "a"},
		{"ab", "ba"},
	}
	for _, tt := range tests {
		got := Reverse(tt.in)
		if got != tt.want {
			t.Errorf("Reverse(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
`,
		}),
		rehearsal.WithGitRepo(),
	)

	referee := rig.GoReferee{WorkDir: dir}
	baselineCheck := referee.Check(context.Background())
	baselineCommits := baselineCheck.GitCommits

	agent := rig.NewRealAgent(t, dir)
	result := rig.RunAgent(t, agent, "Find the TODO in util.go, implement the Reverse function correctly so it actually reverses strings, verify the tests pass, and commit the change.")

	check := referee.Check(context.Background())
	referee.AssertFileNotContains(&check, "util.go", "// TODO")
	referee.AssertNewCommit(&check, baselineCommits)

	if !check.TestsPass {
		t.Fatalf("tests should pass.\nResult: %s\nErrors: %v", result, check.Errors)
	}
	if !check.Pass() {
		t.Fatalf("referee checks failed.\nErrors: %v", check.Errors)
	}

	t.Logf("PASS: agent completed multi-step workflow in %d turns", agent.Result().Turns())
	t.Logf("Commits: %d → %d", baselineCommits, check.GitCommits)
	t.Logf("Result: %s", result)
}
