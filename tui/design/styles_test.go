package design

import (
	"reflect"
	"slices"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestBuildStyles_CoreStylesRender(t *testing.T) {
	ts := DefaultTokens()
	ss := BuildStyles(ts)

	// Verify core styles produce output (non-panic, non-empty).
	if ss.User.Render("test") == "" {
		t.Error("User style should render non-empty")
	}
	if ss.Assistant.Render("test") == "" {
		t.Error("Assistant style should render non-empty")
	}
	if ss.Error.Render("test") == "" {
		t.Error("Error style should render non-empty")
	}
}

func TestBuildStyles_AllFieldsRender(t *testing.T) {
	ts := DefaultTokens()
	ss := BuildStyles(ts)

	// Spot-check that all style categories produce output.
	styles := map[string]string{
		"ToolName":    ss.ToolName.Render("x"),
		"ToolSuccess": ss.ToolSuccess.Render("x"),
		"DiffAdd":     ss.DiffAdd.Render("x"),
		"DiffDel":     ss.DiffDel.Render("x"),
		"HealthGreen": ss.HealthGreen.Render("x"),
		"HealthRed":   ss.HealthRed.Render("x"),
		"BudgetOK":    ss.BudgetOK.Render("x"),
		"ZoneCold":    ss.ZoneCold.Render("x"),
		"DriftGood":   ss.DriftGood.Render("x"),
		"ModeInsert":  ss.ModeInsert.Render("x"),
		"SepFocus":    ss.SepFocus.Render("x"),
	}
	for name, rendered := range styles {
		if rendered == "" {
			t.Errorf("%s style should render non-empty", name)
		}
	}
}

func TestActiveStyles_InitializedAtStartup(t *testing.T) {
	// ActiveStyles is set via init() — should produce output.
	if ActiveStyles.User.Render("test") == "" {
		t.Error("ActiveStyles.User should be initialized")
	}
}

func TestBuildStyles_DifferentThemes(t *testing.T) {
	defaultSS := BuildStyles(TokensFromTheme(DefaultTheme))
	claudeSS := BuildStyles(TokensFromTheme(ClaudeTheme))

	// Different themes should produce output without panic.
	_ = defaultSS.User.Render("test")
	_ = claudeSS.User.Render("test")
}

// ---------------------------------------------------------------------------
// Theme registry tests
// ---------------------------------------------------------------------------

func TestRegisterTheme(t *testing.T) {
	custom := Theme{
		User:      lipgloss.AdaptiveColor{Light: "#111111", Dark: "#222222"},
		Assistant: lipgloss.AdaptiveColor{Light: "#333333", Dark: "#444444"},
		Muted:     lipgloss.AdaptiveColor{Light: "#555555", Dark: "#666666"},
		Success:   lipgloss.AdaptiveColor{Light: "#99aaaa", Dark: "#aabbbb"},
		Error:     lipgloss.AdaptiveColor{Light: "#bbcccc", Dark: "#ccdddd"},
		Warning:   lipgloss.AdaptiveColor{Light: "#777777", Dark: "#888888"},
		Accent:    lipgloss.AdaptiveColor{Light: "#ddeeff", Dark: "#eeffaa"},
		Border:    lipgloss.AdaptiveColor{Light: "#aaaaaa", Dark: "#bbbbbb"},
		FocusDim:  lipgloss.AdaptiveColor{Light: "#aaaaaa", Dark: "#bbbbbb"},
		Thinking:  lipgloss.AdaptiveColor{Light: "#555555", Dark: "#666666"},
		Executing: lipgloss.AdaptiveColor{Light: "#555555", Dark: "#666666"},
		Reading:   lipgloss.AdaptiveColor{Light: "#555555", Dark: "#666666"},
		Surface0:  lipgloss.AdaptiveColor{Light: "#ffffff", Dark: "#000000"},
		Surface1:  lipgloss.AdaptiveColor{Light: "#eeeeee", Dark: "#111111"},
		Surface2:  lipgloss.AdaptiveColor{Light: "#dddddd", Dark: "#222222"},
	}
	RegisterTheme("test-custom", custom)

	got := ThemeByName("test-custom")
	if got.User.Light != custom.User.Light || got.User.Dark != custom.User.Dark {
		t.Errorf("User mismatch: got %+v, want %+v", got.User, custom.User)
	}
	if got.Accent.Light != custom.Accent.Light || got.Accent.Dark != custom.Accent.Dark {
		t.Errorf("Accent mismatch: got %+v, want %+v", got.Accent, custom.Accent)
	}
	if got.Error.Dark != custom.Error.Dark {
		t.Errorf("Error.Dark mismatch: got %s, want %s", got.Error.Dark, custom.Error.Dark)
	}

	// Cleanup to avoid polluting other tests.
	delete(registry, "test-custom")
}

func TestThemeByName_Presets(t *testing.T) {
	presets := []string{"redhat", "claude", "gemini", "codex"}
	for _, name := range presets {
		t.Run(name, func(t *testing.T) {
			th := ThemeByName(name)
			if th.User.Light == "" && th.User.Dark == "" {
				t.Errorf("theme %q returned zero User color", name)
			}
			if th.Accent.Light == "" && th.Accent.Dark == "" {
				t.Errorf("theme %q returned zero Accent color", name)
			}
		})
	}
}

func TestThemeByName_NotFound(t *testing.T) {
	got := ThemeByName("bogus")
	if got.User.Light != DefaultTheme.User.Light || got.User.Dark != DefaultTheme.User.Dark {
		t.Errorf("expected DefaultTheme for unknown name, got User=%+v", got.User)
	}
	if got.Accent.Light != DefaultTheme.Accent.Light {
		t.Errorf("expected DefaultTheme.Accent.Light=%s, got %s", DefaultTheme.Accent.Light, got.Accent.Light)
	}
}

func TestThemeNames(t *testing.T) {
	names := ThemeNames()
	if len(names) < 4 {
		t.Fatalf("expected at least 4 theme names, got %d", len(names))
	}
	for _, req := range []string{"redhat", "claude", "gemini", "codex"} {
		if !slices.Contains(names, req) {
			t.Errorf("ThemeNames() missing %q", req)
		}
	}
}

func TestThemeNames_AfterRegister(t *testing.T) {
	const name = "custom-test-theme-names"
	RegisterTheme(name, DefaultTheme)
	defer delete(registry, name)

	names := ThemeNames()
	if !slices.Contains(names, name) {
		t.Errorf("ThemeNames() should contain %q after RegisterTheme", name)
	}
}

// ---------------------------------------------------------------------------
// TokensFromTheme / DefaultTokens tests
// ---------------------------------------------------------------------------

func TestTokensFromTheme(t *testing.T) {
	ts := TokensFromTheme(DefaultTheme)

	checks := []struct {
		name  string
		color lipgloss.AdaptiveColor
	}{
		{"UserFg", ts.UserFg},
		{"AssistantFg", ts.AssistantFg},
		{"SuccessFg", ts.SuccessFg},
		{"ErrorFg", ts.ErrorFg},
		{"AccentFg", ts.AccentFg},
		{"DiffAddFg", ts.DiffAddFg},
		{"DiffDelFg", ts.DiffDelFg},
		{"DiffHeaderFg", ts.DiffHeaderFg},
		{"HealthGreenFg", ts.HealthGreenFg},
		{"HealthYellowFg", ts.HealthYellowFg},
		{"HealthRedFg", ts.HealthRedFg},
		{"ZoneColdFg", ts.ZoneColdFg},
		{"ZoneFocusedFg", ts.ZoneFocusedFg},
		{"WarningFg", ts.WarningFg},
		{"FocusDimFg", ts.FocusDimFg},
		{"ToolNameFg", ts.ToolNameFg},
		{"ToolArgFg", ts.ToolArgFg},
	}
	for _, c := range checks {
		if c.color.Light == "" && c.color.Dark == "" {
			t.Errorf("TokensFromTheme: %s should have non-empty Light or Dark", c.name)
		}
	}
}

func TestDefaultTokens(t *testing.T) {
	dt := DefaultTokens()
	ft := TokensFromTheme(DefaultTheme)

	// Compare a representative set of fields.
	if dt.UserFg != ft.UserFg {
		t.Errorf("UserFg: DefaultTokens()=%+v, TokensFromTheme()=%+v", dt.UserFg, ft.UserFg)
	}
	if dt.AssistantFg != ft.AssistantFg {
		t.Errorf("AssistantFg mismatch")
	}
	if dt.ErrorFg != ft.ErrorFg {
		t.Errorf("ErrorFg mismatch")
	}
	if dt.AccentFg != ft.AccentFg {
		t.Errorf("AccentFg mismatch")
	}
	if dt.DiffAddFg != ft.DiffAddFg {
		t.Errorf("DiffAddFg mismatch")
	}
	if dt.HealthGreenFg != ft.HealthGreenFg {
		t.Errorf("HealthGreenFg mismatch")
	}
	if dt.ZoneFocusedFg != ft.ZoneFocusedFg {
		t.Errorf("ZoneFocusedFg mismatch")
	}
}

// ---------------------------------------------------------------------------
// BuildStyles comprehensive reflect test
// ---------------------------------------------------------------------------

func TestBuildStyles_AllFieldsNonZero(t *testing.T) {
	ts := DefaultTokens()
	ss := BuildStyles(ts)

	v := reflect.ValueOf(ss)
	typ := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldName := typ.Field(i).Name

		// AccentFg is lipgloss.AdaptiveColor, not lipgloss.Style — skip render check.
		if fieldName == "AccentFg" {
			ac := field.Interface().(lipgloss.AdaptiveColor)
			if ac.Light == "" && ac.Dark == "" {
				t.Errorf("field %s: AdaptiveColor has empty Light and Dark", fieldName)
			}
			continue
		}

		// All other fields are lipgloss.Style — verify they render non-empty.
		style, ok := field.Interface().(lipgloss.Style)
		if !ok {
			t.Errorf("field %s: expected lipgloss.Style, got %s", fieldName, field.Type())
			continue
		}
		rendered := style.Render("x")
		if rendered == "" {
			t.Errorf("field %s: Render(\"x\") returned empty string", fieldName)
		}
	}
}
