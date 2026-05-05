package cerebrum

import (
	"testing"
)

func TestCatalystFromTask_BasicMapping(t *testing.T) {
	card := TaskCard{
		ID:       "TSK-1",
		Title:    "Add foo feature",
		Goal:     "Implement foo to improve bar.",
		Priority: "high",
	}
	cat := CatalystFromTask(card)

	if cat.Trust != 0.5 {
		t.Errorf("trust = %f, want 0.5 for high priority", cat.Trust)
	}
	if cat.Need == "" {
		t.Error("need should not be empty")
	}
	if cat.Desired["tests_pass"] != true {
		t.Error("default criteria should include tests_pass")
	}
	if cat.Desired["build_clean"] != true {
		t.Error("default criteria should include build_clean")
	}
}

func TestCatalystFromTask_TrustMapping(t *testing.T) {
	cases := []struct {
		priority string
		want     float64
	}{
		{"critical", 0.3},
		{"high", 0.5},
		{"medium", 0.7},
		{"low", 0.9},
		{"", 0.5},
	}
	for _, tc := range cases {
		cat := CatalystFromTask(TaskCard{Title: "x", Priority: tc.priority})
		if cat.Trust != tc.want {
			t.Errorf("priority=%q: trust=%f, want %f", tc.priority, cat.Trust, tc.want)
		}
	}
}

func TestCatalystFromTask_AcceptanceDesired(t *testing.T) {
	card := TaskCard{
		Title:    "Deploy service",
		Priority: "critical",
		Sections: map[string]string{
			"acceptance": "- [ ] tests pass\n- [ ] no regressions",
		},
	}
	cat := CatalystFromTask(card)

	if len(cat.Desired) != 2 {
		t.Fatalf("expected 2 criteria from acceptance, got %d: %v", len(cat.Desired), cat.Desired)
	}
	if _, ok := cat.Desired["tests_pass"]; !ok {
		t.Error("missing 'tests_pass' criterion")
	}
	if _, ok := cat.Desired["no_regressions"]; !ok {
		t.Error("missing 'no_regressions' criterion")
	}
}

func TestCatalystFromTask_SectionsInNeed(t *testing.T) {
	card := TaskCard{
		Title: "Fix bug",
		Sections: map[string]string{
			"context":    "Bug in auth flow",
			"acceptance": "- tests pass",
			"notes":      "should be ignored",
		},
	}
	cat := CatalystFromTask(card)

	if len(cat.Need) == 0 {
		t.Fatal("need should not be empty")
	}
	if !contains(cat.Need, "Bug in auth flow") {
		t.Error("need should include context section")
	}
	if contains(cat.Need, "should be ignored") {
		t.Error("need should NOT include notes section")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsImpl(s, sub))
}

func containsImpl(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
