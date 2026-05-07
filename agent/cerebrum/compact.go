package cerebrum

import (
	"fmt"
	"strings"

	"github.com/dpopsuev/tako/agent/reactivity"
	tangle "github.com/dpopsuev/tangle"
)

// Compactor compresses spent triad history to bound context growth.
type Compactor interface {
	Compact(history []tangle.Message, sealedTriad reactivity.Triad) []tangle.Message
}

type SummaryCompactor struct {
	MaxChars int
}

func (sc SummaryCompactor) Compact(history []tangle.Message, sealed reactivity.Triad) []tangle.Message {
	if len(history) == 0 {
		return history
	}

	maxChars := sc.MaxChars
	if maxChars <= 0 {
		maxChars = reactivity.DefaultConfig.CompactMaxChars
	}

	var summary []string
	var compacted []tangle.Message
	triadLabel := sealed.String()

	const toolOutputMax = 2000

	for _, msg := range history {
		if msg.Role == "tool" {
			if len(msg.Content) > toolOutputMax {
				msg.Content = msg.Content[:toolOutputMax] + fmt.Sprintf("... (%d chars truncated)", len(msg.Content)-toolOutputMax)
			}
			compacted = append(compacted, msg)
			continue
		}
		if len(msg.ToolCalls) > 0 {
			compacted = append(compacted, msg)
			continue
		}
		if msg.Role == "assistant" && msg.Content != "" {
			summary = append(summary, msg.Content)
			continue
		}
		if msg.Role == "user" {
			summary = append(summary, msg.Content)
			continue
		}
		compacted = append(compacted, msg)
	}

	if len(summary) > 0 {
		combined := strings.Join(summary, "\n")
		if len(combined) > maxChars {
			combined = combined[:maxChars] + "..."
		}
		compacted = append([]tangle.Message{{
			Role:    "user",
			Content: fmt.Sprintf("[%s phase completed] %s", triadLabel, combined),
		}}, compacted...)
	}

	return compacted
}
