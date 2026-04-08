// Package llmfix provides the fix instrument backed by an LLM provider.
// It reads findings from the prior scan artifact, asks the LLM to generate
// fixes, and applies them to disk. Returns typed sdlctype.FixResult.
package llmfix

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	texttemplate "text/template"
	"time"

	anyllm "github.com/mozilla-ai/any-llm-go/providers"

	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami/engine/trace"
	"github.com/dpopsuev/origami/simulate/sdlc/sdlctype"
)

var (
	errNoChoices      = errors.New("llm fix: no choices returned")
	errWorktreeAdd    = errors.New("llm fix: git worktree add failed")
	errWorktreeBuild  = errors.New("llm fix: build failed in worktree")
	errWorktreeCommit = errors.New("llm fix: commit failed in worktree")
)

const (
	logKeyParsedCount  = "parsed_count"
	logKeyResponseHead = "response_head"
)

// FixTransformer calls an LLM to generate code fixes for scan findings.
type FixTransformer struct {
	provider       anyllm.Provider
	model          string
	repoPath       string
	dryRun         bool   // when true, don't write files — just return what would change
	promptTemplate string // Go text/template for the fix prompt

	mu             sync.Mutex
	lastStationLog trace.StationLogger
}

// FixOption configures the fix transformer.
type FixOption func(*FixTransformer)

// WithPromptTemplate sets a custom prompt template (Go text/template format).
// When empty, uses the default embedded template.
func WithPromptTemplate(tmpl string) FixOption {
	return func(f *FixTransformer) { f.promptTemplate = tmpl }
}

// WithDryRun prevents actual file writes — useful for testing.
func WithDryRun() FixOption {
	return func(f *FixTransformer) { f.dryRun = true }
}

// NewFixTransformer creates a fix transformer with the given LLM provider.
func NewFixTransformer(provider anyllm.Provider, model, repoPath string, opts ...FixOption) *FixTransformer {
	f := &FixTransformer{
		provider: provider,
		model:    model,
		repoPath: repoPath,
	}
	for _, o := range opts {
		o(f)
	}
	return f
}

// Name implements engine.Transformer.
func (f *FixTransformer) Name() string { return "llm-fix" }

// LastStationLog implements engine.StationLoggable.
func (f *FixTransformer) LastStationLog() trace.StationLogger {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastStationLog
}

