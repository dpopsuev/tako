package mcp

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/resource"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// resourceInput is the input for the consolidated "resource" tool.
type resourceInput struct {
	Action string `json:"action"`
	Kind   string `json:"kind,omitempty"`
	Name   string `json:"name,omitempty"`
	FileA  string `json:"file_a,omitempty"`
	FileB  string `json:"file_b,omitempty"`
}

const (
	actionList = "list"
	actionDiff = "diff"
)

func (s *CircuitServer) registerResourceTool() {
	sdkmcp.AddTool(s.MCPServer, &sdkmcp.Tool{
		Name:        "resource",
		Description: "Origami resource API. Actions: kinds (list registered), list (discover all resources), get (load by kind/name), validate (check correctness), diff (compare two resources).",
	}, NoOutputSchema(s.handleResourceDispatch))
}

func (s *CircuitServer) handleResourceDispatch(_ context.Context, _ *sdkmcp.CallToolRequest, input *resourceInput) (*sdkmcp.CallToolResult, any, error) {
	reg := s.Config.ResourceRegistry

	switch input.Action {
	case "kinds":
		return nil, s.handleResourceKinds(reg), nil

	case actionList:
		return s.handleResourceList(reg, input)

	case "get":
		return s.handleResourceGet(reg, input)

	case "validate":
		return s.handleResourceValidate(reg, input)

	case actionDiff:
		return s.handleResourceDiff(reg, input)

	default:
		return nil, nil, fmt.Errorf("%w: %q; valid actions: kinds, list, get, validate, diff", ErrUnknownResourceAction, input.Action)
	}
}

type kindSummary struct {
	Kind    string `json:"kind"`
	Merge   bool   `json:"merge"`
	Builtin bool   `json:"builtin"`
}

func (s *CircuitServer) handleResourceKinds(reg *resource.KindRegistry) []kindSummary {
	kinds := reg.Kinds()
	summaries := make([]kindSummary, len(kinds))
	for i, k := range kinds {
		h := reg.Lookup(k)
		summaries[i] = kindSummary{
			Kind:    string(k),
			Merge:   h != nil && h.SupportsMerge(),
			Builtin: circuit.KnownKinds[k],
		}
	}
	return summaries
}

func (s *CircuitServer) handleResourceList(reg *resource.KindRegistry, input *resourceInput) (*sdkmcp.CallToolResult, any, error) {
	domainFS := s.Config.DomainFS

	type resourceEntry struct {
		Kind    string `json:"kind"`
		Name    string `json:"name"`
		Version string `json:"version,omitempty"`
		Source  string `json:"source,omitempty"`
	}

	var entries []resourceEntry
	err := fs.WalkDir(domainFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !isYAMLFile(path) {
			return nil
		}
		data, readErr := fs.ReadFile(domainFS, path)
		if readErr != nil {
			return nil
		}
		env, envErr := circuit.ParseEnvelope(data)
		if envErr != nil || env.Kind == "" || !reg.Has(env.Kind) {
			return nil
		}
		if input.Kind != "" && string(env.Kind) != input.Kind {
			return nil
		}
		entries = append(entries, resourceEntry{
			Kind:    string(env.Kind),
			Name:    env.Metadata.Name,
			Version: env.Version,
			Source:  path,
		})
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("resource list: %w", err)
	}
	return nil, entries, nil
}

func (s *CircuitServer) handleResourceGet(reg *resource.KindRegistry, input *resourceInput) (*sdkmcp.CallToolResult, any, error) {
	if input.Kind == "" {
		return nil, nil, ErrResourceKindRequired
	}
	if input.Name == "" {
		return nil, nil, ErrResourceNameRequired
	}

	domainFS := s.Config.DomainFS
	kind := circuit.Kind(input.Kind)

	var found []byte
	var foundPath string
	_ = fs.WalkDir(domainFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !isYAMLFile(path) || found != nil {
			return nil
		}
		data, readErr := fs.ReadFile(domainFS, path)
		if readErr != nil {
			return nil
		}
		env, envErr := circuit.ParseEnvelope(data)
		if envErr != nil || env.Kind != kind || env.Metadata.Name != input.Name {
			return nil
		}
		found = data
		foundPath = path
		return nil
	})
	if found == nil {
		return nil, nil, fmt.Errorf("%w: %s/%s", ErrResourceNotFound, input.Kind, input.Name)
	}

	res, typed, err := resource.Load(reg, found, foundPath)
	if err != nil {
		return nil, nil, fmt.Errorf("resource get: %w", err)
	}

	return nil, map[string]any{
		"resource": res,
		"parsed":   typed,
	}, nil
}

func (s *CircuitServer) handleResourceValidate(reg *resource.KindRegistry, input *resourceInput) (*sdkmcp.CallToolResult, any, error) {
	if input.Kind == "" || input.Name == "" {
		return nil, nil, ErrResourceKindRequired
	}

	_, result, err := s.handleResourceGet(reg, input)
	if err != nil {
		return nil, nil, err
	}

	m := result.(map[string]any)
	res := m["resource"].(*resource.Resource)
	parsed := m["parsed"]

	if valErr := resource.Validate(reg, res, parsed); valErr != nil {
		return nil, map[string]any{
			"valid":    false,
			"findings": valErr.Error(),
		}, nil
	}

	return nil, map[string]any{
		"valid":    true,
		"findings": nil,
	}, nil
}

func (s *CircuitServer) handleResourceDiff(reg *resource.KindRegistry, input *resourceInput) (*sdkmcp.CallToolResult, any, error) {
	if input.FileA == "" || input.FileB == "" {
		return nil, nil, ErrResourceFilesRequired
	}

	domainFS := s.Config.DomainFS

	dataA, err := fs.ReadFile(domainFS, input.FileA)
	if err != nil {
		return nil, nil, fmt.Errorf("read file_a: %w", err)
	}
	dataB, err := fs.ReadFile(domainFS, input.FileB)
	if err != nil {
		return nil, nil, fmt.Errorf("read file_b: %w", err)
	}

	resA, _, err := resource.Load(reg, dataA, input.FileA)
	if err != nil {
		return nil, nil, fmt.Errorf("load file_a: %w", err)
	}
	resB, _, err := resource.Load(reg, dataB, input.FileB)
	if err != nil {
		return nil, nil, fmt.Errorf("load file_b: %w", err)
	}

	entries := resource.Diff(resA, resB)
	return nil, map[string]any{
		"file_a":  input.FileA,
		"file_b":  input.FileB,
		"changes": len(entries),
		"entries": entries,
	}, nil
}

func isYAMLFile(path string) bool {
	n := len(path)
	return (n > 5 && path[n-5:] == ".yaml") || (n > 4 && path[n-4:] == ".yml")
}
