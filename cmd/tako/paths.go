package main

import (
	"os"
	"path/filepath"
)

func configDir() string {
	if d := os.Getenv("TAKO_CONFIG_DIR"); d != "" {
		return d
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "tako")
}

func dataDir() string {
	if d := os.Getenv("TAKO_DATA_DIR"); d != "" {
		return d
	}
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "tako")
}

func configFile() string {
	return filepath.Join(configDir(), "config.yaml")
}

func projectDir() string {
	return ".tako"
}

func projectBlueprint() string {
	return filepath.Join(projectDir(), "blueprint.yaml")
}
