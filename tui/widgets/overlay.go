package widgets

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/dpopsuev/tako/tui/core"
)

type ShowOverlayMsg struct {
	Panel core.Panel
}

type HideOverlayMsg struct{}

type OverlayContainer struct {
	panel  core.Panel
	width  int
	height int
}

func NewOverlayContainer() *OverlayContainer {
	return &OverlayContainer{}
}

func (o *OverlayContainer) Show(panel core.Panel) {
	o.panel = panel
	panel.SetFocus(true)
}

func (o *OverlayContainer) Hide() {
	if o.panel != nil {
		o.panel.SetFocus(false)
	}
	o.panel = nil
}

func (o *OverlayContainer) Active() bool {
	return o.panel != nil
}

func (o *OverlayContainer) Panel() core.Panel {
	return o.panel
}

func (o *OverlayContainer) Update(msg tea.Msg) tea.Cmd {
	if o.panel == nil {
		return nil
	}
	_, cmd := o.panel.Update(msg)
	return cmd
}

func (o *OverlayContainer) Render(base string, width, height int) string {
	if o.panel == nil {
		return base
	}

	overlayW := width * 2 / 3
	if overlayW < 30 {
		overlayW = 30
	}

	content := o.panel.View(overlayW - 4)
	box := lipgloss.NewStyle().
		Width(overlayW).
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2).
		Render(content)

	boxLines := strings.Split(box, "\n")
	baseLines := strings.Split(base, "\n")

	for len(baseLines) < height {
		baseLines = append(baseLines, strings.Repeat(" ", width))
	}

	startRow := (height - len(boxLines)) / 2
	startCol := (width - overlayW - 2) / 2
	if startRow < 0 {
		startRow = 0
	}
	if startCol < 0 {
		startCol = 0
	}

	for i, boxLine := range boxLines {
		row := startRow + i
		if row >= len(baseLines) {
			break
		}
		baseLine := baseLines[row]
		padded := baseLine
		for len(padded) < width {
			padded += " "
		}

		before := padded[:startCol]
		after := ""
		endCol := startCol + lipgloss.Width(boxLine)
		if endCol < len(padded) {
			after = padded[endCol:]
		}
		baseLines[row] = before + boxLine + after
	}

	return strings.Join(baseLines[:height], "\n")
}
