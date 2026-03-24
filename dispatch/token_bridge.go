package dispatch

import (
	"fmt"

	"github.com/dpopsuev/bugle/billing"
	"github.com/dpopsuev/origami/format"
)

// FormatTokenSummary returns a human-readable token and cost section.
// An optional CostConfig overrides the default pricing for per-line cost
// breakdown. If omitted, DefaultCostConfig() is used.
func FormatTokenSummary(s billing.TokenSummary, opts ...billing.CostConfig) string {
	cc := billing.DefaultCostConfig()
	if len(opts) > 0 {
		cc = opts[0]
	}

	avgPerCase := 0
	if len(s.PerCase) > 0 {
		avgPerCase = s.TotalTokens / len(s.PerCase)
	}
	avgPerStep := 0
	if s.TotalSteps > 0 {
		avgPerStep = s.TotalTokens / s.TotalSteps
	}

	wallSec := float64(s.TotalWallClockMs) / 1000.0
	minutes := int(wallSec) / 60
	seconds := int(wallSec) % 60

	promptCost := float64(s.TotalPromptTokens) / 1_000_000 * cc.InputPricePerMToken
	artifactCost := float64(s.TotalArtifactTokens) / 1_000_000 * cc.OutputPricePerMToken

	tbl := format.NewTable(format.ASCII)
	tbl.Header("Metric", "Value")
	tbl.Columns(
		format.ColumnConfig{Number: 1, Align: format.AlignLeft},
		format.ColumnConfig{Number: 2, Align: format.AlignRight},
	)
	tbl.Row("Total prompts", fmt.Sprintf("%d tokens ($%.4f)", s.TotalPromptTokens, promptCost))
	tbl.Row("Total artifacts", fmt.Sprintf("%d tokens ($%.4f)", s.TotalArtifactTokens, artifactCost))
	tbl.Row("Total", fmt.Sprintf("%d tokens ($%.4f)", s.TotalTokens, s.TotalCostUSD))
	tbl.Row("Per case avg", fmt.Sprintf("%d tokens", avgPerCase))
	tbl.Row("Per step avg", fmt.Sprintf("%d tokens", avgPerStep))
	tbl.Row("Steps", fmt.Sprintf("%d", s.TotalSteps))
	tbl.Row("Wall clock", fmt.Sprintf("%dm %ds", minutes, seconds))

	return "=== Token & Cost ===\n" + tbl.String() + "\n"
}
