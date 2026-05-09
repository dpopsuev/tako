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

func TestUserStory_RefactorRenameFunction(t *testing.T) {
	testkit.SkipWithoutLLM(t)

	dir := rehearsal.SetupWorkspace(t,
		rehearsal.WithExtraFiles(map[string]string{
			"calc.go": `package main

func Add(a, b int) int {
	return a + b
}

func Multiply(a, b int) int {
	return a * b
}
`,
			"calc_test.go": `package main

import "testing"

func TestAdd(t *testing.T) {
	if Add(2, 3) != 5 {
		t.Error("expected 5")
	}
}

func TestMultiply(t *testing.T) {
	if Multiply(3, 4) != 12 {
		t.Error("expected 12")
	}
}
`,
			"main.go": `package main

import "fmt"

func main() {
	fmt.Println(Add(1, 2))
	fmt.Println(Multiply(3, 4))
}
`,
		}),
		rehearsal.WithGitRepo(),
	)

	referee := testkit.GoReferee{WorkDir: dir}

	agent := testkit.NewRealAgent(t, dir)
	result := testkit.RunAgent(t, agent,
		"Rename the function 'Multiply' to 'Product' in all files. Update the test and main.go. Make sure tests pass.")

	check := referee.Check(context.Background())

	calcGo, _ := os.ReadFile(filepath.Join(dir, "calc.go"))
	calcTest, _ := os.ReadFile(filepath.Join(dir, "calc_test.go"))
	mainGo, _ := os.ReadFile(filepath.Join(dir, "main.go"))

	if strings.Contains(string(calcGo), "Multiply") {
		t.Error("calc.go should not contain 'Multiply' after rename")
	}
	if !strings.Contains(string(calcGo), "Product") {
		t.Error("calc.go should contain 'Product' after rename")
	}
	if !strings.Contains(string(calcTest), "TestProduct") || strings.Contains(string(calcTest), "TestMultiply") {
		t.Error("calc_test.go should rename TestMultiply to TestProduct")
	}
	if !strings.Contains(string(mainGo), "Product") {
		t.Error("main.go should call Product, not Multiply")
	}
	if !check.TestsPass {
		t.Fatalf("tests should pass after rename.\nResult: %s\nErrors: %v", result, check.Errors)
	}

	t.Logf("PASS: refactor rename in %d turns", agent.Result().Turns())
}
