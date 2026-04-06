package engine

// Category: Processing & Support — hook types.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/dpopsuev/origami/circuit"
)

type walkerStateCtxKey struct{}

// WithWalkerState returns a child context carrying the given WalkerState.
// Used by hookingWalker to make walker state available to hooks via Go context.
func WithWalkerState(ctx context.Context, s *circuit.WalkerState) context.Context {
	return context.WithValue(ctx, walkerStateCtxKey{}, s)
}

// WalkerStateFromContext extracts the WalkerState from a Go context.
// Before-hooks use this to inject data into the walker's Context map.
// Returns nil if the context does not carry a WalkerState.
func WalkerStateFromContext(ctx context.Context) *circuit.WalkerState {
	s, _ := ctx.Value(walkerStateCtxKey{}).(*circuit.WalkerState)
	return s
}

// Hook, HookRegistry, HookFunc, NewHookFunc are defined in engine/handler.

// Built-in hook names recognized by the Runner.
const BuiltinHookFileWrite = "file-write"

// FileWriteHook is a built-in hook that writes an artifact to a JSON file.
// The output path is read from NodeConfig.OutputPath and supports
// Go template variables: {{ .NodeName }}.
type FileWriteHook struct {
	NodeConfigs map[string]*circuit.NodeConfig // node name -> config (set by Runner)
}

func (h *FileWriteHook) Name() string { return BuiltinHookFileWrite }

func (h *FileWriteHook) Run(_ context.Context, nodeName string, artifact circuit.Artifact) error {
	cfg := h.NodeConfigs[nodeName]
	pathTmpl := ""
	if cfg != nil {
		pathTmpl = cfg.OutputPath
	}
	if pathTmpl == "" {
		return fmt.Errorf("%w: %q missing config.output_path", ErrFileWriteHookNode, nodeName)
	}

	tmpl, err := template.New("path").Parse(pathTmpl)
	if err != nil {
		return fmt.Errorf("file-write hook: parse path template: %w", err)
	}

	var buf strings.Builder
	data := map[string]string{"NodeName": nodeName}
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("file-write hook: render path: %w", err)
	}
	outPath := buf.String()

	if dir := filepath.Dir(outPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("file-write hook: mkdir: %w", err)
		}
	}

	raw := artifact.Raw()
	jsonData, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("file-write hook: marshal: %w", err)
	}

	if err := os.WriteFile(outPath, jsonData, 0o600); err != nil {
		return fmt.Errorf("file-write hook: write: %w", err)
	}

	return nil
}