// Transform implements engine.Transformer. Reads scan findings from prior
// artifact, asks the LLM for fixes, applies them, returns FixResult.
func (f *FixTransformer) Transform(ctx context.Context, tc *engine.TransformerContext) (any, error) {
	// Extract findings from prior artifact (scan output).
	findings := extractFindings(tc)
	if len(findings) == 0 {
		return &sdlctype.FixResult{Applied: "no findings to fix"}, nil
	}

	// Pick a finding with an actual file path (skip architectural observations).
	finding, ok := pickFixableFinding(findings)
	if !ok {
		return &sdlctype.FixResult{Applied: "no file-level findings to fix (architectural observations only)"}, nil
	}

	// Build prompt from template.
	prompt := f.buildFixPrompt(finding)

	// Call LLM with tool use — structured output, no text parsing.
	resp, err := f.provider.Completion(ctx, anyllm.CompletionParams{
		Model: f.model,
		Messages: []anyllm.Message{
			{Role: "user", Content: prompt},
		},
		Tools: []anyllm.Tool{applyFixTool},
		ToolChoice: anyllm.ToolChoice{
			Type:     "function",
			Function: &anyllm.ToolChoiceFunction{Name: applyFixToolName},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("llm fix: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, errNoChoices
	}

	// Extract file changes from tool_use response.
	choice := resp.Choices[0]
	if contentStr, ok := choice.Message.Content.(string); ok && len(choice.Message.ToolCalls) == 0 {
		slog.InfoContext(ctx, "llm returned text instead of tool_use",
			slog.String(logKeyResponseHead, truncate(contentStr, 300)))
	}
	changes := extractToolUseChanges(choice.Message.ToolCalls)
	slog.InfoContext(ctx, "llm tool use response",
		slog.Int(logKeyParsedCount, len(changes)))

	// Collect modified file names.
	fixed := make([]string, 0, len(changes))
	for _, ch := range changes {
		fixed = append(fixed, ch.File)
	}

	// Apply changes to an isolated worktree (or skip writes in dry-run mode).
	var worktreePath, branch string
	if !f.dryRun {
		var applyErr error
		worktreePath, branch, applyErr = f.applyToWorktree(ctx, changes, fixed, finding)
		if applyErr != nil {
			return nil, applyErr
		}
	}

	f.mu.Lock()
	f.lastStationLog = &sdlctype.FixStationLog{
		PromptLen:     len(prompt),
		ResponseLen:   len(resp.Choices[0].Message.ToolCalls),
		FilesModified: fixed,
		ParsedChanges: len(changes),
		DryRun:        f.dryRun,
	}
	f.mu.Unlock()

	return &sdlctype.FixResult{
		Fixed:        fixed,
		Applied:      fmt.Sprintf("fixed %s: %s", finding.Rule, finding.Message),
		WorktreePath: worktreePath,
		Branch:       branch,
	}, nil
}

// applyToWorktree creates a worktree, writes changes, builds, and commits.
func (f *FixTransformer) applyToWorktree(ctx context.Context, changes []fileChange, files []string, finding sdlctype.Finding) (wtPath, branchName string, err error) {
	wtDir, branch, err := f.createWorktree(ctx)
	if err != nil {
		return "", "", err
	}

	var validFiles []string
	for _, ch := range changes {
		// Validate: must be a .go file path, not a directory or junk.
		if ch.File == "" || !strings.HasSuffix(ch.File, ".go") || ch.Content == "" {
			continue // skip invalid changes
		}
		absPath := filepath.Join(wtDir, ch.File)
		// Verify the target is a file, not a directory.
		if info, statErr := os.Stat(absPath); statErr == nil && info.IsDir() {
			continue // skip — LLM returned a directory path
		}
		if mkErr := os.MkdirAll(filepath.Dir(absPath), 0o750); mkErr != nil {
			return "", "", fmt.Errorf("mkdir for fix %s: %w", ch.File, mkErr)
		}
		if wErr := os.WriteFile(absPath, []byte(ch.Content), 0o600); wErr != nil {
			return "", "", fmt.Errorf("write fix %s: %w", ch.File, wErr)
		}
		validFiles = append(validFiles, ch.File)
	}
	if len(validFiles) == 0 {
		return "", "", fmt.Errorf("%w: LLM produced no valid file changes", errWorktreeBuild)
	}
	files = validFiles

	if buildErr := f.buildInWorktree(ctx, wtDir); buildErr != nil {
		return "", "", buildErr
	}

	if commitErr := f.commitInWorktree(ctx, wtDir, files, finding); commitErr != nil {
		return "", "", commitErr
	}

	return wtDir, branch, nil
}

// worktreeStateDir returns the XDG state directory for SDLC worktrees.
func worktreeStateDir() string {
	xdg := os.Getenv("XDG_STATE_HOME")
	if xdg == "" {
		home, _ := os.UserHomeDir()
		xdg = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(xdg, "origami", "worktrees")
}

// createWorktree creates a git worktree in $XDG_STATE_HOME/origami/worktrees/
// on a new branch. Strips replace directives from the worktree's go.mod so
// it builds against published module versions.
func (f *FixTransformer) createWorktree(ctx context.Context) (wtDir, branch string, err error) {
	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
	branch = "sdlc-fix-" + ts

	stateDir := worktreeStateDir()
	if mkErr := os.MkdirAll(stateDir, 0o750); mkErr != nil {
		return "", "", fmt.Errorf("create worktree state dir: %w", mkErr)
	}
	wtDir = filepath.Join(stateDir, "sdlc-fix-"+ts)

	cmd := exec.CommandContext(ctx, "git", "worktree", "add", wtDir, "-b", branch)
	cmd.Dir = f.repoPath
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if runErr := cmd.Run(); runErr != nil {
		return "", "", fmt.Errorf("%w: %w: %s", errWorktreeAdd, runErr, stderr.String())
	}

	// Strip replace directives — worktree builds against published versions.
	if stripErr := stripGoModReplaces(wtDir); stripErr != nil {
		return "", "", fmt.Errorf("strip go.mod replaces: %w", stripErr)
	}

	return wtDir, branch, nil
}

// stripGoModReplaces removes all replace directives from go.mod in the given
// directory and runs go mod tidy. The worktree builds against published versions.
func stripGoModReplaces(dir string) error {
	modPath := filepath.Join(dir, "go.mod")
	data, err := os.ReadFile(modPath)
	if err != nil {
		return err
	}

	var cleaned strings.Builder
	inReplace := false
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "replace ") {
			continue // single-line replace
		}
		if strings.HasPrefix(trimmed, "replace (") {
			inReplace = true
			continue
		}
		if inReplace {
			if trimmed == ")" {
				inReplace = false
			}
			continue
		}
		cleaned.WriteString(line)
		cleaned.WriteString("\n")
	}

	if wErr := os.WriteFile(modPath, []byte(cleaned.String()), 0o600); wErr != nil {
		return wErr
	}

	// Tidy to resolve dependencies from registry.
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go mod tidy: %w: %s", err, stderr.String())
	}
	return nil
}

// buildInWorktree runs go build ./... inside the worktree to verify correctness.
func (f *FixTransformer) buildInWorktree(ctx context.Context, wtDir string) error {
	cmd := exec.CommandContext(ctx, "go", "build", "./...")
	cmd.Dir = wtDir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %w: %s", errWorktreeBuild, err, stderr.String())
	}
	return nil
}

