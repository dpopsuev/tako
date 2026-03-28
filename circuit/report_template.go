package circuit

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// ReportTemplate is a structured report definition that supports overlay merging.
// When Import is set, the template is an overlay on top of a base template.
type ReportTemplate struct {
	Name        string             `yaml:"name"`
	Description string             `yaml:"description,omitempty"`
	Import      string             `yaml:"import,omitempty"`
	Sections    []ReportSectionDef `yaml:"sections"`
}

// ReportSectionDef describes one section in a report template.
// Overlay fields: Override replaces a base section; InsertAfter places after a named section;
// ExtraColumns appends columns to an existing section.
type ReportSectionDef struct {
	Name         string   `yaml:"name"`
	Title        string   `yaml:"title,omitempty"`
	Template     string   `yaml:"template,omitempty"`      // Go template string
	Columns      []string `yaml:"columns,omitempty"`      // table columns
	InsertAfter  string   `yaml:"insert_after,omitempty"`  // overlay: insert after named section
	Override     bool     `yaml:"override,omitempty"`     // overlay: replace named section
	ExtraColumns []string `yaml:"extra_columns,omitempty"` // overlay: append columns to existing section
}

// LoadReportTemplate parses YAML bytes into a ReportTemplate.
func LoadReportTemplate(data []byte) (*ReportTemplate, error) {
	var t ReportTemplate
	if err := yaml.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("report template: parse YAML: %w", err)
	}
	if t.Name == "" {
		return nil, fmt.Errorf("report template: missing name")
	}
	return &t, nil
}

// MergeReportTemplates merges overlay onto base. Merge semantics:
//   - Sections with override: true replace the base section with the same name
//   - Sections with insert_after are placed after the named base section
//   - Sections with extra_columns append columns to an existing section
//   - New sections (no override/insert_after/extra_columns) are appended at the end
//
// When overlay.Import is empty, the overlay is standalone and is returned as-is.
func MergeReportTemplates(base, overlay *ReportTemplate) (*ReportTemplate, error) {
	if overlay.Import == "" {
		result := *overlay
		return &result, nil
	}

	merged := &ReportTemplate{
		Name:        overlay.Name,
		Description: overlay.Description,
		Import:      "",
		Sections:    make([]ReportSectionDef, 0, len(base.Sections)+len(overlay.Sections)),
	}
	if merged.Name == "" {
		merged.Name = base.Name
	}
	if merged.Description == "" {
		merged.Description = base.Description
	}

	// Start with a copy of base sections
	merged.Sections = append(merged.Sections, base.Sections...)

	// Collect overlay sections by type
	var overrideSections, insertAfterSections, extraColumnsSections, appendSections []ReportSectionDef
	for _, s := range overlay.Sections {
		switch {
		case s.Override:
			overrideSections = append(overrideSections, s)
		case s.InsertAfter != "":
			insertAfterSections = append(insertAfterSections, s)
		case len(s.ExtraColumns) > 0:
			extraColumnsSections = append(extraColumnsSections, s)
		default:
			appendSections = append(appendSections, s)
		}
	}

	// Apply overrides: replace base sections with matching name
	for _, ov := range overrideSections {
		found := false
		for i := range merged.Sections {
			if merged.Sections[i].Name != ov.Name {
				continue
			}
			replacement := ov
			replacement.Override = false
			replacement.InsertAfter = ""
			replacement.ExtraColumns = nil
			merged.Sections[i] = replacement
			found = true
			break
		}
		if !found {
			return nil, fmt.Errorf("report template: override section %q not found in base", ov.Name)
		}
	}

	// Apply extra_columns: append columns to existing sections
	for _, ov := range extraColumnsSections {
		found := false
		for i := range merged.Sections {
			if merged.Sections[i].Name == ov.Name {
				merged.Sections[i].Columns = append(merged.Sections[i].Columns, ov.ExtraColumns...)
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("report template: extra_columns section %q not found in base", ov.Name)
		}
	}

	// Apply insert_after: insert new sections after named sections
	for _, ov := range insertAfterSections {
		afterName := ov.InsertAfter
		insertSec := ov
		insertSec.InsertAfter = ""

		idx := -1
		for i, s := range merged.Sections {
			if s.Name == afterName {
				idx = i
				break
			}
		}
		if idx < 0 {
			return nil, fmt.Errorf("report template: insert_after section %q not found in base", afterName)
		}

		// Insert at idx+1
		newSections := make([]ReportSectionDef, 0, len(merged.Sections)+1)
		newSections = append(newSections, merged.Sections[:idx+1]...)
		newSections = append(newSections, insertSec)
		newSections = append(newSections, merged.Sections[idx+1:]...)
		merged.Sections = newSections
	}

	// Append new sections at the end
	merged.Sections = append(merged.Sections, appendSections...)

	return merged, nil
}
