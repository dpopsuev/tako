package stubs

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/roster"
)

// StubInstrumentTool implements battery.Tool with canned responses.
// Thread-safe, supports error injection, tracks calls.
type StubInstrumentTool struct {
	mu          sync.Mutex
	name        string
	description string
	schema      json.RawMessage
	result      string
	err         error
	calls       []json.RawMessage
}

// NewStubInstrumentTool creates a tool with canned result.
func NewStubInstrumentTool(name, description string) *StubInstrumentTool {
	return &StubInstrumentTool{
		name:        name,
		description: description,
		schema:      json.RawMessage(`{"type":"object"}`),
		result:      `{"ok":true}`,
	}
}

func (s *StubInstrumentTool) Name() string                 { return s.name }
func (s *StubInstrumentTool) Description() string          { return s.description }
func (s *StubInstrumentTool) InputSchema() json.RawMessage { return s.schema }

func (s *StubInstrumentTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	s.mu.Lock()
	s.calls = append(s.calls, input)
	err := s.err
	result := s.result
	s.mu.Unlock()

	if err != nil {
		return "", err
	}
	return result, nil
}

// SetError injects an error for all subsequent Execute calls.
func (s *StubInstrumentTool) SetError(err error) {
	s.mu.Lock()
	s.err = err
	s.mu.Unlock()
}

// SetResult sets the canned result for all subsequent Execute calls.
func (s *StubInstrumentTool) SetResult(result string) {
	s.mu.Lock()
	s.result = result
	s.mu.Unlock()
}

// SetSchema sets the input schema.
func (s *StubInstrumentTool) SetSchema(schema json.RawMessage) {
	s.mu.Lock()
	s.schema = schema
	s.mu.Unlock()
}

// Calls returns the ordered list of inputs that Execute was called with.
func (s *StubInstrumentTool) Calls() []json.RawMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]json.RawMessage, len(s.calls))
	copy(out, s.calls)
	return out
}

// CallCount returns how many times Execute was called.
func (s *StubInstrumentTool) CallCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

// StubInstrumentNode implements circuit.Node backed by a StubInstrumentTool.
// Bridges the battery.Tool interface to circuit.Node for testing.
type StubInstrumentNode struct {
	tool    *StubInstrumentTool
	element roster.Element
}

// NewStubInstrumentNode creates an instrument node wrapping a stub tool.
func NewStubInstrumentNode(tool *StubInstrumentTool) *StubInstrumentNode {
	return &StubInstrumentNode{tool: tool}
}

func (n *StubInstrumentNode) Name() string                    { return n.tool.Name() }
func (n *StubInstrumentNode) ElementAffinity() roster.Element { return n.element }

func (n *StubInstrumentNode) Process(ctx context.Context, nc circuit.NodeContext) (circuit.Artifact, error) {
	// Marshal walker state context as input (mirrors real InstrumentNode behavior).
	var input json.RawMessage
	if nc.WalkerState != nil {
		if data, err := json.Marshal(nc.WalkerState.Context); err == nil {
			input = data
		}
	}
	if input == nil {
		input = json.RawMessage(`{}`)
	}

	result, err := n.tool.Execute(ctx, input)
	if err != nil {
		return nil, err
	}
	return &InstrumentArtifact{
		name: n.tool.Name(),
		raw:  result,
	}, nil
}

// InstrumentArtifact is a test artifact wrapping instrument output.
type InstrumentArtifact struct {
	name string
	raw  string
}

func (a *InstrumentArtifact) Type() string        { return "instrument:" + a.name }
func (a *InstrumentArtifact) Confidence() float64 { return 1.0 }
func (a *InstrumentArtifact) Raw() any            { return a.raw }
