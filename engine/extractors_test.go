package engine

import (
	"context"
	"testing"
)

// --- JSONExtractor ---

type testRecord struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

func TestJSONExtractor_HappyPath_Bytes(t *testing.T) {
	ext := NewJSONExtractor[testRecord]("json-test")
	result, err := ext.Extract(context.Background(), []byte(`{"name":"alice","value":42}`))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	rec, ok := result.(*testRecord)
	if !ok {
		t.Fatalf("result type = %T, want *testRecord", result)
	}
	if rec.Name != "alice" || rec.Value != 42 {
		t.Errorf("got {%q, %d}, want {alice, 42}", rec.Name, rec.Value)
	}
}

func TestJSONExtractor_HappyPath_String(t *testing.T) {
	ext := NewJSONExtractor[testRecord]("json-str")
	result, err := ext.Extract(context.Background(), `{"name":"bob","value":7}`)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	rec := result.(*testRecord)
	if rec.Name != "bob" {
		t.Errorf("Name = %q, want %q", rec.Name, "bob")
	}
}

func TestJSONExtractor_EmptyInput(t *testing.T) {
	ext := NewJSONExtractor[testRecord]("json-empty")
	_, err := ext.Extract(context.Background(), []byte{})
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestJSONExtractor_MalformedJSON(t *testing.T) {
	ext := NewJSONExtractor[testRecord]("json-bad")
	_, err := ext.Extract(context.Background(), []byte(`{not json`))
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestJSONExtractor_WrongInputType(t *testing.T) {
	ext := NewJSONExtractor[testRecord]("json-type")
	_, err := ext.Extract(context.Background(), 42)
	if err == nil {
		t.Fatal("expected error for wrong input type")
	}
}

func TestJSONExtractor_Name(t *testing.T) {
	ext := NewJSONExtractor[testRecord]("my-json")
	if ext.Name() != "my-json" {
		t.Errorf("Name() = %q, want %q", ext.Name(), "my-json")
	}
}

// --- RegexExtractor ---

func TestRegexExtractor_HappyPath(t *testing.T) {
	ext, err := NewRegexExtractor("re-test", `(?P<key>\w+)=(?P<val>\w+)`)
	if err != nil {
		t.Fatalf("NewRegexExtractor: %v", err)
	}
	result, err := ext.Extract(context.Background(), "color=blue")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	m, ok := result.(map[string]string)
	if !ok {
		t.Fatalf("result type = %T, want map[string]string", result)
	}
	if m["key"] != "color" || m["val"] != "blue" {
		t.Errorf("got %v, want key=color val=blue", m)
	}
}

func TestRegexExtractor_NoMatch(t *testing.T) {
	ext := MustRegexExtractor("re-nomatch", `(?P<num>\d+)`)
	_, err := ext.Extract(context.Background(), "no digits here")
	if err == nil {
		t.Fatal("expected error for no match")
	}
}

func TestRegexExtractor_InvalidPattern(t *testing.T) {
	_, err := NewRegexExtractor("re-bad", `(?P<broken`)
	if err == nil {
		t.Fatal("expected error for invalid regex")
	}
}

func TestRegexExtractor_WrongInputType(t *testing.T) {
	ext := MustRegexExtractor("re-type", `(?P<x>\w+)`)
	_, err := ext.Extract(context.Background(), 123)
	if err == nil {
		t.Fatal("expected error for wrong input type")
	}
}

func TestRegexExtractor_EmptyInput(t *testing.T) {
	ext := MustRegexExtractor("re-empty", `(?P<x>\w+)`)
	_, err := ext.Extract(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestMustRegexExtractor_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for invalid pattern")
		}
	}()
	MustRegexExtractor("panic", `(?P<bad`)
}

// --- CodeBlockExtractor ---

func TestCodeBlockExtractor_HappyPath(t *testing.T) {
	ext := NewCodeBlockExtractor("code-test")
	input := "Some text\n```go\nfunc main() {}\n```\nMore text"
	result, err := ext.Extract(context.Background(), input)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	code, ok := result.(string)
	if !ok {
		t.Fatalf("result type = %T, want string", result)
	}
	if code != "func main() {}" {
		t.Errorf("got %q, want %q", code, "func main() {}")
	}
}

func TestCodeBlockExtractor_NoLanguage(t *testing.T) {
	ext := NewCodeBlockExtractor("code-nolang")
	input := "text\n```\nhello world\n```\n"
	result, err := ext.Extract(context.Background(), input)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if result.(string) != "hello world" {
		t.Errorf("got %q, want %q", result, "hello world")
	}
}

func TestCodeBlockExtractor_NoBlock(t *testing.T) {
	ext := NewCodeBlockExtractor("code-none")
	_, err := ext.Extract(context.Background(), "just plain text")
	if err == nil {
		t.Fatal("expected error when no code block")
	}
}

func TestCodeBlockExtractor_EmptyInput(t *testing.T) {
	ext := NewCodeBlockExtractor("code-empty")
	_, err := ext.Extract(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestCodeBlockExtractor_WrongType(t *testing.T) {
	ext := NewCodeBlockExtractor("code-type")
	_, err := ext.Extract(context.Background(), 42)
	if err == nil {
		t.Fatal("expected error for wrong type")
	}
}

// --- LineSplitExtractor ---

func TestLineSplitExtractor_HappyPath(t *testing.T) {
	ext := NewLineSplitExtractor("lines-test")
	result, err := ext.Extract(context.Background(), "a\n\nb\n  \nc\n")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	lines, ok := result.([]string)
	if !ok {
		t.Fatalf("result type = %T, want []string", result)
	}
	if len(lines) != 3 {
		t.Fatalf("len = %d, want 3", len(lines))
	}
	if lines[0] != "a" || lines[1] != "b" || lines[2] != "c" {
		t.Errorf("got %v, want [a b c]", lines)
	}
}

func TestLineSplitExtractor_EmptyInput(t *testing.T) {
	ext := NewLineSplitExtractor("lines-empty")
	result, err := ext.Extract(context.Background(), "")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	lines := result.([]string)
	if len(lines) != 0 {
		t.Errorf("len = %d, want 0 for empty input", len(lines))
	}
}

func TestLineSplitExtractor_AllBlanks(t *testing.T) {
	ext := NewLineSplitExtractor("lines-blank")
	result, err := ext.Extract(context.Background(), "\n\n  \n\t\n")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	lines := result.([]string)
	if len(lines) != 0 {
		t.Errorf("len = %d, want 0 for all-blank input", len(lines))
	}
}

func TestLineSplitExtractor_WrongType(t *testing.T) {
	ext := NewLineSplitExtractor("lines-type")
	_, err := ext.Extract(context.Background(), []byte("bytes"))
	if err == nil {
		t.Fatal("expected error for wrong type")
	}
}

// --- Interface compliance ---

func TestAllExtractors_ImplementInterface(t *testing.T) {
	extractors := []Extractor{
		NewJSONExtractor[testRecord]("json"),
		MustRegexExtractor("regex", `(?P<x>\w+)`),
		NewCodeBlockExtractor("code"),
		NewLineSplitExtractor("lines"),
	}
	for _, ext := range extractors {
		if ext.Name() == "" {
			t.Errorf("extractor has empty name")
		}
		var _ Extractor = ext
	}
}
