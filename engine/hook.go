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

	"github.com/dpopsuev/origami/core"
)

type walkerStateCtxKey struct{}

// WithWalkerState returns a child context carrying the given WalkerState.
// Used by hookingWalker to make walker state available to hooks via Go context.
func WithWalkerState(ctx context.Context, s *core.WalkerState) context.Context {
	return context.WithValue(ctx, walkerStateCtxKey{}, s)
}

// WalkerStateFromContext extracts the WalkerState from a Go context.
// Before-hooks use this to inject data into the walker's Context map.
// Returns nil if the context does not carry a WalkerState.
func WalkerStateFromContext(ctx context.Context) *core.WalkerState {
	s, _ := ctx.Value(walkerStateCtxKey{}).(*core.WalkerState)
	return s
}

// Hook is a side-effect function invoked after a node completes.
// Hooks receive the validated artifact and can perform side effects
// (store writes, notifications) but do NOT affect routing or data flow.
// This is the Ansible notify/handler pattern.
type Hook interface {
	Name() string
	Run(ctx context.Context, nodeName string, artifact core.Artifact) error
}

// HookRegistry maps hook names to implementations.
type HookRegistry map[string]Hook

// Get returns the hook registered under name, or an error if not found.
// Supports FQCN resolution: a dot-qualified name does a direct lookup;
// an unqualified name tries direct first, then scans for a matching suffix.
func (r HookRegistry) Get(name string) (Hook, error) {
	if r == nil {
		return nil, fmt.Errorf("hook registry is nil")
	}
	if h, ok := r[name]; ok {
		return h, nil
	}
	if !strings.Contains(name, ".") {
		suffix := "." + name
		for k, h := range r {
			if strings.HasSuffix(k, suffix) {
				return h, nil
			}
		}
	}
	return nil, fmt.Errorf("hook %q not registered", name)
}

// Register adds a hook. Panics on duplicate.
func (r HookRegistry) Register(h Hook) {
	if _, exists := r[h.Name()]; exists {
		panic(fmt.Sprintf("duplicate hook registration: %q", h.Name()))
	}
	r[h.Name()] = h
}

// HookFunc is a convenience adapter that turns a plain function into a Hook.
type HookFunc struct {
	HookName string
	Fn       func(ctx context.Context, nodeName string, artifact core.Artifact) error
}

// NewHookFunc creates a Hook from a function.
func NewHookFunc(name string, fn func(ctx context.Context, nodeName string, artifact core.Artifact) error) *HookFunc {
	return &HookFunc{HookName: name, Fn: fn}
}

func (h *HookFunc) Name() string { return h.HookName }
func (h *HookFunc) Run(ctx context.Context, nodeName string, artifact core.Artifact) error {
	return h.Fn(ctx, nodeName, artifact)
}

// Built-in hook names recognized by the Runner.
const BuiltinHookFileWrite = "file-write"

// FileWriteHook is a built-in hook that writes an artifact to a JSON file.
// The output path is read from NodeDef.Meta["output_path"] and supports
// Go template variables: {{ .NodeName }}.
type FileWriteHook struct {
	NodeMeta map[string]map[string]any // node name -> meta (set by Runner)
}

func (h *FileWriteHook) Name() string { return BuiltinHookFileWrite }

func (h *FileWriteHook) Run(_ context.Context, nodeName string, artifact core.Artifact) error {
	meta := h.NodeMeta[nodeName]
	pathTmpl, _ := meta["output_path"].(string)
	if pathTmpl == "" {
		return fmt.Errorf("file-write hook: node %q missing meta.output_path", nodeName)
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

	if err := os.WriteFile(outPath, jsonData, 0o644); err != nil {
		return fmt.Errorf("file-write hook: write: %w", err)
	}

	return nil
}
