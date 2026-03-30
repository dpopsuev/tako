package prompt

import (
	"testing"
)

func TestParseSections(t *testing.T) {
	content := `# F1 — Triage

You are a triage analyst.

## Task

Classify the failure.

## Guards

G1: Do not hallucinate.
G2: Check logs.

## Output format

Return JSON.
`
	sections := ParseSections(content)
	if len(sections) != 4 {
		t.Fatalf("expected 4 sections, got %d", len(sections))
	}

	tests := []struct {
		name  string
		level int
	}{
		{"F1 — Triage", 1},
		{"Task", 2},
		{"Guards", 2},
		{"Output format", 2},
	}
	for i, tt := range tests {
		if sections[i].Name != tt.name {
			t.Errorf("section[%d].Name = %q, want %q", i, sections[i].Name, tt.name)
		}
		if sections[i].Level != tt.level {
			t.Errorf("section[%d].Level = %d, want %d", i, sections[i].Level, tt.level)
		}
	}

	if sections[1].Content != "Classify the failure." {
		t.Errorf("Task content = %q", sections[1].Content)
	}
}

func TestParseSections_Empty(t *testing.T) {
	sections := ParseSections("")
	if len(sections) != 0 {
		t.Errorf("expected 0 sections for empty content, got %d", len(sections))
	}
}

func TestParseSections_NoHeadings(t *testing.T) {
	sections := ParseSections("just plain text\nno headings here")
	if len(sections) != 0 {
		t.Errorf("expected 0 sections for content without headings, got %d", len(sections))
	}
}

func TestParseSections_BoldHeading(t *testing.T) {
	content := "## **Task:**\n\nDo the thing."
	sections := ParseSections(content)
	if len(sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(sections))
	}
	if sections[0].Name != "Task" {
		t.Errorf("name = %q, want %q", sections[0].Name, "Task")
	}
}

func TestHeadingLevel(t *testing.T) {
	tests := []struct {
		line string
		want int
	}{
		{"# Heading", 1},
		{"## Heading", 2},
		{"### Heading", 3},
		{"###### Heading", 6},
		{"####### Too many", 0},
		{"#NoSpace", 0},
		{"Not a heading", 0},
		{"", 0},
	}
	for _, tt := range tests {
		got := headingLevel(tt.line)
		if got != tt.want {
			t.Errorf("headingLevel(%q) = %d, want %d", tt.line, got, tt.want)
		}
	}
}
