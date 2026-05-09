package cerebrum

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/dpopsuev/tako/agent/reactivity"
	tangle "github.com/dpopsuev/tangle"
)

func (cb *Cerebrum) oneShot(ctx context.Context, molecule *reactivity.Molecule, need []byte) (ThinkOutcome, error) {
	prompt := cb.assemble(molecule, need, Clear, 0)
	chain := molecule.Chain()

	completer := cb.router.Route(molecule)
	messages := []tangle.Message{{Role: RoleUser, Content: prompt}}

	start := time.Now()
	var onToken func(string)
	if cb.listener != nil {
		onToken = cb.listener.OnToken
	}

	turnCtx, turnCancel := context.WithTimeout(ctx, cb.budget.TurnTimeout)
	completion, err := completer.Complete(turnCtx, tangle.CompletionParams{
		Messages:      messages,
		Tools:         cb.tools(molecule.Phase()),
		ThinkingLevel: ThinkingMinimal,
		OnToken:       onToken,
	})
	elapsed := time.Since(start)
	turnCancel()

	if err != nil {
		return OutcomeSealed, err
	}

	molecule.Tick()

	slog.InfoContext(ctx, "cerebrum.oneshot.response",
		slog.Duration("elapsed", elapsed),
		slog.Int("tool_calls", len(completion.ToolCalls)),
		slog.Int("tokens_in", completion.Tokens.Input),
		slog.Int("tokens_out", completion.Tokens.Output))

	if cb.listener != nil {
		cb.listener.OnTokenUpdate(completion.Tokens.Input, completion.Tokens.Output, len(completion.ToolCalls))
		cb.listener.OnContext(molecule.Phase().String(), 0, molecule.Distance())
	}

	if completion.Content != "" {
		molecule.SetResponse(completion.Content)
	}

	if len(completion.ToolCalls) > 0 {
		for _, tc := range completion.ToolCalls {
			cb.registerPending(tc.ID)
			if cb.listener != nil {
				cb.listener.OnToolCall(tc.Name, tc.Input)
			}
			molecule.Emit(reactivity.Emission{
				Kind:       string(EventOrgan),
				Target:     tc.Name,
				Payload:    tc.Input,
				ToolCallID: tc.ID,
			})
		}

		cb.dispatch(ctx, molecule)

		toolCtx, toolCancel := context.WithTimeout(ctx, cb.budget.TurnTimeout)
		for _, tc := range completion.ToolCalls {
			tr := cb.waitToolResult(toolCtx, tc)
			chain.Append(reactivity.ChainEvent{
				Kind:   organEventRole(tc.Name, cb.capabilities),
				Organ:  tc.Name,
				Input:  tc.Input,
				Output: []byte(tr.Content),
			})
			if cb.listener != nil {
				cb.listener.OnToolResult(tc.Name, []byte(tr.Content), elapsed)
			}
		}
		toolCancel()
	}

	response := molecule.Response()
	if response == "" {
		if last, ok := chain.Last(); ok && len(last.Output) > 0 {
			response = string(last.Output)
			molecule.SetResponse(response)
		}
	}

	cb.reactor.Seal(molecule, reactivity.Atom{
		ID:        fmt.Sprintf("wish-oneshot-%d", time.Now().UnixNano()),
		Type:      reactivity.RetrospectionAtom,
		Taxonomy:  "retrospection.oneshot",
		Content:   []byte(response),
		CreatedAt: time.Now(),
	})

	if cb.listener != nil {
		cb.listener.OnSealed(molecule.ID, molecule.Distance(), molecule.Turns(), response)
	}

	cb.molecule = molecule
	cb.reactor.Monolog().Park()

	slog.InfoContext(ctx, "cerebrum.oneshot.done",
		slog.String("molecule", molecule.ID),
		slog.Int("chain_events", chain.Len()),
		slog.Int("response_len", len(response)))

	return OutcomeSealed, nil
}
