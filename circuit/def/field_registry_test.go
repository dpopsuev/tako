package def

import (
	"reflect"
	"strings"
	"testing"
)

// yamlTags extracts all yaml tag names from a struct type, skipping
// inline embeds and unexported fields.
func yamlTags(t reflect.Type) []string {
	var tags []string
	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		tag := f.Tag.Get("yaml")
		if tag == "" || tag == "-" {
			continue
		}
		// Handle ",inline" embeds — skip them, their fields are on the parent.
		if strings.Contains(tag, ",inline") {
			continue
		}
		// Strip options: "name,omitempty" → "name"
		name, _, _ := strings.Cut(tag, ",")
		if name != "" {
			tags = append(tags, name)
		}
	}
	return tags
}

// assertRegistryComplete verifies every yaml-tagged field on the struct
// has an entry in the registry. Adding a field without registering it
// causes this test to fail — poka-yoke.
func assertRegistryComplete(t *testing.T, structName string, structType reflect.Type, registry FieldRegistry) {
	t.Helper()
	tags := yamlTags(structType)
	for _, tag := range tags {
		if !registry.Has(tag) {
			t.Errorf("%s: yaml field %q has no FieldRegistry entry — register it in field_registry.go", structName, tag)
		}
	}
	// Reverse check: registry shouldn't have entries for fields that don't exist.
	for name := range registry {
		found := false
		for _, tag := range tags {
			if tag == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s: FieldRegistry has entry %q but no matching yaml field — remove stale entry", structName, name)
		}
	}
}

func TestFieldRegistry_CircuitDef_Complete(t *testing.T) {
	assertRegistryComplete(t, "CircuitDef", reflect.TypeOf(CircuitDef{}), CircuitFields)
}

func TestFieldRegistry_NodeDef_Complete(t *testing.T) {
	assertRegistryComplete(t, "NodeDef", reflect.TypeOf(NodeDef{}), NodeFields)
}

func TestFieldRegistry_EdgeDef_Complete(t *testing.T) {
	assertRegistryComplete(t, "EdgeDef", reflect.TypeOf(EdgeDef{}), EdgeFields)
}

func TestFieldRegistry_ZoneDef_Complete(t *testing.T) {
	assertRegistryComplete(t, "ZoneDef", reflect.TypeOf(ZoneDef{}), ZoneFields)
}

func TestFieldRegistry_WalkerDef_Complete(t *testing.T) {
	assertRegistryComplete(t, "WalkerDef", reflect.TypeOf(WalkerDef{}), WalkerFields)
}

func TestFieldRegistry_PortDef_Complete(t *testing.T) {
	assertRegistryComplete(t, "PortDef", reflect.TypeOf(PortDef{}), PortFields)
}

func TestFieldRegistry_RequiredFields(t *testing.T) {
	required := CircuitFields.RequiredFields()
	want := map[string]bool{"circuit": true, "nodes": true, "edges": true, "start": true, "done": true}
	for _, name := range required {
		if !want[name] {
			t.Errorf("unexpected required field: %q", name)
		}
		delete(want, name)
	}
	for name := range want {
		t.Errorf("missing required field: %q", name)
	}
}
