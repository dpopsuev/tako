package mcp

import (
	"strings"
	"testing"
)

func TestGenerateHeraldicName(t *testing.T) {
	name := GenerateHeraldicName()
	parts := strings.Split(name, "-")
	if len(parts) != 2 {
		t.Fatalf("expected adjective-animal, got %q", name)
	}
	if parts[0] == "" || parts[1] == "" {
		t.Fatalf("empty part in %q", name)
	}
}

func TestGenerateHeraldicName_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for range 50 {
		name := GenerateHeraldicName()
		seen[name] = true
	}
	// With 400 combinations, 50 samples should produce at least 30 unique.
	if len(seen) < 30 {
		t.Errorf("expected diversity, got only %d unique names from 50 samples", len(seen))
	}
}
