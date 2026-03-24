package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseReportDef_Valid(t *testing.T) {
	data := []byte(`
name: calibration-report
format: terminal
sections:
  - type: header
    title: "Calibration Report"
    level: 1
  - type: table
    title: "Outcome Metrics"
    columns: [ID, Metric, Value, Pass, Threshold]
    data: outcome_metrics
  - type: text
    content: "Total cases: {{ .total_cases }}, Pass rate: {{ .pass_rate }}"
`)
	def, err := ParseReportDef(data)
	if err != nil {
		t.Fatalf("ParseReportDef: %v", err)
	}
	if def.Name != "calibration-report" {
		t.Errorf("name = %q", def.Name)
	}
	if len(def.Sections) != 3 {
		t.Fatalf("sections = %d, want 3", len(def.Sections))
	}
	if def.Sections[0].Type != "header" {
		t.Errorf("sections[0].type = %q", def.Sections[0].Type)
	}
	if def.Sections[1].DataKey != "outcome_metrics" {
		t.Errorf("sections[1].data = %q", def.Sections[1].DataKey)
	}
}

func TestParseReportDef_MissingName(t *testing.T) {
	_, err := ParseReportDef([]byte(`
format: terminal
sections:
  - type: header
    title: Test
`))
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestParseReportDef_NoSections(t *testing.T) {
	_, err := ParseReportDef([]byte(`
name: empty
format: terminal
`))
	if err == nil {
		t.Fatal("expected error for no sections")
	}
}

func TestLoadReportDef_FromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.yaml")
	content := []byte(`
name: test-report
format: terminal
sections:
  - type: header
    title: Test
    level: 1
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	def, err := LoadReportDef(path)
	if err != nil {
		t.Fatalf("LoadReportDef: %v", err)
	}
	if def.Name != "test-report" {
		t.Errorf("name = %q", def.Name)
	}
}

func TestRender_HeaderASCII(t *testing.T) {
	def := &ReportDef{
		Name:   "test",
		Format: "terminal",
		Sections: []SectionDef{
			{Type: "header", Title: "Report Title", Level: 1},
		},
	}

	out, err := Render(def, nil)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if !strings.Contains(out, "Report Title") {
		t.Errorf("missing title in output: %s", out)
	}
	if !strings.Contains(out, "============") {
		t.Errorf("missing level-1 underline in output: %s", out)
	}
}

func TestRender_HeaderMarkdown(t *testing.T) {
	def := &ReportDef{
		Name:   "test",
		Format: "markdown",
		Sections: []SectionDef{
			{Type: "header", Title: "H1", Level: 1},
			{Type: "header", Title: "H2", Level: 2},
			{Type: "header", Title: "H3", Level: 3},
		},
	}

	out, err := Render(def, nil)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if !strings.Contains(out, "# H1") {
		t.Errorf("missing # H1: %s", out)
	}
	if !strings.Contains(out, "## H2") {
		t.Errorf("missing ## H2: %s", out)
	}
	if !strings.Contains(out, "### H3") {
		t.Errorf("missing ### H3: %s", out)
	}
}

func TestRender_Table(t *testing.T) {
	def := &ReportDef{
		Name:   "test",
		Format: "terminal",
		Sections: []SectionDef{
			{
				Type:    "table",
				Title:   "Metrics",
				Columns: []string{"ID", "Name", "Value"},
				DataKey: "metrics",
			},
		},
	}

	data := map[string]any{
		"metrics": []map[string]any{
			{"ID": "M1", "Name": "Accuracy", "Value": "0.95"},
			{"ID": "M2", "Name": "Rate", "Value": "0.80"},
		},
	}

	out, err := Render(def, data)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if !strings.Contains(out, "M1") {
		t.Errorf("missing M1: %s", out)
	}
	if !strings.Contains(out, "Accuracy") {
		t.Errorf("missing Accuracy: %s", out)
	}
	if !strings.Contains(out, "0.95") {
		t.Errorf("missing 0.95: %s", out)
	}
}

func TestRender_TableMissingData(t *testing.T) {
	def := &ReportDef{
		Name:   "test",
		Format: "terminal",
		Sections: []SectionDef{
			{Type: "table", Columns: []string{"A"}, DataKey: "missing"},
		},
	}

	out, err := Render(def, map[string]any{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(out, "(no data)") {
		t.Errorf("expected '(no data)' in output: %s", out)
	}
}

func TestRender_Text(t *testing.T) {
	def := &ReportDef{
		Name:   "test",
		Format: "terminal",
		Sections: []SectionDef{
			{Type: "text", Content: "Cases: {{ .total_cases }}, Rate: {{ .pass_rate }}"},
		},
	}

	data := map[string]any{
		"total_cases": 30,
		"pass_rate":   "85%",
	}

	out, err := Render(def, data)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(out, "Cases: 30") {
		t.Errorf("missing 'Cases: 30': %s", out)
	}
	if !strings.Contains(out, "Rate: 85%") {
		t.Errorf("missing 'Rate: 85%%': %s", out)
	}
}

func TestRender_UnknownSectionType(t *testing.T) {
	def := &ReportDef{
		Name:   "test",
		Format: "terminal",
		Sections: []SectionDef{
			{Type: "chart"},
		},
	}

	_, err := Render(def, nil)
	if err == nil {
		t.Fatal("expected error for unknown section type")
	}
}

func TestRender_FullReport(t *testing.T) {
	def := &ReportDef{
		Name:   "calibration-report",
		Format: "terminal",
		Sections: []SectionDef{
			{Type: "header", Title: "Calibration Report", Level: 1},
			{
				Type:    "table",
				Title:   "Outcome Metrics",
				Columns: []string{"ID", "Metric", "Value", "Pass"},
				DataKey: "outcome_metrics",
			},
			{Type: "text", Content: "Total cases: {{ .total_cases }}"},
		},
	}

	data := map[string]any{
		"total_cases": 30,
		"outcome_metrics": []map[string]any{
			{"ID": "M1", "Metric": "Defect Accuracy", "Value": "0.93", "Pass": "✓"},
			{"ID": "M15", "Metric": "Component Recall", "Value": "0.85", "Pass": "✓"},
		},
	}

	out, err := Render(def, data)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	for _, want := range []string{
		"Calibration Report",
		"Outcome Metrics",
		"M1",
		"Defect Accuracy",
		"Total cases: 30",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestParseReportDef_RepeatSection(t *testing.T) {
	data := []byte(`
name: repeat-test
format: terminal
sections:
  - type: repeat
    items: components
    body:
      - type: header
        title: "Component"
        level: 2
      - type: text
        content: "Name: {{ .name }}"
`)
	def, err := ParseReportDef(data)
	if err != nil {
		t.Fatalf("ParseReportDef: %v", err)
	}
	if def.Sections[0].Items != "components" {
		t.Errorf("items = %q, want components", def.Sections[0].Items)
	}
	if len(def.Sections[0].Body) != 2 {
		t.Errorf("body sections = %d, want 2", len(def.Sections[0].Body))
	}
}

func TestRender_Repeat(t *testing.T) {
	def := &ReportDef{
		Name:   "test",
		Format: "terminal",
		Sections: []SectionDef{
			{
				Type:  "repeat",
				Items: "components",
				Body: []SectionDef{
					{Type: "header", Title: "{{ .name }}", Level: 2},
					{Type: "text", Content: "Status: {{ .status }}"},
				},
			},
		},
	}

	data := map[string]any{
		"components": []map[string]any{
			{"name": "linuxptp-daemon", "status": "faulty"},
			{"name": "cnf-gotests", "status": "ok"},
		},
	}

	out, err := Render(def, data)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	for _, want := range []string{
		"Status: faulty",
		"Status: ok",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestRender_RepeatEmpty(t *testing.T) {
	def := &ReportDef{
		Name:   "test",
		Format: "terminal",
		Sections: []SectionDef{
			{
				Type:  "repeat",
				Items: "missing_key",
				Body:  []SectionDef{{Type: "text", Content: "never"}},
			},
		},
	}

	out, err := Render(def, map[string]any{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(out, "never") {
		t.Error("should not render body when items key is missing")
	}
}

func TestRender_RepeatMissingItemsField(t *testing.T) {
	def := &ReportDef{
		Name:   "test",
		Format: "terminal",
		Sections: []SectionDef{
			{
				Type: "repeat",
				Body: []SectionDef{{Type: "text", Content: "x"}},
			},
		},
	}

	_, err := Render(def, map[string]any{})
	if err == nil {
		t.Fatal("expected error for repeat without items")
	}
}

func TestRender_RepeatWithTable(t *testing.T) {
	def := &ReportDef{
		Name:   "test",
		Format: "terminal",
		Sections: []SectionDef{
			{
				Type:  "repeat",
				Items: "cases",
				Body: []SectionDef{
					{Type: "text", Content: "Case: {{ .case_id }}"},
					{
						Type:    "table",
						Columns: []string{"Step", "Result"},
						DataKey: "steps",
					},
				},
			},
		},
	}

	data := map[string]any{
		"cases": []map[string]any{
			{
				"case_id": "C1",
				"steps": []map[string]any{
					{"Step": "F0", "Result": "pass"},
					{"Step": "F1", "Result": "fail"},
				},
			},
			{
				"case_id": "C2",
				"steps": []map[string]any{
					{"Step": "F0", "Result": "pass"},
				},
			},
		},
	}

	out, err := Render(def, data)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	for _, want := range []string{"Case: C1", "Case: C2", "F0", "F1", "pass", "fail"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestRender_MarkdownTable(t *testing.T) {
	def := &ReportDef{
		Name:   "md",
		Format: "markdown",
		Sections: []SectionDef{
			{
				Type:    "table",
				Columns: []string{"A", "B"},
				DataKey: "rows",
			},
		},
	}

	data := map[string]any{
		"rows": []map[string]any{
			{"A": "1", "B": "2"},
		},
	}

	out, err := Render(def, data)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if !strings.Contains(out, "|") {
		t.Errorf("expected pipe-separated markdown, got: %s", out)
	}
}
