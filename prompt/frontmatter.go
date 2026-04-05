package prompt

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseFrontMatter extracts YAML front matter from markdown content.
// Front matter is delimited by "---" at the start of the file.
// Returns nil meta and full content if no front matter is present.
func ParseFrontMatter(content string) (meta map[string]string, body string, err error) {
	if !strings.HasPrefix(content, "---") {
		return nil, content, nil
	}

	// Find the closing delimiter.
	rest := content[3:]
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return nil, content, nil // no closing delimiter — treat as no front matter
	}

	frontMatter := rest[:idx]
	body = strings.TrimLeft(rest[idx+4:], "\n")

	meta = make(map[string]string)
	if err := yaml.Unmarshal([]byte(frontMatter), &meta); err != nil {
		return nil, content, nil // malformed YAML — graceful, treat as no front matter
	}

	return meta, body, nil
}
