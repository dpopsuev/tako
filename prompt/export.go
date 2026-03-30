package prompt

import (
	"fmt"
	"os"
	"path/filepath"
)

// Export writes all prompts from a Store to disk as markdown files.
// Each prompt is written to {dir}/{name}.md. Directory structure is
// created as needed. Returns the number of files written.
func Export(store Store, dir string) (int, error) {
	prompts, err := store.List()
	if err != nil {
		return 0, fmt.Errorf("list prompts: %w", err)
	}

	count := 0
	for _, p := range prompts {
		path := filepath.Join(dir, p.Name+".md")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return count, fmt.Errorf("create dir for %q: %w", p.Name, err)
		}
		if err := os.WriteFile(path, []byte(p.Content), 0o600); err != nil {
			return count, fmt.Errorf("write %q: %w", p.Name, err)
		}
		count++
	}
	return count, nil
}
