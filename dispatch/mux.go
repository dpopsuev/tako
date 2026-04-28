package dispatch

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/dpopsuev/tako/circuit"
	"github.com/dpopsuev/tangle/signal"
)

var (
	_ Dispatcher         = (*MuxDispatcher)(nil)
	_ ExternalDispatcher = (*MuxDispatcher)(nil)
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

	promptCh chan Context
	abortCh  chan struct{}
	abortErr error

	bus signal.Bus // optional; for zone_shift signals

	queueMu sync.Mutex
	queue   []Context // buffered overflow for hint-based matching
}

// MuxOption configures a MuxDispatcher.
type MuxOption func(*MuxDispatcher)

// WithMuxSignalBus attaches a signal.Bus for emitting dispatch-level signals
// (e.g. zone_shift on work stealing).
func WithMuxSignalBus(bus signal.Bus) MuxOption {
	return func(d *MuxDispatcher) { d.bus = bus }
}

// NewMuxDispatcher creates a dispatcher with per-dispatch artifact routing.
// The provided context controls the dispatcher's lifetime.
func NewMuxDispatcher(ctx context.Context, opts ...MuxOption) *MuxDispatcher {
	d := &MuxDispatcher{
		ctx:      ctx,
		log:      slog.Default().With(slog.Any(circuit.LogKeyComponent, circuit.LogComponentMuxDispatch)),
		pending:  make(map[int64]chan []byte),
		closed:   make(map[int64]struct{}),
		promptCh: make(chan Context),
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
func (d *MuxDispatcher) Dispatch(ctx context.Context, dc Context) ([]byte, error) {
	dispatchStart := time.Now()

	d.mu.Lock()
	d.nextID++
	id := d.nextID
	responseCh := make(chan []byte, 1)
	d.pending[id] = responseCh
	pendingCount := len(d.pending)
	d.mu.Unlock()

	dc.DispatchID = id

	d.log.DebugContext(ctx, circuit.LogMuxRegistered, slog.Any(circuit.LogKeyDispatchID, id), slog.Any(circuit.LogKeyCaseID, dc.CaseID), slog.Any(circuit.LogKeyStep, dc.Step), slog.Any(circuit.LogKeyPendingCount, pendingCount))

	// Send prompt to the agent side
	select {
	case d.promptCh <- dc:
	case <-ctx.Done():
		d.removePending(id)
		d.log.WarnContext(ctx, circuit.LogMuxCanceledSending, slog.Any(circuit.LogKeyCaseID, dc.CaseID), slog.Any(circuit.LogKeyStep, dc.Step), slog.Any(circuit.LogKeyDispatchID, id))
		return nil, fmt.Errorf("mux dispatch canceled: %w", ctx.Err())
	case <-d.ctx.Done():
		d.removePending(id)
		d.log.WarnContext(ctx, circuit.LogMuxCanceledSending, slog.Any(circuit.LogKeyCaseID, dc.CaseID), slog.Any(circuit.LogKeyStep, dc.Step), slog.Any(circuit.LogKeyDispatchID, id))
		return nil, fmt.Errorf("mux dispatch canceled: %w", d.ctx.Err())
	case <-d.abortCh:
		d.removePending(id)
		d.log.WarnContext(ctx, circuit.LogMuxAbortedSending, slog.Any(circuit.LogKeyCaseID, dc.CaseID), slog.Any(circuit.LogKeyStep, dc.Step), slog.Any(circuit.LogKeyDispatchID, id))
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
		d.log.InfoContext(ctx, circuit.LogDispatchRoundTrip, slog.Any(circuit.LogKeyDispatchID, id), slog.Any(circuit.LogKeyCaseID, dc.CaseID), slog.Any(circuit.LogKeyStep, dc.Step), slog.Any(circuit.LogKeyLatency, latency.Milliseconds()), slog.Any(circuit.LogKeyArtifactBytes, len(data)))
		return data, nil
	case <-timeoutCh:
		d.removePending(id)
		d.log.WarnContext(ctx, circuit.LogDispatchTimeout, slog.Any(circuit.LogKeyDispatchID, id), slog.Any(circuit.LogKeyCaseID, dc.CaseID), slog.Any(circuit.LogKeyStep, dc.Step), slog.Any(circuit.LogKeyTimeout, dc.Timeout))
		return nil, fmt.Errorf("%w: %v for %s/%s", ErrDispatchTimeoutAfter, dc.Timeout, dc.CaseID, dc.Step)
	case <-ctx.Done():
		d.removePending(id)
		d.log.WarnContext(ctx, circuit.LogMuxCanceledWaiting, slog.Any(circuit.LogKeyCaseID, dc.CaseID), slog.Any(circuit.LogKeyStep, dc.Step), slog.Any(circuit.LogKeyDispatchID, id))
		return nil, fmt.Errorf("mux dispatch canceled: %w", ctx.Err())
	case <-d.ctx.Done():
		d.removePending(id)
		d.log.WarnContext(ctx, circuit.LogMuxCanceledWaiting, slog.Any(circuit.LogKeyCaseID, dc.CaseID), slog.Any(circuit.LogKeyStep, dc.Step), slog.Any(circuit.LogKeyDispatchID, id))
		return nil, fmt.Errorf("mux dispatch canceled: %w", d.ctx.Err())
	case <-d.abortCh:
		d.removePending(id)
		return nil, fmt.Errorf("mux dispatch aborted: %w", d.getAbortErr())
	}
}

// GetNextStep blocks until the runner produces the next prompt context.
// Equivalent to GetNextStepWithHints with zero-value hints (FIFO, no preference).
func (d *MuxDispatcher) GetNextStep(ctx context.Context) (Context, error) {
	return d.GetNextStepWithHints(ctx, PullHints{})
}

// GetNextStepWithHints blocks until a prompt matching the given hints is
// available, or falls back based on the stickiness level.
//
// Matching priority: PreferredCaseID > PreferredZone > any.
// Stickiness controls fallback: 0=immediate any, 1-2=steal after ConsecutiveMisses
// threshold, 3=exclusive (never steal, wait for match only).
func (d *MuxDispatcher) GetNextStepWithHints(ctx context.Context, hints PullHints) (Context, error) {
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
			return Context{}, err
		}
		if d.matchesHints(dc, hints) {
			d.emitDispatchRouted(dc, "channel")
			return dc, nil
		}
		d.enqueue(dc)
	}
}

