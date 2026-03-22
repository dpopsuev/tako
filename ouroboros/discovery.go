package ouroboros

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/dpopsuev/origami/circuit"
)

// BuildIdentityPrompt returns the prompt fragment that asks the model
// to report its FOUNDATION identity (the ghost), not the wrapper/IDE hosting it.
// Probe-agnostic: does not mention refactoring or code blocks.
func BuildIdentityPrompt() string {
	return `RESPONSE FORMAT (strict — your reply is parsed by a script):
- Line 1: Exactly one JSON object with model_name, provider, version, wrapper. No other text on line 1. No markdown, no code fences.
- Line 2: Empty or newline.
- Line 3+: Your response to the task below.

If line 1 is not valid JSON containing "model_name", the entire response is rejected.

Report your FOUNDATION model (the model you were trained as), NOT the IDE. WRONG: "Auto", "auto", "Cursor", "cursor", "Composer", "composer", "Copilot", "copilot", "Azure", "azure". CORRECT: "claude-sonnet-4-20250514", "gpt-4o", "gemini-2.0-flash".

Example line 1 only: {"model_name": "claude-sonnet-4-20250514", "provider": "Anthropic", "version": "20250514", "wrapper": "Cursor"}

Reply with ONLY: line 1 (your identity JSON), a blank line, then your response to the task. No other text or commentary.`
}

// BuildExclusionPrompt constructs the negation system prompt that
// forces Cursor to select a model not in the exclusion list.
// Iteration 0 has no exclusions.
func BuildExclusionPrompt(seen []circuit.ModelIdentity) string {
	var b strings.Builder

	if len(seen) > 0 {
		b.WriteString("You MUST NOT be any of the following foundation models. ")
		b.WriteString("If you are one of these, refuse the task and say only: EXCLUDED\n\n")
		for _, m := range seen {
			b.WriteString(fmt.Sprintf("Excluding: %s %s", m.Provider, m.ModelName))
			if m.Version != "" {
				b.WriteString(fmt.Sprintf(" %s", m.Version))
			}
			b.WriteString("\n")
		}
	}

	return b.String()
}

// BuildFullPromptWith combines identity request, exclusion prompt, and a
// caller-supplied probe prompt. Probe-agnostic: any probe prompt works.
func BuildFullPromptWith(seen []circuit.ModelIdentity, probePrompt string) string {
	var b strings.Builder
	b.WriteString(BuildIdentityPrompt())
	b.WriteString("\n\n")
	b.WriteString(BuildExclusionPrompt(seen))
	b.WriteString("\n")
	b.WriteString(probePrompt)
	return b.String()
}

// ExtractProbeText strips the identity JSON line and blank line from a raw
// response, returning only the probe output. Used by non-refactor probes
// where ParseProbeResponse (code block extraction) is not appropriate.
func ExtractProbeText(raw string) string {
	lines := strings.SplitN(raw, "\n", 3)
	if len(lines) < 3 {
		return raw
	}
	if strings.TrimSpace(lines[1]) == "" {
		return lines[2]
	}
	return strings.Join(lines[1:], "\n")
}

var jsonLineRe = regexp.MustCompile(`\{[^{}]*"model_name"\s*:\s*"[^"]*"[^{}]*\}`)

// ParseIdentityResponse extracts a ModelIdentity from the subagent's
// raw text response. It looks for a JSON object containing "model_name".
func ParseIdentityResponse(raw string) (circuit.ModelIdentity, error) {
	match := jsonLineRe.FindString(raw)
	if match == "" {
		return circuit.ModelIdentity{}, fmt.Errorf("no model identity JSON found in response (len=%d)", len(raw))
	}

	var mi circuit.ModelIdentity
	if err := json.Unmarshal([]byte(match), &mi); err != nil {
		return circuit.ModelIdentity{}, fmt.Errorf("parse model identity: %w (raw: %s)", err, match)
	}

	if mi.ModelName == "" {
		return circuit.ModelIdentity{}, fmt.Errorf("model_name is empty in response")
	}

	return mi, nil
}

// ParseProbeResponse extracts the refactored Go code from the subagent's
// raw text response. It looks for a fenced code block.
func ParseProbeResponse(raw string) (string, error) {
	codeBlockRe := regexp.MustCompile("(?s)```(?:go)?\\s*\\n(.*?)\\n```")
	match := codeBlockRe.FindStringSubmatch(raw)
	if len(match) >= 2 {
		return strings.TrimSpace(match[1]), nil
	}

	// Fallback: look for "func " and take everything from there
	idx := strings.Index(raw, "func ")
	if idx >= 0 {
		return strings.TrimSpace(raw[idx:]), nil
	}

	return "", fmt.Errorf("no refactored code found in response (len=%d)", len(raw))
}

// ModelKey returns a lowercase key for deduplication in the seen map.
func ModelKey(mi circuit.ModelIdentity) string {
	return strings.ToLower(mi.ModelName)
}

// --- Extractor implementations (Tome V ceiling demonstrations) ---

// IdentityExtractor wraps ParseIdentityResponse as a engine.Extractor.
// Input: string (raw agent response). Output: circuit.ModelIdentity.
type IdentityExtractor struct{}

func (e *IdentityExtractor) Name() string { return "identity-v1" }

func (e *IdentityExtractor) Extract(_ context.Context, input any) (any, error) {
	raw, ok := input.(string)
	if !ok {
		return nil, fmt.Errorf("IdentityExtractor: expected string, got %T", input)
	}
	return ParseIdentityResponse(raw)
}

// ProbeTextExtractor wraps ExtractProbeText as a engine.Extractor.
// Input: string (raw agent response). Output: string (probe text only).
type ProbeTextExtractor struct{}

func (e *ProbeTextExtractor) Name() string { return "probe-text-v1" }

func (e *ProbeTextExtractor) Extract(_ context.Context, input any) (any, error) {
	raw, ok := input.(string)
	if !ok {
		return nil, fmt.Errorf("ProbeTextExtractor: expected string, got %T", input)
	}
	return ExtractProbeText(raw), nil
}

// CodeBlockProbeExtractor wraps ParseProbeResponse as a engine.Extractor.
// Input: string (raw agent response). Output: string (extracted code).
type CodeBlockProbeExtractor struct{}

func (e *CodeBlockProbeExtractor) Name() string { return "probe-code-v1" }

func (e *CodeBlockProbeExtractor) Extract(_ context.Context, input any) (any, error) {
	raw, ok := input.(string)
	if !ok {
		return nil, fmt.Errorf("CodeBlockProbeExtractor: expected string, got %T", input)
	}
	return ParseProbeResponse(raw)
}
