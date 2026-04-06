package fold

import (
	"fmt"
	"os"
	"path/filepath"
)

// ResolveBoardComposition loads and merges the base board if compose: is set.
// Returns the board unchanged if no composition is declared.
func ResolveBoardComposition(bm *BoardManifest, boardDir string) (*BoardManifest, error) {
	if bm.Compose == nil || bm.Compose.Base == "" {
		return bm, nil
	}
	return resolveCompose(bm, boardDir, make(map[string]bool))
}

func resolveCompose(child *BoardManifest, boardDir string, visited map[string]bool) (*BoardManifest, error) {
	if child.Compose == nil || child.Compose.Base == "" {
		return child, nil
	}
	basePath := filepath.Join(boardDir, child.Compose.Base)
	absPath, err := filepath.Abs(basePath)
	if err != nil {
		return nil, fmt.Errorf("compose: resolve path: %w", err)
	}
	if visited[absPath] {
		return nil, fmt.Errorf("%w: %s", ErrCompositionCycle, absPath)
	}
	visited[absPath] = true

	data, err := os.ReadFile(basePath)
	if err != nil {
		return nil, fmt.Errorf("compose: read base %s: %w", child.Compose.Base, err)
	}
	base, err := ParseBoardManifest(data)
	if err != nil {
		return nil, fmt.Errorf("compose: parse base %s: %w", child.Compose.Base, err)
	}

	// Recursive: base may itself compose another board.
	baseDir := filepath.Dir(basePath)
	base, err = resolveCompose(base, baseDir, visited)
	if err != nil {
		return nil, err
	}

	return mergeBoards(base, child), nil
}

// mergeBoards merges base into child. Child wins on conflicts.
func mergeBoards(base, child *BoardManifest) *BoardManifest {
	merged := *base
	merged.Name = child.Name
	merged.Description = child.Description
	merged.Kind = child.Kind

	// Uses: union, child wins on conflict.
	if len(child.Uses) > 0 {
		if merged.Uses == nil {
			merged.Uses = make(map[string]string)
		}
		for k, v := range child.Uses {
			merged.Uses[k] = v
		}
	}

	// Bind: union, child wins on conflict.
	if len(child.Bind) > 0 {
		if merged.Bind == nil {
			merged.Bind = make(map[string]string)
		}
		for k, v := range child.Bind {
			merged.Bind[k] = v
		}
	}

	// Prompts: union, child wins on conflict.
	if len(child.Prompts) > 0 {
		if merged.Prompts == nil {
			merged.Prompts = make(map[string]string)
		}
		for k, v := range child.Prompts {
			merged.Prompts[k] = v
		}
	}

	// Scalars: child wins if set.
	if child.Domain != "" {
		merged.Domain = child.Domain
	}
	if child.Schema != "" {
		merged.Schema = child.Schema
	}
	if child.Scorecard != "" {
		merged.Scorecard = child.Scorecard
	}
	if child.Report != "" {
		merged.Report = child.Report
	}

	// Blocks: child wins entirely if set.
	if child.Serve != nil {
		merged.Serve = child.Serve
	}
	if child.Circuit != nil {
		merged.Circuit = child.Circuit
	}
	if child.Calibration != nil {
		merged.Calibration = child.Calibration
	}
	if len(child.Params) > 0 {
		merged.Params = child.Params
	}

	// Compose resolved — clear it.
	merged.Compose = nil

	return &merged
}
