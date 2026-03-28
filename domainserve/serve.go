// Package domainserve provides a reusable library for building domain
// data MCP servers. Any product calls domainserve.New(embedFS, config)
// and gets a ready-to-serve http.Handler with /mcp, /healthz, /readyz.
package domainserve

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"sort"
	"strings"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"gopkg.in/yaml.v3"
)

// Config configures the domain data server.
type Config struct {
	Name    string
	Version string
	Assets  *AssetIndex // when non-nil, enables key-based asset resolution
}

// AssetIndex maps manifest sections and files for key-based resolution.
// Sections maps section name (e.g. "circuits") to key-path pairs.
// Files maps singleton asset names (e.g. "vocabulary") to paths.
type AssetIndex struct {
	Sections map[string]map[string]string
	Files    map[string]string
}

// Resolve looks up a file path by section and key. For singleton files
// (no key), pass an empty key and the section name is looked up in Files.
func (idx *AssetIndex) Resolve(section, key string) (string, error) {
	if key == "" {
		if p, ok := idx.Files[section]; ok {
			return p, nil
		}
		return "", fmt.Errorf("unknown file %q", section)
	}
	sec, ok := idx.Sections[section]
	if !ok {
		return "", fmt.Errorf("unknown section %q", section)
	}
	p, ok := sec[key]
	if !ok {
		return "", fmt.Errorf("unknown key %q in section %q", key, section)
	}
	return p, nil
}

// CircuitInfo describes one circuit found in the domain filesystem.
type CircuitInfo struct {
	Name        string `json:"name"`
	Topology    string `json:"topology,omitempty"`
	Description string `json:"description,omitempty"`
}

// DomainInfo is the response payload for the domain_info tool.
type DomainInfo struct {
	Name     string        `json:"name"`
	Version  string        `json:"version"`
	Circuits []CircuitInfo `json:"circuits"`
}

// DirEntry is a single entry returned by the domain_list tool.
type DirEntry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
}

