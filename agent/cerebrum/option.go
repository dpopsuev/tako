package cerebrum

import (
	"time"

	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tako/agent/shell"
	tangle "github.com/dpopsuev/tangle"
)

type Option func(*Cerebrum)

func WithSensory(b Bus) Option {
	return func(cb *Cerebrum) { cb.sensory = b }
}

func WithMotor(b Bus) Option {
	return func(cb *Cerebrum) { cb.motor = b }
}

func WithSignal(b Bus) Option {
	return func(cb *Cerebrum) { cb.signal = b }
}

func WithBudget(b Budget) Option {
	return func(cb *Cerebrum) { cb.budget = b }
}

func WithMaxTurns(n int) Option {
	return func(cb *Cerebrum) { cb.budget.MaxTurns = n }
}

func WithTurnTimeout(d time.Duration) Option {
	return func(cb *Cerebrum) { cb.budget.TurnTimeout = d }
}

func WithMaxTokens(n int) Option {
	return func(cb *Cerebrum) { cb.budget.MaxTokens = n }
}

func WithClassifier(c Classifier) Option {
	return func(cb *Cerebrum) { cb.classifier = c }
}

func WithPromptBuilder(p PromptBuilder) Option {
	return func(cb *Cerebrum) { cb.promptBuilder = p }
}

func WithParser(p ResponseParser) Option {
	return func(cb *Cerebrum) { cb.parser = p }
}

func WithSynapse(s Synapse) Option {
	return func(cb *Cerebrum) { cb.synapse = s }
}

// Deprecated: use WithCapabilities instead. WithTools sets raw tool definitions
// without Mode/Risk metadata. Phase-filtered tools require WithCapabilities.
func WithTools(tools []tangle.Tool) Option {
	return func(cb *Cerebrum) { cb.toolDefs = tools }
}

func WithRouter(r CompleterRouter) Option {
	return func(cb *Cerebrum) { cb.router = r }
}

func WithAssert(a reactivity.Assert) Option {
	return func(cb *Cerebrum) { cb.assert = a }
}

func WithRecollector(r Recollector) Option {
	return func(cb *Cerebrum) { cb.recollector = r }
}

func WithRecorder(r Recorder) Option {
	return func(cb *Cerebrum) { cb.recorder = r }
}

func WithHalter(h Halter) Option {
	return func(cb *Cerebrum) { cb.halter = h }
}

func WithCompactor(c Compactor) Option {
	return func(cb *Cerebrum) { cb.compactor = c }
}

func WithObserver(o Observer) Option {
	return func(cb *Cerebrum) { cb.observer = o }
}

func WithRegulator(r Regulator) Option {
	return func(cb *Cerebrum) { cb.regulator = r }
}

func WithAssembler(a Assembler) Option {
	return func(cb *Cerebrum) { cb.assembler = a }
}

func WithCapabilities(caps []shell.Capability) Option {
	return func(cb *Cerebrum) { cb.capabilities = caps }
}

func WithConfig(cfg *reactivity.Config) Option {
	return func(cb *Cerebrum) { cb.config = cfg }
}

func WithPriorityClassifier(pc PriorityClassifier) Option {
	return func(cb *Cerebrum) { cb.priorityClassifier = pc }
}

func WithWatcher(w tangle.Completer) Option {
	return func(cb *Cerebrum) { cb.watcher = w }
}

func WithAlignmentChecker(a AlignmentChecker) Option {
	return func(cb *Cerebrum) { cb.alignment = a }
}
