package ouroboros

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami/models"
)

func TestBuildExclusionPrompt_NoExclusions(t *testing.T) {
	prompt := BuildExclusionPrompt(nil)

	if prompt != "" {
		t.Errorf("iteration 0 should produce empty exclusion prompt, got: %q", prompt)
	}
}

func TestBuildExclusionPrompt_WithExclusions(t *testing.T) {
	seen := []circuit.ModelIdentity{
		{ModelName: "claude-4-opus", Provider: "Anthropic", Version: "20250514"},
		{ModelName: "gpt-4o", Provider: "OpenAI"},
	}

	prompt := BuildExclusionPrompt(seen)

	if !strings.Contains(prompt, "Excluding: Anthropic claude-4-opus 20250514") {
		t.Errorf("missing Anthropic exclusion in:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Excluding: OpenAI gpt-4o") {
		t.Errorf("missing OpenAI exclusion in:\n%s", prompt)
	}
	if !strings.Contains(prompt, "MUST NOT") {
		t.Error("should contain strong negation language")
	}
}

func TestBuildFullPrompt_CombinesAll(t *testing.T) {
	probePrompt := "Refactor this Go function for production quality."
	prompt := BuildFullPromptWith(nil, probePrompt)

	if !strings.Contains(prompt, "model_name") {
		t.Error("missing identity prompt")
	}
	if !strings.Contains(prompt, "Refactor") {
		t.Error("missing probe prompt")
	}
	if !strings.Contains(prompt, "FOUNDATION") {
		t.Error("missing foundation model instruction")
	}
}

// TestBuildFullPrompt_IdentityFirst ensures identity is placed before the
// probe so the model identifies itself before being primed by task instructions.
func TestBuildFullPrompt_IdentityFirst(t *testing.T) {
	probePrompt := "Refactor this Go function for production quality."
	prompt := BuildFullPromptWith(nil, probePrompt)

	idxIdentity := strings.Index(prompt, "RESPONSE FORMAT")
	idxProbe := strings.Index(prompt, "Refactor")
	if idxIdentity < 0 || idxProbe < 0 {
		t.Fatalf("prompt missing identity or probe block")
	}
	if idxIdentity > idxProbe {
		t.Errorf("identity block must come before probe; identity at %d, probe at %d", idxIdentity, idxProbe)
	}
}

func TestBuildIdentityPrompt_FoundationNotWrapper(t *testing.T) {
	prompt := BuildIdentityPrompt()

	if !strings.Contains(prompt, "FOUNDATION") {
		t.Error("identity prompt must ask for foundation model, not wrapper")
	}
	if !strings.Contains(prompt, "wrapper") {
		t.Error("identity prompt must include wrapper field (aligned with adapt/cursor identityProbePrompt)")
	}
	if !strings.Contains(prompt, "claude-sonnet-4-20250514") {
		t.Error("identity prompt should give Claude-in-Cursor example")
	}
	if !strings.Contains(prompt, "composer") || !strings.Contains(prompt, "Auto") {
		t.Error("identity prompt should explicitly exclude composer and Auto as model_name")
	}
	if strings.Contains(prompt, "refactored code") {
		t.Error("identity prompt must be probe-agnostic — should not mention refactored code")
	}
}

func TestBuildFullPromptWith_CustomProbe(t *testing.T) {
	custom := "Analyze the following log output and identify the root cause."
	prompt := BuildFullPromptWith(nil, custom)

	if !strings.Contains(prompt, "model_name") {
		t.Error("missing identity prompt")
	}
	if !strings.Contains(prompt, custom) {
		t.Error("missing custom probe prompt")
	}
	if strings.Contains(prompt, "Refactor") {
		t.Error("should not contain refactor probe when custom probe is supplied")
	}
}

func TestExtractProbeText_StripsIdentity(t *testing.T) {
	raw := `{"model_name": "gpt-4o", "provider": "OpenAI"}

Here is my analysis of the logs.
The root cause is a goroutine leak.`

	text := ExtractProbeText(raw)
	if strings.Contains(text, "model_name") {
		t.Error("probe text should not contain identity JSON")
	}
	if !strings.Contains(text, "root cause") {
		t.Error("probe text should contain the actual response")
	}
}

func TestExtractProbeText_ShortResponse(t *testing.T) {
	raw := `{"model_name": "gpt-4o"}`
	text := ExtractProbeText(raw)
	if text != raw {
		t.Errorf("short response should be returned as-is, got %q", text)
	}
}

// TestIdentityOnly_ReturnsFoundation reproduces the observed behavior: when the
// identity probe is sent alone (ghost identity wet test), the model returns
// foundation identity. Uses golden response from a live run with Cursor Auto.
func TestIdentityOnly_ReturnsFoundation(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "response_identity_only.txt"))
	if err != nil {
		t.Skipf("testdata not available: %v", err)
	}
	raw := strings.TrimSpace(string(data))

	mi, err := ParseIdentityResponse(raw)
	if err != nil {
		t.Fatalf("parse identity-only response: %v", err)
	}

	if models.IsWrapperName(mi.ModelName) {
		t.Errorf("identity-only response should yield foundation model, not wrapper; got model_name=%q", mi.ModelName)
	}
	if !models.IsKnownModel(mi) {
		t.Logf("identity-only: foundation model %s not yet in KnownModels (add if desired)", mi.String())
	}
}

