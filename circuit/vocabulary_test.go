package circuit

import "testing"

func TestMapVocabulary_ZeroConfig(t *testing.T) {
	v := NewMapVocabulary()
	if got := v.Name("F0"); got != "F0" {
		t.Errorf("empty vocabulary: Name(F0) = %q, want F0", got)
	}
}

func TestMapVocabulary_Register(t *testing.T) {
	v := NewMapVocabulary().
		Register("F0", "Recall").
		Register("F1", "Triage")

	tests := []struct {
		code, want string
	}{
		{"F0", "Recall"},
		{"F1", "Triage"},
		{"F2", "F2"},
	}
	for _, tt := range tests {
		if got := v.Name(tt.code); got != tt.want {
			t.Errorf("Name(%q) = %q, want %q", tt.code, got, tt.want)
		}
	}
}

func TestMapVocabulary_RegisterAll(t *testing.T) {
	v := NewMapVocabulary().RegisterAll(map[string]string{
		"pb001": "Product Bug",
		"ab001": "Automation Bug",
	})
	if got := v.Name("pb001"); got != "Product Bug" {
		t.Errorf("Name(pb001) = %q, want Product Bug", got)
	}
}

func TestNameWithCode(t *testing.T) {
	v := NewMapVocabulary().Register("F0", "Recall")

	if got := NameWithCode(v, "F0"); got != "Recall (F0)" {
		t.Errorf("NameWithCode(F0) = %q, want %q", got, "Recall (F0)")
	}
	if got := NameWithCode(v, "F9"); got != "F9" {
		t.Errorf("NameWithCode(F9) = %q, want F9 (unknown passthrough)", got)
	}
}

func TestVocabularyFunc(t *testing.T) {
	upper := VocabularyFunc(func(code string) string {
		if code == "x" {
			return "X-RAY"
		}
		return code
	})
	if got := upper.Name("x"); got != "X-RAY" {
		t.Errorf("VocabularyFunc(x) = %q, want X-RAY", got)
	}
	if got := upper.Name("y"); got != "y" {
		t.Errorf("VocabularyFunc(y) = %q, want y (passthrough)", got)
	}
}

func TestChainVocabulary(t *testing.T) {
	stages := NewMapVocabulary().Register("F0", "Recall")
	defects := NewMapVocabulary().Register("pb001", "Product Bug")

	chain := ChainVocabulary{stages, defects}

	tests := []struct {
		code, want string
	}{
		{"F0", "Recall"},
		{"pb001", "Product Bug"},
		{"unknown", "unknown"},
	}
	for _, tt := range tests {
		if got := chain.Name(tt.code); got != tt.want {
			t.Errorf("ChainVocabulary.Name(%q) = %q, want %q", tt.code, got, tt.want)
		}
	}
}

func TestChainVocabulary_FirstWins(t *testing.T) {
	first := NewMapVocabulary().Register("X", "First")
	second := NewMapVocabulary().Register("X", "Second")

	chain := ChainVocabulary{first, second}
	if got := chain.Name("X"); got != "First" {
		t.Errorf("chain should pick first match: got %q, want First", got)
	}
}

// --- RichVocabulary tests ---

func TestRichMapVocabulary_Name_FallbackChain(t *testing.T) {
	v := NewRichMapVocabulary()
	v.RegisterEntry("full", VocabEntry{Short: "F", Long: "Full Name", Description: "desc"})
	v.RegisterEntry("short-only", VocabEntry{Short: "S"})
	v.RegisterEntry("empty", VocabEntry{})

	tests := []struct {
		code, want string
	}{
		{"full", "Full Name"},
		{"short-only", "S"},
		{"empty", "empty"},
		{"unknown", "unknown"},
	}
	for _, tt := range tests {
		if got := v.Name(tt.code); got != tt.want {
			t.Errorf("Name(%q) = %q, want %q", tt.code, got, tt.want)
		}
	}
}

func TestRichMapVocabulary_Entry(t *testing.T) {
	v := NewRichMapVocabulary().RegisterEntry("F0_RECALL", VocabEntry{
		Short: "F0", Long: "Recall", Description: "Initial symptom recall from failure data.",
	})

	e, ok := v.Entry("F0_RECALL")
	if !ok {
		t.Fatal("Entry(F0_RECALL) returned false")
	}
	if e.Short != "F0" || e.Long != "Recall" || e.Description != "Initial symptom recall from failure data." {
		t.Errorf("Entry(F0_RECALL) = %+v", e)
	}

	_, ok = v.Entry("UNKNOWN")
	if ok {
		t.Error("Entry(UNKNOWN) should return false")
	}
}

