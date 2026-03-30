package prompt

import (
	"fmt"
	"strings"
)

// Requirement levels for template sections (RFC 2119 style).
const (
	Must   = "must"   // section is required; validation fails without it
	Should = "should" // section is recommended; validation warns without it
	Could  = "could"  // section is optional; no warning
)

// TemplateSection declares an expected section in a prompt template.
type TemplateSection struct {
	Name     string `json:"name"`     // expected section heading
	Level    string `json:"level"`    // must, should, could
	Guidance string `json:"guidance"` // help text for the section
}

// Template declares the expected structure for prompts of a given step.
// Prompts are validated against their template to ensure all required
// sections are present and recommended sections are not missing.
type Template struct {
	Step     string            `json:"step"`     // circuit step this template applies to
	Sections []TemplateSection `json:"sections"` // expected sections
}

// Finding is a single template conformance issue.
type Finding struct {
	Section string `json:"section"`
	Level   string `json:"level"` // must, should
	Message string `json:"message"`
}

// CheckConformance validates a prompt against a template.
// Returns findings for missing must/should sections.
// Could sections are never flagged.
func CheckConformance(p *Prompt, tmpl *Template) []Finding {
	if tmpl == nil || len(tmpl.Sections) == 0 {
		return nil
	}

	// Build set of section names present in the prompt.
	present := make(map[string]bool)
	for _, s := range p.Sections {
		present[normalize(s.Name)] = true
	}

	var findings []Finding
	for _, ts := range tmpl.Sections {
		if ts.Level == Could {
			continue
		}
		if !present[normalize(ts.Name)] {
			findings = append(findings, Finding{
				Section: ts.Name,
				Level:   ts.Level,
				Message: fmt.Sprintf("missing %s section %q: %s", ts.Level, ts.Name, ts.Guidance),
			})
		}
	}
	return findings
}

// HasViolations returns true if any finding is a must-level violation.
func HasViolations(findings []Finding) bool {
	for _, f := range findings {
		if f.Level == Must {
			return true
		}
	}
	return false
}

func normalize(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