// TestCombinedPrompt_BeforeFix_ReturnsWrapper reproduces the problem: when the
// identity prompt was combined with the refactor task and auto-priming appeared,
// the model returned "auto" (wrapper). Golden: response_combined_before_fix.txt.
func TestCombinedPrompt_BeforeFix_ReturnsWrapper(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "response_combined_before_fix.txt"))
	if err != nil {
		t.Skipf("testdata not available: %v", err)
	}
	raw := string(data)

	mi, err := ParseIdentityResponse(raw)
	if err != nil {
		t.Fatalf("parse combined (before fix) response: %v", err)
	}

	if !models.IsWrapperName(mi.ModelName) {
		t.Errorf("before fix, combined response was wrapper; got model_name=%q", mi.ModelName)
	}
}

// TestCombinedPrompt_ReturnsFoundation asserts that the combined prompt
// (identity first, then exclusion, then refactor) elicits foundation identity.
// Golden: response_combined.txt (expected outcome after putting identity first).
func TestCombinedPrompt_ReturnsFoundation(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "response_combined.txt"))
	if err != nil {
		t.Skipf("testdata not available: %v", err)
	}
	raw := string(data)

	mi, err := ParseIdentityResponse(raw)
	if err != nil {
		t.Fatalf("parse combined response: %v", err)
	}

	if models.IsWrapperName(mi.ModelName) {
		t.Errorf("combined prompt must elicit foundation identity, not wrapper; got model_name=%q", mi.ModelName)
	}
}

func TestParseIdentityResponse_ValidJSON(t *testing.T) {
	raw := `Sure, let me identify myself first.
{"model_name": "claude-sonnet-4-20250514", "provider": "Anthropic", "version": "20250514"}
Now let me refactor the code...`

	mi, err := ParseIdentityResponse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mi.ModelName != "claude-sonnet-4-20250514" {
		t.Errorf("model_name: got %q", mi.ModelName)
	}
	if mi.Provider != "Anthropic" {
		t.Errorf("provider: got %q", mi.Provider)
	}
	if mi.Version != "20250514" {
		t.Errorf("version: got %q", mi.Version)
	}
}

func TestParseIdentityResponse_NoJSON(t *testing.T) {
	_, err := ParseIdentityResponse("I'm a helpful assistant, here's the refactored code...")
	if err == nil {
		t.Fatal("expected error for response without identity JSON")
	}
}

func TestParseIdentityResponse_EmptyModelName(t *testing.T) {
	raw := `{"model_name": "", "provider": "Unknown", "version": ""}`
	_, err := ParseIdentityResponse(raw)
	if err == nil {
		t.Fatal("expected error for empty model_name")
	}
}

