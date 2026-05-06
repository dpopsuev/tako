// breakpoint.go — responsive breakpoints for terminal size adaptation.
package layout

// Breakpoint represents a terminal width range.
type Breakpoint int

const (
	Small   Breakpoint = iota // <=80 cols
	Medium                    // 81-120 cols
	Large                     // 121-200 cols
	Massive                   // 201+ cols
)

func (b Breakpoint) String() string {
	switch b {
	case Small:
		return "small"
	case Medium:
		return "medium"
	case Large:
		return "large"
	case Massive:
		return "massive"
	default:
		return "unknown"
	}
}

// Dashboard style constants.
const (
	DashboardStyleCompact = "compact"
	DashboardStyleFull    = "full"
)

// LayoutConfig holds dimension decisions for a given terminal size.
type LayoutConfig struct {
	Breakpoint      Breakpoint
	InputHeight     int    // textarea line count
	ShowLogo        bool   // show ASCII logo in MOTD
	MaxContentWidth int    // cap content width (0 = no cap)
	DashboardStyle  string // "compact" or "full"
	BorderWidth     int    // always 2 (left + right border chars)
}

// ComputeLayout returns the layout config for the given terminal dimensions.
func ComputeLayout(width, height int) LayoutConfig {
	cfg := LayoutConfig{
		BorderWidth: 2,
	}

	switch {
	case width <= 80:
		cfg.Breakpoint = Small
		cfg.InputHeight = 1
		cfg.ShowLogo = false
		cfg.MaxContentWidth = width
		cfg.DashboardStyle = DashboardStyleCompact
	case width <= 120:
		cfg.Breakpoint = Medium
		cfg.InputHeight = 3
		cfg.ShowLogo = true
		cfg.MaxContentWidth = width
		cfg.DashboardStyle = DashboardStyleFull
	case width <= 200:
		cfg.Breakpoint = Large
		cfg.InputHeight = 3
		cfg.ShowLogo = true
		cfg.MaxContentWidth = width
		cfg.DashboardStyle = DashboardStyleFull
	default:
		cfg.Breakpoint = Massive
		cfg.InputHeight = 3
		cfg.ShowLogo = true
		cfg.MaxContentWidth = 200 // cap at readable width
		cfg.DashboardStyle = DashboardStyleFull
	}

	// Adjust input height for very short terminals.
	if height < 20 && cfg.InputHeight > 1 {
		cfg.InputHeight = 1
	}

	return cfg
}

// InnerWidth returns the content width inside borders.
func (c LayoutConfig) InnerWidth() int {
	w := c.MaxContentWidth - c.BorderWidth
	if w < 10 {
		w = 10
	}
	return w
}

// FixedHeight returns lines consumed by non-output panels (input + dashboard + borders).
func (c LayoutConfig) FixedHeight() int {
	// output border(2) + input content + input border(2) + dashboard content(1) + dashboard border(2)
	return 2 + c.InputHeight + 2 + 1 + 2
}
