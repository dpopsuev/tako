// stub_border.go — StubBorderRenderer for deterministic test output.
//
// Implements layout.BorderRenderer without ANSI or lipgloss.
// Uses ASCII box-drawing so golden files and assertions are stable.
//
// GOL-188, TSK-1199
package testutil

import (
	"fmt"
	"strings"
)

// StubBorderRenderer implements layout.BorderRenderer without ANSI.
type StubBorderRenderer struct{}

func (StubBorderRenderer) RenderWithDepth(content string, depth, width int) string {
	return fmt.Sprintf("[d%d|%s]", depth, trimToWidth(content, width))
}

func (StubBorderRenderer) RenderBorderOnly(content string, focused bool, width int) string {
	marker := "-"
	if focused {
		marker = "*"
	}
	return fmt.Sprintf("[%s|%s]", marker, trimToWidth(content, width))
}

func (StubBorderRenderer) FocusDepths(count, focusedIdx int) []int {
	depths := make([]int, count)
	for i := range depths {
		if i == focusedIdx {
			depths[i] = 0
		} else {
			depths[i] = 1
		}
	}
	return depths
}

func trimToWidth(s string, width int) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if len(line) > width {
			lines[i] = line[:width]
		}
	}
	return strings.Join(lines, "\n")
}