func TestParseProbeResponse_FencedCodeBlock(t *testing.T) {
	raw := "Here's the refactored code:\n\n```go\nfunc calculate(nums []int) int {\n\treturn 0\n}\n```\n\nDone!"

	code, err := ParseProbeResponse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(code, "func calculate") {
		t.Errorf("expected function in code, got: %s", code)
	}
}

func TestParseProbeResponse_UnfencedFallback(t *testing.T) {
	raw := "Let me refactor this.\nfunc improved(data []int) int {\n\treturn 42\n}\n"

	code, err := ParseProbeResponse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(code, "func improved") {
		t.Errorf("expected function at start, got: %s", code)
	}
}

func TestParseProbeResponse_NoCode(t *testing.T) {
	_, err := ParseProbeResponse("I don't understand the task.")
	if err == nil {
		t.Fatal("expected error for response with no code")
	}
}

func TestModelKey(t *testing.T) {
	mi := circuit.ModelIdentity{ModelName: "Claude-Sonnet-4"}
	if got := ModelKey(mi); got != "claude-sonnet-4" {
		t.Errorf("got %q, want claude-sonnet-4", got)
	}
}

// TestDiscoverModels_StubRecursive simulates the recursive discovery
// pattern with stub data. This validates the fail-fast table-map design
// without requiring a live Cursor session.
func TestDiscoverModels_StubRecursive(t *testing.T) {
	stubResponses := []struct {
		identity string
		code     string
	}{
		{
			identity: `{"model_name": "gpt-4o", "provider": "OpenAI", "version": "2025-01-01"}`,
			code:     "```go\nfunc computeSum(values []int, label string, verbose bool) (int, string, error) {\n\tresult := 0\n\treturn result, \"\", nil\n}\n```",
		},
		{
			identity: `{"model_name": "claude-sonnet-4", "provider": "Anthropic", "version": "20250514"}`,
			code:     "```go\n// sumAbsolute computes absolute sum.\nfunc sumAbsolute(nums []int, name string, log bool) (int, string, error) {\n\ttotal := 0\n\tfor _, n := range nums {\n\t\tif n > 0 { total += n } else { total -= n }\n\t}\n\treturn total, \"\", nil\n}\n```",
		},
		{
			identity: `{"model_name": "gemini-2.5-pro", "provider": "Google", "version": ""}`,
			code:     "```go\nfunc processData(items []int, prefix string, debug bool) (int, string, error) {\n\tvar sum int\n\tfor _, item := range items {\n\t\tif item >= 0 { sum += item } else { sum -= item }\n\t}\n\treturn sum, \"\", nil\n}\n```",
		},
		// Iteration 3: repeat — should trigger fail-fast
		{
			identity: `{"model_name": "gpt-4o", "provider": "OpenAI", "version": "2025-01-01"}`,
			code:     "```go\nfunc compute(a []int, b string, c bool) (int, string, error) { return 0, \"\", nil }\n```",
		},
	}

	seen := map[string]DiscoveryResult{}
	var lastIteration int

	var discover func(t *testing.T, iteration int)
	discover = func(t *testing.T, iteration int) {
		t.Helper()
		lastIteration = iteration

		if iteration >= len(stubResponses) {
			t.Logf("safety cap: exhausted stub data at iteration %d", iteration)
			return
		}

		stub := stubResponses[iteration]
		raw := stub.identity + "\n" + stub.code

		mi, err := ParseIdentityResponse(raw)
		if err != nil {
			t.Fatalf("iteration %d: parse identity: %v", iteration, err)
		}

		key := ModelKey(mi)
		if prev, exists := seen[key]; exists {
			t.Logf("DISCOVERY EXHAUSTED at iteration %d: model %q already seen at iteration %d",
				iteration, mi.ModelName, prev.Iteration)
			t.Logf("Total unique models discovered: %d", len(seen))
			for k, v := range seen {
				t.Logf("  [%d] %s (%s)", v.Iteration, k, v.Model.String())
			}
			return
		}

		code, err := ParseProbeResponse(raw)
		if err != nil {
			t.Fatalf("iteration %d: parse probe: %v", iteration, err)
		}

		seen[key] = DiscoveryResult{
			Iteration: iteration,
			Model:     mi,
			Probe: ProbeResult{
				ProbeID:   "refactor-v1",
				RawOutput: code,
			},
		}

		t.Logf("iteration %d: discovered %s", iteration, mi.String())

		discover(t, iteration+1)
	}

	discover(t, 0)

	if len(seen) != 3 {
		t.Errorf("expected 3 unique models, got %d", len(seen))
	}
	if lastIteration != 3 {
		t.Errorf("expected to reach iteration 3 (repeat), got %d", lastIteration)
	}
}

