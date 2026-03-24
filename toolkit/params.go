package toolkit

// LoadPriorArtifacts loads JSON artifacts for a list of node names from a
// case directory. Each artifact is read via ReadMapArtifact using the filename
// returned by artifactFn. Missing artifacts are silently skipped.
func LoadPriorArtifacts(caseDir string, nodeNames []string, artifactFn func(string) string) map[string]map[string]any {
	if caseDir == "" {
		return nil
	}
	result := make(map[string]map[string]any, len(nodeNames))
	for _, name := range nodeNames {
		filename := artifactFn(name)
		if filename == "" {
			continue
		}
		data, _ := ReadMapArtifact(caseDir, filename)
		if data != nil {
			result[name] = data
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