func TestRichMapVocabulary_Short(t *testing.T) {
	v := NewRichMapVocabulary().RegisterEntry("F0", VocabEntry{Short: "F0", Long: "Recall"})
	if got := v.Short("F0"); got != "F0" {
		t.Errorf("Short(F0) = %q, want F0", got)
	}
	if got := v.Short("UNKNOWN"); got != "" {
		t.Errorf("Short(UNKNOWN) = %q, want empty", got)
	}
}

func TestRichMapVocabulary_Description(t *testing.T) {
	v := NewRichMapVocabulary().RegisterEntry("F0", VocabEntry{Description: "Recall step"})
	if got := v.Description("F0"); got != "Recall step" {
		t.Errorf("Description(F0) = %q, want %q", got, "Recall step")
	}
	if got := v.Description("UNKNOWN"); got != "" {
		t.Errorf("Description(UNKNOWN) = %q, want empty", got)
	}
}

func TestRichMapVocabulary_RegisterEntries(t *testing.T) {
	v := NewRichMapVocabulary().RegisterEntries(map[string]VocabEntry{
		"pb001": {Short: "PB", Long: "Product Bug"},
		"ab001": {Short: "AB", Long: "Automation Bug"},
	})
	if got := v.Name("pb001"); got != "Product Bug" {
		t.Errorf("Name(pb001) = %q, want Product Bug", got)
	}
	if got := v.Name("ab001"); got != "Automation Bug" {
		t.Errorf("Name(ab001) = %q, want Automation Bug", got)
	}
}

func TestRichMapVocabulary_ImplementsVocabulary(t *testing.T) {
	var _ Vocabulary = NewRichMapVocabulary()
	var _ RichVocabulary = NewRichMapVocabulary()
}

func TestNameWithCode_RichVocabulary_UsesShort(t *testing.T) {
	v := NewRichMapVocabulary().RegisterEntry("F0_RECALL", VocabEntry{
		Short: "F0", Long: "Recall",
	})

	if got := NameWithCode(v, "F0_RECALL"); got != "Recall (F0)" {
		t.Errorf("NameWithCode(F0_RECALL) = %q, want %q", got, "Recall (F0)")
	}
	if got := NameWithCode(v, "UNKNOWN"); got != "UNKNOWN" {
		t.Errorf("NameWithCode(UNKNOWN) = %q, want UNKNOWN", got)
	}
}

func TestRichChainVocabulary(t *testing.T) {
	stages := NewRichMapVocabulary().RegisterEntry("F0", VocabEntry{Short: "F0", Long: "Recall", Description: "Recall step"})
	defects := NewRichMapVocabulary().RegisterEntry("pb001", VocabEntry{Short: "PB", Long: "Product Bug", Description: "Product defect"})

	chain := RichChainVocabulary{stages, defects}

	tests := []struct {
		code      string
		wantName  string
		wantShort string
		wantDesc  string
		wantOK    bool
	}{
		{"F0", "Recall", "F0", "Recall step", true},
		{"pb001", "Product Bug", "PB", "Product defect", true},
		{"unknown", "unknown", "", "", false},
	}
	for _, tt := range tests {
		if got := chain.Name(tt.code); got != tt.wantName {
			t.Errorf("Name(%q) = %q, want %q", tt.code, got, tt.wantName)
		}
		if got := chain.Short(tt.code); got != tt.wantShort {
			t.Errorf("Short(%q) = %q, want %q", tt.code, got, tt.wantShort)
		}
		if got := chain.Description(tt.code); got != tt.wantDesc {
			t.Errorf("Description(%q) = %q, want %q", tt.code, got, tt.wantDesc)
		}
		_, ok := chain.Entry(tt.code)
		if ok != tt.wantOK {
			t.Errorf("Entry(%q) ok = %v, want %v", tt.code, ok, tt.wantOK)
		}
	}
}

func TestRichChainVocabulary_FirstWins(t *testing.T) {
	first := NewRichMapVocabulary().RegisterEntry("X", VocabEntry{Long: "First"})
	second := NewRichMapVocabulary().RegisterEntry("X", VocabEntry{Long: "Second"})

	chain := RichChainVocabulary{first, second}
	if got := chain.Name("X"); got != "First" {
		t.Errorf("chain should pick first match: got %q, want First", got)
	}
}

