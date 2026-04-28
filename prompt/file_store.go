package prompt

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

// FileStore is a read-only Store backed by an fs.FS.
// Prompts are discovered by walking for .md files.
// The prompt name is derived from the file path (e.g., "triage/classify-symptoms").
type FileStore struct {
	fsys fs.FS
	// nameMap allows explicit name→path overrides (e.g., from assets.prompts).
	nameMap map[string]string
}

// NewFileStore creates a FileStore from an fs.FS.
// The optional nameMap provides explicit name→path mappings (as declared in
// tako.yaml assets.prompts). When nil, prompts are discovered by walking.
func NewFileStore(fsys fs.FS, nameMap map[string]string) *FileStore {
	return &FileStore{fsys: fsys, nameMap: nameMap}
}

func (s *FileStore) Get(name string) (*Prompt, error) {
	path, ok := s.resolvePath(name)
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrNotFound, name)
	}
	data, err := fs.ReadFile(s.fsys, path)
	if err != nil {
		return nil, fmt.Errorf("%w: %q: %w", ErrNotFound, name, err)
	}

	p, err := ParsePrompt(data)
	if err != nil {
		return nil, fmt.Errorf("%w: %q: %w", ErrNotFound, name, err)
	}

	// Reject non-prompt kinds in front matter.
	if p.Meta != nil && p.Meta["kind"] != "" && p.Meta["kind"] != "prompt" {
		return nil, fmt.Errorf("%w: %q: expected kind prompt, got %q", ErrNotFound, name, p.Meta["kind"])
	}

	// Default name and step from file path.
	if p.Name == "" {
		p.Name = name
	}
	step := filepath.Dir(path)
	if step == "." {
		step = ""
	}
	if p.Step == "" {
		p.Step = step
	}

	return p, nil
}

func (s *FileStore) List() ([]*Prompt, error) {
	if s.nameMap != nil {
		return s.listFromMap()
	}
	return s.listByWalk()
}

func (s *FileStore) listFromMap() ([]*Prompt, error) {
	prompts := make([]*Prompt, 0, len(s.nameMap))
	for name := range s.nameMap {
		p, err := s.Get(name)
		if err != nil {
			continue // skip missing files
		}
		prompts = append(prompts, p)
	}
	return prompts, nil
}

func (s *FileStore) listByWalk() ([]*Prompt, error) {
	var prompts []*Prompt
	err := fs.WalkDir(s.fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".md") {
			return nil
		}
		name := strings.TrimSuffix(path, ".md")
		p, readErr := s.Get(name)
		if readErr != nil {
			return nil
		}
		prompts = append(prompts, p)
		return nil
	})
	return prompts, err
}

// Update is not supported on FileStore (read-only).
func (s *FileStore) Update(_, _ string) (*Prompt, error) {
	return nil, ErrReadOnly
}

// Create is not supported on FileStore (read-only).
func (s *FileStore) Create(_, _, _ string) (*Prompt, error) {
	return nil, ErrReadOnly
}

// Rollback is not supported on FileStore (read-only).
func (s *FileStore) Rollback(_ string, _ int) (*Prompt, error) {
	return nil, ErrReadOnly
}

func (s *FileStore) resolvePath(name string) (string, bool) {
	if s.nameMap != nil {
		if p, ok := s.nameMap[name]; ok {
			return p, true
		}
	}
	// Try direct path: name.md
	path := name + ".md"
	if _, err := fs.Stat(s.fsys, path); err == nil {
		return path, true
	}
	return "", false
}
