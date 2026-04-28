package circuit

// Category: DSL & Build — artifact schema validation.

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dpopsuev/tako/circuit/def"
)

// ArtifactSchema is a type alias for def.ArtifactSchema.
type ArtifactSchema = def.ArtifactSchema

// FieldSchema is a type alias for def.FieldSchema.
type FieldSchema = def.FieldSchema

// ValidateArtifact checks that an artifact's Raw() value conforms to the schema.
// Returns nil if the schema is nil (no validation) or if the artifact matches.
func ValidateArtifact(schema *ArtifactSchema, artifact Artifact) error {
	if schema == nil {
		return nil
	}

	raw := artifact.Raw()
	if raw == nil {
		return ErrArtifactIsNil
	}

	data, err := json.Marshal(raw)
	if err != nil {
		return fmt.Errorf("marshal artifact for validation: %w", err)
	}

	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("unmarshal artifact for validation: %w", err)
	}

	return validateValue(schema.Type, schema.Required, schema.Fields, nil, value, "")
}

func validateValue(typ string, required []string, fields map[string]FieldSchema, items *FieldSchema, value any, path string) error {
	if path == "" {
		path = "$"
	}

	switch typ {
	case "object":
		return validateObject(required, fields, value, path)
	case "array":
		return validateArray(items, value, path)
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("%s: %w, got %T", path, ErrExpectedString, value)
		}
	case "number":
		if _, ok := value.(float64); !ok {
			return fmt.Errorf("%s: %w, got %T", path, ErrExpectedNumber, value)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("%s: %w, got %T", path, ErrExpectedBoolean, value)
		}
	case "":
		// no type constraint
	default:
		return fmt.Errorf("%s: %w %q", path, ErrUnknownSchemaType, typ)
	}

	return nil
}

func validateObject(required []string, fields map[string]FieldSchema, value any, path string) error {
	obj, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("%s: %w, got %T", path, ErrExpectedObject, value)
	}

	for _, req := range required {
		if _, exists := obj[req]; !exists {
			return fmt.Errorf("%s: %w %q", path, ErrMissingRequiredField, req)
		}
	}

	for fieldName, fieldSchema := range fields {
		fieldVal, exists := obj[fieldName]
		if !exists {
			continue
		}
		fieldPath := path + "." + fieldName
		if err := validateValue(fieldSchema.Type, fieldSchema.Required, fieldSchema.Fields, fieldSchema.Items, fieldVal, fieldPath); err != nil {
			return err
		}
	}

	return nil
}

func validateArray(items *FieldSchema, value any, path string) error {
	arr, ok := value.([]any)
	if !ok {
		return fmt.Errorf("%s: %w, got %T", path, ErrExpectedArray, value)
	}

	if items == nil || len(arr) == 0 {
		return nil
	}

	var errs []string
	for i, elem := range arr {
		elemPath := fmt.Sprintf("%s[%d]", path, i)
		if err := validateValue(items.Type, items.Required, items.Fields, items.Items, elem, elemPath); err != nil {
			errs = append(errs, err.Error())
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("%w: %s", ErrSchemaValidation, strings.Join(errs, "; "))
	}
	return nil
}
