// Package tdd provides LLM-backed transformers for the TDD coding phases:
// write-test (RED), write-code (GREEN), refactor (BLUE). Each uses
// anyllm.Provider with tool-use structured output to generate code.
package tdd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	anyllm "github.com/mozilla-ai/any-llm-go/providers"

	"github.com/dpopsuev/origami/engine/handler"
	"github.com/dpopsuev/origami/engine/trace"
	"github.com/dpopsuev/origami/simulate/sdlc/sdlctype"
)

var errNoChoices = errors.New("tdd: LLM returned no choices")

const (
	logKeyPhase = "phase"
	logKeyFiles = "files"
)

// fileChange represents a single file to create or modify.
type fileChange struct {
	File    string `json:"file"`
	Content string `json:"content"`
}

// --- Shared LLM call ---

func callLLM(ctx context.Context, provider anyllm.Provider, model, prompt string, tool anyllm.Tool, toolName string) ([]fileChange, error) {
	resp, err := provider.Completion(ctx, anyllm.CompletionParams{
		Model: model,
		Messages: []anyllm.Message{
			{Role: "user", Content: prompt},
		},
		Tools: []anyllm.Tool{tool},
		ToolChoice: anyllm.ToolChoice{
			Type:     "function",
			Function: &anyllm.ToolChoiceFunction{Name: toolName},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("tdd llm: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, errNoChoices
	}

	var changes []fileChange
	for _, tc := range resp.Choices[0].Message.ToolCalls {
		if tc.Function.Name != toolName {
			continue
		}
		var fc fileChange
		if json.Unmarshal([]byte(tc.Function.Arguments), &fc) == nil && fc.File != "" {
			changes = append(changes, fc)
		}
	}
	return changes, nil
}

// writeFiles writes file changes to the given directory.
func writeFiles(dir string, changes []fileChange) ([]string, error) {
	written := make([]string, 0, len(changes))
	for _, ch := range changes {
		if ch.File == "" || ch.Content == "" {
			continue
		}
		absPath := filepath.Join(dir, ch.File)
		if err := os.MkdirAll(filepath.Dir(absPath), 0o750); err != nil {
			return written, fmt.Errorf("mkdir %s: %w", ch.File, err)
		}
		if err := os.WriteFile(absPath, []byte(ch.Content), 0o600); err != nil {
			return written, fmt.Errorf("write %s: %w", ch.File, err)
		}
		written = append(written, ch.File)
	}
	return written, nil
}

// extractSpec reads the resolve-context output from walker state.
func extractSpec(tc *handler.TransformerContext) string {
	if tc.WalkerState == nil {
		return ""
	}
	for _, art := range tc.WalkerState.Outputs {
		if rc, ok := art.Raw().(*sdlctype.ResolveContextResult); ok {
			data, _ := json.Marshal(rc.Spec)
			return string(data)
		}
	}
	return ""
}

// extractTestFiles reads the write-test output from walker state.
func extractTestFiles(tc *handler.TransformerContext) []string {
	if tc.WalkerState == nil {
		return nil
	}
	for _, art := range tc.WalkerState.Outputs {
		if wt, ok := art.Raw().(*sdlctype.WriteTestResult); ok {
			return wt.TestFiles
		}
	}
	return nil
}

// extractCodeFiles reads the write-code output from walker state.
func extractCodeFiles(tc *handler.TransformerContext) []string {
	if tc.WalkerState == nil {
		return nil
	}
	for _, art := range tc.WalkerState.Outputs {
		if wc, ok := art.Raw().(*sdlctype.WriteCodeResult); ok {
			return wc.FilesChanged
		}
	}
	return nil
}

// readFileContents reads files from disk and returns a formatted summary.
func readFileContents(dir string, files []string) string {
	var sb strings.Builder
	for _, f := range files {
		content, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			continue
		}
		fmt.Fprintf(&sb, "=== %s ===\n%s\n\n", f, string(content))
	}
	return sb.String()
}

// --- Tool definitions ---

var writeTestTool = anyllm.Tool{
	Type: "function",
	Function: anyllm.Function{
		Name:        "write_test",
		Description: "Write a Go test file. Return the complete file content.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"file":    map[string]any{"type": "string", "description": "test file path (e.g. pkg/handler_test.go)"},
				"content": map[string]any{"type": "string", "description": "complete Go test file content"},
			},
			"required": []string{"file", "content"},
		},
	},
}

var writeCodeTool = anyllm.Tool{
	Type: "function",
	Function: anyllm.Function{
		Name:        "write_code",
		Description: "Write a Go source file. Return the complete file content with minimal code to pass tests.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"file":    map[string]any{"type": "string", "description": "source file path (e.g. pkg/handler.go)"},
				"content": map[string]any{"type": "string", "description": "complete Go source file content"},
			},
			"required": []string{"file", "content"},
		},
	},
}

var refactorTool = anyllm.Tool{
	Type: "function",
	Function: anyllm.Function{
		Name:        "refactor_code",
		Description: "Refactor a Go source file. Clean up, extract, rename. Tests must still pass.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"file":    map[string]any{"type": "string", "description": "source file path"},
				"content": map[string]any{"type": "string", "description": "complete refactored file content"},
			},
			"required": []string{"file", "content"},
		},
	},
}

// --- WriteTest (RED) ---

// WriteTest generates test files from a spec using an LLM.
type WriteTest struct {
	provider anyllm.Provider
	model    string
	repoPath string

	mu             sync.Mutex
	lastStationLog trace.StationLogger
}

// NewWriteTest creates a write-test transformer.
func NewWriteTest(provider anyllm.Provider, model, repoPath string) *WriteTest {
	return &WriteTest{provider: provider, model: model, repoPath: repoPath}
}

