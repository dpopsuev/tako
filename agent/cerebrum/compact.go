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

// SummaryCompactor replaces sealed triad messages with a single summary.
// Tool call/result pairs are preserved (factual, reusable).
type SummaryCompactor struct{}

func (SummaryCompactor) Compact(history []tangle.Message, sealed reactivity.Triad) []tangle.Message {
	if len(history) == 0 {
		return history
	}

	var summary []string
	var compacted []tangle.Message
	triadLabel := sealed.String()

	for _, msg := range history {
		if msg.Role == "tool" || len(msg.ToolCalls) > 0 {
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
		if len(combined) > 500 {
			combined = combined[:500] + "..."
		}
		compacted = append([]tangle.Message{{
			Role:    "user",
			Content: fmt.Sprintf("[%s phase completed] %s", triadLabel, combined),
		}}, compacted...)
	}

	return compacted
}