// commitInWorktree stages and commits the fixed files in the worktree.
func (f *FixTransformer) commitInWorktree(ctx context.Context, wtDir string, files []string, finding sdlctype.Finding) error {
	args := append([]string{"add", "--"}, files...)
	addCmd := exec.CommandContext(ctx, "git", args...)
	addCmd.Dir = wtDir
	var addErr bytes.Buffer
	addCmd.Stderr = &addErr
	if err := addCmd.Run(); err != nil {
		return fmt.Errorf("%w: git add: %w: %s", errWorktreeCommit, err, addErr.String())
	}

	msg := fmt.Sprintf("fix(%s): %s", finding.Rule, finding.Message)
	commitCmd := exec.CommandContext(ctx, "git", "commit", "--no-verify", "-m", msg)
	commitCmd.Dir = wtDir
	var commitErr bytes.Buffer
	commitCmd.Stderr = &commitErr
	if err := commitCmd.Run(); err != nil {
		return fmt.Errorf("%w: git commit: %w: %s", errWorktreeCommit, err, commitErr.String())
	}
	return nil
}

// CleanupWorktrees removes all sdlc-fix-* worktrees from the XDG state dir
// and prunes stale git worktree entries. Call after circuit completion or
// on operator startup to reclaim disk space.
func CleanupWorktrees(ctx context.Context, repoPath string) error {
	stateDir := worktreeStateDir()
	entries, err := os.ReadDir(stateDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // nothing to clean
		}
		return fmt.Errorf("read worktree state dir: %w", err)
	}
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "sdlc-fix-") {
			continue
		}
		wtDir := filepath.Join(stateDir, e.Name())
		cmd := exec.CommandContext(ctx, "git", "worktree", "remove", "--force", wtDir)
		cmd.Dir = repoPath
		_ = cmd.Run() // best-effort
		// If git worktree remove fails (stale entry), force rm.
		_ = os.RemoveAll(wtDir)
	}
	// Prune stale worktree references.
	pruneCmd := exec.CommandContext(ctx, "git", "worktree", "prune")
	pruneCmd.Dir = repoPath
	_ = pruneCmd.Run()
	return nil
}

const applyFixToolName = "apply_fix"

// applyFixTool is the tool definition sent to the LLM for structured output.
var applyFixTool = anyllm.Tool{
	Type: "function",
	Function: anyllm.Function{
		Name:        applyFixToolName,
		Description: "Apply a code fix to a single Go file. Return the complete file content.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"file":    map[string]any{"type": "string", "description": "relative file path (e.g. engine/graph.go)"},
				"content": map[string]any{"type": "string", "description": "complete file content after fix"},
			},
			"required": []any{"file", "content"},
		},
	},
}

// fileChange represents a single file modification from the LLM.
type fileChange struct {
	File    string `json:"file"`
	Content string `json:"content"`
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// extractToolUseChanges extracts file changes from tool_use responses.
// No text parsing, no JSON extraction — structured by the SDK.
func extractToolUseChanges(toolCalls []anyllm.ToolCall) []fileChange {
	var changes []fileChange
	for _, tc := range toolCalls {
		if tc.Function.Name != applyFixToolName {
			continue
		}
		var fc fileChange
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &fc); err != nil {
			continue
		}
		if fc.File != "" && fc.Content != "" {
			changes = append(changes, fc)
		}
	}
	return changes
}

// extractFindings pulls scan findings from the walker context or prior artifact.
func extractFindings(tc *engine.TransformerContext) []sdlctype.Finding {
	if tc.Input == nil {
		return nil
	}

	// The prior artifact is the scan result. Try typed first.
	if sr, ok := tc.Input.(*sdlctype.ScanResult); ok {
		return sr.Findings
	}

	// Fall back to map extraction (from JSON round-trip).
	m, ok := tc.Input.(map[string]any)
	if !ok {
		return nil
	}
	findingsRaw, ok := m["findings"]
	if !ok {
		return nil
	}

	// JSON round-trip to typed.
	data, err := json.Marshal(findingsRaw)
	if err != nil {
		return nil
	}
	var findings []sdlctype.Finding
	if err := json.Unmarshal(data, &findings); err != nil {
		return nil
	}
	return findings
}

