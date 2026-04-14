package operator

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dpopsuev/origami/engine/gate"
	"github.com/dpopsuev/troupe/signal"
)

// CLIGatekeeper implements collective.Gatekeeper by parking items in
// the ApprovalStore and blocking until the human resolves them via
// the MCP approval tool. Push-based: emits gate.parked signal so the
// CLI session knows to present the gate to the user.
type CLIGatekeeper struct {
	Store    gate.ApprovalStore
	Bus      signal.Bus
	Operator string // operator name for audit trail
}

// pollInterval is how often the gatekeeper checks for resolution.
const pollInterval = 500 * time.Millisecond

// Pass parks the content for human review and blocks until approved or rejected.
// Returns (true, comment, nil) on approval, (false, reason, nil) on rejection.
func (g *CLIGatekeeper) Pass(ctx context.Context, content string) (allowed bool, reason string, err error) {
	approvalID := fmt.Sprintf("cli-gate:%d", time.Now().UnixNano())

	item := gate.ApprovalItem{
		ID:         approvalID,
		CircuitRun: "cli",
		NodeName:   "cli-gate",
		Output:     json.RawMessage(content),
		ParkedAt:   time.Now(),
		Status:     gate.ApprovalPending,
	}

	if err := g.Store.Park(ctx, item); err != nil {
		return false, "", fmt.Errorf("cli gatekeeper park: %w", err)
	}

	// Emit signal so the CLI session knows a gate is waiting.
	if g.Bus != nil {
		g.Bus.Emit(&signal.Signal{
			Event: EventGateParked,
			Agent: g.Operator,
			Meta: map[string]string{
				MetaKeyApprovalID: approvalID,
				MetaKeyNodeName:   "cli-gate",
			},
		})
	}

	// Poll until resolved or context canceled.
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false, "context canceled", ctx.Err()
		case <-ticker.C:
			resolved, err := g.Store.Get(ctx, approvalID)
			if err != nil {
				return false, "", fmt.Errorf("cli gatekeeper poll: %w", err)
			}

			switch resolved.Status {
			case gate.ApprovalApproved:
				comment := ""
				if resolved.Decision != nil {
					comment = resolved.Decision.Comment
				}
				return true, comment, nil

			case gate.ApprovalRejected:
				reason := ""
				if resolved.Decision != nil {
					reason = resolved.Decision.Comment
				}
				return false, reason, nil

			case gate.ApprovalPending:
				continue
			}
		}
	}
}
