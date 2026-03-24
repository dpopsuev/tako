package toolkit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// CaseDir returns the per-case directory path: {basePath}/{suiteID}/{caseID}/
func CaseDir(basePath string, suiteID, caseID int64) string {
	return filepath.Join(basePath, fmt.Sprintf("%d", suiteID), fmt.Sprintf("%d", caseID))
}

// EnsureCaseDir creates the per-case directory if it doesn't exist.
func EnsureCaseDir(basePath string, suiteID, caseID int64) (string, error) {
	dir := CaseDir(basePath, suiteID, caseID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create case dir: %w", err)
	}
	return dir, nil
}

// ListCaseDirs lists all case directories under a suite.
func ListCaseDirs(basePath string, suiteID int64) ([]string, error) {
	suiteDir := filepath.Join(basePath, fmt.Sprintf("%d", suiteID))
	entries, err := os.ReadDir(suiteDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list case dirs: %w", err)
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, filepath.Join(suiteDir, e.Name()))
		}
	}
	return dirs, nil
}

// NodeArtifactFilename returns the artifact filename for a node.
// Uses the override map if provided, otherwise falls back to the
// convention: <nodeName>-result.json.
func NodeArtifactFilename(nodeName string, overrides map[string]string) string {
	if overrides != nil {
		if f, ok := overrides[nodeName]; ok {
			return f
		}
	}
	return nodeName + "-result.json"
}

// NodePromptFilename returns the prompt output filename for a node and loop iteration.
func NodePromptFilename(nodeName string, loopIter int) string {
	if nodeName == "" {
		return ""
	}
	if loopIter > 0 {
		return fmt.Sprintf("prompt-%s-loop-%d.md", nodeName, loopIter)
	}
	return fmt.Sprintf("prompt-%s.md", nodeName)
}

// ReadMapArtifact reads a JSON artifact from a directory into map[string]any.
func ReadMapArtifact(dir, filename string) (map[string]any, error) {
	path := filepath.Join(dir, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read artifact %s: %w", filename, err)
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse artifact %s: %w", filename, err)
	}
	return result, nil
}

// WriteArtifact writes a JSON artifact to a directory.
func WriteArtifact(dir, filename string, data any) error {
	path := filepath.Join(dir, filename)
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal artifact %s: %w", filename, err)
	}
	if err := os.WriteFile(path, raw, 0644); err != nil {
		return fmt.Errorf("write artifact %s: %w", filename, err)
	}
	return nil
}

// WriteNodePrompt writes a filled prompt to a directory using a node name.
func WriteNodePrompt(dir string, nodeName string, loopIter int, content string) (string, error) {
	filename := NodePromptFilename(nodeName, loopIter)
	if filename == "" {
		return "", fmt.Errorf("no prompt filename for node %s", nodeName)
	}
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("write prompt: %w", err)
	}
	return path, nil
}
