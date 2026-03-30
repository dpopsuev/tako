package prompt

import (
	"testing"
)

func TestCheckConformance_AllPresent(t *testing.T) {
	p := &Prompt{
		Sections: []Section{
			{Name: "Task"},
			{Name: "Guards"},
			{Name: "Output format"},
		},
	}
	tmpl := &Template{
		Step: "triage",
		Sections: []TemplateSection{
			{Name: "Task", Level: Must, Guidance: "What the LLM should do"},
			{Name: "Guards", Level: Should, Guidance: "Cognitive bias guards"},
			{Name: "Output format", Level: Must, Guidance: "Expected JSON schema"},
		},
	}
	findings := CheckConformance(p, tmpl)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %v", findings)
	}
}

func TestCheckConformance_MissingMust(t *testing.T) {
	p := &Prompt{
		Sections: []Section{
			{Name: "Guards"},
		},
	}
	tmpl := &Template{
		Sections: []TemplateSection{
			{Name: "Task", Level: Must, Guidance: "Required"},
			{Name: "Guards", Level: Should, Guidance: "Recommended"},
			{Name: "Output format", Level: Must, Guidance: "Required"},
		},
	}
	findings := CheckConformance(p, tmpl)
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d: %v", len(findings), findings)
	}
	if !HasViolations(findings) {
		t.Error("expected violations (must-level)")
	}
}

func TestCheckConformance_MissingShouldOnly(t *testing.T) {
	p := &Prompt{
		Sections: []Section{
			{Name: "Task"},
			{Name: "Output format"},
		},
	}
	tmpl := &Template{
		Sections: []TemplateSection{
			{Name: "Task", Level: Must, Guidance: "Required"},
			{Name: "Guards", Level: Should, Guidance: "Recommended"},
			{Name: "Output format", Level: Must, Guidance: "Required"},
		},
	}
	findings := CheckConformance(p, tmpl)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Level != Should {
		t.Errorf("expected should level, got %s", findings[0].Level)
	}
	if HasViolations(findings) {
		t.Error("should-only findings should not count as violations")
	}
}

func TestCheckConformance_CouldIgnored(t *testing.T) {
	p := &Prompt{
		Sections: []Section{
			{Name: "Task"},
		},
	}
	tmpl := &Template{
		Sections: []TemplateSection{
			{Name: "Task", Level: Must, Guidance: "Required"},
			{Name: "Examples", Level: Could, Guidance: "Nice to have"},
		},
	}
	findings := CheckConformance(p, tmpl)
	if len(findings) != 0 {
		t.Errorf("could sections should not generate findings, got %v", findings)
	}
}

func TestCheckConformance_CaseInsensitive(t *testing.T) {
	p := &Prompt{
		Sections: []Section{
			{Name: "task"},
			{Name: "OUTPUT FORMAT"},
		},
	}
	tmpl := &Template{
		Sections: []TemplateSection{
			{Name: "Task", Level: Must, Guidance: "Required"},
			{Name: "Output format", Level: Must, Guidance: "Required"},
		},
	}
	findings := CheckConformance(p, tmpl)
	if len(findings) != 0 {
		t.Errorf("case-insensitive match failed, got %v", findings)
	}
}

func TestCheckConformance_NilTemplate(t *testing.T) {
	p := &Prompt{Sections: []Section{{Name: "Task"}}}
	findings := CheckConformance(p, nil)
	if findings != nil {
		t.Errorf("nil template should return nil findings")
	}
}
