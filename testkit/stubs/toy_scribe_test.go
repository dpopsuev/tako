package stubs_test

import (
	"testing"

	"github.com/dpopsuev/tako/testkit/stubs"
)

func TestToyScribeStore_CreateAndGet(t *testing.T) {
	s := stubs.NewToyScribeStore()

	result := s.List("")
	if len(result) != 0 {
		t.Fatalf("empty store should have 0 items, got %d", len(result))
	}

	s.Handle(t.Context(), stubs.ScribeInput("create", map[string]string{
		"title": "Fix lint issue",
		"kind":  "task",
		"scope": "toy-repo",
	}))

	if s.Count() != 1 {
		t.Fatalf("count = %d, want 1", s.Count())
	}

	item := s.Get("TSK-1")
	if item == nil {
		t.Fatal("Get(TSK-1) returned nil")
	}
	if item.Title != "Fix lint issue" {
		t.Errorf("title = %q, want %q", item.Title, "Fix lint issue")
	}
	if item.Status != "draft" {
		t.Errorf("status = %q, want draft", item.Status)
	}
	if item.Kind != "task" {
		t.Errorf("kind = %q, want task", item.Kind)
	}
}

func TestToyScribeStore_ListByStatus(t *testing.T) {
	s := stubs.NewToyScribeStore()
	s.Seed(
		&stubs.ToyArtifact{ID: "TSK-1", Title: "Draft task", Status: "draft", Kind: "task"},
		&stubs.ToyArtifact{ID: "TSK-2", Title: "Mature task", Status: "mature", Kind: "task"},
		&stubs.ToyArtifact{ID: "TSK-3", Title: "Another draft", Status: "draft", Kind: "task"},
	)

	drafts := s.List("draft")
	if len(drafts) != 2 {
		t.Errorf("drafts = %d, want 2", len(drafts))
	}

	mature := s.List("mature")
	if len(mature) != 1 {
		t.Errorf("mature = %d, want 1", len(mature))
	}

	all := s.List("")
	if len(all) != 3 {
		t.Errorf("all = %d, want 3", len(all))
	}
}

func TestToyScribeStore_SetStatus(t *testing.T) {
	s := stubs.NewToyScribeStore()
	s.Seed(&stubs.ToyArtifact{ID: "TSK-1", Title: "Task", Status: "mature", Kind: "task"})

	s.Handle(t.Context(), stubs.ScribeInput("set", map[string]string{
		"id":    "TSK-1",
		"field": "status",
		"value": "allocated",
	}))

	item := s.Get("TSK-1")
	if item.Status != "allocated" {
		t.Errorf("status = %q, want allocated", item.Status)
	}
}

func TestToyScribeStore_AttachSection(t *testing.T) {
	s := stubs.NewToyScribeStore()
	s.Seed(&stubs.ToyArtifact{ID: "TSK-1", Title: "Task", Status: "draft", Kind: "task"})

	s.Handle(t.Context(), stubs.ScribeInput("attach_section", map[string]string{
		"id":   "TSK-1",
		"name": "findings",
		"text": "unused import fmt in main.go",
	}))

	item := s.Get("TSK-1")
	if item.Sections["findings"] != "unused import fmt in main.go" {
		t.Errorf("section findings = %q", item.Sections["findings"])
	}
}

func TestToyScribeStore_FullLifecycle(t *testing.T) {
	s := stubs.NewToyScribeStore()

	// Create
	s.Handle(t.Context(), stubs.ScribeInput("create", map[string]string{
		"title": "Fix lint",
		"kind":  "task",
	}))

	// Set mature
	s.Handle(t.Context(), stubs.ScribeInput("set", map[string]string{
		"id": "TSK-1", "field": "status", "value": "mature",
	}))

	// Attach section
	s.Handle(t.Context(), stubs.ScribeInput("attach_section", map[string]string{
		"id": "TSK-1", "name": "plan", "text": "Remove unused import",
	}))

	// Allocate
	s.Handle(t.Context(), stubs.ScribeInput("set", map[string]string{
		"id": "TSK-1", "field": "status", "value": "allocated",
	}))

	// Mark done
	s.Handle(t.Context(), stubs.ScribeInput("set", map[string]string{
		"id": "TSK-1", "field": "status", "value": "done",
	}))

	item := s.Get("TSK-1")
	if item.Status != "done" {
		t.Errorf("final status = %q, want done", item.Status)
	}
	if item.Sections["plan"] != "Remove unused import" {
		t.Errorf("plan section = %q", item.Sections["plan"])
	}

	// No mature tasks remain
	mature := s.List("mature")
	if len(mature) != 0 {
		t.Errorf("mature after done = %d, want 0", len(mature))
	}
}
