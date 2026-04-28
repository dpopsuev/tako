package autodoc

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Enricher sends deterministic docs to an LLM for narrative enhancement.
type Enricher interface {
	Enrich(ctx context.Context, prompt string) (string, error)
}

// EnrichConfig configures stochastic enrichment.
type EnrichConfig struct {
	OutputDir string
	Enricher  Enricher
	Manifest  *Manifest
}

// EnrichDocs reads auto-generated pages and asks the LLM to add narrative depth.
// Only pages with autodoc markers are candidates. Hand-written content is preserved.
func EnrichDocs(ctx context.Context, cfg EnrichConfig) error {
	circuitsDir := filepath.Join(cfg.OutputDir, "circuits")
	entries, err := os.ReadDir(circuitsDir)
	if err != nil {
		return fmt.Errorf("read circuits dir: %w", err)
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || e.Name() == "index.md" {
			continue
		}
		path := filepath.Join(circuitsDir, e.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if !strings.Contains(string(content), markerBegin) {
			continue
		}

		prompt := fmt.Sprintf(
			"You are a technical writer for the %s project (built on Tako circuit framework). "+
				"Below is an auto-generated circuit documentation page. Add a brief introductory paragraph "+
				"explaining what this circuit does and why the topology is designed this way. "+
				"Keep the existing Mermaid diagram and node table intact. "+
				"Only add explanatory prose above the auto-generated section.\n\n---\n\n%s",
			cfg.Manifest.Name, string(content))

		enriched, err := cfg.Enricher.Enrich(ctx, prompt)
		if err != nil {
			return fmt.Errorf("enrich %s: %w", e.Name(), err)
		}

		if err := os.WriteFile(path, []byte(enriched), 0o600); err != nil {
			return fmt.Errorf("write enriched %s: %w", e.Name(), err)
		}
	}

	return nil
}