func (w *WriteTest) Name() string { return "write-test" }
func (w *WriteTest) LastStationLog() trace.StationLogger {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.lastStationLog
}

func (w *WriteTest) Transform(ctx context.Context, tc *handler.TransformerContext) (any, error) {
	spec := extractSpec(tc)
	prompt := fmt.Sprintf("Write Go tests for this specification. Use table-driven tests, stdlib only (no testify). Return one test file.\n\nSpec:\n%s", spec)

	changes, err := callLLM(ctx, w.provider, w.model, prompt, writeTestTool, "write_test")
	if err != nil {
		return nil, fmt.Errorf("write-test: %w", err)
	}

	files, writeErr := writeFiles(w.repoPath, changes)
	if writeErr != nil {
		return nil, fmt.Errorf("write-test: %w", writeErr)
	}

	slog.InfoContext(ctx, "write-test complete",
		slog.String(logKeyPhase, "RED"),
		slog.Any(logKeyFiles, files))

	w.mu.Lock()
	w.lastStationLog = &tddStationLog{Phase: "RED", Files: files, PromptLen: len(prompt)}
	w.mu.Unlock()

	return &sdlctype.WriteTestResult{TestFiles: files}, nil
}

// --- WriteCode (GREEN) ---

// WriteCode generates minimal code to pass tests using an LLM.
type WriteCode struct {
	provider anyllm.Provider
	model    string
	repoPath string

	mu             sync.Mutex
	lastStationLog trace.StationLogger
}

// NewWriteCode creates a write-code transformer.
func NewWriteCode(provider anyllm.Provider, model, repoPath string) *WriteCode {
	return &WriteCode{provider: provider, model: model, repoPath: repoPath}
}

func (w *WriteCode) Name() string { return "write-code" }
func (w *WriteCode) LastStationLog() trace.StationLogger {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.lastStationLog
}

func (w *WriteCode) Transform(ctx context.Context, tc *handler.TransformerContext) (any, error) {
	testFiles := extractTestFiles(tc)
	testContent := readFileContents(w.repoPath, testFiles)
	spec := extractSpec(tc)

	prompt := fmt.Sprintf("Write minimal Go code to make these tests pass. No extra features, no premature abstractions.\n\nSpec:\n%s\n\nTests:\n%s", spec, testContent)

	changes, err := callLLM(ctx, w.provider, w.model, prompt, writeCodeTool, "write_code")
	if err != nil {
		return nil, fmt.Errorf("write-code: %w", err)
	}

	files, writeErr := writeFiles(w.repoPath, changes)
	if writeErr != nil {
		return nil, fmt.Errorf("write-code: %w", writeErr)
	}

	slog.InfoContext(ctx, "write-code complete",
		slog.String(logKeyPhase, "GREEN"),
		slog.Any(logKeyFiles, files))

	w.mu.Lock()
	w.lastStationLog = &tddStationLog{Phase: "GREEN", Files: files, PromptLen: len(prompt)}
	w.mu.Unlock()

	return &sdlctype.WriteCodeResult{FilesChanged: files}, nil
}

// --- Refactor (BLUE) ---

// Refactor cleans up code with tests green using an LLM.
type Refactor struct {
	provider anyllm.Provider
	model    string
	repoPath string

	mu             sync.Mutex
	lastStationLog trace.StationLogger
}

// NewRefactor creates a refactor transformer.
func NewRefactor(provider anyllm.Provider, model, repoPath string) *Refactor {
	return &Refactor{provider: provider, model: model, repoPath: repoPath}
}

func (r *Refactor) Name() string { return "refactor" }
func (r *Refactor) LastStationLog() trace.StationLogger {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastStationLog
}

func (r *Refactor) Transform(ctx context.Context, tc *handler.TransformerContext) (any, error) {
	codeFiles := extractCodeFiles(tc)
	codeContent := readFileContents(r.repoPath, codeFiles)
	testFiles := extractTestFiles(tc)
	testContent := readFileContents(r.repoPath, testFiles)

	prompt := fmt.Sprintf("Refactor this Go code. Extract helpers, improve naming, remove duplication. Tests must still pass — do not change test files.\n\nCode:\n%s\n\nTests:\n%s", codeContent, testContent)

	changes, err := callLLM(ctx, r.provider, r.model, prompt, refactorTool, "refactor_code")
	if err != nil {
		return nil, fmt.Errorf("refactor: %w", err)
	}

	files, writeErr := writeFiles(r.repoPath, changes)
	if writeErr != nil {
		return nil, fmt.Errorf("refactor: %w", writeErr)
	}

	slog.InfoContext(ctx, "refactor complete",
		slog.String(logKeyPhase, "BLUE"),
		slog.Any(logKeyFiles, files))

	r.mu.Lock()
	r.lastStationLog = &tddStationLog{Phase: "BLUE", Files: files, PromptLen: len(prompt)}
	r.mu.Unlock()

	return &sdlctype.RefactorResult{FilesChanged: files}, nil
}

// --- Station log ---

type tddStationLog struct {
	Phase     string
	Files     []string
	PromptLen int
}

func (t *tddStationLog) StationLogType() string { return "tdd-" + strings.ToLower(t.Phase) }

// Compile-time interface checks.
var (
	_ handler.Transformer     = (*WriteTest)(nil)
	_ handler.Transformer     = (*WriteCode)(nil)
	_ handler.Transformer     = (*Refactor)(nil)
	_ handler.StationLoggable = (*WriteTest)(nil)
	_ handler.StationLoggable = (*WriteCode)(nil)
	_ handler.StationLoggable = (*Refactor)(nil)
	_ trace.StationLogger     = (*tddStationLog)(nil)
)
