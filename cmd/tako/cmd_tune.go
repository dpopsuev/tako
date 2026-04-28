package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dpopsuev/tako/engine"
	"github.com/dpopsuev/tako/fold"
	"gopkg.in/yaml.v3"
)

func tuneCmd(args []string) error {
	fs := flag.NewFlagSet("tune", flag.ContinueOnError)
	sum := fs.Bool("sum", false, "compute and write binary checksums to instrument manifests")
	verbose := fs.Bool("v", false, "verbose output")
	if err := fs.Parse(args); err != nil {
		return err
	}

	manifest := "tako.yaml"
	if fs.NArg() > 0 {
		manifest = fs.Arg(0)
	}

	data, err := os.ReadFile(manifest)
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}

	// Try board manifest first, fall back to legacy.
	var instruments map[string]string
	bm, bmErr := fold.ParseBoardManifest(data)
	if bmErr == nil {
		instruments = bm.Instruments
	} else {
		m, mErr := fold.ParseManifest(data)
		if mErr != nil {
			return fmt.Errorf("parse manifest: %w", mErr)
		}
		instruments = m.Instruments
	}

	if len(instruments) == 0 {
		fmt.Fprintln(os.Stderr, "no instruments declared in manifest")
		return nil
	}

	baseDir := filepath.Dir(manifest)
	loaded, err := fold.LoadInstruments(instruments, baseDir)
	if err != nil {
		return err
	}

	// Build registry for TuneAll.
	reg := make(engine.ManifestRegistry, len(loaded))
	for _, inst := range loaded {
		reg[inst.Name] = inst.Manifest
	}

	// Run preflight tune.
	if err := engine.TuneAll(context.Background(), reg, ""); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "all %d instruments tuned successfully\n", len(loaded))

	if !*sum {
		return nil
	}

	// Compute and write checksums.
	for _, inst := range loaded {
		cs, err := engine.ComputeChecksum(inst.Manifest)
		if err != nil {
			return fmt.Errorf("instrument %s: %w", inst.Name, err)
		}

		if inst.Manifest.Checksum == cs {
			if *verbose {
				fmt.Fprintf(os.Stderr, "%s: checksum unchanged\n", inst.Name)
			}
			continue
		}

		absPath := filepath.Join(baseDir, inst.Path)
		if err := writeChecksumToManifest(absPath, cs); err != nil {
			return fmt.Errorf("instrument %s: %w", inst.Name, err)
		}
		fmt.Fprintf(os.Stderr, "%s: checksum updated → %s\n", inst.Name, cs)
	}

	return nil
}

// writeChecksumToManifest reads an instrument YAML, sets spec.checksum, and writes it back.
func writeChecksumToManifest(path, checksum string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parse YAML: %w", err)
	}

	if err := setYAMLField(&doc, []string{"spec", "checksum"}, checksum); err != nil {
		return fmt.Errorf("set checksum: %w", err)
	}

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return fmt.Errorf("marshal YAML: %w", err)
	}
	return os.WriteFile(path, out, 0o600)
}

// setYAMLField sets a nested field in a yaml.Node tree.
func setYAMLField(doc *yaml.Node, path []string, value string) error {
	node := doc
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		node = node.Content[0]
	}

	for i, key := range path {
		if node.Kind != yaml.MappingNode {
			return fmt.Errorf("%w: expected mapping at %q", errYAMLPath, key)
		}
		found := false
		for j := 0; j+1 < len(node.Content); j += 2 {
			if node.Content[j].Value == key {
				if i == len(path)-1 {
					node.Content[j+1].Value = value
					node.Content[j+1].Tag = "!!str"
					node.Content[j+1].Kind = yaml.ScalarNode
					return nil
				}
				node = node.Content[j+1]
				found = true
				break
			}
		}
		if !found {
			if i == len(path)-1 {
				// Append new key-value pair.
				node.Content = append(node.Content,
					&yaml.Node{Kind: yaml.ScalarNode, Value: key, Tag: "!!str"},
					&yaml.Node{Kind: yaml.ScalarNode, Value: value, Tag: "!!str"},
				)
				return nil
			}
			return fmt.Errorf("%w: %q", errYAMLPath, key)
		}
	}
	return nil
}
