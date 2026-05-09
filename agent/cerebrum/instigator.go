package cerebrum

import (
	"fmt"
	"log/slog"

	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
)

type InstigatorContext struct {
	HasDesired      bool    `expr:"HasDesired"`
	Distance        float64 `expr:"Distance"`
	MassIntent      int     `expr:"MassIntent"`
	MassAssessment  int     `expr:"MassAssessment"`
	MassKnowledge   int     `expr:"MassKnowledge"`
	MassExecution   int     `expr:"MassExecution"`
	SenseCount      int     `expr:"SenseCount"`
	MotorCount      int     `expr:"MotorCount"`
	HasVerification bool    `expr:"HasVerification"`
	TurnCount       int     `expr:"TurnCount"`
}

func BuildInstigatorContext(m *reactivity.Molecule, chain *reactivity.EventChain) InstigatorContext {
	hasCatalyst := m.Catalyst() != nil && len(m.Catalyst().Desired) > 0
	return InstigatorContext{
		HasDesired:      hasCatalyst,
		Distance:        m.Distance(),
		MassIntent:      m.Mass(reactivity.IntentAtom),
		MassAssessment:  m.Mass(reactivity.AssessmentAtom),
		MassKnowledge:   m.Mass(reactivity.KnowledgeAtom),
		MassExecution:   m.Mass(reactivity.ExecutionAtom),
		SenseCount:      len(chain.Senses()),
		MotorCount:      len(chain.Motors()),
		HasVerification: chain.HasSenseAfterMotor(),
		TurnCount:       m.Turns(),
	}
}

type TriadContract struct {
	From      string
	To        string
	predicate *vm.Program
	expr      string
}

type Instigator struct {
	contracts []TriadContract
}

var DefaultContracts = []struct {
	From, To, Predicate string
}{
	{"think", "reflect", "!HasDesired"},
	{"think", "implement", "HasDesired && MassKnowledge > 0"},
	{"think", "compose", "SenseCount > 0 && HasDesired"},
	{"compose", "implement", "MassAssessment > 0"},
	{"implement", "reflect", "HasVerification && Distance <= 0"},
}

func NewInstigator(contracts []struct{ From, To, Predicate string }) (*Instigator, error) {
	if contracts == nil {
		contracts = DefaultContracts
	}

	compiled := make([]TriadContract, 0, len(contracts))
	for _, c := range contracts {
		program, err := expr.Compile(c.Predicate,
			expr.Env(InstigatorContext{}),
			expr.AsBool(),
		)
		if err != nil {
			return nil, fmt.Errorf("instigator: compile %q: %w", c.Predicate, err)
		}
		compiled = append(compiled, TriadContract{
			From:      c.From,
			To:        c.To,
			predicate: program,
			expr:      c.Predicate,
		})
	}
	return &Instigator{contracts: compiled}, nil
}

func MustInstigator(contracts []struct{ From, To, Predicate string }) *Instigator {
	ins, err := NewInstigator(contracts)
	if err != nil {
		panic(err)
	}
	return ins
}

var triadNames = map[reactivity.Triad]string{
	reactivity.ThinkTriad:     "think",
	reactivity.ComposeTriad:   "compose",
	reactivity.ImplementTriad: "implement",
	reactivity.ReflectTriad:   "reflect",
}

var nameToTriad = map[string]reactivity.Triad{
	"think":     reactivity.ThinkTriad,
	"compose":   reactivity.ComposeTriad,
	"implement": reactivity.ImplementTriad,
	"reflect":   reactivity.ReflectTriad,
}

func (ins *Instigator) NextTriad(currentTriad reactivity.Triad, ictx InstigatorContext) reactivity.Triad {
	current := triadNames[currentTriad]

	for _, c := range ins.contracts {
		if c.From != current {
			continue
		}
		result, err := expr.Run(c.predicate, ictx)
		if err != nil {
			slog.Warn("instigator.eval_error",
				slog.String("from", c.From),
				slog.String("to", c.To),
				slog.String("expr", c.expr),
				slog.Any("error", err))
			continue
		}
		if matched, ok := result.(bool); ok && matched {
			if target, ok := nameToTriad[c.To]; ok {
				slog.Info("instigator.transition",
					slog.String("from", c.From),
					slog.String("to", c.To),
					slog.String("predicate", c.expr))
				return target
			}
		}
	}

	return currentTriad
}
