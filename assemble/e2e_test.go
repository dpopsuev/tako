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
	"github.com/dpopsuev/tako/shells/code"
	tangle "github.com/dpopsuev/tangle"
)

func TestE2E_ReadEditTestCommit(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module testproject\n\ngo 1.21\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(`package testproject

func Hello() string {
	return "hello"
}
`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main_test.go"), []byte(`package testproject

import "testing"

func TestHello(t *testing.T) {
	if Hello() != "hello" {
		t.Error("expected hello")
	}
}
`), 0644); err != nil {
		t.Fatal(err)
	}

	gitInit(t, dir)

	completer := &scriptedCompleter{
		turns: []tangle.Completion{
			{
				Content: "Let me read the code first.",
				ToolCalls: []tangle.ToolCall{
					{ID: "c1", Name: "read_file", Input: json.RawMessage(`{"path":"main.go"}`)},
				},
			},
			{
				Content: "I'll change the greeting.",
				ToolCalls: []tangle.ToolCall{
					{ID: "c2", Name: "edit", Input: json.RawMessage(`{"path":"main.go","old_string":"return \"hello\"","new_string":"return \"world\""}`)},
				},
			},
			{
				Content: "Now update the test.",
				ToolCalls: []tangle.ToolCall{
					{ID: "c3", Name: "edit", Input: json.RawMessage(`{"path":"main_test.go","old_string":"\"hello\"","new_string":"\"world\""}`)},
				},
			},
			{
				Content: "Let me run the tests.",
				ToolCalls: []tangle.ToolCall{
					{ID: "c4", Name: "go_test", Input: json.RawMessage(`{"package":"./..."}`)},
				},
			},
			{
				Content: "Tests pass. Committing.",
				ToolCalls: []tangle.ToolCall{
					{ID: "c5", Name: "git_commit", Input: json.RawMessage(`{"message":"feat: change greeting to world","files":["main.go","main_test.go"]}`)},
				},
			},
			{
				Content: `{"atoms":[{"type":"retrospection","taxonomy":"retrospection.done","content":"Changed greeting from hello to world, tests pass, committed."}]}`,
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
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	err := agent.Think(ctx, "Change the greeting from hello to world, update the test, verify it passes, and commit")
	if err != nil {
		t.Fatalf("Think: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"world"`) {
		t.Errorf("main.go should contain 'world', got:\n%s", data)
	}

	testData, err := os.ReadFile(filepath.Join(dir, "main_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(testData), `"world"`) {
		t.Errorf("main_test.go should contain 'world', got:\n%s", testData)
	}

	out, err := exec.CommandContext(ctx, "git", "-C", dir, "log", "--oneline", "-1").Output()
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	if !strings.Contains(string(out), "change greeting to world") {
		t.Errorf("expected commit message, got: %s", out)
	}

	if completer.call < 6 {
		t.Errorf("expected at least 6 completer calls, got %d", completer.call)
	}

	t.Logf("E2E success: read → edit → edit → test → commit")
	t.Logf("Final main.go:\n%s", data)
	t.Logf("Git log: %s", strings.TrimSpace(string(out)))
}

func gitInit(t *testing.T, dir string) {
	t.Helper()
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
}
