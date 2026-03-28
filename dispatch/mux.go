package dispatch

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/dpopsuev/origami/agentport"
)

var (
	_ agentport.Dispatcher         = (*MuxDispatcher)(nil)
	_ agentport.ExternalDispatcher = (*MuxDispatcher)(nil)
)

// MuxDispatcher bridges the calibration runner (which calls Dispatch from
// potentially many goroutines) with an external agent (which calls
// GetNextStep / SubmitArtifact). Each Dispatch call gets a unique dispatch ID
// and its own response channel, so artifacts are routed to the correct caller
// even under high parallelism.
type MuxDispatcher struct {
	ctx     context.Context
	log     *slog.Logger
	mu      sync.Mutex
	nextID  int64
	pending map[int64]chan []byte
	closed  map[int64]struct{}

	promptCh chan agentport.Context
	abortCh  chan struct{}
	abortErr error

	bus agentport.Bus // optional; for zone_shift signals

	queueMu sync.Mutex
	queue   []agentport.Context // buffered overflow for hint-based matching
}

// MuxOption configures a MuxDispatcher.
type MuxOption func(*MuxDispatcher)

// WithMuxSignalBus attaches a agentport.Bus for emitting dispatch-level signals
// (e.g. zone_shift on work stealing).
func WithMuxSignalBus(bus agentport.Bus) MuxOption {
	return func(d *MuxDispatcher) { d.bus = bus }
}

