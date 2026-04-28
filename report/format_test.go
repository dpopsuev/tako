package report_test

import (
	"strings"
	"testing"
	"time"

	"github.com/dpopsuev/tako/report"
)

func TestASCII_BasicTable(t *testing.T) {
	tb := report.NewTable(report.ASCII)
	tb.Header("ID", "Name", "Score")
	tb.Row("M1", "Overall Accuracy", 0.95)
	tb.Row("M2", "Precision", 0.88)
	out := tb.String()

	// ASCII mode uses StyleLight which has box-drawing chars
	if !strings.Contains(out, "ID") {
		t.Errorf("expected header 'ID' in output:\n%s", out)
	}
	if !strings.Contains(out, "Overall Accuracy") {
		t.Errorf("expected 'Overall Accuracy' in output:\n%s", out)
	}
	if !strings.Contains(out, "0.95") {
		t.Errorf("expected '0.95' in output:\n%s", out)
	}
	// Should NOT contain markdown pipe-only syntax (no leading/trailing |)
	// ASCII uses box-drawing characters from StyleLight
	if strings.Contains(out, "───") == false {
		t.Errorf("expected box-drawing characters in ASCII output:\n%s", out)
	}
}

func TestMarkdown_BasicTable(t *testing.T) {
	tb := report.NewTable(report.Markdown)
	tb.Header("Step", "Calls", "Cost")
	tb.Row("Recall (F0)", 30, "$0.075")
	tb.Row("Triage (F1)", 30, "$0.120")
	out := tb.String()

	// Markdown tables have | delimiters and --- separator
	if !strings.Contains(out, "| Step") {
		t.Errorf("expected markdown header with '| Step':\n%s", out)
	}
	if !strings.Contains(out, "---") {
		t.Errorf("expected markdown separator '---':\n%s", out)
	}
	if !strings.Contains(out, "Recall (F0)") {
		t.Errorf("expected 'Recall (F0)' in output:\n%s", out)
	}
}

func TestMarkdown_WithFooter(t *testing.T) {
	tb := report.NewTable(report.Markdown)
	tb.Header("Step", "Total")
	tb.Row("F0", 100)
	tb.Row("F1", 200)
	tb.Footer("TOTAL", 300)
	out := tb.String()

	if !strings.Contains(out, "TOTAL") {
		t.Errorf("expected footer 'TOTAL' in output:\n%s", out)
	}
	if !strings.Contains(out, "300") {
		t.Errorf("expected footer value '300' in output:\n%s", out)
	}
}

func TestColumns_RightAlign(t *testing.T) {
	tb := report.NewTable(report.ASCII)
	tb.Header("Name", "Value")
	tb.Row("tokens", 12345)
	tb.Columns(report.ColumnConfig{Number: 2, Align: report.AlignRight})
	out := tb.String()

	if !strings.Contains(out, "12345") {
		t.Errorf("expected '12345' in output:\n%s", out)
	}
}

func TestSameData_DualFormat(t *testing.T) {
	build := func(m report.Mode) string {
		tb := report.NewTable(m)
		tb.Header("A", "B")
		tb.Row("x", "y")
		return tb.String()
	}

	ascii := build(report.ASCII)
	md := build(report.Markdown)

	if ascii == md {
		t.Error("ASCII and Markdown output should differ")
	}
	// Both should contain the data
	for _, out := range []string{ascii, md} {
		if !strings.Contains(out, "x") || !strings.Contains(out, "y") {
			t.Errorf("expected data in output:\n%s", out)
		}
	}
}

// --- Helper tests ---

func TestFmtTokens(t *testing.T) {
	tests := []struct {
		in   int
		want string
	}{
		{0, "0"},
		{500, "500"},
		{999, "999"},
		{1000, "1.0K"},
		{1500, "1.5K"},
		{20000, "20.0K"},
		{1000000, "1.0M"},
		{2500000, "2.5M"},
	}
	for _, tc := range tests {
		got := report.FmtTokens(tc.in)
		if got != tc.want {
			t.Errorf("FmtTokens(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestFmtDuration(t *testing.T) {
	tests := []struct {
		in   time.Duration
		want string
	}{
		{0, "0s"},
		{30 * time.Second, "30s"},
		{59 * time.Second, "59s"},
		{60 * time.Second, "1m 0s"},
		{90 * time.Second, "1m 30s"},
		{5*time.Minute + 15*time.Second, "5m 15s"},
	}
	for _, tc := range tests {
		got := report.FmtDuration(tc.in)
		if got != tc.want {
			t.Errorf("FmtDuration(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		in     string
		maxLen int
		want   string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello world", 8, "hello..."},
		{"ab", 3, "ab"},
		{"abcdef", 3, "abc"},
	}
	for _, tc := range tests {
		got := report.Truncate(tc.in, tc.maxLen)
		if got != tc.want {
			t.Errorf("Truncate(%q, %d) = %q, want %q", tc.in, tc.maxLen, got, tc.want)
		}
	}
}

func TestBoolMark(t *testing.T) {
	if report.BoolMark(true) != "✓" {
		t.Error("BoolMark(true) should be ✓")
	}
	if report.BoolMark(false) != "✗" {
		t.Error("BoolMark(false) should be ✗")
	}
}
