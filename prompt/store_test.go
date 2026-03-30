package prompt

import (
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"
)

func newTestFS() fs.FS {
	return fstest.MapFS{
		"triage/classify-symptoms.md": &fstest.MapFile{
			Data: []byte("# Triage\n\n## Task\n\nClassify the failure.\n"),
		},
		"investigate/deep-rca.md": &fstest.MapFile{
			Data: []byte("# Investigate\n\n## Task\n\nFind root cause.\n"),
		},
	}
}

func TestFileStore_Get(t *testing.T) {
	store := NewFileStore(newTestFS(), nil)
	p, err := store.Get("triage/classify-symptoms")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if p.Name != "triage/classify-symptoms" {
		t.Errorf("Name = %q", p.Name)
	}
	if p.Step != "triage" {
		t.Errorf("Step = %q, want %q", p.Step, "triage")
	}
	if p.Version != 1 {
		t.Errorf("Version = %d, want 1", p.Version)
	}
	if len(p.Sections) != 2 {
		t.Errorf("Sections = %d, want 2", len(p.Sections))
	}
}

func TestFileStore_GetWithNameMap(t *testing.T) {
	nameMap := map[string]string{
		"triage": "triage/classify-symptoms.md",
	}
	store := NewFileStore(newTestFS(), nameMap)
	p, err := store.Get("triage")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if p.Name != "triage" {
		t.Errorf("Name = %q", p.Name)
	}
}

func TestFileStore_GetNotFound(t *testing.T) {
	store := NewFileStore(newTestFS(), nil)
	_, err := store.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing prompt")
	}
}

func TestFileStore_List(t *testing.T) {
	store := NewFileStore(newTestFS(), nil)
	prompts, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(prompts) != 2 {
		t.Errorf("List returned %d prompts, want 2", len(prompts))
	}
}

func TestFileStore_ReadOnly(t *testing.T) {
	store := NewFileStore(newTestFS(), nil)
	if _, err := store.Update("triage", "new content"); err == nil {
		t.Error("Update should fail on FileStore")
	}
	if _, err := store.Create("new", "step", "content"); err == nil {
		t.Error("Create should fail on FileStore")
	}
	if _, err := store.Rollback("triage", 1); err == nil {
		t.Error("Rollback should fail on FileStore")
	}
}

func TestLiveStore_CRUD(t *testing.T) {
	store := NewLiveStore()

	// Create
	p, err := store.Create("triage", "f1", "# Triage\n\nClassify.")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if p.Version != 1 {
		t.Errorf("Version = %d, want 1", p.Version)
	}

	// Get
	p, err = store.Get("triage")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if p.Name != "triage" || p.Step != "f1" {
		t.Errorf("Get returned %+v", p)
	}

	// Update
	p, err = store.Update("triage", "# Triage v2\n\nRevised.")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if p.Version != 2 {
		t.Errorf("Version = %d, want 2", p.Version)
	}

	// Verify updated content
	p, err = store.Get("triage")
	if err != nil {
		t.Fatalf("Get after update: %v", err)
	}
	if p.Version != 2 {
		t.Errorf("Get version = %d, want 2", p.Version)
	}

	// List
	all, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 1 {
		t.Errorf("List = %d, want 1", len(all))
	}
}

func TestLiveStore_Rollback(t *testing.T) {
	store := NewLiveStore()
	store.Create("triage", "f1", "version 1 content")
	store.Update("triage", "version 2 content")
	store.Update("triage", "version 3 content")

	// Rollback to v1
	p, err := store.Rollback("triage", 1)
	if err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if p.Version != 4 { // rollback creates new version
		t.Errorf("Version = %d, want 4", p.Version)
	}
	if p.Content != "version 1 content" {
		t.Errorf("Content = %q, want v1 content", p.Content)
	}
}

func TestLiveStore_RollbackNotFound(t *testing.T) {
	store := NewLiveStore()
	store.Create("triage", "f1", "content")

	_, err := store.Rollback("triage", 99)
	if err == nil {
		t.Fatal("expected error for missing version")
	}
}

func TestLiveStore_History(t *testing.T) {
	store := NewLiveStore()
	store.Create("triage", "f1", "v1")
	store.Update("triage", "v2")
	store.Update("triage", "v3")

	history, err := store.History("triage")
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(history) != 3 {
		t.Fatalf("History = %d entries, want 3", len(history))
	}
	if history[0].Version != 1 {
		t.Errorf("history[0].Version = %d, want 1", history[0].Version)
	}
	if history[2].Version != 3 {
		t.Errorf("history[2].Version = %d, want 3", history[2].Version)
	}
}

func TestLiveStore_CreateDuplicate(t *testing.T) {
	store := NewLiveStore()
	store.Create("triage", "f1", "content")
	_, err := store.Create("triage", "f1", "other")
	if err == nil {
		t.Fatal("expected error for duplicate create")
	}
}

func TestLiveStore_UpdateNotFound(t *testing.T) {
	store := NewLiveStore()
	_, err := store.Update("nonexistent", "content")
	if err == nil {
		t.Fatal("expected error for missing prompt")
	}
}

func TestLiveStore_Validation(t *testing.T) {
	store := NewLiveStore()
	if _, err := store.Create("", "step", "content"); !errors.Is(err, ErrNameRequired) {
		t.Errorf("expected ErrNameRequired, got %v", err)
	}
	if _, err := store.Create("name", "step", ""); !errors.Is(err, ErrContentEmpty) {
		t.Errorf("expected ErrContentEmpty, got %v", err)
	}
	store.Create("x", "s", "c")
	if _, err := store.Update("x", ""); !errors.Is(err, ErrContentEmpty) {
		t.Errorf("expected ErrContentEmpty, got %v", err)
	}
}

func TestNewLiveStoreFrom(t *testing.T) {
	fileStore := NewFileStore(newTestFS(), nil)
	liveStore, err := NewLiveStoreFrom(fileStore)
	if err != nil {
		t.Fatalf("NewLiveStoreFrom: %v", err)
	}

	all, err := liveStore.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("List = %d, want 2", len(all))
	}

	// Should be editable now.
	p, err := liveStore.Update("triage/classify-symptoms", "edited content")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if p.Version != 2 {
		t.Errorf("Version = %d, want 2", p.Version)
	}
}
