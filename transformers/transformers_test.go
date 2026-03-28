package transformers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/dpopsuev/origami/agentport"
	"github.com/dpopsuev/origami/engine"
)

// --- JQ Transformer ---

func TestJQ_Transform(t *testing.T) {
	jq := NewJQ()
	if jq.Name() != "jq" {
		t.Fatalf("Name() = %q, want jq", jq.Name())
	}

	tc := &engine.TransformerContext{
		Input:  map[string]any{"items": []any{1.0, 2.0, 3.0}},
		Config: map[string]any{"multiplier": 10.0},
		Meta:   map[string]any{"expr": "len(input.items)"},
	}
	result, err := jq.Transform(context.Background(), tc)
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}
	if result != 3 {
		t.Errorf("result = %v, want 3", result)
	}
}

func TestJQ_TransformWithConfig(t *testing.T) {
	jq := NewJQ()
	tc := &engine.TransformerContext{
		Input:  map[string]any{"value": 5.0},
		Config: map[string]any{"threshold": 3.0},
		Meta:   map[string]any{"expr": "input.value > config.threshold"},
	}
	result, err := jq.Transform(context.Background(), tc)
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}
	if result != true {
		t.Errorf("result = %v, want true", result)
	}
}

func TestJQ_NoExpression(t *testing.T) {
	jq := NewJQ()
	tc := &engine.TransformerContext{Meta: map[string]any{}}
	_, err := jq.Transform(context.Background(), tc)
	if err == nil {
		t.Fatal("expected error for missing expr")
	}
}

func TestJQ_InvalidExpression(t *testing.T) {
	jq := NewJQ()
	tc := &engine.TransformerContext{
		Input: map[string]any{},
		Meta:  map[string]any{"expr": ">>>invalid"},
	}
	_, err := jq.Transform(context.Background(), tc)
	if err == nil {
		t.Fatal("expected error for invalid expression")
	}
}

// --- File Transformer ---

func TestFile_ReadJSON(t *testing.T) {
	dir := t.TempDir()
	data := map[string]any{"key": "value"}
	raw, _ := json.Marshal(data)
	os.WriteFile(filepath.Join(dir, "test.json"), raw, 0o644)

	ft := NewFile(WithRootDir(dir))
	if ft.Name() != "file" {
		t.Fatalf("Name() = %q, want file", ft.Name())
	}

	tc := &engine.TransformerContext{Prompt: "test.json"}
	result, err := ft.Transform(context.Background(), tc)
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	if m["key"] != "value" {
		t.Errorf("key = %v, want value", m["key"])
	}
}

func TestFile_ReadText(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hello world"), 0o644)

	ft := NewFile(WithRootDir(dir))
	tc := &engine.TransformerContext{Prompt: "readme.txt"}
	result, err := ft.Transform(context.Background(), tc)
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	if m["content"] != "hello world" {
		t.Errorf("content = %v, want hello world", m["content"])
	}
}

func TestFile_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	ft := NewFile(WithRootDir(dir))
	tc := &engine.TransformerContext{Prompt: "../../../etc/passwd"}
	_, err := ft.Transform(context.Background(), tc)
	if err == nil {
		t.Fatal("expected error for path traversal")
	}
}

func TestFile_NoPath(t *testing.T) {
	ft := NewFile()
	tc := &engine.TransformerContext{}
	_, err := ft.Transform(context.Background(), tc)
	if err == nil {
		t.Fatal("expected error for missing path")
	}
}

// --- HTTP Transformer ---

func TestHTTP_Get(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
	}))
	defer ts.Close()

	ht := NewHTTP()
	if ht.Name() != "http" {
		t.Fatalf("Name() = %q, want http", ht.Name())
	}

	tc := &engine.TransformerContext{
		Meta: map[string]any{"url": ts.URL},
	}
	result, err := ht.Transform(context.Background(), tc)
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	if m["status"] != "ok" {
		t.Errorf("status = %v, want ok", m["status"])
	}
}

func TestHTTP_NoURL(t *testing.T) {
	ht := NewHTTP()
	tc := &engine.TransformerContext{Meta: map[string]any{}}
	_, err := ht.Transform(context.Background(), tc)
	if err == nil {
		t.Fatal("expected error for missing url")
	}
}

func TestHTTP_AllowedHosts(t *testing.T) {
	ht := NewHTTP(WithAllowedHosts("api.example.com"))
	tc := &engine.TransformerContext{
		Meta: map[string]any{"url": "https://evil.com/steal"},
	}
	_, err := ht.Transform(context.Background(), tc)
	if err == nil {
		t.Fatal("expected error for disallowed host")
	}
}

func TestHTTP_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("internal error"))
	}))
	defer ts.Close()

	ht := NewHTTP()
	tc := &engine.TransformerContext{Meta: map[string]any{"url": ts.URL}}
	_, err := ht.Transform(context.Background(), tc)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

// --- LLM Transformer ---

type mockDispatcher struct {
	response []byte
	err      error
}

//nolint:gocritic // hugeParam: interface conformance (agentport.Dispatcher)
func (m *mockDispatcher) Dispatch(_ context.Context, ctx agentport.Context) ([]byte, error) {
	return m.response, m.err
}

func TestLLM_Transform(t *testing.T) {
	data, _ := json.Marshal(map[string]any{"answer": "42"})
	d := &mockDispatcher{response: data}

	llm := NewLLM(d)
	if llm.Name() != "llm" {
		t.Fatalf("Name() = %q, want llm", llm.Name())
	}

	tc := &engine.TransformerContext{
		NodeName: "test-node",
		Prompt:   "test-prompt.md",
		Meta:     map[string]any{"case_id": "C1"},
	}
	result, err := llm.Transform(context.Background(), tc)
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	if m["answer"] != "42" {
		t.Errorf("answer = %v, want 42", m["answer"])
	}
}

func TestLLM_InvalidJSON(t *testing.T) {
	d := &mockDispatcher{response: []byte("not json")}
	llm := NewLLM(d)
	tc := &engine.TransformerContext{NodeName: "test"}
	_, err := llm.Transform(context.Background(), tc)
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
}

// --- Transformer Registry ---

func TestTransformerRegistry(t *testing.T) {
	reg := engine.TransformerRegistry{}
	jq := NewJQ()
	reg.Register(jq)

	got, err := reg.Get("jq")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name() != "jq" {
		t.Errorf("Name() = %q, want jq", got.Name())
	}

	_, err = reg.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent transformer")
	}
}

func TestTransformerRegistry_Nil(t *testing.T) {
	var reg engine.TransformerRegistry
	_, err := reg.Get("jq")
	if err == nil {
		t.Fatal("expected error for nil registry")
	}
}

func TestTransformerRegistry_DuplicatePanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on duplicate registration")
		}
	}()
	reg := engine.TransformerRegistry{}
	jq := NewJQ()
	reg.Register(jq)
	reg.Register(jq)
}
