package cerebrum

import (
	"strings"

	"github.com/dpopsuev/tako/agent/reactivity"
)

// TaskCard is the minimal representation of a work item.
// Populated from Scribe, Kanban, or any external work graph.
type TaskCard struct {
	ID       string
	Title    string
	Goal     string
	Sections map[string]string
	Priority string // critical, high, medium, low
	Labels   []string
}

// CatalystFromTask converts a TaskCard into a Catalyst.
// Need = title + goal + relevant sections.
// Trust = priority mapping (critical=0.3, high=0.5, medium=0.7, low=0.9).
// Criteria = derived from acceptance section or defaults to {tests_pass: true, build_clean: true}.
func CatalystFromTask(card TaskCard) reactivity.Catalyst {
	parts := []string{card.Title}
	if card.Goal != "" {
		parts = append(parts, card.Goal)
	}
	for _, name := range []string{"context", "acceptance", "checklist"} {
		if text, ok := card.Sections[name]; ok {
			parts = append(parts, "## "+name+"\n"+text)
		}
	}

	return reactivity.Catalyst{
		Need:    strings.Join(parts, "\n\n"),
		Desired: criteriaFromCard(card),
		Trust:   trustFromPriority(card.Priority),
	}
}

func trustFromPriority(priority string) float64 {
	switch strings.ToLower(priority) {
	case "critical":
		return 0.3
	case "high":
		return 0.5
	case "medium":
		return 0.7
	case "low":
		return 0.9
	default:
		return 0.5
	}
}

func criteriaFromCard(card TaskCard) map[string]any {
	if acc, ok := card.Sections["acceptance"]; ok {
		criteria := make(map[string]any)
		for _, line := range strings.Split(acc, "\n") {
			line = strings.TrimSpace(line)
			line = strings.TrimPrefix(line, "- [ ] ")
			line = strings.TrimPrefix(line, "- ")
			if line == "" {
				continue
			}
			key := strings.ToLower(strings.ReplaceAll(line, " ", "_"))
			criteria[key] = true
		}
		if len(criteria) > 0 {
			return criteria
		}
	}
	return map[string]any{
		"tests_pass":  true,
		"build_clean": true,
	}
}
