// Package report provides a YAML-driven report template engine.
// Report layouts are defined declaratively; rendering walks the sections
// and assembles the final output using format.TableBuilder for tables
// and text/template for free-form text.
package report

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/dpopsuev/origami/format"

	"gopkg.in/yaml.v3"
)

// ReportDef is a YAML-loadable report layout.
type ReportDef struct {
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Name     string       `yaml:"name"`
	Format   string       `yaml:"format"`
	Sections []SectionDef `yaml:"sections"`
}

// SectionDef describes one section in the report.
type SectionDef struct {
	Type    string       `yaml:"type"`              // "table", "text", "header", "repeat"
	Title   string       `yaml:"title,omitempty"`
	Columns []string     `yaml:"columns,omitempty"` // for table
	DataKey string       `yaml:"data,omitempty"`    // key into report data
	Content string       `yaml:"content,omitempty"` // for text (Go template)
	Level   int          `yaml:"level,omitempty"`   // for header (1-3)
	Items   string       `yaml:"items,omitempty"`   // for repeat: key into data for []map[string]any
	Body    []SectionDef `yaml:"body,omitempty"`    // for repeat: nested sections per item
}

// LoadReportDef reads a YAML report template from disk.
func LoadReportDef(path string) (*ReportDef, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("report: read %s: %w", path, err)
	}
	return ParseReportDef(data)
}

// ParseReportDef parses raw YAML bytes into a ReportDef.
// Name is resolved from the bare name: field, falling back to metadata.name.
func ParseReportDef(data []byte) (*ReportDef, error) {
	var def ReportDef
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("report: parse YAML: %w", err)
	}
	if def.Name == "" && def.Metadata.Name != "" {
		def.Name = def.Metadata.Name
	}
	if def.Name == "" {
		return nil, fmt.Errorf("report: missing name")
	}
	if len(def.Sections) == 0 {
		return nil, fmt.Errorf("report: no sections defined")
	}
	return &def, nil
}

// Render walks the report sections and assembles the final output.
// data is a map of keys to values; table sections look up DataKey
// to find []map[string]any rows.
func Render(def *ReportDef, data map[string]any) (string, error) {
	mode := format.ASCII
	if def.Format == "markdown" {
		mode = format.Markdown
	}

	var buf strings.Builder
	if err := renderSections(&buf, def.Sections, data, mode); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func renderSections(buf *strings.Builder, sections []SectionDef, data map[string]any, mode format.Mode) error {
	for i, sec := range sections {
		if i > 0 {
			buf.WriteString("\n")
		}
		switch sec.Type {
		case "header":
			renderHeader(buf, sec, mode)
		case "table":
			if err := renderTable(buf, sec, data, mode); err != nil {
				return fmt.Errorf("section %d (%s): %w", i, sec.Title, err)
			}
		case "text":
			if err := renderText(buf, sec, data); err != nil {
				return fmt.Errorf("section %d (text): %w", i, err)
			}
		case "repeat":
			if err := renderRepeat(buf, sec, data, mode); err != nil {
				return fmt.Errorf("section %d (repeat): %w", i, err)
			}
		default:
			return fmt.Errorf("section %d: unknown type %q", i, sec.Type)
		}
	}
	return nil
}

func renderRepeat(buf *strings.Builder, sec SectionDef, data map[string]any, mode format.Mode) error {
	if sec.Items == "" {
		return fmt.Errorf("repeat section requires 'items' field")
	}
	rawItems, ok := data[sec.Items]
	if !ok {
		return nil
	}

	items, ok := rawItems.([]map[string]any)
	if !ok {
		return fmt.Errorf("data[%q] must be []map[string]any, got %T", sec.Items, rawItems)
	}

	for i, item := range items {
		merged := make(map[string]any, len(data)+len(item))
		for k, v := range data {
			merged[k] = v
		}
		for k, v := range item {
			merged[k] = v
		}
		merged["_index"] = i
		merged["_item"] = item

		if err := renderSections(buf, sec.Body, merged, mode); err != nil {
			return fmt.Errorf("item %d: %w", i, err)
		}
		if i < len(items)-1 {
			buf.WriteString("\n")
		}
	}
	return nil
}

func renderHeader(buf *strings.Builder, sec SectionDef, mode format.Mode) {
	level := sec.Level
	if level < 1 {
		level = 1
	}
	if level > 3 {
		level = 3
	}

	if mode == format.Markdown {
		buf.WriteString(strings.Repeat("#", level) + " " + sec.Title + "\n")
	} else {
		buf.WriteString(sec.Title + "\n")
		switch level {
		case 1:
			buf.WriteString(strings.Repeat("=", len(sec.Title)) + "\n")
		case 2:
			buf.WriteString(strings.Repeat("-", len(sec.Title)) + "\n")
		default:
			buf.WriteString(strings.Repeat("~", len(sec.Title)) + "\n")
		}
	}
}

func renderTable(buf *strings.Builder, sec SectionDef, data map[string]any, mode format.Mode) error {
	if sec.Title != "" {
		buf.WriteString(sec.Title + "\n")
	}

	rawRows, ok := data[sec.DataKey]
	if !ok {
		buf.WriteString("(no data)\n")
		return nil
	}

	rows, ok := rawRows.([]map[string]any)
	if !ok {
		return fmt.Errorf("data[%q] must be []map[string]any, got %T", sec.DataKey, rawRows)
	}

	tb := format.NewTable(mode)
	tb.Header(sec.Columns...)

	for _, row := range rows {
		vals := make([]any, len(sec.Columns))
		for j, col := range sec.Columns {
			if v, ok := row[col]; ok {
				vals[j] = v
			} else {
				vals[j] = ""
			}
		}
		tb.Row(vals...)
	}

	buf.WriteString(tb.String())
	buf.WriteString("\n")
	return nil
}

func renderText(buf *strings.Builder, sec SectionDef, data map[string]any) error {
	tmpl, err := template.New("text").Parse(sec.Content)
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, data); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	buf.WriteString(rendered.String())
	buf.WriteString("\n")
	return nil
}

