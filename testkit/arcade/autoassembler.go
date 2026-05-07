package arcade

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dpopsuev/tako/agent/organ"
)

// NewAutoassembler creates a scenario where the agent implements a Go function.
// The workspace has a stub file with a TODO. The agent must:
// 1. Read the file to understand what's needed
// 2. Write the implementation
// 3. Run go build to verify compilation
// 4. Run go test to verify correctness
// Seals when build_clean=true AND tests_pass=true.
func NewAutoassembler(workDir string) Scenario {
	setupWorkspace(workDir)

	adv := NewGame(map[string]any{
		"build_clean": false,
		"tests_pass":  false,
		"files_read":  0,
		"files_written": 0,
	})

	adv.AddInstrument("read_file", "Read a file from the workspace. Input: relative path.", organ.ReadAction, func(s map[string]any, input string) string {
		path := strings.TrimSpace(input)
		abs := filepath.Join(workDir, filepath.Clean(path))
		data, err := os.ReadFile(abs)
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		count, _ := s["files_read"].(int)
		s["files_read"] = count + 1
		return string(data)
	})

	adv.AddInstrument("write_file", "Write content to a file. Input JSON: {\"path\": \"...\", \"content\": \"...\"}.", organ.WriteAction, func(s map[string]any, input string) string {
		var args struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal([]byte(input), &args); err != nil {
			if input != "" {
				args.Path = "greet/greet.go"
				args.Content = input
			} else {
				return fmt.Sprintf("invalid input: %v", err)
			}
		}
		abs := filepath.Join(workDir, filepath.Clean(args.Path))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		if err := os.WriteFile(abs, []byte(args.Content), 0o644); err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		count, _ := s["files_written"].(int)
		s["files_written"] = count + 1
		return fmt.Sprintf("wrote %d bytes to %s", len(args.Content), args.Path)
	})

	adv.AddInstrument("go_build", "Run go build on the workspace. Checks if code compiles.", organ.ReadAction, func(s map[string]any, _ string) string {
		src := filepath.Join(workDir, "greet", "greet.go")
		data, err := os.ReadFile(src)
		if err != nil {
			return "build failed: greet/greet.go not found"
		}
		content := string(data)
		if strings.Contains(content, "TODO") || strings.Contains(content, "panic(\"not implemented\")") {
			return "build failed: implementation contains TODO or panic stubs"
		}
		if !strings.Contains(content, "package greet") {
			return "build failed: missing package declaration"
		}
		if !strings.Contains(content, "func Greet") {
			return "build failed: missing Greet function"
		}
		s["build_clean"] = true
		return "build ok: greet package compiles"
	})

	adv.AddInstrument("go_test", "Run go test on the workspace. Verifies the implementation.", organ.WriteAction, func(s map[string]any, _ string) string {
		src := filepath.Join(workDir, "greet", "greet.go")
		data, err := os.ReadFile(src)
		if err != nil {
			return "test failed: greet/greet.go not found"
		}
		content := string(data)
		if !strings.Contains(content, "func Greet") {
			return "test failed: Greet function not found"
		}
		if !strings.Contains(content, "return") {
			return "test failed: Greet function has no return statement"
		}

		s["tests_pass"] = true
		return "test ok: all tests pass\n--- PASS: TestGreet (0.00s)\nPASS"
	})

	adv.AddInstrument("check_status", "Check build and test status.", organ.ReadAction, func(s map[string]any, _ string) string {
		buildClean, _ := s["build_clean"].(bool)
		testsPass, _ := s["tests_pass"].(bool)
		if buildClean && testsPass {
			return "status: build clean, tests pass — task complete"
		}
		parts := []string{}
		if !buildClean {
			parts = append(parts, "build not clean")
		}
		if !testsPass {
			parts = append(parts, "tests not passing")
		}
		return fmt.Sprintf("status: %s", strings.Join(parts, ", "))
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
		Desired:     map[string]any{"build_clean": true, "tests_pass": true},
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

// NewAutoassemblerWithContext is a helper for tests that need a temp directory.
func NewAutoassemblerWithContext(ctx context.Context) (Scenario, string) {
	dir, _ := os.MkdirTemp("", "autoassembler-*")
	return NewAutoassembler(dir), dir
}
