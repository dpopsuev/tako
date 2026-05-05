package rehearsal

type Rule struct {
	On     string
	Weight int
}

type Scorecard struct {
	Name      string
	Threshold int
	Rules     []Rule
}

func ReadOnlyRules() []Rule {
	return []Rule{
		{On: "text", Weight: 10},
		{On: "done", Weight: 10},
		{On: "tool.write", Weight: -15},
		{On: "tool.edit", Weight: -15},
		{On: "error", Weight: -20},
	}
}

func WriteRules() []Rule {
	return []Rule{
		{On: "text", Weight: 5},
		{On: "done", Weight: 10},
		{On: "tool.write", Weight: 15},
		{On: "tool.edit", Weight: 10},
		{On: "error", Weight: -20},
	}
}

func BaseRules(t Template) []Rule {
	switch t {
	case Write, MultiTurn:
		return WriteRules()
	default:
		return ReadOnlyRules()
	}
}

func BuildScorecard(r Rehearsal) Scorecard {
	base := BaseRules(r.Template)
	rules := make([]Rule, 0, len(base)+len(r.ExtraRules))
	rules = append(rules, base...)
	rules = append(rules, r.ExtraRules...)
	return Scorecard{
		Name:      r.Name,
		Threshold: 20,
		Rules:     rules,
	}
}

func (sc Scorecard) Score(events []string) (int, bool) {
	total := 0
	for _, event := range events {
		for _, rule := range sc.Rules {
			if rule.On == event {
				total += rule.Weight
			}
		}
	}
	return total, total >= sc.Threshold
}
