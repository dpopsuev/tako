package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStripANSI_RemovesEscapes(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "hello", "hello"},
		{"color", "\x1b[31mred\x1b[0m", "red"},
		{"bold", "\x1b[1mbold\x1b[22m", "bold"},
		{"multi", "\x1b[1;31mbold red\x1b[0m plain", "bold red plain"},
		{"empty", "", ""},
		{"cursor", "\x1b[2Jcleared", "cleared"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripANSI(tt.in)
			if got != tt.want {
				t.Errorf("StripANSI(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestGoldenPath_DerivesFromTestName(t *testing.T) {
	path := GoldenPath(t)
	want := filepath.Join("testdata", "TestGoldenPath_DerivesFromTestName.golden")
	if path != want {
		t.Errorf("GoldenPath = %q, want %q", path, want)
	}
}

func TestGoldenPath_SlashesToUnderscores(t *testing.T) {
	// Subtests have slashes in t.Name() — verify they become underscores.
	t.Run("sub/test", func(t *testing.T) {
		path := GoldenPath(t)
		want := filepath.Join("testdata", "TestGoldenPath_SlashesToUnderscores_sub_test.golden")
		if path != want {
			t.Errorf("GoldenPath = %q, want %q", path, want)
		}
	})
}

func TestRequireGolden_MatchesExisting(t *testing.T) {
	// Create a temporary golden file to match against.
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) }) //nolint:errcheck // test cleanup

	// Write the golden file.
	testdata := filepath.Join(dir, "testdata")
	if err := os.MkdirAll(testdata, 0o755); err != nil {
		t.Fatal(err)
	}
	goldenFile := filepath.Join(testdata, "TestRequireGolden_MatchesExisting.golden")
	if err := os.WriteFile(goldenFile, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Should pass — got matches golden.
	RequireGolden(t, "hello world")
}

func TestRequireGolden_StripsANSI(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) }) //nolint:errcheck // test cleanup

	testdata := filepath.Join(dir, "testdata")
	if err := os.MkdirAll(testdata, 0o755); err != nil {
		t.Fatal(err)
	}
	goldenFile := filepath.Join(testdata, "TestRequireGolden_StripsANSI.golden")
	if err := os.WriteFile(goldenFile, []byte("red text"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Input has ANSI, golden has stripped — should match.
	RequireGolden(t, "\x1b[31mred text\x1b[0m")
}
