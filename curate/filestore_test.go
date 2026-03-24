package curate

import (
	"context"
	"os"
	"testing"
)

func TestFileStore_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}

	ds := &Dataset{
		Name: "test-dataset",
		Records: []Record{
			{
				ID: "R01",
				Fields: map[string]Field{
					"title": {Name: "title", Value: "hello", Source: "manual"},
					"score": {Name: "score", Value: 0.95, Confidence: 0.8},
				},
				Tags: map[string]string{"origin": "test"},
			},
			{
				ID:     "R02",
				Fields: map[string]Field{"title": {Name: "title", Value: "world"}},
			},
		},
	}

	ctx := context.Background()

	if err := store.Save(ctx, ds); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := store.Load(ctx, "test-dataset")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Name != "test-dataset" {
		t.Errorf("Name = %q, want test-dataset", loaded.Name)
	}
	if len(loaded.Records) != 2 {
		t.Fatalf("len(Records) = %d, want 2", len(loaded.Records))
	}
	if loaded.Records[0].ID != "R01" {
		t.Errorf("Records[0].ID = %q, want R01", loaded.Records[0].ID)
	}

	f, ok := loaded.Records[0].Fields["title"]
	if !ok {
		t.Fatal("title field missing after round-trip")
	}
	if f.Value != "hello" {
		t.Errorf("title.Value = %v, want hello", f.Value)
	}
}

func TestFileStore_List(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}

	ctx := context.Background()

	store.Save(ctx, &Dataset{Name: "alpha", Records: []Record{NewRecord("R01")}})
	store.Save(ctx, &Dataset{Name: "beta", Records: []Record{NewRecord("R02")}})

	names, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(names) != 2 {
		t.Errorf("len = %d, want 2", len(names))
	}
}

func TestFileStore_LoadNotFound(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	_, err = store.Load(context.Background(), "nonexistent")
	if err == nil {
		t.Error("Load should fail for nonexistent dataset")
	}
}

func TestFileStore_CreatesDir(t *testing.T) {
	dir := t.TempDir() + "/sub/nested"
	store, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	info, err := os.Stat(store.Dir)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !info.IsDir() {
		t.Error("should have created directory")
	}
}

// Verify FileStore satisfies the Store interface at compile time.
var _ Store = (*FileStore)(nil)