// pickFixableFinding returns the first finding that has an actual file path
// (not a package/component name). Hot-spots use package names like "engine"
// which aren't fixable by editing a single file. Layer-violations have real
// file paths. Returns the finding and true, or zero value and false if none.
func pickFixableFinding(findings []sdlctype.Finding) (sdlctype.Finding, bool) {
	for _, f := range findings {
		if f.File != "" && strings.HasSuffix(f.File, ".go") {
			return f, true
		}
	}
	// No file-level findings — these are architectural observations.
	return sdlctype.Finding{}, false
}

// promptContext is the data passed to the fix prompt template.
type promptContext struct {
	Rule        string
	File        string
	Line        int
	Message     string
	Severity    string
	FileContent string
	ModulePath  string
}

// defaultPromptTemplate is used when no custom template is provided.
const defaultPromptTemplate = `You are a Go developer fixing a single code issue.

## Finding
- Rule: {{ .Rule }}
- File: {{ .File }}
{{- if .Line }}
- Line: {{ .Line }}
{{- end }}
- Message: {{ .Message }}
- Severity: {{ .Severity }}

## Module
{{ .ModulePath }}

## Current File Content
{{ .FileContent }}

## Guards
- ONLY modify the file: {{ .File }}
- Do NOT create new files
- Do NOT add new packages or directories
- Do NOT change import paths or module structure
- Return the COMPLETE file content, not a diff

## Output
Return JSON — no markdown, no explanation:
[{"file": "{{ .File }}", "content": "... complete file content ..."}]
`

// buildFixPrompt renders the prompt template with finding context.
func (f *FixTransformer) buildFixPrompt(finding sdlctype.Finding) string {
	tmplStr := f.promptTemplate
	if tmplStr == "" {
		tmplStr = defaultPromptTemplate
	}

	tmpl, err := texttemplate.New("fix").Parse(tmplStr)
	if err != nil {
		// Fallback: return the raw finding info.
		return fmt.Sprintf("Fix %s in %s: %s", finding.Rule, finding.File, finding.Message)
	}

	pctx := promptContext{
		Rule:     finding.Rule,
		File:     finding.File,
		Line:     finding.Line,
		Message:  finding.Message,
		Severity: finding.Severity,
	}

	// Read current file content.
	if finding.File != "" {
		absPath := filepath.Join(f.repoPath, finding.File)
		if content, readErr := os.ReadFile(absPath); readErr == nil {
			pctx.FileContent = string(content)
		}
	}

	// Read module path from go.mod.
	if modData, readErr := os.ReadFile(filepath.Join(f.repoPath, "go.mod")); readErr == nil {
		for _, line := range strings.Split(string(modData), "\n") {
			if strings.HasPrefix(line, "module ") {
				pctx.ModulePath = strings.TrimPrefix(line, "module ")
				break
			}
		}
	}

	var buf strings.Builder
	if execErr := tmpl.Execute(&buf, pctx); execErr != nil {
		return fmt.Sprintf("Fix %s in %s: %s", finding.Rule, finding.File, finding.Message)
	}
	return buf.String()
}

// parseChanges extracts file changes from the LLM response.
func parseChanges(content string) []fileChange {
	// Try to find JSON in the response (may be wrapped in markdown fences).
	cleaned := extractJSON(content)
	if cleaned == "" {
		return nil
	}

	var changes []fileChange
	if err := json.Unmarshal([]byte(cleaned), &changes); err != nil {
		return nil
	}
	return changes
}

// extractJSON finds the first JSON array of objects in the content.
// Handles: markdown fences, reasoning text before JSON, nested brackets.
func extractJSON(s string) string {
	// Look for ```json ... ``` blocks first.
	if idx := strings.Index(s, "```json"); idx >= 0 {
		start := idx + len("```json")
		if end := strings.Index(s[start:], "```"); end >= 0 {
			return strings.TrimSpace(s[start : start+end])
		}
	}
	// Look for ``` ... ``` blocks (without json tag).
	if idx := strings.Index(s, "```\n["); idx >= 0 {
		start := idx + len("```\n")
		if end := strings.Index(s[start:], "```"); end >= 0 {
			return strings.TrimSpace(s[start : start+end])
		}
	}
	// Look for [{ — the start of a JSON array of objects.
	// More specific than bare [ to avoid matching brackets in prose.
	if idx := strings.Index(s, "[{"); idx >= 0 {
		// Find the matching ] by counting bracket depth.
		depth := 0
		for i := idx; i < len(s); i++ {
			switch s[i] {
			case '[':
				depth++
			case ']':
				depth--
				if depth == 0 {
					return s[idx : i+1]
				}
			}
		}
	}
	return ""
}
