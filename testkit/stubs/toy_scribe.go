package stubs

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/dpopsuev/origami/tool"
	mcpserver "github.com/dpopsuev/origami/tool/mcp"
	"github.com/dpopsuev/origami/tool/server"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// ScribeInput builds a ScribeToolInput for use in tests via Handle().
func ScribeInput(action string, fields map[string]string) ScribeToolInput {
	input := ScribeToolInput{Action: action}
	for k, v := range fields {
		switch k {
		case "id":
			input.ID = v
		case "kind":
			input.Kind = v
		case "title":
			input.Title = v
		case "status":
			input.Status = v
		case "scope":
			input.Scope = v
		case "field":
			input.Field = v
		case "value":
			input.Value = v
		case "name":
			input.Name = v
		case "text":
			input.Text = v
		case "priority":
			input.Priority = v
		}
	}
	return input
}

// ToyArtifact is a minimal artifact for testing. No parchment import.
type ToyArtifact struct {
	ID       string            `json:"id"`
	Kind     string            `json:"kind"`
	Title    string            `json:"title"`
	Status   string            `json:"status"`
	Priority string            `json:"priority,omitempty"`
	Scope    string            `json:"scope,omitempty"`
	Sections map[string]string `json:"sections,omitempty"`
}

// ToyScribeStore is a stateful map-based artifact CRUD store for testing.
// Thread-safe. Exposes both MCP tool interface and direct Go accessors.
type ToyScribeStore struct {
	mu    sync.Mutex
	items map[string]*ToyArtifact
	seq   int
}

// NewToyScribeStore creates an empty store.
func NewToyScribeStore() *ToyScribeStore {
	return &ToyScribeStore{items: make(map[string]*ToyArtifact)}
}

// Seed pre-populates the store with artifacts for test setup.
func (s *ToyScribeStore) Seed(artifacts ...*ToyArtifact) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range artifacts {
		cp := *a
		if cp.Sections == nil {
			cp.Sections = make(map[string]string)
		}
		s.items[a.ID] = &cp
	}
}

// Get returns an artifact by ID. Direct Go access for assertions.
func (s *ToyScribeStore) Get(id string) *ToyArtifact {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[id]
	if !ok {
		return nil
	}
	cp := *item
	return &cp
}

// List returns artifacts matching the given status. Empty string matches all.
func (s *ToyScribeStore) List(status string) []*ToyArtifact {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result []*ToyArtifact
	for _, item := range s.items {
		if status == "" || item.Status == status {
			cp := *item
			result = append(result, &cp)
		}
	}
	return result
}

// Count returns the total number of artifacts.
func (s *ToyScribeStore) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.items)
}

// ScribeToolInput is the MCP tool input for artifact operations.
type ScribeToolInput struct {
	Action string `json:"action"`
	ID     string `json:"id,omitempty"`
	Kind   string `json:"kind,omitempty"`
	Title  string `json:"title,omitempty"`
	Status string `json:"status,omitempty"`
	Scope  string `json:"scope,omitempty"`
	Field  string `json:"field,omitempty"`
	Value  string `json:"value,omitempty"`
	Name   string `json:"name,omitempty"`
	Text   string `json:"text,omitempty"`
	// list filters
	Sort     string   `json:"sort,omitempty"`
	Fields   []string `json:"fields,omitempty"`
	Priority string   `json:"priority,omitempty"`
}

// Handle processes an MCP tool call for the artifact tool.
func (s *ToyScribeStore) Handle(_ context.Context, input ScribeToolInput) (any, error) {
	switch input.Action {
	case "create":
		return s.handleCreate(input)
	case "list":
		return s.handleList(input)
	case "get":
		return s.handleGet(input)
	case "set":
		return s.handleSet(input)
	case "attach_section":
		return s.handleAttachSection(input)
	default:
		return nil, fmt.Errorf("unknown action: %q", input.Action)
	}
}

func (s *ToyScribeStore) handleCreate(input ScribeToolInput) (any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.seq++
	id := fmt.Sprintf("TSK-%d", s.seq)
	status := input.Status
	if status == "" {
		status = "draft"
	}
	kind := input.Kind
	if kind == "" {
		kind = "task"
	}

	a := &ToyArtifact{
		ID:       id,
		Kind:     kind,
		Title:    input.Title,
		Status:   status,
		Priority: input.Priority,
		Scope:    input.Scope,
		Sections: make(map[string]string),
	}
	s.items[id] = a

	return map[string]string{"id": id, "status": status}, nil
}

func (s *ToyScribeStore) handleList(input ScribeToolInput) (any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]map[string]string, 0, len(s.items))
	for _, item := range s.items {
		if input.Status != "" && item.Status != input.Status {
			continue
		}
		if input.Kind != "" && item.Kind != input.Kind {
			continue
		}
		entry := map[string]string{
			"id":     item.ID,
			"title":  item.Title,
			"status": item.Status,
			"kind":   item.Kind,
		}
		if item.Priority != "" {
			entry["priority"] = item.Priority
		}
		if item.Scope != "" {
			entry["scope"] = item.Scope
		}
		result = append(result, entry)
	}
	return result, nil
}

func (s *ToyScribeStore) handleGet(input ScribeToolInput) (any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.items[input.ID]
	if !ok {
		return nil, fmt.Errorf("artifact not found: %q", input.ID)
	}
	return item, nil
}

func (s *ToyScribeStore) handleSet(input ScribeToolInput) (any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.items[input.ID]
	if !ok {
		return nil, fmt.Errorf("artifact not found: %q", input.ID)
	}

	switch input.Field {
	case "status":
		item.Status = input.Value
	case "priority":
		item.Priority = input.Value
	case "title":
		item.Title = input.Value
	case "scope":
		item.Scope = input.Value
	default:
		return nil, fmt.Errorf("unknown field: %q", input.Field)
	}

	return map[string]string{"id": input.ID, input.Field: input.Value}, nil
}

func (s *ToyScribeStore) handleAttachSection(input ScribeToolInput) (any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.items[input.ID]
	if !ok {
		return nil, fmt.Errorf("artifact not found: %q", input.ID)
	}

	if item.Sections == nil {
		item.Sections = make(map[string]string)
	}
	item.Sections[input.Name] = input.Text

	return map[string]string{"id": input.ID, "section": input.Name}, nil
}

// Serve creates an in-memory MCP server with an "artifact" tool and returns
// the client-side transport. The server runs until the test ends.
func (s *ToyScribeStore) Serve(t *testing.T) sdkmcp.Transport {
	t.Helper()

	handler := mcpserver.Handler(func(ctx context.Context, raw json.RawMessage) (tool.Result, error) {
		var input ScribeToolInput
		if err := json.Unmarshal(raw, &input); err != nil {
			return tool.Result{}, fmt.Errorf("unmarshal: %w", err)
		}
		result, err := s.Handle(ctx, input)
		if err != nil {
			return tool.Result{}, err
		}
		data, _ := json.Marshal(result)
		return tool.TextResult(string(data)), nil
	})

	srv := mcpserver.NewServer("toy-scribe", "test").
		WithInstructions("Toy Scribe for testing. Actions: create, list, get, set, attach_section.").
		Tool(server.ToolMeta{
			Name:        "artifact",
			Description: "Toy artifact CRUD. Actions: create, list, get, set, attach_section.",
		}, handler)

	clientTransport, serverTransport := sdkmcp.NewInMemoryTransports()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	go func() {
		_ = srv.Serve(ctx, serverTransport)
	}()

	return clientTransport
}
