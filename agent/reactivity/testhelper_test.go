package reactivity

import "time"

func mkAtom(id string, t AtomType, taxonomy string, source AtomSource, targets ...string) Atom {
	return Atom{
		ID:        id,
		Type:      t,
		Source:    source,
		Taxonomy:  taxonomy,
		Content:   []byte(id),
		Targets:   targets,
		CreatedAt: time.Now(),
	}
}

func addReasonAtoms(c *Core, m *Molecule, domain string) {
	c.Add(m, mkAtom("intent-"+domain, IntentAtom, "intent.goal."+domain, Fresh))
	c.Add(m, mkAtom("assess-"+domain, AssessmentAtom, "assessment.eval."+domain, Fresh))
	c.Add(m, mkAtom("understand-"+domain, UnderstandingAtom, "understanding.synth."+domain, Fresh))
}

func addFormationAtoms(c *Core, m *Molecule, domain string) {
	c.Add(m, mkAtom("plan-"+domain, PlanAtom, "plan.task."+domain, Fresh))
	c.Add(m, mkAtom("risk-"+domain, RiskAtom, "risk.eval."+domain, Fresh))
	c.Add(m, mkAtom("strategy-"+domain, StrategyAtom, "strategy.synth."+domain, Fresh))
}

func addActionAtoms(c *Core, m *Molecule, domain string) {
	c.Add(m, mkAtom("exec-"+domain, ExecutionAtom, "execution.result."+domain, Fresh))
	c.Add(m, mkAtom("observe-"+domain, ObservationAtom, "observation.eval."+domain, Fresh))
	c.Add(m, mkAtom("adapt-"+domain, AdaptationAtom, "adaptation.synth."+domain, Fresh))
}

func addFullChain(c *Core, m *Molecule, domain string) {
	addReasonAtoms(c, m, domain)
	addFormationAtoms(c, m, domain)
	addActionAtoms(c, m, domain)
	c.Add(m, mkAtom("retro-"+domain, RetrospectionAtom, "retrospection.reflect."+domain, Fresh))
}
