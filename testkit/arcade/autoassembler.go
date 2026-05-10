package arcade

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dpopsuev/tako/agent/organ"
)

func NewAutoassembler(workDir string) Scenario {
	setupWorkspace(workDir)

	adv := NewGame(map[string]any{
		"build_clean":   false,
		"tests_pass":    false,
		"files_read":    0,
		"files_written": 0,
	})

	adv.Organ("file_read", "Read a file from the workspace",
		json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"relative file path"}},"required":["path"]}`),
		organ.ReadAction,
		func(s map[string]any, input json.RawMessage) (organ.Result, error) {
			var args struct{ Path string `json:"path"` }
			json.Unmarshal(input, &args)
			abs := filepath.Join(workDir, filepath.Clean(args.Path))
			data, err := os.ReadFile(abs)
			if err != nil {
				return organ.TextResult(fmt.Sprintf("error: %v", err)), nil
			}
			count, _ := s["files_read"].(int)
			s["files_read"] = count + 1
			return organ.TextResult(string(data)), nil
		})

	adv.Organ("file_write", "Write content to a file",
		json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"relative file path"},"content":{"type":"string","description":"file content"}},"required":["path","content"]}`),
		organ.WriteAction,
		func(s map[string]any, input json.RawMessage) (organ.Result, error) {
			var args struct {
				Path    string `json:"path"`
				Content string `json:"content"`
			}
			if err := json.Unmarshal(input, &args); err != nil {
				return organ.ErrorResult(fmt.Sprintf("invalid input: %v", err)), nil
			}
			abs := filepath.Join(workDir, filepath.Clean(args.Path))
			if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
				return organ.TextResult(fmt.Sprintf("error: %v", err)), nil
			}
			if err := os.WriteFile(abs, []byte(args.Content), 0o644); err != nil {
				return organ.TextResult(fmt.Sprintf("error: %v", err)), nil
			}
			count, _ := s["files_written"].(int)
			s["files_written"] = count + 1
			return organ.TextResult(fmt.Sprintf("wrote %d bytes to %s", len(args.Content), args.Path)), nil
		})

	adv.Organ("go_build", "Run go build on the workspace. Checks if code compiles.", emptySchema, organ.ReadAction,
		func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
			src := filepath.Join(workDir, "greet", "greet.go")
			data, err := os.ReadFile(src)
			if err != nil {
				return organ.TextResult("build failed: greet/greet.go not found"), nil
			}
			content := string(data)
			if strings.Contains(content, "TODO") || strings.Contains(content, "panic(\"not implemented\")") {
				return organ.TextResult("build failed: implementation contains TODO or panic stubs"), nil
			}
			if !strings.Contains(content, "package greet") {
				return organ.TextResult("build failed: missing package declaration"), nil
			}
			if !strings.Contains(content, "func Greet") {
				return organ.TextResult("build failed: missing Greet function"), nil
			}
			s["build_clean"] = true
			return organ.TextResult("build ok: greet package compiles"), nil
		})

	adv.Organ("go_test", "Run go test on the workspace. Verifies the implementation.", emptySchema, organ.WriteAction,
		func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
			src := filepath.Join(workDir, "greet", "greet.go")
			data, err := os.ReadFile(src)
			if err != nil {
				return organ.TextResult("test failed: greet/greet.go not found"), nil
			}
			content := string(data)
			if !strings.Contains(content, "func Greet") {
				return organ.TextResult("test failed: Greet function not found"), nil
			}
			if !strings.Contains(content, "return") {
				return organ.TextResult("test failed: Greet function has no return statement"), nil
			}
			s["tests_pass"] = true
			return organ.TextResult("test ok: all tests pass\n--- PASS: TestGreet (0.00s)\nPASS"), nil
		})

	adv.Organ("check_status", "Check build and test status.", emptySchema, organ.ReadAction,
		func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
			buildClean, _ := s["build_clean"].(bool)
			testsPass, _ := s["tests_pass"].(bool)
			if buildClean && testsPass {
				return organ.TextResult("status: build clean, tests pass — task complete"), nil
			}
			var parts []string
			if !buildClean {
				parts = append(parts, "build not clean")
			}
			if !testsPass {
				parts = append(parts, "tests not passing")
			}
			return organ.TextResult(fmt.Sprintf("status: %s", strings.Join(parts, ", "))), nil
		})

	return Scenario{
		Name: "autoassembler",
		Need: `You are a code agent. Your task: implement the Greet function in greet/greet.go.

The file exists with a TODO stub. Read it first, then write the implementation.
The function should accept a name string and return a greeting string like "Hello, <name>!".
After writing, run go_build to verify it compiles, then go_test to verify correctness.
Use check_status to confirm both build_clean and tests_pass are true.`,
		Adventure:    adv,
		IsSolved:     func(s map[string]any) bool { return s["build_clean"] == true && s["tests_pass"] == true },
		OptimalTurns: 5,
		Desired:      map[string]any{"build_clean": true, "tests_pass": true},
	}
}

func setupWorkspace(dir string) {
	greetDir := filepath.Join(dir, "greet")
	os.MkdirAll(greetDir, 0o755)
	os.WriteFile(filepath.Join(greetDir, "greet.go"), []byte(`package greet

// Greet returns a greeting for the given name.
// TODO: implement this function
func Greet(name string) string {
	panic("not implemented")
}
`), 0o644)

	os.WriteFile(filepath.Join(greetDir, "greet_test.go"), []byte(`package greet

import "testing"

func TestGreet(t *testing.T) {
	got := Greet("World")
	if got != "Hello, World!" {
		t.Errorf("Greet(\"World\") = %q, want \"Hello, World!\"", got)
	}
}

func TestGreetEmpty(t *testing.T) {
	got := Greet("")
	if got != "Hello, !" {
		t.Errorf("Greet(\"\") = %q, want \"Hello, !\"", got)
	}
}
`), 0o644)
}

func NewAutoassemblerWithContext() (Scenario, string) {
	dir, _ := os.MkdirTemp("", "autoassembler-*")
	return NewAutoassembler(dir), dir
}
