package def

import (
	"reflect"
	"testing"
	"unicode"
)

// TestKindTrap_YAMLFieldsMustUseKindType scans all exported structs in the
// package. Any field tagged yaml:"kind" must be of type Kind, not string.
// This prevents divergent kind systems from creeping in.
func TestKindTrap_YAMLFieldsMustUseKindType(t *testing.T) {
	kindType := reflect.TypeOf(Kind(""))

	structs := []struct {
		name string
		typ  reflect.Type
	}{
		{"Envelope", reflect.TypeOf(Envelope{})},
		{"ComponentManifest", reflect.TypeOf(ComponentManifest{})},
		{"InstrumentManifest", reflect.TypeOf(InstrumentManifest{})},
		{"StoreSchema", reflect.TypeOf(StoreSchema{})},
	}

	for _, s := range structs {
		t.Run(s.name, func(t *testing.T) {
			for i := range s.typ.NumField() {
				f := s.typ.Field(i)
				tag := f.Tag.Get("yaml")
				if tag == "" || tag == "-" {
					continue
				}
				name, _, _ := cut(tag, ",")
				if name != "kind" {
					continue
				}
				if f.Type != kindType {
					t.Errorf("%s.%s has yaml:\"kind\" but type %s — must use def.Kind", s.name, f.Name, f.Type)
				}
			}
		})
	}
}

// TestKindTrap_AllConstantsRegistered verifies every Kind constant has an
// entry in KnownKinds. Adding a Kind constant without registering it causes
// this test to fail.
func TestKindTrap_AllConstantsRegistered(t *testing.T) {
	allKinds := []struct {
		name  string
		value Kind
	}{
		{"KindSchematic", KindSchematic},
		{"KindComponent", KindComponent},
		{"KindBoard", KindBoard},
		{"KindCircuit", KindCircuit},
		{"KindStoreSchema", KindStoreSchema},
		{"KindScorecard", KindScorecard},
		{"KindScenario", KindScenario},
		{"KindArtifactSchema", KindArtifactSchema},
		{"KindReportTemplate", KindReportTemplate},
		{"KindVocabulary", KindVocabulary},
		{"KindHeuristicRules", KindHeuristicRules},
		{"KindSourcePack", KindSourcePack},
		{"KindTuning", KindTuning},
		{"KindDataset", KindDataset},
		{"KindInstrument", KindInstrument},
		{"KindPrompt", KindPrompt},
	}

	for _, k := range allKinds {
		t.Run(k.name, func(t *testing.T) {
			if !KnownKinds[k.value] {
				t.Errorf("%s = %q is not registered in KnownKinds — add it to envelope.go", k.name, k.value)
			}
		})
	}

	// Reverse: KnownKinds entries must match a constant above.
	registered := make(map[Kind]bool, len(allKinds))
	for _, k := range allKinds {
		registered[k.value] = true
	}
	for kind := range KnownKinds {
		if !registered[kind] {
			t.Errorf("KnownKinds has %q but no matching Kind constant — remove stale entry or add constant", kind)
		}
	}
}

// TestKindTrap_ValuesMustBePascalCase ensures all KnownKinds keys follow
// K8s PascalCase convention: start with uppercase, no hyphens, no spaces.
func TestKindTrap_ValuesMustBePascalCase(t *testing.T) {
	for kind := range KnownKinds {
		t.Run(string(kind), func(t *testing.T) {
			s := string(kind)
			if s == "" {
				t.Error("empty kind value in KnownKinds")
				return
			}
			if !unicode.IsUpper(rune(s[0])) {
				t.Errorf("kind %q must start with uppercase letter (PascalCase)", s)
			}
			for _, r := range s {
				if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
					t.Errorf("kind %q contains %q — must be PascalCase (letters and digits only)", s, string(r))
				}
			}
		})
	}
}

// TestKindTrap_ParseKindGateway verifies ParseKind validates known kinds
// and rejects unknown ones. This is the gateway contract test.
func TestKindTrap_ParseKindGateway(t *testing.T) {
	t.Run("KnownKind", func(t *testing.T) {
		data := []byte("kind: Schematic\ncircuit: test\n")
		kind, err := ParseKind(data)
		if err != nil {
			t.Fatalf("ParseKind: %v", err)
		}
		if kind != KindSchematic {
			t.Errorf("kind = %q, want %q", kind, KindSchematic)
		}
	})

	t.Run("UnknownKind", func(t *testing.T) {
		data := []byte("kind: bogus\ncircuit: test\n")
		_, err := ParseKind(data)
		if err == nil {
			t.Fatal("expected error for unknown kind")
		}
	})

	t.Run("EmptyKind", func(t *testing.T) {
		data := []byte("circuit: test\n")
		kind, err := ParseKind(data)
		if err != nil {
			t.Fatalf("ParseKind: %v", err)
		}
		if kind != "" {
			t.Errorf("kind = %q, want empty", kind)
		}
	})
}

// cut is strings.Cut inlined to avoid import for a test helper.
func cut(s, sep string) (before, after string, found bool) {
	for i := range len(s) {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			return s[:i], s[i+len(sep):], true
		}
	}
	return s, "", false
}
