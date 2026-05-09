package e2e

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dpopsuev/tako/testkit"
	"github.com/dpopsuev/tako/testkit/rehearsal"
)

func TestUserStory_WriteTestsForUntested(t *testing.T) {
	testkit.SkipWithoutLLM(t)

	dir := rehearsal.SetupWorkspace(t,
		rehearsal.WithExtraFiles(map[string]string{
			"math.go": `package main

// Max returns the larger of a or b.
func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Clamp restricts v to the range [lo, hi].
func Clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Abs returns the absolute value of n.
func Abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
`,
		}),
		rehearsal.WithGitRepo(),
	)

	referee := testkit.GoReferee{WorkDir: dir}

	agent := testkit.NewRealAgent(t, dir)
	result := testkit.RunAgent(t, agent,
		"Write tests for the functions in math.go. Cover edge cases: zero, negative, equal values, boundary conditions. Run the tests to verify.")

	check := referee.Check(context.Background())

	testFile := filepath.Join(dir, "math_test.go")
	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("math_test.go should exist: %v\nResult: %s", err, result)
	}

	content := string(data)
	if !strings.Contains(content, "TestMax") {
		t.Error("should contain TestMax")
	}
	if !strings.Contains(content, "TestClamp") {
		t.Error("should contain TestClamp")
	}
	if !strings.Contains(content, "TestAbs") {
		t.Error("should contain TestAbs")
	}
	if !check.TestsPass {
		t.Fatalf("tests should pass.\nResult: %s\nErrors: %v", result, check.Errors)
	}

	t.Logf("PASS: wrote tests in %d turns", agent.Result().Turns())
}
