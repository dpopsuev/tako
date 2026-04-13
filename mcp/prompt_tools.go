package mcp

import (
	"context"
	"fmt"

	"github.com/dpopsuev/origami/prompt"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// promptInput is the input for the consolidated "prompt" tool.
type promptInput struct {
	Action  string `json:"action"`
	Name    string `json:"name,omitempty"`
	Step    string `json:"step,omitempty"`
	Content string `json:"content,omitempty"`
	Version int    `json:"version,omitempty"`
}

func (s *CircuitServer) registerPromptTool() {
	s.MCPServer.AddTool(&sdkmcp.Tool{
		Name:        "prompt",
		Description: "Prompt CRUD. Actions: list (all prompts), get (by name), update (edit content), create (new prompt), rollback (revert to version).",
		InputSchema: map[string]any{"type": "object"},
	}, rawHandler(s.handlePromptDispatch))
}

func (s *CircuitServer) handlePromptDispatch(_ context.Context, _ *sdkmcp.CallToolRequest, input *promptInput) (*sdkmcp.CallToolResult, any, error) {
	store := s.Config.PromptStore
	switch input.Action {
	case "list":
		prompts, err := store.List()
		if err != nil {
			return nil, nil, fmt.Errorf("prompt list: %w", err)
		}
		type promptSummary struct {
			Name     string `json:"name"`
			Step     string `json:"step,omitempty"`
			Version  int    `json:"version"`
			Sections int    `json:"sections"`
		}
		summaries := make([]promptSummary, len(prompts))
		for i, p := range prompts {
			summaries[i] = promptSummary{
				Name:     p.Name,
				Step:     p.Step,
				Version:  p.Version,
				Sections: len(p.Sections),
			}
		}
		return nil, summaries, nil

	case "get":
		if input.Name == "" {
			return nil, nil, ErrPromptNameRequired
		}
		p, err := store.Get(input.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("prompt get: %w", err)
		}
		return nil, p, nil

	case "update":
		if input.Name == "" {
			return nil, nil, ErrPromptNameRequired
		}
		p, err := store.Update(input.Name, input.Content)
		if err != nil {
			return nil, nil, fmt.Errorf("prompt update: %w", err)
		}
		return nil, p, nil

	case "create":
		if input.Name == "" {
			return nil, nil, ErrPromptNameRequired
		}
		p, err := store.Create(input.Name, input.Step, input.Content)
		if err != nil {
			return nil, nil, fmt.Errorf("prompt create: %w", err)
		}
		return nil, p, nil

	case "rollback":
		if input.Name == "" {
			return nil, nil, ErrPromptNameRequired
		}
		p, err := store.Rollback(input.Name, input.Version)
		if err != nil {
			return nil, nil, fmt.Errorf("prompt rollback: %w", err)
		}
		return nil, p, nil

	case "export":
		dir := input.Content
		if dir == "" {
			dir = "prompts-export"
		}
		count, err := prompt.Export(store, dir)
		if err != nil {
			return nil, nil, fmt.Errorf("prompt export: %w", err)
		}
		return nil, map[string]any{"exported": count, "directory": dir}, nil

	default:
		return nil, nil, fmt.Errorf("%w: %q; valid actions: list, get, update, create, rollback, export", ErrUnknownPromptAction, input.Action)
	}
}
