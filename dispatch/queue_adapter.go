package dispatch

import (
	"context"
	"math"
	"time"

	"github.com/dpopsuev/troupe/providers"
)

// QueueAdapter wraps a MuxDispatcher as a troupe/providers.Queue.
// This is the bridge between Origami's dispatch protocol and Troupe's
// generic work distribution interface. Djinn and other consumers
// import troupe/providers.Queue — Origami provides the implementation.
type QueueAdapter struct {
	mux *MuxDispatcher
}

// NewQueueAdapter creates a Queue backed by the given MuxDispatcher.
func NewQueueAdapter(mux *MuxDispatcher) *QueueAdapter {
	return &QueueAdapter{mux: mux}
}

// muxWorkItem wraps Context as providers.WorkItem.
type muxWorkItem struct {
	dc Context
}

func (w *muxWorkItem) ID() uint64             { return uint64(max(w.dc.DispatchID, 0)) } //nolint:gosec // DispatchID is always positive
func (w *muxWorkItem) Input() string          { return w.dc.PromptContent }
func (w *muxWorkItem) Timeout() time.Duration { return w.dc.Timeout }

// Enqueue dispatches work through the MuxDispatcher. Blocks until
// result is submitted via Submit.
func (a *QueueAdapter) Enqueue(ctx context.Context, item providers.WorkItem) error {
	dc := Context{
		DispatchID:    int64(min(item.ID(), uint64(math.MaxInt64))), //nolint:gosec // safe clamp
		PromptContent: item.Input(),
		Timeout:       item.Timeout(),
	}
	_, err := a.mux.Dispatch(ctx, dc)
	return err
}

// Pull returns the next available work item from the MuxDispatcher.
func (a *QueueAdapter) Pull(ctx context.Context) (providers.WorkItem, error) {
	dc, err := a.mux.GetNextStep(ctx)
	if err != nil {
		return nil, err
	}
	return &muxWorkItem{dc: dc}, nil
}

// PullWithHints returns work matching the hints from the MuxDispatcher.
func (a *QueueAdapter) PullWithHints(ctx context.Context, hints providers.WorkerHints) (providers.WorkItem, error) {
	dc, err := a.mux.GetNextStepWithHints(ctx, PullHints{
		PreferredCaseID:   hints.PreferredTag,
		Stickiness:        hints.Stickiness,
		ConsecutiveMisses: hints.ConsecutiveMisses,
	})
	if err != nil {
		return nil, err
	}
	return &muxWorkItem{dc: dc}, nil
}

// Submit delivers a result back to the MuxDispatcher.
func (a *QueueAdapter) Submit(ctx context.Context, id uint64, result []byte) error {
	return a.mux.SubmitArtifact(ctx, int64(min(id, uint64(math.MaxInt64))), result) //nolint:gosec // safe clamp
}

// ActiveCount returns the number of pending dispatches.
func (a *QueueAdapter) ActiveCount() int {
	a.mux.mu.Lock()
	defer a.mux.mu.Unlock()
	return len(a.mux.pending)
}
