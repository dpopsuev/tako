package cerebrum

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrSchemaValidation   = errors.New("schema validation failed")
	ErrExpectedObject     = errors.New("expected object")
	ErrExpectedArray      = errors.New("expected array")
	ErrExpectedString     = errors.New("expected string")
	ErrExpectedNumber     = errors.New("expected number")
	ErrExpectedBoolean    = errors.New("expected boolean")
	ErrMissingRequired    = errors.New("missing required field")
)

type OutputSchema struct {
	Type     string                  `json:"type" yaml:"type"`
	Required []string                `json:"required,omitempty" yaml:"required,omitempty"`
	Fields   map[string]FieldSchema  `json:"fields,omitempty" yaml:"fields,omitempty"`
	Items    *FieldSchema            `json:"items,omitempty" yaml:"items,omitempty"`
}

type FieldSchema struct {
	Type     string                  `json:"type" yaml:"type"`
	Required []string                `json:"required,omitempty" yaml:"required,omitempty"`
	Fields   map[string]FieldSchema  `json:"fields,omitempty" yaml:"fields,omitempty"`
	Items    *FieldSchema            `json:"items,omitempty" yaml:"items,omitempty"`
}

func ValidateOutput(schema *OutputSchema, raw []byte) error {
	if schema == nil {
		return nil
	}

	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return fmt.Errorf("unmarshal for validation: %w", err)
	}

	return validateValue(schema.Type, schema.Required, schema.Fields, schema.Items, value, "$")
}

func validateValue(typ string, required []string, fields map[string]FieldSchema, items *FieldSchema, value any, path string) error {
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
		// no constraint
	default:
		return fmt.Errorf("%s: unknown schema type %q", path, typ)
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
			return fmt.Errorf("%s: %w %q", path, ErrMissingRequired, req)
		}
	}
	for name, schema := range fields {
		val, exists := obj[name]
		if !exists {
			continue
		}
		if err := validateValue(schema.Type, schema.Required, schema.Fields, schema.Items, val, path+"."+name); err != nil {
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
		if err := validateValue(items.Type, items.Required, items.Fields, items.Items, elem, fmt.Sprintf("%s[%d]", path, i)); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%w: %s", ErrSchemaValidation, strings.Join(errs, "; "))
	}
	return nil
}
