package transformers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dpopsuev/origami/engine"
)

// FileTransformer reads file(s) from disk and returns their contents.
// Paths are resolved relative to a configurable root directory.
type FileTransformer struct {
	rootDir string
}

// FileOption configures the file transformer.
type FileOption func(*FileTransformer)

// WithRootDir sets the root directory for path resolution.
// Paths with ".." components that escape this root are rejected.
func WithRootDir(dir string) FileOption {
	return func(t *FileTransformer) { t.rootDir = dir }
}

// NewFile creates a transformer that reads files from disk.
func NewFile(opts ...FileOption) *FileTransformer {
	t := &FileTransformer{rootDir: "."}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

const transformerNameFile = "file"

func (t *FileTransformer) Name() string        { return transformerNameFile }
func (t *FileTransformer) Deterministic() bool { return true }

func (t *FileTransformer) Transform(ctx context.Context, tc *engine.TransformerContext) (any, error) {
	path := tc.Prompt
	if path == "" && tc.NodeConfig != nil {
		if p, ok := tc.NodeConfig.Extras["path"].(string); ok {
			path = p
		}
	}
	if path == "" {
		return nil, fmt.Errorf("file transformer: file path required (set prompt: or config.extras.path)")
	}

	if !filepath.IsAbs(path) {
		path = filepath.Join(t.rootDir, path)
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("file transformer: resolve path: %w", err)
	}
	absRoot, err := filepath.Abs(t.rootDir)
	if err != nil {
		return nil, fmt.Errorf("file transformer: resolve root: %w", err)
	}
	if !strings.HasPrefix(absPath, absRoot) {
		return nil, fmt.Errorf("file transformer: path %q escapes root %q", path, t.rootDir)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("file transformer: read %q: %w", absPath, err)
	}

	var jsonResult any
	if err := json.Unmarshal(data, &jsonResult); err == nil {
		return jsonResult, nil
	}

	return map[string]any{
		"path":    absPath,
		"content": string(data),
	}, nil
}
