package tui

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dpopsuev/tako/tui/widgets"
	"github.com/muesli/termenv"
)

var update = flag.Bool("update", false, "update .golden files")

func init() {
	lipgloss.SetColorProfile(termenv.Ascii)
}

func goldenPath(name string) string {
	return filepath.Join("testdata", name+".golden")
}

func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	path := goldenPath(name)

	if *update {
		os.MkdirAll(filepath.Dir(path), 0o750)
		os.WriteFile(path, []byte(got), 0o644)
		t.Logf("updated %s", path)
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("golden file %s not found — run with -update to create", path)
	}

	if got != string(want) {
		gotLines := strings.Split(got, "\n")
		wantLines := strings.Split(string(want), "\n")

		maxLines := len(gotLines)
		if len(wantLines) > maxLines {
			maxLines = len(wantLines)
		}

		var diff strings.Builder
		for i := 0; i < maxLines; i++ {
			g := ""
			w := ""
			if i < len(gotLines) {
				g = gotLines[i]
			}
			if i < len(wantLines) {
				w = wantLines[i]
			}
			if g != w {
				diff.WriteString("--- line ")
				diff.WriteString(strings.Repeat(" ", 0))
				diff.WriteString(string(rune('0' + i/10)))
				diff.WriteString(string(rune('0' + i%10)))
				diff.WriteString(" ---\n")
				diff.WriteString("  want: ")
				diff.WriteString(w)
				diff.WriteString("\n")
				diff.WriteString("  got:  ")
				diff.WriteString(g)
				diff.WriteString("\n")
			}
		}
		t.Fatalf("golden mismatch %s:\n%s", path, diff.String())
	}
}

func renderModel(runner Runner, model string, width, height int) string {
	m := NewModel(runner, model)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	return updated.View()
}

func TestGolden_Cabin_80x24(t *testing.T) {
	got := renderModel(nil, "test-model", 80, 24)
	assertGolden(t, "cabin_80x24", got)
}

func TestGolden_Cabin_120x40(t *testing.T) {
	got := renderModel(nil, "test-model", 120, 40)
	assertGolden(t, "cabin_120x40", got)
}

func TestGolden_Cabin_40x12(t *testing.T) {
	got := renderModel(nil, "test-model", 40, 12)
	assertGolden(t, "cabin_40x12", got)
}

func TestGolden_CabinWithContent(t *testing.T) {
	m := NewModel(nil, "test-model")
	m.width = 80
	m.height = 24
	m.engine.Resize(80, 24)
	m.output.Update(widgets.AppendOutputMsg{Line: "> Hello world"})
	m.output.Update(widgets.AppendOutputMsg{Line: "I can help you with that."})
	m.footer.Update(widgets.TokenUpdateMsg{TokensIn: 1500, TokensOut: 200, ToolCalls: 3})
	m.footer.Update(widgets.PhaseChangeMsg{Phase: "thinking", Turn: 2})
	got := m.View()
	assertGolden(t, "cabin_with_content", got)
}
