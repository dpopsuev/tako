package circuit

import "testing"

type schemaTestArtifact struct {
	typ  string
	conf float64
	raw  any
}

func (a *schemaTestArtifact) Type() string       { return a.typ }
func (a *schemaTestArtifact) Confidence() float64 { return a.conf }
func (a *schemaTestArtifact) Raw() any            { return a.raw }

func TestValidateArtifact_NilSchema(t *testing.T) {
	a := &schemaTestArtifact{raw: map[string]any{"x": 1}}
	if err := ValidateArtifact(nil, a); err != nil {
		t.Fatalf("nil schema should pass: %v", err)
	}
}

func TestValidateArtifact_NilRaw(t *testing.T) {
	schema := &ArtifactSchema{Type: "object"}
	a := &schemaTestArtifact{raw: nil}
	if err := ValidateArtifact(schema, a); err == nil {
		t.Fatal("nil raw should fail")
	}
}

func TestValidateArtifact_ObjectWithRequiredFields(t *testing.T) {
	schema := &ArtifactSchema{
		Type:     "object",
		Required: []string{"id", "score"},
		Fields: map[string]FieldSchema{
			"id":    {Type: "string"},
			"score": {Type: "number"},
		},
	}

	valid := &schemaTestArtifact{raw: map[string]any{"id": "C1", "score": 0.85}}
	if err := ValidateArtifact(schema, valid); err != nil {
		t.Fatalf("valid object should pass: %v", err)
	}

	missingField := &schemaTestArtifact{raw: map[string]any{"id": "C1"}}
	if err := ValidateArtifact(schema, missingField); err == nil {
		t.Fatal("missing required field should fail")
	}

	wrongType := &schemaTestArtifact{raw: map[string]any{"id": 42, "score": 0.85}}
	if err := ValidateArtifact(schema, wrongType); err == nil {
		t.Fatal("wrong field type should fail")
	}
}

func TestValidateArtifact_NestedObject(t *testing.T) {
	schema := &ArtifactSchema{
		Type:     "object",
		Required: []string{"meta"},
		Fields: map[string]FieldSchema{
			"meta": {
				Type:     "object",
				Required: []string{"version"},
				Fields: map[string]FieldSchema{
					"version": {Type: "string"},
				},
			},
		},
	}

	valid := &schemaTestArtifact{raw: map[string]any{
		"meta": map[string]any{"version": "1.0"},
	}}
	if err := ValidateArtifact(schema, valid); err != nil {
		t.Fatalf("nested valid object should pass: %v", err)
	}

	missingNested := &schemaTestArtifact{raw: map[string]any{
		"meta": map[string]any{},
	}}
	if err := ValidateArtifact(schema, missingNested); err == nil {
		t.Fatal("missing nested required field should fail")
	}
}

func TestValidateArtifact_Array(t *testing.T) {
	schema := &ArtifactSchema{
		Type: "object",
		Fields: map[string]FieldSchema{
			"items": {
				Type: "array",
				Items: &FieldSchema{
					Type:     "object",
					Required: []string{"name"},
					Fields: map[string]FieldSchema{
						"name": {Type: "string"},
					},
				},
			},
		},
	}

	valid := &schemaTestArtifact{raw: map[string]any{
		"items": []any{
			map[string]any{"name": "a"},
			map[string]any{"name": "b"},
		},
	}}
	if err := ValidateArtifact(schema, valid); err != nil {
		t.Fatalf("valid array should pass: %v", err)
	}

	invalidItem := &schemaTestArtifact{raw: map[string]any{
		"items": []any{
			map[string]any{"name": "a"},
			map[string]any{},
		},
	}}
	if err := ValidateArtifact(schema, invalidItem); err == nil {
		t.Fatal("array with invalid item should fail")
	}
}

func TestValidateArtifact_BooleanField(t *testing.T) {
	schema := &ArtifactSchema{
		Type: "object",
		Fields: map[string]FieldSchema{
			"active": {Type: "boolean"},
		},
	}

	valid := &schemaTestArtifact{raw: map[string]any{"active": true}}
	if err := ValidateArtifact(schema, valid); err != nil {
		t.Fatalf("boolean field should pass: %v", err)
	}

	wrongType := &schemaTestArtifact{raw: map[string]any{"active": "yes"}}
	if err := ValidateArtifact(schema, wrongType); err == nil {
		t.Fatal("string in boolean field should fail")
	}
}

func TestValidateArtifact_EmptyArray(t *testing.T) {
	schema := &ArtifactSchema{
		Type: "object",
		Fields: map[string]FieldSchema{
			"items": {
				Type:  "array",
				Items: &FieldSchema{Type: "string"},
			},
		},
	}

	empty := &schemaTestArtifact{raw: map[string]any{"items": []any{}}}
	if err := ValidateArtifact(schema, empty); err != nil {
		t.Fatalf("empty array should pass: %v", err)
	}
}

func TestValidateArtifact_UnknownType(t *testing.T) {
	schema := &ArtifactSchema{Type: "unicorn"}
	a := &schemaTestArtifact{raw: "hello"}
	if err := ValidateArtifact(schema, a); err == nil {
		t.Fatal("unknown schema type should fail")
	}
}

func TestValidateArtifact_NoTypeConstraint(t *testing.T) {
	schema := &ArtifactSchema{Type: ""}
	a := &schemaTestArtifact{raw: "anything"}
	if err := ValidateArtifact(schema, a); err != nil {
		t.Fatalf("empty type should pass: %v", err)
	}
}

func TestValidateArtifact_ExtraFieldsIgnored(t *testing.T) {
	schema := &ArtifactSchema{
		Type:     "object",
		Required: []string{"id"},
		Fields: map[string]FieldSchema{
			"id": {Type: "string"},
		},
	}
	a := &schemaTestArtifact{raw: map[string]any{"id": "C1", "extra": 42, "more": true}}
	if err := ValidateArtifact(schema, a); err != nil {
		t.Fatalf("extra fields should be allowed: %v", err)
	}
}

func TestValidateArtifact_TopLevelString(t *testing.T) {
	schema := &ArtifactSchema{Type: "string"}
	a := &schemaTestArtifact{raw: "hello"}
	if err := ValidateArtifact(schema, a); err != nil {
		t.Fatalf("top-level string should pass: %v", err)
	}

	notString := &schemaTestArtifact{raw: 42}
	if err := ValidateArtifact(schema, notString); err == nil {
		t.Fatal("number for string schema should fail")
	}
}

func TestValidateArtifact_TopLevelNumber(t *testing.T) {
	schema := &ArtifactSchema{Type: "number"}
	a := &schemaTestArtifact{raw: 3.14}
	if err := ValidateArtifact(schema, a); err != nil {
		t.Fatalf("top-level number should pass: %v", err)
	}
}
