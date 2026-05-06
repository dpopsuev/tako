// constraints.go — Typed layout constraints for ergonomic panel sizing (GOL-43).
//
// Constraints declare how a panel wants to be sized. The solver resolves
// them into pixel allocations given available space.
package layout

// Constraint declares a sizing preference for a panel.
type Constraint interface {
	constraintMarker() // sealed interface
}

// Fixed requests an exact height in lines.
type Fixed int

func (Fixed) constraintMarker() {}

// MinHeight requests at least N lines.
type MinHeight int

func (MinHeight) constraintMarker() {}

// MaxHeight requests at most N lines.
type MaxHeight int

func (MaxHeight) constraintMarker() {}

// Fill expands to consume remaining space. Weight determines proportion
// when multiple Fill constraints compete.
type Fill struct {
	Weight int // default 1
}

func (Fill) constraintMarker() {}

// Percentage requests a fraction of available space (0.0-1.0).
type Percentage float64

func (Percentage) constraintMarker() {}

// PanelConstraints combines multiple constraints for a single panel.
type PanelConstraints struct {
	Min  int     // minimum height (0 = no minimum)
	Max  int     // maximum height (0 = no maximum)
	Fill int     // flex weight (0 = fixed, >0 = flex)
	Pct  float64 // percentage of available (0.0 = not used)
}

// Solve resolves a set of panel constraints into pixel allocations.
// Returns a height for each panel. Respects Min/Max/Fill/Pct in priority order.
func Solve(constraints []PanelConstraints, available int) []int {
	n := len(constraints)
	if n == 0 {
		return nil
	}

	heights := make([]int, n)
	remaining := available

	// Pass 1: Allocate fixed and percentage-based panels.
	for i, c := range constraints {
		if c.Pct > 0 {
			h := int(c.Pct * float64(available))
			h = clampHeight(h, c.Min, c.Max)
			heights[i] = h
			remaining -= h
		} else if c.Fill == 0 {
			// Fixed panel — use Min as the requested height.
			h := c.Min
			if h == 0 {
				h = 1 // minimum 1 line
			}
			heights[i] = h
			remaining -= h
		}
	}

	if remaining < 0 {
		remaining = 0
	}

	// Pass 2: Distribute remaining space to Fill panels by weight.
	totalWeight := 0
	for i, c := range constraints {
		if c.Fill > 0 && heights[i] == 0 {
			totalWeight += c.Fill
		}
	}

	if totalWeight > 0 {
		for i, c := range constraints {
			if c.Fill > 0 && heights[i] == 0 {
				h := (remaining * c.Fill) / totalWeight
				h = clampHeight(h, c.Min, c.Max)
				heights[i] = h
			}
		}
	}

	return heights
}

func clampHeight(h, lower, upper int) int {
	if lower > 0 && h < lower {
		h = lower
	}
	if upper > 0 && h > upper {
		h = upper
	}
	return h
}

// HeightBreakpoint returns a height classification for responsive decisions.
type HeightBreakpoint int

const (
	HeightTiny   HeightBreakpoint = iota // < 15 lines
	HeightSmall                          // 15-24 lines
	HeightMedium                         // 25-39 lines
	HeightLarge                          // 40+ lines
)

// ClassifyHeight returns the height breakpoint for responsive layout.
func ClassifyHeight(height int) HeightBreakpoint {
	switch {
	case height < 15: //nolint:mnd // breakpoint
		return HeightTiny
	case height < 25: //nolint:mnd // breakpoint
		return HeightSmall
	case height < 40: //nolint:mnd // breakpoint
		return HeightMedium
	default:
		return HeightLarge
	}
}
