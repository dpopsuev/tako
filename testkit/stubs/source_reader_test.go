package stubs_test

import (
	"context"
	"errors"
	"testing"

	"github.com/dpopsuev/origami/toolkit"
	"github.com/dpopsuev/origami/testkit/stubs"
)

func TestStubSourceReader_Read_CannedData(t *testing.T) {
	sr := stubs.NewStubSourceReader(map[string][]byte{
		"my-repo:README.md": []byte("hello world"),
	})

	src := &toolkit.Source{Name: "my-repo"}
	data, err := sr.Read(context.Background(), src, "README.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello world" {
		t.Errorf("got %q, want %q", string(data), "hello world")
	}
}

func TestStubSourceReader_Read_MissingKey(t *testing.T) {
	sr := stubs.NewStubSourceReader(nil)

	src := &toolkit.Source{Name: "my-repo"}
	_, err := sr.Read(context.Background(), src, "nonexistent.txt")
	if err == nil {
		t.Error("expected error for missing key, got nil")
	}
}

func TestStubSourceReader_ErrorInjection(t *testing.T) {
	sr := stubs.NewStubSourceReader(map[string][]byte{
		"my-repo:file.txt": []byte("data"),
	})
	injected := errors.New("connection failed")
	sr.SetError(injected)

	src := &toolkit.Source{Name: "my-repo"}

	_, err := sr.Read(context.Background(), src, "file.txt")
	if !errors.Is(err, injected) {
		t.Errorf("Read: got %v, want injected error", err)
	}

	err = sr.Ensure(context.Background(), src)
	if !errors.Is(err, injected) {
		t.Errorf("Ensure: got %v, want injected error", err)
	}

	_, err = sr.Search(context.Background(), src, "query", 10)
	if !errors.Is(err, injected) {
		t.Errorf("Search: got %v, want injected error", err)
	}

	_, err = sr.List(context.Background(), src, "/", 2)
	if !errors.Is(err, injected) {
		t.Errorf("List: got %v, want injected error", err)
	}
}

func TestStubSourceReader_Ensure(t *testing.T) {
	sr := stubs.NewStubSourceReader(nil)

	src := &toolkit.Source{Name: "my-repo"}
	err := sr.Ensure(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}

	ensured := sr.EnsuredSources()
	if len(ensured) != 1 || ensured[0] != "my-repo" {
		t.Errorf("EnsuredSources = %v, want [my-repo]", ensured)
	}
}

func TestStubSourceReader_Search(t *testing.T) {
	sr := stubs.NewStubSourceReader(nil)
	sr.WithSearchData("my-repo:error", []toolkit.SearchResult{
		{Source: "my-repo", Path: "main.go", Snippet: "log.Error(...)"},
	})

	src := &toolkit.Source{Name: "my-repo"}
	results, err := sr.Search(context.Background(), src, "error", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Path != "main.go" {
		t.Errorf("Path = %q, want %q", results[0].Path, "main.go")
	}
}

func TestStubSourceReader_List(t *testing.T) {
	sr := stubs.NewStubSourceReader(nil)
	sr.WithListData("my-repo:/", []toolkit.ContentEntry{
		{Path: "main.go", IsDir: false},
		{Path: "pkg", IsDir: true},
	})

	src := &toolkit.Source{Name: "my-repo"}
	entries, err := sr.List(context.Background(), src, "/", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
}

func TestStubSourceReader_CallTracking(t *testing.T) {
	sr := stubs.NewStubSourceReader(map[string][]byte{
		"r:f": []byte("x"),
	})
	src := &toolkit.Source{Name: "r"}
	sr.Ensure(context.Background(), src)
	sr.Read(context.Background(), src, "f")

	calls := sr.Calls()
	if len(calls) != 2 {
		t.Fatalf("got %d calls, want 2", len(calls))
	}
	if calls[0] != "Ensure:r" {
		t.Errorf("calls[0] = %q, want %q", calls[0], "Ensure:r")
	}
	if calls[1] != "Read:r:f" {
		t.Errorf("calls[1] = %q, want %q", calls[1], "Read:r:f")
	}
}

func TestStubSourceReader_Reset(t *testing.T) {
	sr := stubs.NewStubSourceReader(map[string][]byte{
		"r:f": []byte("x"),
	})
	sr.SetError(errors.New("e"))
	sr.Ensure(context.Background(), &toolkit.Source{Name: "r"})
	sr.Reset()

	if len(sr.Calls()) != 0 {
		t.Error("calls not cleared after Reset")
	}
	if len(sr.EnsuredSources()) != 0 {
		t.Error("ensured sources not cleared after Reset")
	}
	// error should be cleared
	src := &toolkit.Source{Name: "r"}
	_, err := sr.Read(context.Background(), src, "f")
	if err != nil {
		t.Error("error should be cleared after Reset")
	}
}
