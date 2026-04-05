package prompt

import (
	"strings"
	"testing"
)

func TestParseFrontMatter_ValidPrompt(t *testing.T) {
	content := "---\nkind: prompt\nname: recall\nstep: ci-analysis\n---\n# Recall\n\nSome content."
	meta, body, err := ParseFrontMatter(content)
	if err != nil {
		t.Fatalf("ParseFrontMatter: %v", err)
	}
	if meta == nil {
		t.Fatal("expected non-nil meta")
	}
	if meta["kind"] != "prompt" {
		t.Errorf("kind = %q, want prompt", meta["kind"])
	}
	if meta["name"] != "recall" {
		t.Errorf("name = %q, want recall", meta["name"])
	}
	if meta["step"] != "ci-analysis" {
		t.Errorf("step = %q, want ci-analysis", meta["step"])
	}
	if !strings.HasPrefix(body, "# Recall") {
		t.Errorf("body should start with '# Recall', got %q", body[:min(len(body), 30)])
	}
}

func TestParseFrontMatter_NoFrontMatter(t *testing.T) {
	content := "# Just Markdown\n\nNo front matter here."
	meta, body, err := ParseFrontMatter(content)
	if err != nil {
		t.Fatalf("ParseFrontMatter: %v", err)
	}
	if meta != nil {
		t.Errorf("expected nil meta for no front matter, got %v", meta)
	}
	if body != content {
		t.Error("body should be full content when no front matter")
	}
}

func TestParseFrontMatter_MalformedYAML(t *testing.T) {
	content := "---\n[not: valid: yaml\n---\n# Content"
	meta, body, err := ParseFrontMatter(content)
	if err != nil {
		t.Fatalf("ParseFrontMatter should not error on malformed: %v", err)
	}
	if meta != nil {
		t.Error("expected nil meta for malformed YAML")
	}
	if body != content {
		t.Error("body should be full content on malformed front matter")
	}
}

func TestParseFrontMatter_EmptyFrontMatter(t *testing.T) {
	content := "---\n---\n# Content"
	meta, body, err := ParseFrontMatter(content)
	if err != nil {
		t.Fatalf("ParseFrontMatter: %v", err)
	}
	if meta == nil {
		t.Fatal("expected non-nil meta (empty map)")
	}
	if len(meta) != 0 {
		t.Errorf("expected empty meta, got %v", meta)
	}
	if !strings.HasPrefix(body, "# Content") {
		t.Errorf("body = %q", body)
	}
}

func TestParseFrontMatter_NoClosingDelimiter(t *testing.T) {
	content := "---\nkind: prompt\n# No closing delimiter"
	meta, body, err := ParseFrontMatter(content)
	if err != nil {
		t.Fatalf("ParseFrontMatter: %v", err)
	}
	if meta != nil {
		t.Error("expected nil meta when no closing delimiter")
	}
	if body != content {
		t.Error("body should be full content")
	}
}
