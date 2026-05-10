package assemble

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadBlueprint_YAML(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `
model: claude-sonnet-4-6
organs:
  - code
budget:
  max_turns: 50
  turn_timeout: "60s"
config:
  distance_close: 0.2
  distance_mid: 0.4
work_dir: /tmp/test
`
	path := filepath.Join(dir, "blueprint.yaml")
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadBlueprint(path)
	if err != nil {
		t.Fatalf("LoadBlueprint: %v", err)
	}

	if cfg.Model != "claude-sonnet-4-6" {
		t.Errorf("model: got %s, want claude-sonnet-4-6", cfg.Model)
	}
	if len(cfg.Organs) != 1 || cfg.Organs[0] != "code" {
		t.Errorf("organs: got %v, want [code]", cfg.Organs)
	}
	if cfg.Budget.MaxTurns != 50 {
		t.Errorf("max_turns: got %d, want 50", cfg.Budget.MaxTurns)
	}
	if cfg.Config.DistanceClose != 0.2 {
		t.Errorf("distance_close: got %v, want 0.2", cfg.Config.DistanceClose)
	}
}

func TestBlueprintConfig_ToBlueprint(t *testing.T) {
	cfg := BlueprintConfig{
		Model:        "claude-sonnet-4-6",
		Organs: []string{"code"},
		Budget: BudgetConfig{
			MaxTurns:    25,
			TurnTimeout: "15s",
		},
		Config: ConfigValues{
			DistanceClose: 0.15,
		},
	}

	bp := cfg.ToBlueprint()

	if bp.Model != "claude-sonnet-4-6" {
		t.Errorf("model: got %s", bp.Model)
	}
	if bp.Budget.MaxTurns != 25 {
		t.Errorf("max_turns: got %d, want 25", bp.Budget.MaxTurns)
	}
	if bp.Budget.TurnTimeout.Seconds() != 15 {
		t.Errorf("turn_timeout: got %v, want 15s", bp.Budget.TurnTimeout)
	}
	if bp.Config.DistanceClose != 0.15 {
		t.Errorf("distance_close: got %v, want 0.15", bp.Config.DistanceClose)
	}
	if len(bp.Organs) < 10 {
		t.Errorf("expected 10+ capabilities from 'code' bundle, got %d", len(bp.Organs))
	}
}

func TestLoadBlueprint_StoredFiles(t *testing.T) {
	files := []struct {
		path      string
		model     string
		minTurns  int
	}{
		{"../blueprints/code.yaml", "claude-sonnet-4-6", 30},
		{"../blueprints/explore.yaml", "claude-haiku-4-5-20251001", 10},
		{"../blueprints/plan.yaml", "claude-sonnet-4-6", 15},
		{"../blueprints/general.yaml", "claude-sonnet-4-6", 20},
	}

	for _, f := range files {
		t.Run(filepath.Base(f.path), func(t *testing.T) {
			cfg, err := LoadBlueprint(f.path)
			if err != nil {
				t.Fatalf("LoadBlueprint(%s): %v", f.path, err)
			}
			if cfg.Model != f.model {
				t.Errorf("model: got %s, want %s", cfg.Model, f.model)
			}
			if cfg.Budget.MaxTurns != f.minTurns {
				t.Errorf("max_turns: got %d, want %d", cfg.Budget.MaxTurns, f.minTurns)
			}
			bp := cfg.ToBlueprint()
			if len(bp.Organs) < 10 {
				t.Errorf("organs: got %d, want 10+", len(bp.Organs))
			}
		})
	}
}

func TestBlueprintConfig_Defaults(t *testing.T) {
	cfg := BlueprintConfig{
		Model:        "test",
		Organs: []string{"code"},
	}

	bp := cfg.ToBlueprint()

	if bp.Budget.MaxTurns != 100 {
		t.Errorf("default max_turns: got %d, want 100", bp.Budget.MaxTurns)
	}
	if bp.Budget.TurnTimeout.Seconds() != 30 {
		t.Errorf("default turn_timeout: got %v, want 30s", bp.Budget.TurnTimeout)
	}
	if bp.Config.DistanceClose != 0.3 {
		t.Errorf("default distance_close: got %v, want 0.3", bp.Config.DistanceClose)
	}
}
