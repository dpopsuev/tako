package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dpopsuev/tangle/providers"
	"gopkg.in/yaml.v3"
)

type takoConfig struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model,omitempty"`
}

func loadConfig() *takoConfig {
	data, err := os.ReadFile(configFile())
	if err != nil {
		return nil
	}
	var cfg takoConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil
	}
	return &cfg
}

func saveConfig(cfg takoConfig) error {
	path := configFile()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func resolveProvider(flagProvider string) string {
	if flagProvider != "" {
		return flagProvider
	}
	if env := os.Getenv("TAKO_PROVIDER"); env != "" {
		return env
	}
	if cfg := loadConfig(); cfg != nil && cfg.Provider != "" {
		return cfg.Provider
	}
	return ""
}

func interactiveSetup() (string, error) {
	names := providers.ProviderNames()

	fmt.Println("\nNo LLM provider configured.")
	fmt.Println("Select a provider:")
	for i, name := range names {
		fmt.Printf("  %d. %s\n", i+1, name)
	}
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Choice [1-%d]: ", len(names))
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("setup: %w", err)
	}

	choice, err := strconv.Atoi(strings.TrimSpace(input))
	if err != nil || choice < 1 || choice > len(names) {
		return "", fmt.Errorf("invalid choice: %s", strings.TrimSpace(input))
	}

	provider := names[choice-1]

	if err := saveConfig(takoConfig{Provider: provider}); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not save config: %v\n", err)
	} else {
		fmt.Printf("\nSaved to %s\n", configFile())
	}

	return provider, nil
}
