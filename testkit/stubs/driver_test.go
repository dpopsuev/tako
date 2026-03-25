package stubs

import (
	"context"
	"sync"
	"testing"

	"github.com/dpopsuev/origami/toolkit"
)

func TestStubDriver_SatisfiesDriverInterface(t *testing.T) {
	var _ toolkit.Driver = (*StubDriver)(nil)
}

func TestStubDriver_Handles_ReturnsConfiguredKind(t *testing.T) {
	d := NewStubDriver(toolkit.SourceKindRepo)
	if got := d.Handles(); got != toolkit.SourceKindRepo {
		t.Errorf("Handles() = %v, want %v", got, toolkit.SourceKindRepo)
	}
}

func TestStubDriver_Handles_ReturnsDocKind(t *testing.T) {
	d := NewStubDriver(toolkit.SourceKindDoc)
	if got := d.Handles(); got != toolkit.SourceKindDoc {
		t.Errorf("Handles() = %v, want %v", got, toolkit.SourceKindDoc)
	}
}

func TestStubDriver_Ensure_TracksCall(t *testing.T) {
	d := NewStubDriver(toolkit.SourceKindRepo)
	src := toolkit.Source{Name: "test-repo"}

	if err := d.Ensure(context.Background(), src); err != nil {
		t.Fatalf("Ensure: %v", err)
	}

	calls := d.Calls()
	if len(calls) != 1 {
		t.Fatalf("Calls() = %d, want 1", len(calls))
	}
	if calls[0] != "Ensure:test-repo" {
		t.Errorf("Calls()[0] = %q, want %q", calls[0], "Ensure:test-repo")
	}
}

func TestStubDriver_Read_ReturnsCannedData(t *testing.T) {
	d := NewStubDriver(toolkit.SourceKindRepo)
	d.WithReadData("myrepo:main.go", []byte("package main"))

	data, err := d.Read(context.Background(), toolkit.Source{Name: "myrepo"}, "main.go")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(data) != "package main" {
		t.Errorf("Read = %q, want %q", string(data), "package main")
	}
}

func TestStubDriver_Search_ReturnsCannedResults(t *testing.T) {
	d := NewStubDriver(toolkit.SourceKindRepo)
	want := []toolkit.SearchResult{{Path: "foo.go", Line: 10, Snippet: "func Foo()"}}
	d.WithSearchData("myrepo:Foo", want)

	got, err := d.Search(context.Background(), toolkit.Source{Name: "myrepo"}, "Foo", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 1 || got[0].Path != "foo.go" {
		t.Errorf("Search = %v, want %v", got, want)
	}
}

func TestStubDriver_List_ReturnsCannedEntries(t *testing.T) {
	d := NewStubDriver(toolkit.SourceKindRepo)
	want := []toolkit.ContentEntry{{Path: "src/", IsDir: true}}
	d.WithListData("myrepo:.", want)

	got, err := d.List(context.Background(), toolkit.Source{Name: "myrepo"}, ".", 1)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 || got[0].Path != "src/" {
		t.Errorf("List = %v, want %v", got, want)
	}
}

func TestStubDriver_SetError_InjectsError(t *testing.T) {
	d := NewStubDriver(toolkit.SourceKindRepo)
	injected := &testError{"injected"}
	d.SetError(injected)

	_, err := d.Read(context.Background(), toolkit.Source{Name: "x"}, "y")
	if err != injected {
		t.Errorf("Read error = %v, want %v", err, injected)
	}

	err = d.Ensure(context.Background(), toolkit.Source{Name: "x"})
	if err != injected {
		t.Errorf("Ensure error = %v, want %v", err, injected)
	}
}

func TestStubDriver_Reset_ClearsState(t *testing.T) {
	d := NewStubDriver(toolkit.SourceKindRepo)
	d.SetError(&testError{"boom"})
	_ = d.Ensure(context.Background(), toolkit.Source{Name: "x"})

	d.Reset()

	if len(d.Calls()) != 0 {
		t.Errorf("Calls after Reset = %d, want 0", len(d.Calls()))
	}
	if err := d.Ensure(context.Background(), toolkit.Source{Name: "x"}); err != nil {
		t.Errorf("Ensure after Reset = %v, want nil", err)
	}
}

func TestStubDriver_ThreadSafety(t *testing.T) {
	d := NewStubDriver(toolkit.SourceKindRepo)
	d.WithReadData("repo:file.go", []byte("data"))

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = d.Read(context.Background(), toolkit.Source{Name: "repo"}, "file.go")
			_ = d.Ensure(context.Background(), toolkit.Source{Name: "repo"})
		}()
	}
	wg.Wait()

	calls := d.Calls()
	if len(calls) != 40 {
		t.Errorf("concurrent calls = %d, want 40", len(calls))
	}
}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }
