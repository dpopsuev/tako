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
			g, w := "", ""
			if i < len(gotLines) {
				g = gotLines[i]
			}
			if i < len(wantLines) {
				w = wantLines[i]
			}
			if g != w {
				diff.WriteString("--- line ")
				diff.WriteString(string(rune('0'+i/10)))
				diff.WriteString(string(rune('0'+i%10)))
				diff.WriteString(" ---\n  want: ")
				diff.WriteString(w)
				diff.WriteString("\n  got:  ")
				diff.WriteString(g)
				diff.WriteByte('\n')
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
	assertGolden(t, "cabin_80x24", renderModel(nil, "test-model", 80, 24))
}

func TestGolden_Cabin_120x40(t *testing.T) {
	assertGolden(t, "cabin_120x40", renderModel(nil, "test-model", 120, 40))
}

func TestGolden_Cabin_40x12(t *testing.T) {
	assertGolden(t, "cabin_40x12", renderModel(nil, "test-model", 40, 12))
}

func TestGolden_CabinStructure_80x24(t *testing.T) {
	got := renderModel(nil, "test-model", 80, 24)
	lines := strings.Split(got, "\n")

	if len(lines) < 10 {
		t.Fatalf("expected 10+ lines, got %d", len(lines))
	}

	first := lines[0]
	if !strings.Contains(first, "╔") || !strings.Contains(first, "tako") {
		t.Errorf("top border missing ╔ or title: %q", first)
	}

	last := lines[len(lines)-1]
	if !strings.Contains(last, "╚") || !strings.Contains(last, "╝") {
		t.Errorf("bottom border missing ╚╝: %q", last)
	}
	if !strings.Contains(last, "↑") {
		t.Errorf("footer missing token stats: %q", last)
	}

	hasOuter := false
	hasInner := false
	hasSeparator := false
	hasInput := false
	for _, line := range lines[1 : len(lines)-1] {
		if strings.Contains(line, "║") {
			hasOuter = true
		}
		if strings.Contains(line, "┃") {
			hasInner = true
		}
		if strings.Count(line, "━") > 10 {
			hasSeparator = true
		}
		if strings.Contains(line, "Type a task") {
			hasInput = true
		}
	}

	if !hasOuter {
		t.Error("missing outer ║ borders")
	}
	if !hasInner {
		t.Error("missing inner ┃ borders")
	}
	if !hasSeparator {
		t.Error("missing ━ separator")
	}
	if !hasInput {
		t.Error("missing input placeholder")
	}
}

func TestGolden_CabinWithContent(t *testing.T) {
	m := NewModel(nil, "test-model")
	m.width = 80
	m.height = 24
	m.engine.Resize(80, 24)
	m.output.Update(widgets.AppendOutputMsg{Line: "> Hello world"})
	m.output.Update(widgets.AppendOutputMsg{Line: "I can help you with that."})
	m.cabin.Update(widgets.TokenUpdateMsg{TokensIn: 1500, TokensOut: 200, ToolCalls: 3})
	m.cabin.Update(widgets.PhaseChangeMsg{Phase: "thinking", Turn: 2})
	assertGolden(t, "cabin_with_content", m.View())
}
