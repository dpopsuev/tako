package github

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxTreeDepth = 4
	gitDir       = ".git"
)

// LocalTreeEntry represents a file or directory entry in a repository listing.
// This is the internal type; git_driver.go maps these to skn.ContentEntry.
type LocalTreeEntry struct {
	Path  string
	IsDir bool
}

// ReadFile reads a single file from the local clone.
func ReadFile(_ context.Context, localPath, filePath string) ([]byte, error) {
	full := filepath.Join(localPath, filePath)
	if !strings.HasPrefix(full, localPath) {
		return nil, fmt.Errorf("%w: %s", ErrPathTraversal, filePath)
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", filePath, err)
	}
	return data, nil
}

// ListTree walks the local clone and returns directory/file entries.
func ListTree(_ context.Context, localPath string, maxDepth int) ([]LocalTreeEntry, error) {
	if maxDepth <= 0 {
		maxDepth = maxTreeDepth
	}

	var entries []LocalTreeEntry
	baseLen := len(localPath)

	err := filepath.WalkDir(localPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		rel := path[baseLen:]
		if rel != "" && rel[0] == filepath.Separator {
			rel = rel[1:]
		}
		if rel == "" {
			return nil
		}

		if d.Name() == gitDir {
			return filepath.SkipDir
		}
		if strings.HasPrefix(d.Name(), ".") && d.IsDir() {
			return filepath.SkipDir
		}

		depth := strings.Count(rel, string(filepath.Separator))
		if d.IsDir() && depth >= maxDepth {
			return filepath.SkipDir
		}

		entries = append(entries, LocalTreeEntry{
			Path:  rel,
			IsDir: d.IsDir(),
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", localPath, err)
	}
	return entries, nil
}
