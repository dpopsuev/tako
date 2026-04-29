package cerebrum

import (
	"context"
	"fmt"
	"time"

	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tako/instrument"
)

func dispatch(ctx context.Context, shell instrument.Shell, tc *ToolCall) (reactivity.Atom, error) {
	if shell == nil {
		return reactivity.Atom{}, fmt.Errorf("cerebrum: no shell for dispatch")
	}

	result, err := shell.Exec(ctx, tc.Name, tc.Input)
	if err != nil {
		return reactivity.Atom{}, fmt.Errorf("cerebrum: instrument %s: %w", tc.Name, err)
	}

	return reactivity.Atom{
		ID:        fmt.Sprintf("instrument-%s-%d", tc.Name, time.Now().UnixNano()),
		Type:      reactivity.ExecutionAtom,
		Source:    reactivity.Instrument,
		Taxonomy:  fmt.Sprintf("execution.instrument.%s", tc.Name),
		Content:   result.Text(),
		CreatedAt: time.Now(),
	}, nil
}
