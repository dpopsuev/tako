package cerebrum

import (
	"time"

	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tako/agent/organ"
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

func WithSynapse(s Synapse) Option {
	return func(cb *Cerebrum) { cb.synapse = s }
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

func WithOrgans(caps []organ.Func) Option {
	return func(cb *Cerebrum) { cb.organs = caps }
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

type SightProvider func() CellSight

func WithContextListener(l ContextListener) Option {
	return func(cb *Cerebrum) { cb.listener = l }
}

func WithSight(s SightProvider) Option {
	return func(cb *Cerebrum) { cb.sight = s }
}

func WithEmbedder(e Embedder) Option {
	return func(cb *Cerebrum) { cb.embedder = e }
}

func WithReflexStore(s ReflexStore) Option {
	return func(cb *Cerebrum) { cb.reflexStore = s }
}

func WithConsolidator(c Consolidator) Option {
	return func(cb *Cerebrum) { cb.consolidator = c }
}

func WithSealStrategy(s SealStrategy) Option {
	return func(cb *Cerebrum) { cb.sealStrategy = s }
}

func WithInstigator(ins *Instigator) Option {
	return func(cb *Cerebrum) { cb.instigator = ins }
}
