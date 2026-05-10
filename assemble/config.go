package assemble

import (
	"fmt"
	"os"
	"time"

	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/agent/organ"
	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tako/organs/code"
	"github.com/dpopsuev/tako/organs/dialog"
	"gopkg.in/yaml.v3"
)

type BlueprintConfig struct {
	Model        string       `yaml:"model"`
	ModelWatcher string       `yaml:"model_watcher"`
	ModelThinker string       `yaml:"model_thinker"`
	Organs []string     `yaml:"organs"`
	Budget       BudgetConfig `yaml:"budget"`
	Config       ConfigValues `yaml:"config"`
	WorkDir      string       `yaml:"work_dir"`
}

type BudgetConfig struct {
	MaxTurns    int    `yaml:"max_turns"`
	TurnTimeout string `yaml:"turn_timeout"`
	MaxTokens   int    `yaml:"max_tokens"`
}

type ConfigValues struct {
	DistanceClose    float64 `yaml:"distance_close"`
	DistanceMid      float64 `yaml:"distance_mid"`
	RecollectionMin  float64 `yaml:"recollection_min"`
	UnmetDimMax      int     `yaml:"unmet_dim_max"`
	BackwardTurnLimit int    `yaml:"backward_turn_limit"`
}

func LoadBlueprint(path string) (*BlueprintConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load blueprint: %w", err)
	}
	var cfg BlueprintConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse blueprint: %w", err)
	}
	return &cfg, nil
}

func (bc *BlueprintConfig) ToBlueprint() Blueprint {
	bp := Blueprint{
		Model:        bc.Model,
		ModelWatcher: bc.ModelWatcher,
	}
	if bp.ModelWatcher == "" && bc.ModelThinker != "" {
		bp.Model = bc.ModelThinker
	}

	bp.Budget = cerebrum.Budget{
		MaxTurns:    bc.Budget.MaxTurns,
		MaxTokens:   bc.Budget.MaxTokens,
	}
	if bp.Budget.MaxTurns == 0 {
		bp.Budget.MaxTurns = 100
	}
	if bc.Budget.TurnTimeout != "" {
		d, err := time.ParseDuration(bc.Budget.TurnTimeout)
		if err == nil {
			bp.Budget.TurnTimeout = d
		}
	}
	if bp.Budget.TurnTimeout == 0 {
		bp.Budget.TurnTimeout = 30 * time.Second
	}

	cfg := reactivity.DefaultConfig
	if bc.Config.DistanceClose > 0 {
		cfg.DistanceClose = bc.Config.DistanceClose
	}
	if bc.Config.DistanceMid > 0 {
		cfg.DistanceMid = bc.Config.DistanceMid
	}
	if bc.Config.RecollectionMin > 0 {
		cfg.RecollectionMin = bc.Config.RecollectionMin
	}
	if bc.Config.UnmetDimMax > 0 {
		cfg.UnmetDimMax = bc.Config.UnmetDimMax
	}
	if bc.Config.BackwardTurnLimit > 0 {
		cfg.BackwardTurnLimit = bc.Config.BackwardTurnLimit
	}
	bp.Config = &cfg

	bp.Organs = resolveOrgans(bc.Organs, bc.WorkDir)

	return bp
}

func resolveOrgans(names []string, workDir string) []organ.Func {
	if workDir == "" {
		workDir = "."
	}
	caps := []organ.Func{organ.ControllerFunc(dialog.New())}
	for _, name := range names {
		switch name {
		case "code":
			caps = append(caps, code.Organs(workDir)...)
		}
	}
	return caps
}
