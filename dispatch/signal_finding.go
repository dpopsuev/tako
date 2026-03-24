package dispatch

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/dpopsuev/bugle/signal"
	"github.com/dpopsuev/origami/circuit"
)

const findingEventPrefix = "enforcer:"

// Finding meta key constants for EmitFinding/DecodeFinding.
const (
	MetaKeyDomain   = "domain"
	MetaKeySource   = "source"
	MetaKeyNodeName = "node_name"
	MetaKeyMessage  = "message"
	MetaKeyEvidence = "evidence"
)

// EmitFinding encodes a Finding as a Signal on the bus.
// Event format: "enforcer:<severity>". Meta carries domain, source,
// node_name, message, and JSON-encoded evidence.
func EmitFinding(bus signal.Bus, f circuit.Finding) {
	meta := map[string]string{
		MetaKeyDomain:   f.Domain,
		MetaKeySource:   f.Source,
		MetaKeyNodeName: f.NodeName,
		MetaKeyMessage:  f.Message,
	}
	if len(f.Evidence) > 0 {
		if data, err := json.Marshal(f.Evidence); err == nil {
			meta[MetaKeyEvidence] = string(data)
		}
	}
	bus.Emit(&signal.Signal{
		Event: findingEventPrefix + string(f.Severity),
		Agent: f.Source,
		Step:  f.NodeName,
		Meta:  meta,
	})
}

// DecodeFinding converts a Signal back to a Finding.
// Returns false if the signal is not a finding signal.
func DecodeFinding(s signal.Signal) (circuit.Finding, bool) {
	if !strings.HasPrefix(s.Event, findingEventPrefix) {
		return circuit.Finding{}, false
	}
	severity := circuit.FindingSeverity(strings.TrimPrefix(s.Event, findingEventPrefix))

	var evidence map[string]any
	if raw := s.Meta[MetaKeyEvidence]; raw != "" {
		_ = json.Unmarshal([]byte(raw), &evidence)
	}

	ts, _ := time.Parse(time.RFC3339, s.Timestamp)

	return circuit.Finding{
		Severity:  severity,
		Domain:    s.Meta[MetaKeyDomain],
		Source:    s.Meta[MetaKeySource],
		NodeName:  s.Meta[MetaKeyNodeName],
		Message:   s.Meta[MetaKeyMessage],
		Evidence:  evidence,
		Timestamp: ts,
	}, true
}

// FindingsSince returns all findings emitted on the bus since index idx.
// Non-finding signals are skipped.
func FindingsSince(bus signal.Bus, idx int) []circuit.Finding {
	signals := bus.Since(idx)
	var findings []circuit.Finding
	for _, s := range signals {
		if f, ok := DecodeFinding(s); ok {
			findings = append(findings, f)
		}
	}
	return findings
}
