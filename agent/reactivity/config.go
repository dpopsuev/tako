package reactivity

import "log/slog"

type Config struct {
	DistanceClose      float64
	DistanceMid        float64
	RecollectionMin    float64
	UnmetDimMax        int
	BackwardTurnLimit  int

	ChaoticUnsealMin     int
	ComplexUnsealMin     int
	ComplexMassMin       int
	ClearRecollectionMin float64
	ClearMassMax         int

	CompactMaxChars    int
	ContractSummaryMax int
}

func (c *Config) Validate() {
	if c.DistanceClose <= 0 || c.DistanceClose >= 1 {
		slog.Warn("config.extreme", slog.String("param", "DistanceClose"), slog.Float64("value", c.DistanceClose), slog.String("safe_range", "(0, 1)"))
	}
	if c.DistanceMid <= 0 || c.DistanceMid >= 1 {
		slog.Warn("config.extreme", slog.String("param", "DistanceMid"), slog.Float64("value", c.DistanceMid), slog.String("safe_range", "(0, 1)"))
	}
	if c.DistanceClose >= c.DistanceMid {
		slog.Warn("config.conflict", slog.String("issue", "DistanceClose >= DistanceMid"), slog.Float64("close", c.DistanceClose), slog.Float64("mid", c.DistanceMid))
	}
	if c.RecollectionMin <= 0 || c.RecollectionMin >= 1 {
		slog.Warn("config.extreme", slog.String("param", "RecollectionMin"), slog.Float64("value", c.RecollectionMin), slog.String("safe_range", "(0, 1)"))
	}
	if c.BackwardTurnLimit < 1 {
		slog.Warn("config.extreme", slog.String("param", "BackwardTurnLimit"), slog.Int("value", c.BackwardTurnLimit), slog.String("safe_range", "[1, ∞)"))
	}
	if c.CompactMaxChars < 100 {
		slog.Warn("config.extreme", slog.String("param", "CompactMaxChars"), slog.Int("value", c.CompactMaxChars), slog.String("safe_range", "[100, ∞)"))
	}
	if c.ContractSummaryMax < 20 {
		slog.Warn("config.extreme", slog.String("param", "ContractSummaryMax"), slog.Int("value", c.ContractSummaryMax), slog.String("safe_range", "[20, ∞)"))
	}
}

var DefaultConfig = Config{
	DistanceClose:        0.3,
	DistanceMid:          0.5,
	RecollectionMin:      0.3,
	UnmetDimMax:          2,
	BackwardTurnLimit:    3,
	ChaoticUnsealMin:     2,
	ComplexUnsealMin:     1,
	ComplexMassMin:       10,
	ClearRecollectionMin: 0.3,
	ClearMassMax:         10,
	CompactMaxChars:      500,
	ContractSummaryMax:   120,
}
