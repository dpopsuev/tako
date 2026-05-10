package cerebrum

import (
	"fmt"
	"log/slog"

	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tako/agent/organ"
)

type Observer func() map[string]any

type RawContext struct {
	Need         []byte
	Observer     Observer
	Molecule     *reactivity.Molecule
	Organs []organ.Func
	Domain       Domain
	Contracts    []reactivity.ContractInfo
	Directives   []reactivity.Directive
	Config       *reactivity.Config
	Turn         int
	Sight        CellSight
}

type Context struct {
	Need          string
	State         map[string]any
	StateChanges  map[string][2]any
	Desired       map[string]any
	Residual      map[string]float64
	Organs        []organ.Func
	Phase         reactivity.AtomType
	Domain        Domain
	Contracts     []reactivity.ContractInfo
	Directives    []reactivity.Directive
	Filled        map[string]string
	Distance      float64
	DeltaDistance  float64
	Turn          int
	StagnantTurns int
	Sight         CellSight
}

type Regulator interface {
	Regulate(raw RawContext) Context
}

func defaultRegulate(raw RawContext) Context {
	m := raw.Molecule

	var state map[string]any
	if raw.Observer != nil {
		state = raw.Observer()
	}

	var desired map[string]any
	if m.Catalyst() != nil {
		desired = m.Catalyst().Desired
	}

	summaryMax := reactivity.DefaultConfig.ContractSummaryMax
	if raw.Config != nil {
		summaryMax = raw.Config.ContractSummaryMax
	}

	filled := make(map[string]string, len(raw.Contracts))
	for _, c := range raw.Contracts {
		if m.Mass(c.Phase) > 0 {
			if synth := m.Synthesis(c.Phase.Triad); synth != nil {
				filled[c.Phase.String()] = truncate(string(synth.Content), summaryMax)
			} else {
				atoms := m.Atoms(c.Phase)
				if len(atoms) > 0 {
					filled[c.Phase.String()] = truncate(string(atoms[len(atoms)-1].Content), summaryMax)
				}
			}
		}
	}

	return Context{
		Need:         string(raw.Need),
		State:        state,
		Desired:      desired,
		Residual:     m.Residual(),
		Organs: raw.Organs,
		Phase:        m.Phase(),
		Domain:       raw.Domain,
		Contracts:    raw.Contracts,
		Directives:   raw.Directives,
		Filled:       filled,
		Distance:     m.Distance(),
		DeltaDistance: m.DeltaDistance(),
		Turn:         raw.Turn,
		Sight:        raw.Sight,
	}
}

type DeltaRegulator struct {
	prevState    map[string]any
	prevResidual map[string]float64
	stagnant     int
}

func (d *DeltaRegulator) Regulate(raw RawContext) Context {
	ctx := defaultRegulate(raw)

	if d.prevState != nil && ctx.State != nil {
		changes := make(map[string][2]any)
		for k, v := range ctx.State {
			if prev, ok := d.prevState[k]; ok && fmt.Sprintf("%v", prev) != fmt.Sprintf("%v", v) {
				changes[k] = [2]any{prev, v}
			}
		}
		if len(changes) > 0 {
			ctx.StateChanges = changes
			ctx.State = nil
		}
	}

	if d.prevResidual != nil && ctx.Residual != nil {
		same := true
		changed := 0
		for k, v := range ctx.Residual {
			if d.prevResidual[k] != v {
				same = false
				changed++
			}
		}
		if same {
			d.stagnant++
		} else {
			d.stagnant = 0
		}
		ctx.StagnantTurns = d.stagnant

		if d.stagnant >= 2 {
			slog.Warn("regulator.stagnant_residual",
				slog.Int("stagnant_turns", d.stagnant),
				slog.Int("turn", ctx.Turn),
				slog.Float64("distance", ctx.Distance))
		}

		if ctx.DeltaDistance > 0 {
			slog.Warn("regulator.distance_regressing",
				slog.Float64("delta", ctx.DeltaDistance),
				slog.Int("turn", ctx.Turn))
		}

		slog.Info("regulator.delta",
			slog.Int("turn", ctx.Turn),
			slog.Int("dimensions_changed", changed),
			slog.Int("state_changes", len(ctx.StateChanges)),
			slog.Float64("distance", ctx.Distance),
			slog.Float64("delta_distance", ctx.DeltaDistance))
	}

	d.prevState = ctx.State
	if ctx.StateChanges != nil && d.prevState == nil {
		d.prevState = make(map[string]any)
		if raw.Observer != nil {
			d.prevState = raw.Observer()
		}
	}
	d.prevResidual = ctx.Residual

	return ctx
}
