package assemble

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/organs/code"
	tangle "github.com/dpopsuev/tangle"
)

func TestDogfood_GrepEditTestCommit(t *testing.T) {
	dir := setupGoProject(t, map[string]string{
		"go.mod": "module dogfood\n\ngo 1.21\n",
		"calc.go": `package dogfood

func Add(a, b int) int {
	return a + b
}
`,
		"calc_test.go": `package dogfood

import "testing"

func TestAdd(t *testing.T) {
	if Add(1, 2) != 3 {
		t.Error("expected 3")
	}
}
`,
	})

	completer := &scriptedCompleter{
		turns: []tangle.Completion{
			{
				Content: "Let me search for the Add function.",
				ToolCalls: []tangle.ToolCall{
					{ID: "c1", Name: "grep", Input: json.RawMessage(`{"pattern":"func Add","path":"."}`), },
				},
			},
			{
				Content: "Found it. Let me read the file.",
				ToolCalls: []tangle.ToolCall{
					{ID: "c2", Name: "file_read", Input: json.RawMessage(`{"path":"calc.go"}`)},
				},
			},
			{
				Content: "I'll add a Subtract function.",
				ToolCalls: []tangle.ToolCall{
					{ID: "c3", Name: "edit", Input: json.RawMessage(`{"path":"calc.go","old_string":"func Add(a, b int) int {\n\treturn a + b\n}","new_string":"func Add(a, b int) int {\n\treturn a + b\n}\n\nfunc Subtract(a, b int) int {\n\treturn a - b\n}"}`)},
				},
			},
			{
				Content: "Now add a test for Subtract.",
				ToolCalls: []tangle.ToolCall{
					{ID: "c4", Name: "edit", Input: json.RawMessage(`{"path":"calc_test.go","old_string":"func TestAdd(t *testing.T) {\n\tif Add(1, 2) != 3 {\n\t\tt.Error(\"expected 3\")\n\t}\n}","new_string":"func TestAdd(t *testing.T) {\n\tif Add(1, 2) != 3 {\n\t\tt.Error(\"expected 3\")\n\t}\n}\n\nfunc TestSubtract(t *testing.T) {\n\tif Subtract(5, 3) != 2 {\n\t\tt.Error(\"expected 2\")\n\t}\n}"}`)},
				},
			},
			{
				Content: "Running tests.",
				ToolCalls: []tangle.ToolCall{
					{ID: "c5", Name: "go_test", Input: json.RawMessage(`{"package":"./..."}`)},
				},
			},
			{
				Content: "Tests pass. Committing.",
				ToolCalls: []tangle.ToolCall{
					{ID: "c6", Name: "git_commit", Input: json.RawMessage(`{"message":"feat: add Subtract function with test","files":["calc.go","calc_test.go"]}`)},
				},
			},
			{
				Content: `{"atoms":[{"type":"retrospection","taxonomy":"retrospection.done","content":"Added Subtract function and test. All tests pass. Committed."}]}`,
			},
		},
	}

	caps := code.Capabilities(dir)
	bp := Blueprint{
		Model:        "stub",
		Capabilities: caps,
		Budget:       cerebrum.Budget{MaxTurns: 15, TurnTimeout: 30 * time.Second},
	}

	agent := Assemble(bp, completer)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	_, err := agent.Think(ctx, "Add a Subtract function to calc.go with a test")
	if err != nil {
		t.Fatalf("Think: %v", err)
	}

	calcGo, err := os.ReadFile(filepath.Join(dir, "calc.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(calcGo), "func Subtract") {
		t.Error("calc.go should contain Subtract function")
	}

	calcTest, err := os.ReadFile(filepath.Join(dir, "calc_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(calcTest), "TestSubtract") {
		t.Error("calc_test.go should contain TestSubtract")
	}

	testCmd := exec.CommandContext(ctx, "go", "test", "./...")
	testCmd.Dir = dir
	testOut, err := testCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go test failed: %s\n%s", err, testOut)
	}

	gitLog, err := exec.Command("git", "-C", dir, "log", "--oneline", "-1").Output()
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	if !strings.Contains(string(gitLog), "add Subtract") {
		t.Errorf("commit message should mention Subtract, got: %s", gitLog)
	}

	t.Logf("Dogfood proof: grep → read → edit(x2) → test → commit")
	t.Logf("calc.go:\n%s", calcGo)
	t.Logf("Git: %s", strings.TrimSpace(string(gitLog)))
}

func TestDogfood_ToolErrorRecovery(t *testing.T) {
	dir := setupGoProject(t, map[string]string{
		"go.mod":  "module dogfood\n\ngo 1.21\n",
		"main.go": "package dogfood\n",
	})

	completer := &scriptedCompleter{
		turns: []tangle.Completion{
			{
				Content: "Reading nonexistent file.",
				ToolCalls: []tangle.ToolCall{
					{ID: "c1", Name: "file_read", Input: json.RawMessage(`{"path":"nonexistent.go"}`)},
				},
			},
			{
				Content: "File doesn't exist. Reading main.go instead.",
				ToolCalls: []tangle.ToolCall{
					{ID: "c2", Name: "file_read", Input: json.RawMessage(`{"path":"main.go"}`)},
				},
			},
			{
				Content: `{"atoms":[{"type":"retrospection","taxonomy":"retrospection.done","content":"recovered from error"}]}`,
			},
		},
	}

	caps := code.Capabilities(dir)
	bp := Blueprint{
		Model:        "stub",
		Capabilities: caps,
		Budget:       cerebrum.Budget{MaxTurns: 10, TurnTimeout: 10 * time.Second},
	}

	agent := Assemble(bp, completer)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := agent.Think(ctx, "read a file")
	if err != nil {
		t.Fatalf("Think should not fail on tool error: %v", err)
	}

	if completer.call < 3 {
		t.Errorf("expected 3 calls (error → recovery → done), got %d", completer.call)
	}
}

func TestDogfood_SubagentExplore(t *testing.T) {
	dir := setupGoProject(t, map[string]string{
		"go.mod":  "module dogfood\n\ngo 1.21\n",
		"main.go": "package dogfood\n\nfunc Hello() string { return \"hello\" }\n",
	})

	completer := &scriptedCompleter{
		turns: []tangle.Completion{
			{
				Content: "Delegating exploration to subagent.",
				ToolCalls: []tangle.ToolCall{
					{ID: "c1", Name: "agent_spawn", Input: json.RawMessage(`{"task":"find all Go files","type":"explore","max_turns":3}`)},
				},
			},
			{
				Content: `{"atoms":[{"type":"retrospection","taxonomy":"retrospection.done","content":"subagent found the files"}]}`,
			},
		},
	}

	caps := code.Capabilities(dir)
	bp := Blueprint{
		Model:        "stub",
		Capabilities: caps,
		Budget:       cerebrum.Budget{MaxTurns: 10, TurnTimeout: 30 * time.Second},
	}

	agent := Assemble(bp, completer)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := agent.Think(ctx, "explore the codebase")
	if err != nil {
		t.Fatalf("Think: %v", err)
	}
}

func setupGoProject(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()

	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "add", "."},
		{"git", "commit", "-m", "initial"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s\n%s", args, err, out)
		}
	}

	return dir
}