// --- Extractor interface compliance tests ---

func TestIdentityExtractor_ImplementsExtractor(t *testing.T) {
	var ext engine.Extractor = &IdentityExtractor{}
	if ext.Name() != "identity-v1" {
		t.Errorf("Name() = %q, want %q", ext.Name(), "identity-v1")
	}
}

func TestIdentityExtractor_SameAsParseIdentityResponse(t *testing.T) {
	raw := `{"model_name": "gpt-4o", "provider": "OpenAI", "version": "2024-05-13"}

Some probe output here`

	ext := &IdentityExtractor{}
	result, err := ext.Extract(context.Background(), raw)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	mi, ok := result.(circuit.ModelIdentity)
	if !ok {
		t.Fatalf("result type = %T, want ModelIdentity", result)
	}

	expected, err := ParseIdentityResponse(raw)
	if err != nil {
		t.Fatalf("ParseIdentityResponse: %v", err)
	}
	if mi.ModelName != expected.ModelName || mi.Provider != expected.Provider {
		t.Errorf("got {%s, %s}, want {%s, %s}", mi.ModelName, mi.Provider, expected.ModelName, expected.Provider)
	}
}

func TestIdentityExtractor_WrongType(t *testing.T) {
	ext := &IdentityExtractor{}
	_, err := ext.Extract(context.Background(), 42)
	if err == nil {
		t.Fatal("expected error for non-string input")
	}
}

func TestProbeTextExtractor_ImplementsExtractor(t *testing.T) {
	var ext engine.Extractor = &ProbeTextExtractor{}
	if ext.Name() != "probe-text-v1" {
		t.Errorf("Name() = %q, want %q", ext.Name(), "probe-text-v1")
	}
}

func TestProbeTextExtractor_SameAsExtractProbeText(t *testing.T) {
	raw := `{"model_name": "test", "provider": "test"}

This is the probe output.
Second line.`

	ext := &ProbeTextExtractor{}
	result, err := ext.Extract(context.Background(), raw)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	text, ok := result.(string)
	if !ok {
		t.Fatalf("result type = %T, want string", result)
	}
	expected := ExtractProbeText(raw)
	if text != expected {
		t.Errorf("got %q, want %q", text, expected)
	}
}

func TestCodeBlockProbeExtractor_ImplementsExtractor(t *testing.T) {
	var ext engine.Extractor = &CodeBlockProbeExtractor{}
	if ext.Name() != "probe-code-v1" {
		t.Errorf("Name() = %q, want %q", ext.Name(), "probe-code-v1")
	}
}

func TestCodeBlockProbeExtractor_SameAsParseProbeResponse(t *testing.T) {
	raw := "identity line\n\n```go\nfunc hello() {}\n```\n"

	ext := &CodeBlockProbeExtractor{}
	result, err := ext.Extract(context.Background(), raw)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	code, ok := result.(string)
	if !ok {
		t.Fatalf("result type = %T, want string", result)
	}
	expected, _ := ParseProbeResponse(raw)
	if code != expected {
		t.Errorf("got %q, want %q", code, expected)
	}
}
