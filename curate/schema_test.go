package curate

import "testing"

func TestSchema_RequiredFields(t *testing.T) {
	s := Schema{
		Name: "test",
		Fields: []FieldSpec{
			{Name: "a", Requirement: Required},
			{Name: "b", Requirement: Optional},
			{Name: "c", Requirement: Required},
		},
	}
	req := s.RequiredFields()
	if len(req) != 2 {
		t.Fatalf("RequiredFields len = %d, want 2", len(req))
	}
	if req[0].Name != "a" || req[1].Name != "c" {
		t.Errorf("RequiredFields = %v, want [a, c]", req)
	}
}

func TestCheckCompleteness_AllPresent(t *testing.T) {
	s := Schema{
		Name: "test",
		Fields: []FieldSpec{
			{Name: "title", Requirement: Required},
			{Name: "defect_type", Requirement: Required},
		},
	}
	r := NewRecord("R01")
	r.Set(Field{Name: "title", Value: "something"})
	r.Set(Field{Name: "defect_type", Value: "product_bug"})

	result := CheckCompleteness(r, s)
	if result.RecordID != "R01" {
		t.Errorf("RecordID = %q, want R01", result.RecordID)
	}
	if result.Score != 1.0 {
		t.Errorf("Score = %.2f, want 1.0", result.Score)
	}
	if !result.Promotable {
		t.Error("should be promotable when all required fields present")
	}
	if len(result.Missing) != 0 {
		t.Errorf("Missing = %v, want empty", result.Missing)
	}
}

func TestCheckCompleteness_MissingFields(t *testing.T) {
	s := Schema{
		Name: "test",
		Fields: []FieldSpec{
			{Name: "title", Requirement: Required},
			{Name: "defect_type", Requirement: Required},
			{Name: "description", Requirement: Optional},
		},
	}
	r := NewRecord("R01")
	r.Set(Field{Name: "title", Value: "something"})

	result := CheckCompleteness(r, s)
	if result.Score != 0.5 {
		t.Errorf("Score = %.2f, want 0.5", result.Score)
	}
	if result.Promotable {
		t.Error("should not be promotable with missing required field")
	}
	if len(result.Missing) != 1 || result.Missing[0] != "defect_type" {
		t.Errorf("Missing = %v, want [defect_type]", result.Missing)
	}
}

func TestCheckCompleteness_InvalidFields(t *testing.T) {
	s := Schema{
		Name: "test",
		Fields: []FieldSpec{
			{Name: "score", Requirement: Required, Validate: func(v any) bool {
				f, ok := v.(float64)
				return ok && f >= 0 && f <= 1
			}},
		},
	}
	r := NewRecord("R01")
	r.Set(Field{Name: "score", Value: 5.0})

	result := CheckCompleteness(r, s)
	if result.Promotable {
		t.Error("should not be promotable with invalid field")
	}
	if len(result.Invalid) != 1 || result.Invalid[0] != "score" {
		t.Errorf("Invalid = %v, want [score]", result.Invalid)
	}
}

func TestCheckCompleteness_NilValue(t *testing.T) {
	s := Schema{
		Name: "test",
		Fields: []FieldSpec{
			{Name: "x", Requirement: Required},
		},
	}
	r := NewRecord("R01")
	r.Set(Field{Name: "x", Value: nil})

	result := CheckCompleteness(r, s)
	if result.Promotable {
		t.Error("should not be promotable when field value is nil")
	}
	if len(result.Missing) != 1 {
		t.Errorf("Missing = %v, want [x]", result.Missing)
	}
}

func TestCheckCompleteness_EmptySchema(t *testing.T) {
	s := Schema{Name: "empty"}
	r := NewRecord("R01")

	result := CheckCompleteness(r, s)
	if !result.Promotable {
		t.Error("should be promotable with no required fields")
	}
	if result.Score != 0 {
		t.Errorf("Score = %.2f, want 0 (no required fields)", result.Score)
	}
}

func TestCheckCompleteness_OnlyOptional(t *testing.T) {
	s := Schema{
		Name: "optional-only",
		Fields: []FieldSpec{
			{Name: "notes", Requirement: Optional},
		},
	}
	r := NewRecord("R01")

	result := CheckCompleteness(r, s)
	if !result.Promotable {
		t.Error("should be promotable when only optional fields exist (even if missing)")
	}
}
