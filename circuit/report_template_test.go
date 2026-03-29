package circuit

import (
	"reflect"
	"testing"
)

func TestLoadReportTemplate_BasicParsing(t *testing.T) {
	data := []byte(`
name: my-report
description: A test report
sections:
  - name: summary
    title: Summary
    template: "{{.Title}}"
    columns: [a, b, c]
  - name: details
    title: Details
    columns: [x, y]
`)
	tmpl, err := LoadReportTemplate(data)
	if err != nil {
		t.Fatalf("LoadReportTemplate: %v", err)
	}
	if tmpl.Name != "my-report" {
		t.Errorf("Name = %q, want %q", tmpl.Name, "my-report")
	}
	if tmpl.Description != "A test report" {
		t.Errorf("Description = %q, want %q", tmpl.Description, "A test report")
	}
	if len(tmpl.Sections) != 2 {
		t.Fatalf("len(Sections) = %d, want 2", len(tmpl.Sections))
	}
	if tmpl.Sections[0].Name != "summary" {
		t.Errorf("Sections[0].Name = %q, want %q", tmpl.Sections[0].Name, "summary")
	}
	if !reflect.DeepEqual(tmpl.Sections[0].Columns, []string{"a", "b", "c"}) {
		t.Errorf("Sections[0].Columns = %v, want [a b c]", tmpl.Sections[0].Columns)
	}
	if tmpl.Sections[1].Name != "details" {
		t.Errorf("Sections[1].Name = %q, want %q", tmpl.Sections[1].Name, "details")
	}
}

func TestLoadReportTemplate_MissingName(t *testing.T) {
	data := []byte(`
description: no name
sections:
  - name: x
`)
	_, err := LoadReportTemplate(data)
	if err == nil {
		t.Fatal("LoadReportTemplate: expected error for missing name")
	}
}

func TestMergeReportTemplates_SectionOverride(t *testing.T) {
	base := &ReportTemplate{
		Name: "base",
		Sections: []ReportSectionDef{
			{Name: "a", Title: "Original A", Columns: []string{"x"}},
			{Name: "b", Title: "B", Columns: []string{"y"}},
		},
	}
	overlay := &ReportTemplate{
		Name:   "overlay",
		Import: "base",
		Sections: []ReportSectionDef{
			{Name: "a", Title: "Overridden A", Columns: []string{"p", "q"}, Override: true},
		},
	}
	merged, err := MergeReportTemplates(base, overlay)
	if err != nil {
		t.Fatalf("MergeReportTemplates: %v", err)
	}
	if len(merged.Sections) != 2 {
		t.Fatalf("len(Sections) = %d, want 2", len(merged.Sections))
	}
	if merged.Sections[0].Name != "a" || merged.Sections[0].Title != "Overridden A" {
		t.Errorf("Sections[0] = %+v, want overridden section a", merged.Sections[0])
	}
	if !reflect.DeepEqual(merged.Sections[0].Columns, []string{"p", "q"}) {
		t.Errorf("Sections[0].Columns = %v, want [p q]", merged.Sections[0].Columns)
	}
	if merged.Sections[1].Name != "b" {
		t.Errorf("Sections[1].Name = %q, want b", merged.Sections[1].Name)
	}
}

func TestMergeReportTemplates_InsertAfter(t *testing.T) {
	base := &ReportTemplate{
		Name: "base",
		Sections: []ReportSectionDef{
			{Name: "intro", Title: "Intro"},
			{Name: "outro", Title: "Outro"},
		},
	}
	overlay := &ReportTemplate{
		Name:   "overlay",
		Import: "base",
		Sections: []ReportSectionDef{
			{Name: "middle", Title: "Middle", InsertAfter: "intro"},
		},
	}
	merged, err := MergeReportTemplates(base, overlay)
	if err != nil {
		t.Fatalf("MergeReportTemplates: %v", err)
	}
	if len(merged.Sections) != 3 {
		t.Fatalf("len(Sections) = %d, want 3", len(merged.Sections))
	}
	names := []string{merged.Sections[0].Name, merged.Sections[1].Name, merged.Sections[2].Name}
	want := []string{"intro", "middle", "outro"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("section order = %v, want %v", names, want)
	}
}

func TestMergeReportTemplates_ExtraColumns(t *testing.T) {
	base := &ReportTemplate{
		Name: "base",
		Sections: []ReportSectionDef{
			{Name: "table1", Title: "Table 1", Columns: []string{"a", "b"}},
		},
	}
	overlay := &ReportTemplate{
		Name:   "overlay",
		Import: "base",
		Sections: []ReportSectionDef{
			{Name: "table1", ExtraColumns: []string{"c", "d"}},
		},
	}
	merged, err := MergeReportTemplates(base, overlay)
	if err != nil {
		t.Fatalf("MergeReportTemplates: %v", err)
	}
	if len(merged.Sections) != 1 {
		t.Fatalf("len(Sections) = %d, want 1", len(merged.Sections))
	}
	wantCols := []string{"a", "b", "c", "d"}
	if !reflect.DeepEqual(merged.Sections[0].Columns, wantCols) {
		t.Errorf("Sections[0].Columns = %v, want %v", merged.Sections[0].Columns, wantCols)
	}
}

func TestMergeReportTemplates_OverlayWithoutImport(t *testing.T) {
	base := &ReportTemplate{
		Name:     "base",
		Sections: []ReportSectionDef{{Name: "x"}},
	}
	overlay := &ReportTemplate{
		Name:     "standalone",
		Sections: []ReportSectionDef{{Name: "y", Title: "Y"}},
	}
	merged, err := MergeReportTemplates(base, overlay)
	if err != nil {
		t.Fatalf("MergeReportTemplates: %v", err)
	}
	// Overlay without import returns as-is (overlay)
	if merged.Name != "standalone" {
		t.Errorf("Name = %q, want standalone", merged.Name)
	}
	if len(merged.Sections) != 1 {
		t.Fatalf("len(Sections) = %d, want 1", len(merged.Sections))
	}
	if merged.Sections[0].Name != "y" || merged.Sections[0].Title != "Y" {
		t.Errorf("Sections[0] = %+v, want section y with title Y", merged.Sections[0])
	}
}
