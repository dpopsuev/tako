package circuit

import "testing"

func TestReadOnlyContext(t *testing.T) {
	original := map[string]any{"key1": "val1", "key2": 42}
	snapshot := ReadOnlyContext(original)

	if snapshot["key1"] != "val1" {
		t.Errorf("snapshot[key1] = %v, want val1", snapshot["key1"])
	}

	snapshot["key1"] = "mutated"
	if original["key1"] != "val1" {
		t.Error("mutating snapshot should not affect original")
	}

	snapshot["new_key"] = "new_val"
	if _, ok := original["new_key"]; ok {
		t.Error("adding to snapshot should not affect original")
	}
}

func TestReadOnlyContext_Nil(t *testing.T) {
	if ReadOnlyContext(nil) != nil {
		t.Error("ReadOnlyContext(nil) should return nil")
	}
}