func (d *MuxDispatcher) stealNext(ctx context.Context, hints PullHints) (Context, error) {
	if dc, ok := d.dequeueAny(); ok {
		d.emitZoneShift(hints, dc)
		d.emitDispatchRouted(dc, "steal")
		return dc, nil
	}
	// Queue empty — block for the next item and return it regardless.
	dc, err := d.receiveOne(ctx)
	if err != nil {
		return Context{}, err
	}
	if !d.matchesHints(dc, hints) {
		d.emitZoneShift(hints, dc)
		d.emitDispatchRouted(dc, "steal")
	} else {
		d.emitDispatchRouted(dc, "hint_match")
	}
	return dc, nil
}

func (d *MuxDispatcher) getNextFIFO(ctx context.Context) (Context, error) {
	if dc, ok := d.dequeueAny(); ok {
		return dc, nil
	}
	return d.receiveOne(ctx)
}

func (d *MuxDispatcher) receiveOne(ctx context.Context) (Context, error) {
	select {
	case <-ctx.Done():
		return Context{}, ctx.Err()
	case <-d.ctx.Done():
		return Context{}, fmt.Errorf("dispatcher shutdown: %w", d.ctx.Err())
	case dc, ok := <-d.promptCh:
		if !ok {
			return Context{}, ErrDispatcherClosed
		}
		return dc, nil
	}
}

