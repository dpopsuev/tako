package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dpopsuev/origami/circuit"
)

func componentCmd(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: origami component <list|inspect|validate> [flags]")
	}
	switch args[0] {
	case "list":
		return componentList(args[1:])
	case "inspect":
		return componentInspect(args[1:])
	case "validate":
		return componentValidate(args[1:])
	default:
		return fmt.Errorf("unknown component subcommand: %s", args[0])
	}
}

func componentList(args []string) error {
	fs := flag.NewFlagSet("component list", flag.ContinueOnError)
	dir := fs.String("dir", ".", "directory to scan for component.yaml files")
	if err := fs.Parse(args); err != nil {
		return err
	}

	var manifests []*circuit.ComponentManifest
	_ = filepath.Walk(*dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.Name() == "component.yaml" {
			m, loadErr := circuit.LoadComponentManifest(path)
			if loadErr == nil {
				manifests = append(manifests, m)
			}
		}
		return nil
	})

	if len(manifests) == 0 {
		fmt.Println("No components found.")
		return nil
	}

	fmt.Printf("%-20s %-10s %-12s %s\n", "NAMESPACE", "VERSION", "COMPONENT", "PROVIDES")
	for _, m := range manifests {
		provides := make([]string, 0)
		if len(m.Provides.Transformers) > 0 {
			provides = append(provides, fmt.Sprintf("T:%s", strings.Join(m.Provides.Transformers, ",")))
		}
		if len(m.Provides.Extractors) > 0 {
			provides = append(provides, fmt.Sprintf("E:%s", strings.Join(m.Provides.Extractors, ",")))
		}
		if len(m.Provides.Hooks) > 0 {
			provides = append(provides, fmt.Sprintf("H:%s", strings.Join(m.Provides.Hooks, ",")))
		}
		fmt.Printf("%-20s %-10s %-12s %s\n", m.Namespace, m.Version, m.Component, strings.Join(provides, " "))
	}
	return nil
}

func componentInspect(args []string) error {
	fs := flag.NewFlagSet("component inspect", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return fmt.Errorf("usage: origami component inspect <component.yaml>")
	}

	m, err := circuit.LoadComponentManifest(fs.Arg(0))
	if err != nil {
		return err
	}

	fmt.Printf("Component:   %s\n", m.Component)
	fmt.Printf("Namespace:   %s\n", m.Namespace)
	fmt.Printf("Version:     %s\n", m.Version)
	if m.Description != "" {
		fmt.Printf("Description: %s\n", m.Description)
	}
	if m.Requires.Origami != "" {
		fmt.Printf("Requires:    origami %s\n", m.Requires.Origami)
	}
	if len(m.Provides.Transformers) > 0 {
		fmt.Printf("Transformers: %s\n", strings.Join(m.Provides.Transformers, ", "))
	}
	if len(m.Provides.Extractors) > 0 {
		fmt.Printf("Extractors:   %s\n", strings.Join(m.Provides.Extractors, ", "))
	}
	if len(m.Provides.Hooks) > 0 {
		fmt.Printf("Hooks:        %s\n", strings.Join(m.Provides.Hooks, ", "))
	}
	return nil
}

func componentValidate(args []string) error {
	fs := flag.NewFlagSet("component validate", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return fmt.Errorf("usage: origami component validate <component.yaml>")
	}

	path := fs.Arg(0)
	m, err := circuit.LoadComponentManifest(path)
	if err != nil {
		return err
	}

	var issues []string
	if m.Component == "" {
		issues = append(issues, "missing component name")
	}
	if m.Namespace == "" {
		issues = append(issues, "missing namespace")
	}
	if m.Version == "" {
		issues = append(issues, "missing version")
	}
	total := len(m.Provides.Transformers) + len(m.Provides.Extractors) + len(m.Provides.Hooks)
	if total == 0 {
		issues = append(issues, "provides section is empty")
	}

	if len(issues) > 0 {
		for _, issue := range issues {
			fmt.Fprintf(os.Stderr, "  ✗ %s\n", issue)
		}
		return fmt.Errorf("component manifest %s has %d issue(s)", path, len(issues))
	}

	fmt.Printf("OK: %s (%s/%s) is valid\n", path, m.Namespace, m.Component)
	return nil
}
