package cerebrum

import (
	"time"

	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tako/ergograph"
	"github.com/dpopsuev/tako/service/andon"
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

func WithTools(tools []tangle.Tool) Option {
	return func(cb *Cerebrum) { cb.toolDefs = tools }
}

func WithRouter(r CompleterRouter) Option {
	return func(cb *Cerebrum) { cb.router = r }
}

func WithAssert(a reactivity.Assert) Option {
	return func(cb *Cerebrum) { cb.assert = a }
}

func WithPool(pool ergograph.Ledger) Option {
	return func(cb *Cerebrum) { cb.pool = pool }
}

func WithAndon(signal andon.Signal) Option {
	return func(cb *Cerebrum) { cb.andon = signal }
}