func (d *MuxDispatcher) matchesHints(dc Context, hints PullHints) bool {
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
func (d *MuxDispatcher) shouldSteal(hints PullHints) bool {
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

func (d *MuxDispatcher) enqueue(dc Context) {
	d.queueMu.Lock()
	d.queue = append(d.queue, dc)
	d.queueMu.Unlock()
}

func (d *MuxDispatcher) dequeueAny() (Context, bool) {
	d.queueMu.Lock()
	defer d.queueMu.Unlock()
	if len(d.queue) == 0 {
		return Context{}, false
	}
	dc := d.queue[0]
	d.queue = d.queue[1:]
	return dc, true
}

func (d *MuxDispatcher) emitZoneShift(hints PullHints, dc Context) {
	if d.bus == nil {
		return
	}
	fromZone := hints.PreferredZone
	if fromZone == "" {
		fromZone = hints.PreferredCaseID
	}
	d.bus.Emit(&signal.Signal{
		Event:  signal.EventZoneShift,
		Agent:  signal.AgentWorker,
		CaseID: dc.CaseID,
		Step:   dc.Step,
		Meta: map[string]string{
			signal.MetaKeyFromZone: fromZone,
			signal.MetaKeyToZone:   dc.Provider,
		},
	})
}

func (d *MuxDispatcher) emitDispatchRouted(dc Context, reason string) {
	if d.bus == nil {
		return
	}
	d.bus.Emit(&signal.Signal{
		Event:  signal.EventDispatchRouted,
		Agent:  signal.AgentWorker,
		CaseID: dc.CaseID,
		Step:   dc.Step,
		Meta: map[string]string{
			signal.MetaKeyDispatchReason: reason,
			signal.MetaKeyQueueDepth:     strconv.Itoa(d.queueLen()),
		},
	})
}

func (d *MuxDispatcher) queueLen() int {
	d.queueMu.Lock()
	defer d.queueMu.Unlock()
	return len(d.queue)
}

func (d *MuxDispatcher) tryMatchFromQueue(hints PullHints) (Context, bool) {
	d.queueMu.Lock()
	defer d.queueMu.Unlock()
	for i, dc := range d.queue {
		if d.matchesHints(dc, hints) {
			d.queue = append(d.queue[:i], d.queue[i+1:]...)
			return dc, true
		}
	}
	return Context{}, false
}

// SubmitArtifact routes the artifact to the Dispatch call with the given ID.
// Implements ExternalDispatcher.
func (d *MuxDispatcher) SubmitArtifact(ctx context.Context, dispatchID int64, data []byte) error {
	d.mu.Lock()
	ch, ok := d.pending[dispatchID]
	if !ok {
		if _, wasClosed := d.closed[dispatchID]; wasClosed {
			d.mu.Unlock()
			d.log.ErrorContext(ctx, circuit.LogDoubleSubmit, slog.Any(circuit.LogKeyDispatchID, dispatchID))
			return fmt.Errorf("%w: %d already submitted", ErrDispatchId, dispatchID)
		}
		pendingCount := len(d.pending)
		d.mu.Unlock()
		d.log.WarnContext(ctx, circuit.LogSubmitUnknownDispatch, slog.Any(circuit.LogKeyDispatchID, dispatchID), slog.Any(circuit.LogKeyActiveDispatches, pendingCount))
		return fmt.Errorf("%w: %d", ErrUnknownDispatchId, dispatchID)
	}
	delete(d.pending, dispatchID)
	d.closed[dispatchID] = struct{}{}
	d.mu.Unlock()

	select {
	case ch <- data:
		d.log.DebugContext(ctx, circuit.LogMuxArtifactRouted, slog.Any(circuit.LogKeyDispatchID, dispatchID), slog.Any(circuit.LogKeyBytes, len(data)))
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
	d.log.WarnContext(context.Background(), circuit.LogMuxAbort, slog.Any(circuit.LogKeyError, err.Error()))

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
func (d *MuxDispatcher) PromptCh() <-chan Context {
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
	return ErrAborted
}
