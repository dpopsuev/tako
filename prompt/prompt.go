// Package prompt provides first-class prompt types, storage, and versioning
// for Tako circuits. Prompts are Go text/template markdown files with
// named sections. Two store implementations exist: FilePromptStore (read-only,
// backed by fs.FS) and LivePromptStore (in-memory, editable, versioned).
package prompt

import (
	"bufio"
	"strings"
)

// Prompt is a versioned prompt template.
type Prompt struct {
	Name     string            `json:"name"`
	Step     string            `json:"step,omitempty"`     // circuit step (grouping key)
	Version  int               `json:"version"`            // monotonic version counter
	Content  string            `json:"content"`            // raw markdown template
	Sections []Section         `json:"sections,omitempty"` // parsed sections
	Meta     map[string]string `json:"meta,omitempty"`     // arbitrary metadata
}

// Section is a named section extracted from a markdown prompt.
type Section struct {
	Name    string `json:"name"`
	Level   int    `json:"level"`   // heading level (1–6)
	Content string `json:"content"` // body text below the heading
}

// ParseSections extracts markdown heading sections from raw content.
// Each section spans from its heading to the next heading of equal or higher level.
func ParseSections(content string) []Section {
	var sections []Section
	var current *Section
	var body strings.Builder

	flush := func() {
		if current != nil {
			current.Content = strings.TrimSpace(body.String())
			sections = append(sections, *current)
			body.Reset()
		}
	}

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if level := headingLevel(trimmed); level > 0 {
			flush()
			name := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			// Strip formatting (bold markers, colons, etc.)
			name = strings.ReplaceAll(name, "**", "")
			name = strings.TrimRight(name, " :")
			current = &Section{Name: name, Level: level}
			continue
		}

		if current != nil {
			body.WriteString(line)
			body.WriteByte('\n')
		}
	}
	flush()
	return sections
}

// ParsePrompt parses raw prompt bytes (markdown with optional YAML front matter)
// into a Prompt. This is the canonical parser for kind: Prompt resources.
func ParsePrompt(data []byte) (*Prompt, error) {
	content := string(data)
	meta, body, _ := ParseFrontMatter(content)

	p := &Prompt{
		Version:  1,
		Content:  body,
		Sections: ParseSections(body),
		Meta:     meta,
	}

	if meta != nil {
		if v := meta["name"]; v != "" {
			p.Name = v
		}
		if v := meta["step"]; v != "" {
			p.Step = v
		}
	}

	return p, nil
}

// headingLevel returns the ATX heading level (1–6) or 0 if not a heading.
func headingLevel(line string) int {
	if !strings.HasPrefix(line, "#") {
		return 0
	}
	level := 0
	for _, ch := range line {
		if ch == '#' {
			level++
		} else {
			break
		}
	}
	if level > 6 || level == 0 {
		return 0
	}
	// Must have a space after the hashes (ATX heading spec).
	if len(line) > level && line[level] != ' ' {
		return 0
	}
	return level
}
