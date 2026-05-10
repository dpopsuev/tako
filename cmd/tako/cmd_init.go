package main

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/dpopsuev/tako/assemble"
)

func initCmd(args []string) error {
	cfgPath := configFile()
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		provider, err := interactiveSetup()
		if err != nil {
			return err
		}
		fmt.Printf("Provider: %s\n", provider)
	} else {
		cfg := loadConfig()
		if cfg != nil {
			fmt.Printf("Config exists: %s (provider: %s)\n", cfgPath, cfg.Provider)
		}
	}

	bpPath := projectBlueprint()
	if _, err := os.Stat(bpPath); err == nil {
		fmt.Printf("Blueprint exists: %s\n", bpPath)
		return nil
	}

	wd, _ := os.Getwd()
	bp := assemble.BlueprintConfig{
		Model:        resolveModel(),
		Organs: []string{"code"},
		WorkDir:      wd,
		Budget: assemble.BudgetConfig{
			MaxTurns:    30,
			TurnTimeout: "120s",
		},
	}

	if err := os.MkdirAll(filepath.Dir(bpPath), 0o750); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	data, err := yaml.Marshal(&bp)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	if err := os.WriteFile(bpPath, data, 0o644); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	fmt.Printf("Blueprint: %s\n", bpPath)
	return nil
}
