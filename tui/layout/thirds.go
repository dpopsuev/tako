// thirds.go — Law of Thirds layout calculations.
//
// Pure math: given terminal height and input line count, compute
// the height of each panel (output, input, dashboard).
//
// Rules:
//   - Input bottom pinned between 1/3 and 1/2 from bottom (60% from top)
//   - Dashboard pinned size below input
//   - Output flexible above input
//   - Input expands upward on multi-line, steals from output
//   - Output never below MinOutputHeight (3 lines)
package layout

// ThirdsLayout holds the computed heights for the three-panel layout.
type ThirdsLayout struct {
	OutputHeight    int
	InputHeight     int
	DashboardHeight int
}

// Layout constraints.
const (
	MinOutputHeight    = 3
	MinInputHeight     = 1
	MinDashboardHeight = 3
	InputAnchorRatio   = 0.60 // input bottom at 60% from top
	DashboardRatio     = 0.25 // dashboard gets ~25% of terminal
)

// ComputeThirdsLayout calculates panel heights from terminal height
// and current input line count.
func ComputeThirdsLayout(termHeight, inputLines int) ThirdsLayout {
	if termHeight <= 0 {
		return ThirdsLayout{MinOutputHeight, MinInputHeight, MinDashboardHeight}
	}

	if inputLines < MinInputHeight {
		inputLines = MinInputHeight
	}

	// Dashboard: fixed proportion, clamped.
	dashHeight := int(float64(termHeight) * DashboardRatio)
	if dashHeight < MinDashboardHeight {
		dashHeight = MinDashboardHeight
	}

	// Input anchor: 60% from top is where input bottom sits.
	inputBottom := int(float64(termHeight) * InputAnchorRatio)

	// Input top = inputBottom - inputLines.
	inputTop := inputBottom - inputLines
	if inputTop < MinOutputHeight {
		inputTop = MinOutputHeight
	}

	// Actual input height (may be clamped by output minimum).
	actualInputHeight := inputBottom - inputTop

	// Output gets everything above input.
	outputHeight := inputTop

	// Validate totals — if dashboard + input + output > termHeight, shrink dashboard.
	total := outputHeight + actualInputHeight + dashHeight
	if total > termHeight {
		dashHeight = termHeight - outputHeight - actualInputHeight
		if dashHeight < MinDashboardHeight {
			dashHeight = MinDashboardHeight
			outputHeight = termHeight - actualInputHeight - dashHeight
			if outputHeight < MinOutputHeight {
				outputHeight = MinOutputHeight
			}
		}
	}

	return ThirdsLayout{
		OutputHeight:    outputHeight,
		InputHeight:     actualInputHeight,
		DashboardHeight: dashHeight,
	}
}
