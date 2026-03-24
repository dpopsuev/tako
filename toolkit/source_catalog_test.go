package toolkit

import "testing"

func TestSliceCatalog_Sources(t *testing.T) {
	t.Parallel()
	cat := &SliceCatalog{Items: []Source{
		{Name: "a"},
		{Name: "b"},
	}}
	if len(cat.Sources()) != 2 {
		t.Errorf("Sources() len = %d, want 2", len(cat.Sources()))
	}
}

func TestSliceCatalog_NilSafe(t *testing.T) {
	t.Parallel()
	var cat *SliceCatalog
	if cat.Sources() != nil {
		t.Error("nil catalog Sources() should return nil")
	}
	if cat.AlwaysReadSources() != nil {
		t.Error("nil catalog AlwaysReadSources() should return nil")
	}
}

func TestSliceCatalog_AlwaysReadSources(t *testing.T) {
	t.Parallel()
	cat := &SliceCatalog{Items: []Source{
		{Name: "always-1", ReadPolicy: ReadAlways},
		{Name: "cond-1", ReadPolicy: ReadConditional},
		{Name: "always-2", ReadPolicy: ReadAlways},
		{Name: "empty"},
	}}
	got := cat.AlwaysReadSources()
	if len(got) != 2 {
		t.Fatalf("AlwaysReadSources() len = %d, want 2", len(got))
	}
	if got[0].Name != "always-1" || got[1].Name != "always-2" {
		t.Errorf("unexpected sources: %v", got)
	}
}

func TestSliceCatalog_EmptyItems(t *testing.T) {
	t.Parallel()
	cat := &SliceCatalog{}
	if len(cat.Sources()) != 0 {
		t.Error("empty catalog should return empty slice")
	}
	if cat.AlwaysReadSources() != nil {
		t.Error("empty catalog AlwaysReadSources() should return nil")
	}
}

func TestSliceCatalog_ImplementsSourceCatalog(t *testing.T) {
	t.Parallel()
	var _ SourceCatalog = (*SliceCatalog)(nil)
}
