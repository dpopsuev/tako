package cerebrum

import (
	"errors"
	"testing"
)

func TestValidateOutput_NilSchema(t *testing.T) {
	if err := ValidateOutput(nil, []byte(`{}`)); err != nil {
		t.Errorf("nil schema should pass: %v", err)
	}
}

func TestValidateOutput_ValidObject(t *testing.T) {
	schema := &OutputSchema{
		Type:     "object",
		Required: []string{"type", "content"},
		Fields: map[string]FieldSchema{
			"type":    {Type: "string"},
			"content": {Type: "string"},
		},
	}
	raw := []byte(`{"type":"assessment","content":"analysis complete"}`)
	if err := ValidateOutput(schema, raw); err != nil {
		t.Errorf("valid object should pass: %v", err)
	}
}

func TestValidateOutput_MissingRequired(t *testing.T) {
	schema := &OutputSchema{
		Type:     "object",
		Required: []string{"type", "content"},
	}
	raw := []byte(`{"type":"assessment"}`)
	err := ValidateOutput(schema, raw)
	if err == nil {
		t.Fatal("missing required should fail")
	}
	if !errors.Is(err, ErrMissingRequired) {
		t.Errorf("expected ErrMissingRequired, got: %v", err)
	}
}

func TestValidateOutput_WrongType(t *testing.T) {
	schema := &OutputSchema{
		Type: "object",
		Fields: map[string]FieldSchema{
			"score": {Type: "number"},
		},
	}
	raw := []byte(`{"score":"not a number"}`)
	err := ValidateOutput(schema, raw)
	if err == nil {
		t.Fatal("wrong type should fail")
	}
	if !errors.Is(err, ErrExpectedNumber) {
		t.Errorf("expected ErrExpectedNumber, got: %v", err)
	}
}

func TestValidateOutput_ValidArray(t *testing.T) {
	schema := &OutputSchema{
		Type: "object",
		Fields: map[string]FieldSchema{
			"atoms": {
				Type:  "array",
				Items: &FieldSchema{Type: "object", Required: []string{"type"}, Fields: map[string]FieldSchema{"type": {Type: "string"}}},
			},
		},
	}
	raw := []byte(`{"atoms":[{"type":"assessment"},{"type":"plan"}]}`)
	if err := ValidateOutput(schema, raw); err != nil {
		t.Errorf("valid array should pass: %v", err)
	}
}

func TestValidateOutput_InvalidJSON(t *testing.T) {
	schema := &OutputSchema{Type: "object"}
	err := ValidateOutput(schema, []byte(`not json`))
	if err == nil {
		t.Fatal("invalid JSON should fail")
	}
}

func TestValidateOutput_ArrayItemFails(t *testing.T) {
	schema := &OutputSchema{
		Type: "array",
		Items: &FieldSchema{Type: "string"},
	}
	raw := []byte(`["hello", 42]`)
	err := ValidateOutput(schema, raw)
	if err == nil {
		t.Fatal("array with wrong item type should fail")
	}
	if !errors.Is(err, ErrSchemaValidation) {
		t.Errorf("expected ErrSchemaValidation, got: %v", err)
	}
}
