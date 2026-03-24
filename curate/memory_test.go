package curate

import (
	"context"
	"testing"
)

var _ Store = (*MemoryStore)(nil)

func TestMemoryStore_SaveAndLoad(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()

	ds := &Dataset{
		Name: "test-dataset",
		Records: []Record{
			{ID: "r1", Fields: map[string]Field{"name": {Name: "name", Value: "Alice"}}},
		},
	}
	if err := s.Save(ctx, ds); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := s.Load(ctx, "test-dataset")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Name != "test-dataset" {
		t.Errorf("Name = %q, want test-dataset", loaded.Name)
	}
	if len(loaded.Records) != 1 {
		t.Errorf("len(Records) = %d, want 1", len(loaded.Records))
	}
}

func TestMemoryStore_List(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()

	names, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(names) != 0 {
		t.Errorf("empty store should list 0 datasets, got %d", len(names))
	}

	s.Save(ctx, &Dataset{Name: "ds1"})
	s.Save(ctx, &Dataset{Name: "ds2"})

	names, err = s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(names) != 2 {
		t.Errorf("List = %d datasets, want 2", len(names))
	}
}

func TestMemoryStore_Load_NotFound(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()

	_, err := s.Load(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for missing dataset")
	}
}

func TestMemoryStore_Save_EmptyName(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()

	err := s.Save(ctx, &Dataset{Name: ""})
	if err == nil {
		t.Fatal("expected error for empty dataset name")
	}
}

func TestMemoryStore_Save_Overwrites(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()

	s.Save(ctx, &Dataset{Name: "ds", Records: []Record{{ID: "r1"}}})
	s.Save(ctx, &Dataset{Name: "ds", Records: []Record{{ID: "r2"}, {ID: "r3"}}})

	loaded, _ := s.Load(ctx, "ds")
	if len(loaded.Records) != 2 {
		t.Errorf("Records = %d, want 2 (overwritten)", len(loaded.Records))
	}
}
