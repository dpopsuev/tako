package engine

// Category: Execution — narration observer.

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/core"
)

// NarrationSink receives a single human-readable narration line.
type NarrationSink func(line string)

// NarrationOption configures a NarrationObserver.
type NarrationOption func(*NarrationObserver)

// WithVocabulary sets the vocabulary for translating node/edge names.
func WithVocabulary(v circuit.Vocabulary) NarrationOption {
	return func(n *NarrationObserver) { n.vocab = v }
}

// WithSink sets the output destination for narration lines.
func WithSink(s NarrationSink) NarrationOption {
	return func(n *NarrationObserver) { n.sink = s }
}

// WithMilestoneInterval sets how often milestone summaries are emitted.
// A value of 0 disables milestones. Default is 5.
func WithMilestoneInterval(every int) NarrationOption {
	return func(n *NarrationObserver) { n.milestoneEvery = every }
}

// WithETA enables or disables ETA estimation in narration output.
func WithETA(enabled bool) NarrationOption {
	return func(n *NarrationObserver) { n.showETA = enabled }
}

// Progress captures a snapshot of walk progress.
type Progress struct {
	NodesVisited int
	Elapsed      time.Duration
	CurrentNode  string
	LastWalker   string
}

// NarrationObserver is a WalkObserver that produces human-readable narration
// lines from walk events. It translates node names via a Vocabulary, tracks
// progress, computes ETA, and emits milestone summaries.
//
// Zero-config: NewNarrationObserver() with no options logs to slog.Info.
type NarrationObserver struct {
	mu             sync.Mutex
	vocab          circuit.Vocabulary
	sink           NarrationSink
	milestoneEvery int
	showETA        bool

	walkStart    time.Time
	nodesVisited int
	currentNode  string
	lastWalker   string
	errors       int
}

// NewNarrationObserver creates a narration observer with sensible defaults.
// Pass NarrationOption values to customize vocabulary, sink, etc.
func NewNarrationObserver(opts ...NarrationOption) *NarrationObserver {
	n := &NarrationObserver{
		vocab:          circuit.VocabularyFunc(func(code string) string { return code }),
		sink:           func(line string) { slog.Info(line) },
		milestoneEvery: 5,
		showETA:        true,
	}
	for _, opt := range opts {
		opt(n)
	}
	return n
}

// Progress returns a snapshot of current walk progress.
func (n *NarrationObserver) Progress() Progress {
	n.mu.Lock()
	defer n.mu.Unlock()
	elapsed := time.Duration(0)
	if !n.walkStart.IsZero() {
		elapsed = time.Since(n.walkStart)
	}
	return Progress{
		NodesVisited: n.nodesVisited,
		Elapsed:      elapsed,
		CurrentNode:  n.currentNode,
		LastWalker:   n.lastWalker,
	}
}

// OnEvent implements WalkObserver.
func (n *NarrationObserver) OnEvent(e core.WalkEvent) {
	n.mu.Lock()
	defer n.mu.Unlock()

	switch e.Type {
	case core.EventNodeEnter:
		if n.walkStart.IsZero() {
			n.walkStart = time.Now()
		}
		n.currentNode = e.Node
		if e.Walker != "" {
			n.lastWalker = e.Walker
		}
		name := n.vocab.Name(e.Node)
		if e.Walker != "" {
			n.emit(fmt.Sprintf("[%s] Entering %s", e.Walker, name))
		} else {
			n.emit(fmt.Sprintf("Entering %s", name))
		}

	case core.EventNodeExit:
		n.nodesVisited++
		name := n.vocab.Name(e.Node)
		if e.Error != nil {
			n.errors++
			n.emit(fmt.Sprintf("Failed at %s: %v", name, e.Error))
		} else if e.Elapsed > 0 {
			n.emit(fmt.Sprintf("Completed %s (%s)", name, FmtNarrateDuration(e.Elapsed)))
		} else {
			n.emit(fmt.Sprintf("Completed %s", name))
		}
		if n.milestoneEvery > 0 && n.nodesVisited%n.milestoneEvery == 0 {
			n.emitMilestone()
		}

	case core.EventWalkerSwitch:
		n.lastWalker = e.Walker
		name := n.vocab.Name(e.Node)
		n.emit(fmt.Sprintf("Handing off to %s at %s", e.Walker, name))

	case core.EventTransition:
		// silent by default; transitions are high-frequency noise

	case core.EventEdgeEvaluate:
		// silent by default

	case core.EventWalkComplete:
		elapsed := time.Since(n.walkStart)
		n.emit(fmt.Sprintf("Walk complete — %d nodes visited in %s",
			n.nodesVisited, FmtNarrateDuration(elapsed)))

	case core.EventWalkError:
		n.errors++
		node := e.Node
		if node != "" {
			node = n.vocab.Name(node)
		}
		if node != "" {
			n.emit(fmt.Sprintf("Walk failed at %s: %v", node, e.Error))
		} else {
			n.emit(fmt.Sprintf("Walk failed: %v", e.Error))
		}
	}
}

func (n *NarrationObserver) emit(line string) {
	n.sink(line)
}

func (n *NarrationObserver) emitMilestone() {
	elapsed := time.Since(n.walkStart)
	line := fmt.Sprintf("--- progress: %d nodes visited | Elapsed: %s",
		n.nodesVisited, FmtNarrateDuration(elapsed))
	if n.showETA && n.nodesVisited > 0 {
		avgPerNode := elapsed / time.Duration(n.nodesVisited)
		line += fmt.Sprintf(" | Avg: %s/node", FmtNarrateDuration(avgPerNode))
	}
	if n.errors > 0 {
		line += fmt.Sprintf(" | Errors: %d", n.errors)
	}
	line += " ---"
	n.emit(line)
}

// FmtNarrateDuration formats a duration for narration output.
func FmtNarrateDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	s := d.Seconds()
	if s < 60 {
		return fmt.Sprintf("%.1fs", s)
	}
	m := int(s) / 60
	sec := int(s) % 60
	return fmt.Sprintf("%dm%ds", m, sec)
}
