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
	c.Add(m, mkAtom("understand-"+domain, KnowledgeAtom, "understanding.synth."+domain, Fresh))
}

func addFormationAtoms(c *Core, m *Molecule, domain string) {
	c.Add(m, mkAtom("plan-"+domain, ExpansionAtom, "plan.task."+domain, Fresh))
	c.Add(m, mkAtom("risk-"+domain, ReductionAtom, "risk.eval."+domain, Fresh))
	c.Add(m, mkAtom("strategy-"+domain, SelectionAtom, "strategy.synth."+domain, Fresh))
}

func addActionAtoms(c *Core, m *Molecule, domain string) {
	c.Add(m, mkAtom("exec-"+domain, ExecutionAtom, "execution.result."+domain, Fresh))
	c.Add(m, mkAtom("observe-"+domain, AcclimationAtom, "observation.eval."+domain, Fresh))
	c.Add(m, mkAtom("adapt-"+domain, RefinementAtom, "adaptation.synth."+domain, Fresh))
}

func addFullChain(c *Core, m *Molecule, domain string) {
	addReasonAtoms(c, m, domain)
	addFormationAtoms(c, m, domain)
	addActionAtoms(c, m, domain)
	c.Add(m, mkAtom("retro-"+domain, RetrospectionAtom, "retrospection.reflect."+domain, Fresh))
}
