package main

import (
	"os"
	"strings"
	"testing"
)

// TestNoBatchWalkShortcut prevents regression — cmd/operator must not
// call sdlc.Run() or engine.BatchWalk directly. The production path
// is MCPActor via SessionFactory.
func TestNoBatchWalkShortcut(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	source := string(data)

	banned := []string{
		"sdlc.Run(",
		"engine.BatchWalk(",
		"BatchWalk(",
	}
	for _, pattern := range banned {
		if strings.Contains(source, pattern) {
			t.Errorf("cmd/operator/main.go contains banned pattern %q — use MCPActor instead", pattern)
		}
	}
}
