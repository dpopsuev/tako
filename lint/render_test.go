package lint

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderer_PlainText_Error(t *testing.T) {
	var buf bytes.Buffer
	r := NewRendererWithColor(&buf, false)
	r.Render(Finding{
		RuleID:   "S2",
		Severity: SeverityError,
		Message:  "invalid approach",
		File:     "circuits/alpha.yaml",
		Line:     15,
		Column:   5,
		Expected: `one of [rapid, aggressive, methodical, rigorous, analytical, holistic]`,
		Found:    "aggressive-fast",
		HelpText: `did you mean "aggressive"?`,
	})

	out := buf.String()
	if !strings.Contains(out, "error[S2]: invalid approach") {
		t.Errorf("missing header in output:\n%s", out)
	}
	if !strings.Contains(out, "--> circuits/alpha.yaml:15:5") {
		t.Errorf("missing location in output:\n%s", out)
	}
	if !strings.Contains(out, "expected:") {
		t.Errorf("missing expected in output:\n%s", out)
	}
	if !strings.Contains(out, "found:") {
		t.Errorf("missing found in output:\n%s", out)
	}
	if !strings.Contains(out, `help: did you mean "aggressive"?`) {
		t.Errorf("missing help text in output:\n%s", out)
	}
}

func TestRenderer_PlainText_Warning(t *testing.T) {
	var buf bytes.Buffer
	r := NewRendererWithColor(&buf, false)
	r.Render(Finding{
		RuleID:   "S5",
		Severity: SeverityWarning,
		Message:  "missing edge name",
		File:     "circuits/alpha.yaml",
		Line:     20,
	})

	out := buf.String()
	if !strings.Contains(out, "warning[S5]: missing edge name") {
		t.Errorf("missing header in output:\n%s", out)
	}
	if !strings.Contains(out, "--> circuits/alpha.yaml:20") {
		t.Errorf("missing location in output:\n%s", out)
	}
}

func TestRenderer_WithRelatedSpans(t *testing.T) {
	var buf bytes.Buffer
	r := NewRendererWithColor(&buf, false)
	r.Render(Finding{
		RuleID:   "S10",
		Severity: SeverityError,
		Message:  "type mismatch in wiring",
		File:     "circuits/alpha.yaml",
		Line:     10,
		Related: []Span{
			{File: "circuits/beta.yaml", Line: 5, Label: "target port declared here"},
		},
		Reason: "port types must match for wiring",
	})

	out := buf.String()
	if !strings.Contains(out, "circuits/beta.yaml:5 (target port declared here)") {
		t.Errorf("missing related span in output:\n%s", out)
	}
	if !strings.Contains(out, "reason: port types must match") {
		t.Errorf("missing reason in output:\n%s", out)
	}
}

func TestRenderer_ColorOutput(t *testing.T) {
	var buf bytes.Buffer
	r := NewRendererWithColor(&buf, true)
	r.Render(Finding{
		RuleID:   "S1",
		Severity: SeverityError,
		Message:  "test",
		File:     "test.yaml",
		Line:     1,
	})

	out := buf.String()
	if !strings.Contains(out, ansiRed) {
		t.Error("expected ANSI red in colored output")
	}
	if !strings.Contains(out, ansiReset) {
		t.Error("expected ANSI reset in colored output")
	}
}

func TestRenderer_RenderAll(t *testing.T) {
	var buf bytes.Buffer
	r := NewRendererWithColor(&buf, false)
	findings := []Finding{
		{RuleID: "S1", Severity: SeverityError, Message: "error one"},
		{RuleID: "S2", Severity: SeverityWarning, Message: "warning one"},
		{RuleID: "S3", Severity: SeverityError, Message: "error two"},
	}
	count := r.RenderAll(findings)
	if count != 3 {
		t.Errorf("RenderAll = %d, want 3", count)
	}
	out := buf.String()
	if !strings.Contains(out, "2 error(s)") {
		t.Errorf("missing error count in summary:\n%s", out)
	}
	if !strings.Contains(out, "1 warning(s)") {
		t.Errorf("missing warning count in summary:\n%s", out)
	}
}

func TestRenderer_FallbackSuggestion(t *testing.T) {
	var buf bytes.Buffer
	r := NewRendererWithColor(&buf, false)
	r.Render(Finding{
		RuleID:     "S1",
		Severity:   SeverityWarning,
		Message:    "something wrong",
		Suggestion: "try this instead",
	})
	out := buf.String()
	if !strings.Contains(out, "help: try this instead") {
		t.Errorf("suggestion should appear as help when HelpText is empty:\n%s", out)
	}
}

func TestRenderer_NoFile(t *testing.T) {
	var buf bytes.Buffer
	r := NewRendererWithColor(&buf, false)
	r.Render(Finding{
		RuleID:   "S1",
		Severity: SeverityInfo,
		Message:  "info only",
	})
	out := buf.String()
	if strings.Contains(out, "-->") {
		t.Errorf("should not have location arrow when File is empty:\n%s", out)
	}
}
