package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/dpopsuev/tako/tui/widgets"
	"github.com/muesli/termenv"
)

func init() {
	lipgloss.SetColorProfile(termenv.Ascii)
}

func TestGolden_CabinStructure_80x24(t *testing.T) {
	tm := teatest.NewTestModel(t, NewModel(nil, "test-model"),
		teatest.WithInitialTermSize(80, 24))
	tm.Quit()
	got := string(readAll(t, tm.FinalOutput(t, teatest.WithFinalTimeout(time.Second))))
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

	hasOuter, hasInner, hasSeparator, hasInput := false, false, false, false
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
	tm := teatest.NewTestModel(t, NewModel(nil, "test-model"),
		teatest.WithInitialTermSize(80, 24))

	tm.Send(widgets.AppendOutputMsg{Line: "> Hello world"})
	tm.Send(widgets.AppendOutputMsg{Line: "I can help you with that."})
	tm.Send(widgets.TokenUpdateMsg{TokensIn: 1500, TokensOut: 200, ToolCalls: 3})
	tm.Send(widgets.PhaseChangeMsg{Phase: "thinking", Turn: 2})

	time.Sleep(100 * time.Millisecond)
	tm.Quit()
	got := string(readAll(t, tm.FinalOutput(t, teatest.WithFinalTimeout(time.Second))))

	if !strings.Contains(got, "Hello world") {
		t.Error("missing output content")
	}
	if !strings.Contains(got, "1.5k") || !strings.Contains(got, "200") {
		t.Error("missing token stats in footer")
	}
	if !strings.Contains(got, "thinking") {
		t.Error("missing phase in footer")
	}
}

func TestGolden_SpinnerActivatesOnPhase(t *testing.T) {
	tm := teatest.NewTestModel(t, NewModel(nil, "test-model"),
		teatest.WithInitialTermSize(80, 24))

	tm.Send(widgets.PhaseChangeMsg{Phase: "assessment", Turn: 1})
	time.Sleep(200 * time.Millisecond)
	tm.Quit()

	got := string(readAll(t, tm.FinalOutput(t, teatest.WithFinalTimeout(time.Second))))

	hasSpinnerChar := false
	for _, ch := range "◐◓◑◒◇◈◆" {
		if strings.ContainsRune(got, ch) {
			hasSpinnerChar = true
			break
		}
	}
	if !hasSpinnerChar {
		t.Error("spinner should show geometric shape during active phase")
	}
}

func readAll(t *testing.T, r interface{ Read([]byte) (int, error) }) []byte {
	t.Helper()
	var buf []byte
	tmp := make([]byte, 4096)
	for {
		n, err := r.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			break
		}
	}
	return buf
}
