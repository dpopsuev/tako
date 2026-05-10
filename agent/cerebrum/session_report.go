package cerebrum

import (
	"encoding/json"

	"github.com/dpopsuev/tako/agent/reactivity"
)

type ChainEventRecord struct {
	Kind       string `json:"kind"`
	Organ      string `json:"organ"`
	Output     string `json:"output"`
	IsResponse bool   `json:"is_response"`
}

type SessionReport struct {
	SessionSummary
	ChainEvents []ChainEventRecord `json:"chain_events"`
	ToolCalls   []ToolEvent        `json:"tool_calls"`
	Responses   []string           `json:"responses"`
	Errors      []string           `json:"errors,omitempty"`
	Pressure    float64            `json:"pressure"`
}

func buildSessionReport(summary SessionSummary, m *reactivity.Molecule, log *EventLog) SessionReport {
	chain := m.Chain()
	var chainEvents []ChainEventRecord
	for _, e := range chain.All() {
		out := string(e.Output)
		if len(out) > 500 {
			out = out[:500] + "..."
		}
		chainEvents = append(chainEvents, ChainEventRecord{
			Kind:       e.Kind.String(),
			Organ:      e.Organ,
			Output:     out,
			IsResponse: e.IsResponse,
		})
	}

	report := SessionReport{
		SessionSummary: summary,
		ChainEvents:    chainEvents,
		Pressure:       m.Pressure(),
	}

	if log != nil {
		log.mu.Lock()
		report.ToolCalls = append([]ToolEvent(nil), log.ToolCalls...)
		report.Responses = append([]string(nil), log.Responses...)
		report.Errors = append([]string(nil), log.Errors...)
		log.mu.Unlock()
	}

	return report
}

func (r SessionReport) JSON() string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data)
}
