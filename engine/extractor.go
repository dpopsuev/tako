package engine

// Category: Processing & Support — extractor types.

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/roster"
)

// Extractor, ExtractorRegistry are defined in engine/handler.

// extractorNode is a Node that delegates processing to an Extractor.
type extractorNode struct {
	name    string
	element roster.Element
	ext     Extractor
}

func (n *extractorNode) Name() string                    { return n.name }
func (n *extractorNode) ElementAffinity() roster.Element { return n.element }

func (n *extractorNode) Process(ctx context.Context, nc circuit.NodeContext) (circuit.Artifact, error) {
	var input any
	if nc.PriorArtifact != nil {
		input = nc.PriorArtifact.Raw()
	}
	result, err := n.ext.Extract(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("extractor %q: %w", n.ext.Name(), err)
	}
	return &extractorArtifact{
		typeName:   n.ext.Name(),
		confidence: 1.0,
		raw:        result,
	}, nil
}

// extractorArtifact wraps the output of an Extractor as an Artifact.
type extractorArtifact struct {
	typeName   string
	confidence float64
	raw        any
}

func (a *extractorArtifact) Type() string        { return a.typeName }
func (a *extractorArtifact) Confidence() float64 { return a.confidence }
func (a *extractorArtifact) Raw() any            { return a.raw }

// Built-in extractor names recognized by resolveNode.
const (
	BuiltinExtractorJSONSchema = "json-schema"
	BuiltinExtractorRegex      = "regex"
)

// JSONSchemaExtractor is a built-in extractor that unmarshals JSON input
// and validates it against an circuit.ArtifactSchema.
type JSONSchemaExtractor struct {
	Schema *circuit.ArtifactSchema
}

func (e *JSONSchemaExtractor) Name() string { return BuiltinExtractorJSONSchema }

func (e *JSONSchemaExtractor) Extract(_ context.Context, input any) (any, error) {
	var data []byte
	switch v := input.(type) {
	case []byte:
		data = v
	case json.RawMessage:
		data = []byte(v)
	case string:
		data = []byte(v)
	default:
		b, err := json.Marshal(input)
		if err != nil {
			return nil, fmt.Errorf("json-schema extractor: marshal input: %w", err)
		}
		data = b
	}

	var result any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("json-schema extractor: unmarshal: %w", err)
	}

	if e.Schema != nil {
		art := &extractorArtifact{typeName: BuiltinExtractorJSONSchema, confidence: 1.0, raw: result}
		if err := ValidateArtifact(e.Schema, art); err != nil {
			return nil, fmt.Errorf("json-schema extractor: %w", err)
		}
	}

	return result, nil
}

// --- Additional built-in extractors ---

// NewJSONExtractor parses JSON bytes into a typed Go struct.
func NewJSONExtractor[T any](name string) Extractor {
	return &jsonExtractor[T]{name: name}
}

type jsonExtractor[T any] struct {
	name string
}

func (e *jsonExtractor[T]) Name() string { return e.name }

func (e *jsonExtractor[T]) Extract(_ context.Context, input any) (any, error) {
	data, ok := input.([]byte)
	if !ok {
		if s, ok2 := input.(string); ok2 {
			data = []byte(s)
		} else {
			return nil, fmt.Errorf("%w: %q: expected []byte or string, got %T", ErrJSONExtractor, e.name, input)
		}
	}
	var result T
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("JSONExtractor %q: %w", e.name, err)
	}
	return &result, nil
}

// RegexExtractor extracts named capture groups from text.
func NewRegexExtractor(name, pattern string) (Extractor, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("RegexExtractor %q: compile pattern: %w", name, err)
	}
	return &regexExtractor{name: name, re: re}, nil
}

// MustRegexExtractor is like NewRegexExtractor but panics on invalid pattern.
func MustRegexExtractor(name, pattern string) Extractor {
	ext, err := NewRegexExtractor(name, pattern)
	if err != nil {
		panic(err)
	}
	return ext
}

type regexExtractor struct {
	name string
	re   *regexp.Regexp
}

func (e *regexExtractor) Name() string { return e.name }

func (e *regexExtractor) Extract(_ context.Context, input any) (any, error) {
	text, ok := input.(string)
	if !ok {
		return nil, fmt.Errorf("%w: %q: expected string, got %T", ErrRegexExtractor, e.name, input)
	}
	match := e.re.FindStringSubmatch(text)
	if match == nil {
		return nil, fmt.Errorf("%w: %q: no match in input (len=%d)", ErrRegexExtractor, e.name, len(text))
	}
	result := make(map[string]string)
	for i, name := range e.re.SubexpNames() {
		if i == 0 || name == "" {
			continue
		}
		result[name] = match[i]
	}
	return result, nil
}

// NewCodeBlockExtractor extracts the content of the first fenced code block.
func NewCodeBlockExtractor(name string) Extractor {
	return &codeBlockExtractor{name: name}
}

var codeBlockRe = regexp.MustCompile("(?s)```(?:\\w+)?\\s*\\n(.*?)\\n```")

type codeBlockExtractor struct {
	name string
}

func (e *codeBlockExtractor) Name() string { return e.name }

func (e *codeBlockExtractor) Extract(_ context.Context, input any) (any, error) {
	text, ok := input.(string)
	if !ok {
		return nil, fmt.Errorf("%w: %q: expected string, got %T", ErrCodeBlockExtractor, e.name, input)
	}
	match := codeBlockRe.FindStringSubmatch(text)
	if len(match) >= 2 {
		return strings.TrimSpace(match[1]), nil
	}
	return nil, fmt.Errorf("%w: %q: no fenced code block found (len=%d)", ErrCodeBlockExtractor, e.name, len(text))
}

// NewLineSplitExtractor splits text on newlines and removes blank lines.
func NewLineSplitExtractor(name string) Extractor {
	return &lineSplitExtractor{name: name}
}

type lineSplitExtractor struct {
	name string
}

func (e *lineSplitExtractor) Name() string { return e.name }

func (e *lineSplitExtractor) Extract(_ context.Context, input any) (any, error) {
	text, ok := input.(string)
	if !ok {
		return nil, fmt.Errorf("%w: %q: expected string, got %T", ErrLineSplitExtractor, e.name, input)
	}
	raw := strings.Split(text, "\n")
	lines := make([]string, 0, len(raw))
	for _, l := range raw {
		if trimmed := strings.TrimSpace(l); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines, nil
}