// New creates an http.Handler that serves domain data from fsys over
// MCP. The handler exposes /mcp (Streamable HTTP), /healthz, /readyz.
func New(fsys fs.FS, cfg Config) http.Handler {
	server := sdkmcp.NewServer(
		&sdkmcp.Implementation{Name: cfg.Name, Version: cfg.Version},
		nil,
	)

	registerTools(server, fsys, cfg)

	mcpHandler := sdkmcp.NewStreamableHTTPHandler(
		func(_ *http.Request) *sdkmcp.Server { return server },
		&sdkmcp.StreamableHTTPOptions{Stateless: true},
	)

	mux := http.NewServeMux()
	mux.Handle("/mcp", mcpHandler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return mux
}

func registerTools(server *sdkmcp.Server, fsys fs.FS, cfg Config) {
	server.AddTool(
		&sdkmcp.Tool{
			Name:        "domain_info",
			Description: "Return domain metadata including available circuits",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		},
		func(_ context.Context, _ *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
			info := DomainInfo{
				Name:     cfg.Name,
				Version:  cfg.Version,
				Circuits: scanCircuits(fsys, cfg.Assets),
			}
			data, err := json.Marshal(info)
			if err != nil {
				return errResult("marshal domain info: " + err.Error()), nil
			}
			return textResult(string(data)), nil
		},
	)

	server.AddTool(
		&sdkmcp.Tool{
			Name:        "domain_read",
			Description: "Read a file from the domain filesystem by path",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"File path to read"}},"required":["path"]}`),
		},
		func(_ context.Context, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
			var args struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
				return errResult("invalid arguments: " + err.Error()), nil
			}
			if !fs.ValidPath(args.Path) {
				return errResult("invalid path: " + args.Path), nil
			}
			data, err := fs.ReadFile(fsys, args.Path)
			if err != nil {
				return errResult(err.Error()), nil
			}
			return textResult(string(data)), nil
		},
	)

	server.AddTool(
		&sdkmcp.Tool{
			Name:        "domain_list",
			Description: "List entries in a domain filesystem directory",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"Directory path to list"}},"required":["path"]}`),
		},
		func(_ context.Context, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
			var args struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
				return errResult("invalid arguments: " + err.Error()), nil
			}
			if !fs.ValidPath(args.Path) {
				return errResult("invalid path: " + args.Path), nil
			}
			entries, err := fs.ReadDir(fsys, args.Path)
			if err != nil {
				return errResult(err.Error()), nil
			}
			result := make([]DirEntry, len(entries))
			for i, e := range entries {
				result[i] = DirEntry{Name: e.Name(), IsDir: e.IsDir()}
			}
			data, err := json.Marshal(result)
			if err != nil {
				return errResult("marshal entries: " + err.Error()), nil
			}
			return textResult(string(data)), nil
		},
	)

	if cfg.Assets != nil {
		server.AddTool(
			&sdkmcp.Tool{
				Name:        "domain_resolve",
				Description: "Resolve a domain asset by section and key, returning its file content",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"section":{"type":"string","description":"Asset section (e.g. circuits, prompts) or file name (e.g. vocabulary)"},"key":{"type":"string","description":"Asset key within the section (omit for singleton files)"}},"required":["section"]}`),
			},
			func(_ context.Context, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
				var args struct {
					Section string `json:"section"`
					Key     string `json:"key"`
				}
				if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
					return errResult("invalid arguments: " + err.Error()), nil
				}
				path, err := cfg.Assets.Resolve(args.Section, args.Key)
				if err != nil {
					return errResult(err.Error()), nil
				}
				data, err := fs.ReadFile(fsys, path)
				if err != nil {
					return errResult(err.Error()), nil
				}
				return textResult(string(data)), nil
			},
		)
	}
}

// scanCircuits discovers circuits either from the AssetIndex (preferred)
// or by scanning the circuits/ directory (fallback).
func scanCircuits(fsys fs.FS, assets *AssetIndex) []CircuitInfo {
	if assets != nil {
		if circuits, ok := assets.Sections["circuits"]; ok {
			return circuitsFromMap(fsys, circuits)
		}
	}
	return circuitsFromDir(fsys)
}

func circuitsFromMap(fsys fs.FS, circuits map[string]string) []CircuitInfo {
	names := make([]string, 0, len(circuits))
	for name := range circuits {
		names = append(names, name)
	}
	sort.Strings(names)

	result := make([]CircuitInfo, 0, len(circuits))
	for _, name := range names {
		ci := CircuitInfo{Name: name}
		data, err := fs.ReadFile(fsys, circuits[name])
		if err == nil {
			var header struct {
				Topology    string `yaml:"topology"`
				Description string `yaml:"description"`
			}
			if yaml.Unmarshal(data, &header) == nil {
				ci.Topology = header.Topology
				ci.Description = header.Description
			}
		}
		result = append(result, ci)
	}
	return result
}

func circuitsFromDir(fsys fs.FS) []CircuitInfo {
	entries, err := fs.ReadDir(fsys, "circuits")
	if err != nil {
		return nil
	}
	circuits := make([]CircuitInfo, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".yaml")
		ci := CircuitInfo{Name: name}

		data, err := fs.ReadFile(fsys, "circuits/"+e.Name())
		if err == nil {
			var header struct {
				Topology    string `yaml:"topology"`
				Description string `yaml:"description"`
			}
			if yaml.Unmarshal(data, &header) == nil {
				ci.Topology = header.Topology
				ci.Description = header.Description
			}
		}
		circuits = append(circuits, ci)
	}
	return circuits
}

func textResult(text string) *sdkmcp.CallToolResult {
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: text}},
	}
}

func errResult(msg string) *sdkmcp.CallToolResult {
	return &sdkmcp.CallToolResult{
		IsError: true,
		Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: msg}},
	}
}
