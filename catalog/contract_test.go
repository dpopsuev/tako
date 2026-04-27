package catalog

import (
	"errors"
	"testing"
)

func TestStubCatalogList(t *testing.T) {
	c := NewStubCatalog()
	entries := c.List()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Name != "echo" {
		t.Errorf("expected 'echo', got %q", entries[0].Name)
	}
	if entries[0].TrustLayer != Builtin {
		t.Errorf("expected Builtin trust layer, got %d", entries[0].TrustLayer)
	}
}

func TestStubCatalogResolve(t *testing.T) {
	c := NewStubCatalog()
	entry, err := c.Resolve("echo")
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if entry.Name != "echo" {
		t.Errorf("expected 'echo', got %q", entry.Name)
	}
}

func TestStubCatalogNotFound(t *testing.T) {
	c := NewStubCatalog()
	_, err := c.Resolve("missing")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
