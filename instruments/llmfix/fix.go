// Package llmfix provides the fix instrument backed by an LLM provider.
// It reads findings from the prior scan artifact, asks the LLM to generate
// fixes, and applies them to disk. Returns typed sdlctype.FixResult.
package llmfix

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	anyllm "github.com/mozilla-ai/any-llm-go/providers"

	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami/simulate/sdlc/sdlctype"
)

var (
	errNoChoices      = errors.New("llm fix: no choices returned")
	errContentNotText = errors.New("llm fix: content is not a string")
)

// FixTransformer calls an LLM to generate code fixes for scan findings.
type FixTransformer struct {
	provider anyllm.Provider
	model    string
	repoPath string
	dryRun   bool // when true, don't write files — just return what would change
}

// FixOption configures the fix transformer.
type FixOption func(*FixTransformer)

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

// Transform implements engine.Transformer. Reads scan findings from prior
// artifact, asks the LLM for fixes, applies them, returns FixResult.
func (f *FixTransformer) Transform(ctx context.Context, tc *engine.TransformerContext) (any, error) {
	// Extract findings from prior artifact (scan output).
	findings := extractFindings(tc)
	if len(findings) == 0 {
		return &sdlctype.FixResult{Applied: "no findings to fix"}, nil
	}

	// Pick the highest severity finding to fix first.
	finding := pickHighestSeverity(findings)

	// Build prompt.
	prompt := buildFixPrompt(finding, f.repoPath)

	// Call LLM.
	resp, err := f.provider.Completion(ctx, anyllm.CompletionParams{
		Model: f.model,
		Messages: []anyllm.Message{
			{Role: "user", Content: prompt},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("llm fix: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, errNoChoices
	}

	content, ok := resp.Choices[0].Message.Content.(string)
	if !ok {
		return nil, errContentNotText
	}

	// Parse the LLM response as file changes.
	changes := parseChanges(content)

	// Apply changes to disk (unless dry run).
	var fixed []string
	if !f.dryRun {
		for _, ch := range changes {
			absPath := filepath.Join(f.repoPath, ch.File)
			if err := os.WriteFile(absPath, []byte(ch.Content), 0o600); err != nil {
				return nil, fmt.Errorf("write fix %s: %w", ch.File, err)
			}
			fixed = append(fixed, ch.File)
		}
	} else {
		for _, ch := range changes {
			fixed = append(fixed, ch.File)
		}
	}

	return &sdlctype.FixResult{
		Fixed:   fixed,
		Applied: fmt.Sprintf("fixed %s: %s", finding.Rule, finding.Message),
	}, nil
}

// fileChange represents a single file modification from the LLM.
type fileChange struct {
	File    string `json:"file"`
	Content string `json:"content"`
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

// pickHighestSeverity returns the first error-severity finding, or the first finding.
func pickHighestSeverity(findings []sdlctype.Finding) sdlctype.Finding {
	for _, f := range findings {
		if f.Severity == "error" {
			return f
		}
	}
	return findings[0]
}

// buildFixPrompt constructs a prompt asking the LLM to fix a specific finding.
func buildFixPrompt(finding sdlctype.Finding, repoPath string) string {
	var b strings.Builder
	b.WriteString("You are a Go developer fixing a code issue.\n\n")
	b.WriteString("## Finding\n\n")
	fmt.Fprintf(&b, "- **Rule:** %s\n", finding.Rule)
	fmt.Fprintf(&b, "- **File:** %s\n", finding.File)
	if finding.Line > 0 {
		fmt.Fprintf(&b, "- **Line:** %d\n", finding.Line)
	}
	fmt.Fprintf(&b, "- **Message:** %s\n", finding.Message)
	fmt.Fprintf(&b, "- **Severity:** %s\n", finding.Severity)

	// Read the file content if it exists.
	if finding.File != "" {
		absPath := filepath.Join(repoPath, finding.File)
		if content, err := os.ReadFile(absPath); err == nil {
			b.WriteString("\n## Current File Content\n\n```go\n")
			b.Write(content)
			b.WriteString("\n```\n")
		}
	}

	b.WriteString("\n## Instructions\n\n")
	b.WriteString("Fix this issue. Respond with a JSON array of file changes:\n\n")
	b.WriteString("```json\n")
	b.WriteString(`[{"file": "path/to/file.go", "content": "... full file content ..."}]`)
	b.WriteString("\n```\n\n")
	b.WriteString("Only include files that need to change. Return the complete file content, not diffs.\n")

	return b.String()
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

// extractJSON finds the first JSON array in the content, stripping markdown fences.
func extractJSON(s string) string {
	// Look for ```json ... ``` blocks.
	if idx := strings.Index(s, "```json"); idx >= 0 {
		start := idx + len("```json")
		if end := strings.Index(s[start:], "```"); end >= 0 {
			return strings.TrimSpace(s[start : start+end])
		}
	}
	// Look for bare JSON array.
	if idx := strings.Index(s, "["); idx >= 0 {
		if end := strings.LastIndex(s, "]"); end > idx {
			return s[idx : end+1]
		}
	}
	return ""
}
