package curate

import "testing"

func TestNewRecord(t *testing.T) {
	r := NewRecord("R01")
	if r.ID != "R01" {
		t.Errorf("ID = %q, want R01", r.ID)
	}
	if r.Fields == nil {
		t.Error("Fields should be initialized")
	}
	if r.Tags == nil {
		t.Error("Tags should be initialized")
	}
}

func TestRecord_SetAndGet(t *testing.T) {
	r := NewRecord("R01")
	r.Set(Field{Name: "title", Value: "hello", Source: "manual"})

	f, ok := r.Get("title")
	if !ok {
		t.Fatal("Get returned false for existing field")
	}
	if f.Value != "hello" {
		t.Errorf("Value = %v, want hello", f.Value)
	}
	if f.Source != "manual" {
		t.Errorf("Source = %q, want manual", f.Source)
	}
}

func TestRecord_GetMissing(t *testing.T) {
	r := NewRecord("R01")
	_, ok := r.Get("nonexistent")
	if ok {
		t.Error("Get should return false for missing field")
	}
}

func TestRecord_Has(t *testing.T) {
	r := NewRecord("R01")
	r.Set(Field{Name: "present", Value: "yes"})
	r.Set(Field{Name: "nil_value", Value: nil})

	if !r.Has("present") {
		t.Error("Has should return true for field with non-nil value")
	}
	if r.Has("nil_value") {
		t.Error("Has should return false for field with nil value")
	}
	if r.Has("missing") {
		t.Error("Has should return false for missing field")
	}
}

func TestRecord_SetOverwrites(t *testing.T) {
	r := NewRecord("R01")
	r.Set(Field{Name: "x", Value: "old"})
	r.Set(Field{Name: "x", Value: "new"})

	f, _ := r.Get("x")
	if f.Value != "new" {
		t.Errorf("Value = %v, want new (should overwrite)", f.Value)
	}
}

func TestRecord_SetOnZeroFields(t *testing.T) {
	r := Record{ID: "R02"}
	r.Set(Field{Name: "x", Value: 1})
	if !r.Has("x") {
		t.Error("Set should initialize nil Fields map")
	}
}

func TestDataset(t *testing.T) {
	ds := Dataset{
		Name: "test",
		Records: []Record{
			NewRecord("R01"),
			NewRecord("R02"),
		},
	}
	if len(ds.Records) != 2 {
		t.Errorf("len(Records) = %d, want 2", len(ds.Records))
	}
}