// NewMuxDispatcher creates a dispatcher with per-dispatch artifact routing.
// The provided context controls the dispatcher's lifetime.
func NewMuxDispatcher(ctx context.Context, opts ...MuxOption) *MuxDispatcher {
	d := &MuxDispatcher{
		ctx:      ctx,
		log:      slog.Default().With("component", "mux-dispatch"),
		pending:  make(map[int64]chan []byte),
		closed:   make(map[int64]struct{}),
		promptCh: make(chan agentport.Context),
		abortCh:  make(chan struct{}),
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// Dispatch assigns a unique dispatch ID, sends the prompt to the agent side,
// and blocks until the matching SubmitArtifact delivers the response.
// Satisfies the Dispatcher interface.
//
//nolint:gocritic // hugeParam: interface conformance (agentport.Dispatcher)
func (d *MuxDispatcher) Dispatch(ctx context.Context, dc agentport.Context) ([]byte, error) {
	dispatchStart := time.Now()

	d.mu.Lock()
	d.nextID++
	id := d.nextID
	responseCh := make(chan []byte, 1)
	d.pending[id] = responseCh
	pendingCount := len(d.pending)
	d.mu.Unlock()

	dc.DispatchID = id

	d.log.DebugContext(ctx, "mux dispatch registered",
		"dispatch_id", id,
		"case_id", dc.CaseID,
		"step", dc.Step,
		"pending_count", pendingCount,
	)

	// Send prompt to the agent side
	select {
	case d.promptCh <- dc:
	case <-ctx.Done():
		d.removePending(id)
		d.log.WarnContext(ctx, "mux dispatch cancelled while sending prompt",
			"case_id", dc.CaseID,
			"step", dc.Step,
			"dispatch_id", id,
		)
		return nil, fmt.Errorf("mux dispatch cancelled: %w", ctx.Err())
	case <-d.ctx.Done():
		d.removePending(id)
		d.log.WarnContext(ctx, "mux dispatch cancelled while sending prompt",
			"case_id", dc.CaseID,
			"step", dc.Step,
			"dispatch_id", id,
		)
		return nil, fmt.Errorf("mux dispatch cancelled: %w", d.ctx.Err())
	case <-d.abortCh:
		d.removePending(id)
		d.log.WarnContext(ctx, "mux dispatch aborted while sending prompt",
			"case_id", dc.CaseID,
			"step", dc.Step,
			"dispatch_id", id,
		)
		return nil, fmt.Errorf("mux dispatch aborted: %w", d.getAbortErr())
	}

	// Wait for the routed artifact
	var timeoutCh <-chan time.Time
	if dc.Timeout > 0 {
		timeoutCh = time.After(dc.Timeout)
	}

	select {
	case data, ok := <-responseCh:
		if !ok {
			return nil, fmt.Errorf("mux dispatch aborted: %w", d.getAbortErr())
		}
		latency := time.Since(dispatchStart)
		d.log.InfoContext(ctx, "dispatch round-trip",
			"dispatch_id", id,
			"case_id", dc.CaseID,
			"step", dc.Step,
			"latency_ms", latency.Milliseconds(),
			"artifact_bytes", len(data),
		)
		return data, nil
	case <-timeoutCh:
		d.removePending(id)
		d.log.WarnContext(ctx, "dispatch timeout",
			"dispatch_id", id,
			"case_id", dc.CaseID,
			"step", dc.Step,
			"timeout", dc.Timeout,
		)
		return nil, fmt.Errorf("dispatch timeout after %v for %s/%s", dc.Timeout, dc.CaseID, dc.Step)
	case <-ctx.Done():
		d.removePending(id)
		d.log.WarnContext(ctx, "mux dispatch cancelled while waiting for artifact",
			"case_id", dc.CaseID,
			"step", dc.Step,
			"dispatch_id", id,
		)
		return nil, fmt.Errorf("mux dispatch cancelled: %w", ctx.Err())
	case <-d.ctx.Done():
		d.removePending(id)
		d.log.WarnContext(ctx, "mux dispatch cancelled while waiting for artifact",
			"case_id", dc.CaseID,
			"step", dc.Step,
			"dispatch_id", id,
		)
		return nil, fmt.Errorf("mux dispatch cancelled: %w", d.ctx.Err())
	case <-d.abortCh:
		d.removePending(id)
		return nil, fmt.Errorf("mux dispatch aborted: %w", d.getAbortErr())
	}
}

// GetNextStep blocks until the runner produces the next prompt context.
// Equivalent to GetNextStepWithHints with zero-value hints (FIFO, no preference).
func (d *MuxDispatcher) GetNextStep(ctx context.Context) (agentport.Context, error) {
	return d.GetNextStepWithHints(ctx, agentport.PullHints{})
}

// GetNextStepWithHints blocks until a prompt matching the given hints is
// available, or falls back based on the stickiness level.
//
// Matching priority: PreferredCaseID > PreferredZone > any.
// Stickiness controls fallback: 0=immediate any, 1-2=steal after ConsecutiveMisses
// threshold, 3=exclusive (never steal, wait for match only).
func (d *MuxDispatcher) GetNextStepWithHints(ctx context.Context, hints agentport.PullHints) (agentport.Context, error) {
	hasPreference := hints.PreferredCaseID != "" || hints.PreferredZone != ""

	if !hasPreference {
		dc, err := d.getNextFIFO(ctx)
		if err != nil {
			return dc, err
		}
		d.emitDispatchRouted(dc, "fifo")
		return dc, nil
	}

	// Drain all immediately-available items so we can search the full snapshot.
	d.drainAvailable()

	if dc, ok := d.tryMatchFromQueue(hints); ok {
		d.emitDispatchRouted(dc, "hint_match")
		return dc, nil
	}

	// No match in queue. If stickiness allows stealing, return any queued item.
	if d.shouldSteal(hints) {
		return d.stealNext(ctx, hints)
	}

	// High stickiness: keep pulling items, queue non-matches, wait for a match.
	for {
		dc, err := d.receiveOne(ctx)
		if err != nil {
			return agentport.Context{}, err
		}
		if d.matchesHints(dc, hints) {
			d.emitDispatchRouted(dc, "channel")
			return dc, nil
		}
		d.enqueue(dc)
	}
}

func (d *MuxDispatcher) stealNext(ctx context.Context, hints agentport.PullHints) (agentport.Context, error) {
	if dc, ok := d.dequeueAny(); ok {
		d.emitZoneShift(hints, dc)
		d.emitDispatchRouted(dc, "steal")
		return dc, nil
	}
	// Queue empty — block for the next item and return it regardless.
	dc, err := d.receiveOne(ctx)
	if err != nil {
		return agentport.Context{}, err
	}
	if !d.matchesHints(dc, hints) {
		d.emitZoneShift(hints, dc)
		d.emitDispatchRouted(dc, "steal")
	} else {
		d.emitDispatchRouted(dc, "hint_match")
	}
	return dc, nil
}

func (d *MuxDispatcher) getNextFIFO(ctx context.Context) (agentport.Context, error) {
	if dc, ok := d.dequeueAny(); ok {
		return dc, nil
	}
	return d.receiveOne(ctx)
}

func (d *MuxDispatcher) receiveOne(ctx context.Context) (agentport.Context, error) {
	select {
	case <-ctx.Done():
		return agentport.Context{}, ctx.Err()
	case <-d.ctx.Done():
		return agentport.Context{}, fmt.Errorf("dispatcher shutdown: %w", d.ctx.Err())
	case dc, ok := <-d.promptCh:
		if !ok {
			return agentport.Context{}, fmt.Errorf("dispatcher closed")
		}
		return dc, nil
	}
}

//nolint:gocritic // hugeParam: consistent with Dispatcher interface value type
func (d *MuxDispatcher) matchesHints(dc agentport.Context, hints agentport.PullHints) bool {
	if hints.PreferredCaseID != "" && dc.CaseID == hints.PreferredCaseID {
		return true
	}
	if hints.PreferredZone != "" && dc.Provider == hints.PreferredZone {
		return true
	}
	return false
}

// shouldSteal returns true if the worker's stickiness and miss count allow
// taking a non-matching step (work stealing).
func (d *MuxDispatcher) shouldSteal(hints agentport.PullHints) bool {
	switch hints.Stickiness {
	case 0:
		return true
	case 1:
		return hints.ConsecutiveMisses >= 1
	case 2:
		return hints.ConsecutiveMisses >= 3
	case 3:
		return false
	default:
		return true
	}
}

// drainAvailable moves all immediately-available items from the channel into the queue
// without blocking.
func (d *MuxDispatcher) drainAvailable() {
	for {
		select {
		case dc, ok := <-d.promptCh:
			if !ok {
				return
			}
			d.enqueue(dc)
		default:
			return
		}
	}
}

//nolint:gocritic // hugeParam: consistent with Dispatcher interface value type
func (d *MuxDispatcher) enqueue(dc agentport.Context) {
	d.queueMu.Lock()
	d.queue = append(d.queue, dc)
	d.queueMu.Unlock()
}

func (d *MuxDispatcher) dequeueAny() (agentport.Context, bool) {
	d.queueMu.Lock()
	defer d.queueMu.Unlock()
	if len(d.queue) == 0 {
		return agentport.Context{}, false
	}
	dc := d.queue[0]
	d.queue = d.queue[1:]
	return dc, true
}

//nolint:gocritic // hugeParam: consistent with Dispatcher interface value type
func (d *MuxDispatcher) emitZoneShift(hints agentport.PullHints, dc agentport.Context) {
	if d.bus == nil {
		return
	}
	fromZone := hints.PreferredZone
	if fromZone == "" {
		fromZone = hints.PreferredCaseID
	}
	d.bus.Emit(&agentport.Signal{
		Event:  agentport.EventZoneShift,
		Agent:  agentport.AgentWorker,
		CaseID: dc.CaseID,
		Step:   dc.Step,
		Meta: map[string]string{
			agentport.MetaKeyFromZone: fromZone,
			agentport.MetaKeyToZone:   dc.Provider,
		},
	})
}

//nolint:gocritic // hugeParam: consistent with Dispatcher interface value type
func (d *MuxDispatcher) emitDispatchRouted(dc agentport.Context, reason string) {
	if d.bus == nil {
		return
	}
	d.bus.Emit(&agentport.Signal{
		Event:  agentport.EventDispatchRouted,
		Agent:  agentport.AgentWorker,
		CaseID: dc.CaseID,
		Step:   dc.Step,
		Meta: map[string]string{
			agentport.MetaKeyDispatchReason: reason,
			agentport.MetaKeyQueueDepth:     strconv.Itoa(d.queueLen()),
		},
	})
}

func (d *MuxDispatcher) queueLen() int {
	d.queueMu.Lock()
	defer d.queueMu.Unlock()
	return len(d.queue)
}

func (d *MuxDispatcher) tryMatchFromQueue(hints agentport.PullHints) (agentport.Context, bool) {
	d.queueMu.Lock()
	defer d.queueMu.Unlock()
	for i, dc := range d.queue {
		if d.matchesHints(dc, hints) {
			d.queue = append(d.queue[:i], d.queue[i+1:]...)
			return dc, true
		}
	}
	return agentport.Context{}, false
}

// SubmitArtifact routes the artifact to the Dispatch call with the given ID.
// Implements ExternalDispatcher.
func (d *MuxDispatcher) SubmitArtifact(ctx context.Context, dispatchID int64, data []byte) error {
	d.mu.Lock()
	ch, ok := d.pending[dispatchID]
	if !ok {
		if _, wasClosed := d.closed[dispatchID]; wasClosed {
			d.mu.Unlock()
			d.log.ErrorContext(ctx, "double submit detected",
				"dispatch_id", dispatchID,
			)
			return fmt.Errorf("dispatch_id %d already submitted", dispatchID)
		}
		pendingCount := len(d.pending)
		d.mu.Unlock()
		d.log.WarnContext(ctx, "submit for unknown dispatch ID",
			"dispatch_id", dispatchID,
			"active_dispatches", pendingCount,
		)
		return fmt.Errorf("unknown dispatch_id %d", dispatchID)
	}
	delete(d.pending, dispatchID)
	d.closed[dispatchID] = struct{}{}
	d.mu.Unlock()

	select {
	case ch <- data:
		d.log.DebugContext(ctx, "mux artifact routed",
			"dispatch_id", dispatchID,
			"bytes", len(data),
		)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-d.ctx.Done():
		return fmt.Errorf("dispatcher shutdown: %w", d.ctx.Err())
	}
}

// Abort broadcasts an error to all waiting Dispatch calls.
func (d *MuxDispatcher) Abort(err error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	select {
	case <-d.abortCh:
		return // already aborted
	default:
	}

	d.abortErr = err
	close(d.abortCh)
	d.log.WarnContext(context.Background(), "mux dispatcher abort", "error", err.Error())

	for id, ch := range d.pending {
		close(ch)
		delete(d.pending, id)
	}
}

// ActiveDispatches returns the number of steps dispatched but not yet submitted.
func (d *MuxDispatcher) ActiveDispatches() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.pending)
}

// PromptCh returns the read-only prompt channel (for session integration).
func (d *MuxDispatcher) PromptCh() <-chan agentport.Context {
	return d.promptCh
}

func (d *MuxDispatcher) removePending(id int64) {
	d.mu.Lock()
	delete(d.pending, id)
	d.mu.Unlock()
}

func (d *MuxDispatcher) getAbortErr() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.abortErr != nil {
		return d.abortErr
	}
	return fmt.Errorf("aborted")
}
