package domainfs_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/dpopsuev/origami/domainfs"
)

type mockCaller struct {
	handler func(name string, args map[string]any) (*sdkmcp.CallToolResult, error)
}

func (m *mockCaller) CallTool(_ context.Context, name string, args map[string]any) (*sdkmcp.CallToolResult, error) {
	return m.handler(name, args)
}

func textResult(text string) *sdkmcp.CallToolResult {
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: text}},
	}
}

func errResult(msg string) *sdkmcp.CallToolResult {
	return &sdkmcp.CallToolResult{
		IsError: true,
		Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: msg}},
	}
}

func TestOpen_ReadFile(t *testing.T) {
	caller := &mockCaller{handler: func(name string, args map[string]any) (*sdkmcp.CallToolResult, error) {
		if name == "domain_read" && args["path"] == "prompts/recall.md" {
			return textResult("You are a recall judge."), nil
		}
		return errResult("unexpected call: " + name), nil
	}}

	rfs := domainfs.New(caller)
	f, err := rfs.Open("prompts/recall.md")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(data) != "You are a recall judge." {
		t.Errorf("content = %q, want %q", string(data), "You are a recall judge.")
	}

	info, err := f.Stat()
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Name() != "recall.md" {
		t.Errorf("Name = %q, want recall.md", info.Name())
	}
	if info.IsDir() {
		t.Error("should not be a directory")
	}
	if info.Size() != int64(len("You are a recall judge.")) {
		t.Errorf("Size = %d, want %d", info.Size(), len("You are a recall judge."))
	}
}

func TestOpen_ReadDir(t *testing.T) {
	entries := []struct {
		Name  string `json:"name"`
		IsDir bool   `json:"is_dir"`
	}{
		{Name: "recall.md", IsDir: false},
		{Name: "triage.md", IsDir: false},
		{Name: "sub", IsDir: true},
	}
	entriesJSON, _ := json.Marshal(entries)

	caller := &mockCaller{handler: func(name string, args map[string]any) (*sdkmcp.CallToolResult, error) {
		if name == "domain_read" {
			return errResult("is a directory"), nil
		}
		if name == "domain_list" {
			return textResult(string(entriesJSON)), nil
		}
		return errResult("unexpected: " + name), nil
	}}

	rfs := domainfs.New(caller)
	f, err := rfs.Open("prompts")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory")
	}

	dirFile, ok := f.(fs.ReadDirFile)
	if !ok {
		t.Fatal("expected ReadDirFile")
	}

	dirEntries, err := dirFile.ReadDir(-1)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(dirEntries) != 3 {
		t.Fatalf("entries = %d, want 3", len(dirEntries))
	}
	if dirEntries[0].Name() != "recall.md" {
		t.Errorf("entry[0] = %q, want recall.md", dirEntries[0].Name())
	}
	if !dirEntries[2].IsDir() {
		t.Error("entry[2] should be a directory")
	}
}

func TestOpen_Missing(t *testing.T) {
	caller := &mockCaller{handler: func(name string, args map[string]any) (*sdkmcp.CallToolResult, error) {
		return errResult("file does not exist"), nil
	}}

	rfs := domainfs.New(caller)
	_, err := rfs.Open("nope.txt")
	if err == nil {
		t.Fatal("expected error")
	}
	if !isNotExist(err) {
		t.Errorf("expected fs.ErrNotExist, got %v", err)
	}
}

func TestOpen_InvalidPath(t *testing.T) {
	callCount := 0
	caller := &mockCaller{handler: func(_ string, _ map[string]any) (*sdkmcp.CallToolResult, error) {
		callCount++
		return textResult("should not reach"), nil
	}}

	rfs := domainfs.New(caller)
	for _, path := range []string{"../etc/passwd", "/absolute", ""} {
		_, err := rfs.Open(path)
		if err == nil {
			t.Errorf("expected error for path %q", path)
		}
	}
	if callCount > 0 {
		t.Errorf("made %d MCP calls for invalid paths, expected 0", callCount)
	}
}

func TestOpen_TransportError(t *testing.T) {
	caller := &mockCaller{handler: func(_ string, _ map[string]any) (*sdkmcp.CallToolResult, error) {
		return nil, fmt.Errorf("connection refused")
	}}

	rfs := domainfs.New(caller)
	_, err := rfs.Open("file.txt")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestReadDir_Paging(t *testing.T) {
	entries := []struct {
		Name  string `json:"name"`
		IsDir bool   `json:"is_dir"`
	}{
		{Name: "a.txt"}, {Name: "b.txt"}, {Name: "c.txt"},
	}
	entriesJSON, _ := json.Marshal(entries)

	caller := &mockCaller{handler: func(name string, _ map[string]any) (*sdkmcp.CallToolResult, error) {
		if name == "domain_read" {
			return errResult("is a directory"), nil
		}
		return textResult(string(entriesJSON)), nil
	}}

	rfs := domainfs.New(caller)
	f, err := rfs.Open("dir")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	dirFile := f.(fs.ReadDirFile)

	// Read 2 at a time
	batch1, err := dirFile.ReadDir(2)
	if err != nil {
		t.Fatalf("ReadDir(2): %v", err)
	}
	if len(batch1) != 2 {
		t.Fatalf("batch1 = %d, want 2", len(batch1))
	}

	batch2, err := dirFile.ReadDir(2)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("expected io.EOF, got %v", err)
	}
	if len(batch2) != 1 {
		t.Fatalf("batch2 = %d, want 1", len(batch2))
	}
}

func isNotExist(err error) bool {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return errors.Is(pe.Err, fs.ErrNotExist)
	}
	return false
}
