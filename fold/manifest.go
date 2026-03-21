// Package fold compiles a YAML manifest into a standalone Go binary.
// The manifest declares embedded assets and MCP server config.
// Fold generates a main.go for a domain-serve binary, then invokes go build.
package fold

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Manifest is the top-level origami.yaml schema.
type Manifest struct {
	Name        string                  `yaml:"name"`
	Description string                  `yaml:"description"`
	Version     string                  `yaml:"version"`
	Domains     []string                `yaml:"domains,omitempty"`
	DomainServe *DomainServeConfig      `yaml:"domain_serve,omitempty"`
	Schematics  map[string]SchematicRef `yaml:"schematics,omitempty"`
	Connectors  map[string]ConnectorRef `yaml:"connectors,omitempty"`
}

// SchematicRef declares a schematic component and its socket bindings.
// Path is relative to the Origami module root (locates component.yaml).
// Bindings maps socket name to the connector or schematic name that fills it.
type SchematicRef struct {
	Path     string            `yaml:"path"`
	Bindings map[string]string `yaml:"bindings,omitempty"`
}

// ConnectorRef declares a connector component.
// Path is relative to the Origami module root (locates component.yaml).
type ConnectorRef struct {
	Path string `yaml:"path"`
}

// DomainServeConfig controls generation of a domain data MCP server binary.
// When present, origami fold produces a binary (<name>-domain-serve)
// that embeds the specified directory and serves it via domainserve.New().
type DomainServeConfig struct {
	Port   int          `yaml:"port"`             // listen port (default 9300)
	Assets *AssetMap    `yaml:"assets,omitempty"` // keyed file map
	Store  *StoreConfig `yaml:"store,omitempty"`  // storage engine config
}

// StoreConfig declares the storage backend for the domain-serve binary.
type StoreConfig struct {
	Engine string `yaml:"engine"` // e.g. "sqlite"
	Schema string `yaml:"schema"` // path to schema file, included in AllPaths
}

// AssetMap declares domain files by section and key. Each map section
// (circuits, prompts, ...) maps a logical key to a file path relative
// to origami.yaml. Vocabulary and Store are promoted scalar fields.
// The Files section holds legacy singleton assets that don't belong
// to a typed section; new manifests should use the promoted fields.
type AssetMap struct {
	Circuits   map[string]string `yaml:"circuits,omitempty"`
	Prompts    map[string]string `yaml:"prompts,omitempty"`
	Schemas    map[string]string `yaml:"schemas,omitempty"`
	Scenarios  map[string]string `yaml:"scenarios,omitempty"`
	Scorecards map[string]string `yaml:"scorecards,omitempty"`
	Reports    map[string]string `yaml:"reports,omitempty"`
	Sources    map[string]string `yaml:"sources,omitempty"`
	Vocabulary string            `yaml:"vocabulary,omitempty"`
	Files      map[string]string `yaml:"files,omitempty"`
}

// AllPaths returns a deduplicated, sorted list of every file path
// referenced by the asset map.
func (a *AssetMap) AllPaths() []string {
	seen := make(map[string]struct{})
	for _, section := range a.allSections() {
		for _, p := range section {
			seen[p] = struct{}{}
		}
	}
	if a.Vocabulary != "" {
		seen[a.Vocabulary] = struct{}{}
	}
	paths := make([]string, 0, len(seen)+1)
	for p := range seen {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths
}

// Sections returns the named map sections as a map of section name to
// key-path pairs. Only non-nil sections are included.
// Files is excluded — use ScalarFiles() for singleton assets.
func (a *AssetMap) Sections() map[string]map[string]string {
	result := make(map[string]map[string]string)
	for name, section := range map[string]map[string]string{
		"circuits":   a.Circuits,
		"prompts":    a.Prompts,
		"schemas":    a.Schemas,
		"scenarios":  a.Scenarios,
		"scorecards": a.Scorecards,
		"reports":    a.Reports,
		"sources":    a.Sources,
	} {
		if len(section) > 0 {
			result[name] = section
		}
	}
	return result
}

// ScalarFiles returns singleton asset entries as a map of name to path.
// Includes both the legacy Files map and promoted scalar fields.
func (a *AssetMap) ScalarFiles() map[string]string {
	cp := make(map[string]string)
	for k, v := range a.Files {
		cp[k] = v
	}
	if a.Vocabulary != "" {
		cp["vocabulary"] = a.Vocabulary
	}
	if len(cp) == 0 {
		return nil
	}
	return cp
}

func (a *AssetMap) allSections() []map[string]string {
	return []map[string]string{
		a.Circuits, a.Prompts, a.Schemas, a.Scenarios,
		a.Scorecards, a.Reports, a.Sources, a.Files,
	}
}

// LoadManifest reads and parses an origami.yaml manifest file.
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	return ParseManifest(data)
}

// ParseManifest parses YAML bytes into a Manifest.
func ParseManifest(data []byte) (*Manifest, error) {
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	if m.Name == "" {
		return nil, fmt.Errorf("manifest: name is required")
	}
	if ds := m.DomainServe; ds != nil {
		if ds.Assets == nil {
			return nil, fmt.Errorf("domain_serve: assets is required")
		}
	}
	for name, s := range m.Schematics {
		if s.Path == "" {
			return nil, fmt.Errorf("schematic %q: path is required", name)
		}
	}
	for name, c := range m.Connectors {
		if c.Path == "" {
			return nil, fmt.Errorf("connector %q: path is required", name)
		}
	}
	return &m, nil
}

// HasBindings returns true when the manifest declares schematics
// and connectors for declarative wiring.
func (m *Manifest) HasBindings() bool {
	return len(m.Schematics) > 0
}

// domainSubdirs maps directory names found inside a domain to AssetMap sections.
var domainSubdirs = []struct {
	Dir     string
	Section string
}{
	{"scenarios", "scenarios"},
	{"sources", "sources"},
	{"datasets", "datasets"},
	{"tuning", "tuning"},
}

// domainRecursiveDirs are directories inside a domain that are walked
// recursively and embedded with their full relative path structure.
var domainRecursiveDirs = []string{"offline"}

// domainFiles maps individual files found inside a domain to AssetMap.Files keys.
var domainFiles = []string{"heuristics.yaml"}

// MergeDiscoveredAssets scans each domain directory and merges discovered files
// into the AssetMap. Files are registered with flat paths (e.g., "scenarios/x.yaml")
// so the embedded FS layout matches what the runtime expects. A separate
// copyDomainFiles step handles the physical->flat copy during fold.
func (m *Manifest) MergeDiscoveredAssets(manifestDir string) error {
	if len(m.Domains) == 0 || m.DomainServe == nil || m.DomainServe.Assets == nil {
		return nil
	}
	a := m.DomainServe.Assets

	for _, domain := range m.Domains {
		domainDir := filepath.Join(manifestDir, "domains", domain)
		info, err := os.Stat(domainDir)
		if err != nil || !info.IsDir() {
			return fmt.Errorf("domain %q: directory domains/%s/ not found", domain, domain)
		}

		for _, sub := range domainSubdirs {
			subDir := filepath.Join(domainDir, sub.Dir)
			entries, err := os.ReadDir(subDir)
			if err != nil {
				continue
			}
			for _, e := range entries {
				if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
					continue
				}
				key := strings.TrimSuffix(e.Name(), ".yaml")
				flatPath := sub.Dir + "/" + e.Name()
				switch sub.Section {
				case "scenarios":
					if a.Scenarios == nil {
						a.Scenarios = make(map[string]string)
					}
					a.Scenarios[key] = flatPath
				case "sources":
					if a.Sources == nil {
						a.Sources = make(map[string]string)
					}
					a.Sources[key] = flatPath
				default:
					if a.Files == nil {
						a.Files = make(map[string]string)
					}
					a.Files[sub.Section+"/"+key] = flatPath
				}
			}
		}

		for _, f := range domainFiles {
			fPath := filepath.Join(domainDir, f)
			if _, err := os.Stat(fPath); err == nil {
				key := strings.TrimSuffix(f, ".yaml")
				if a.Files == nil {
					a.Files = make(map[string]string)
				}
				a.Files[key] = f
			}
		}

		for _, rd := range domainRecursiveDirs {
			rdPath := filepath.Join(domainDir, rd)
			if info, err := os.Stat(rdPath); err != nil || !info.IsDir() {
				continue
			}
			filepath.Walk(rdPath, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return nil
				}
				rel, _ := filepath.Rel(domainDir, path)
				flatPath := filepath.ToSlash(rel)
				key := flatPath
				if a.Files == nil {
					a.Files = make(map[string]string)
				}
				a.Files[key] = flatPath
				return nil
			})
		}
	}
	return nil
}

// domainPathMappings returns physical-source -> flat-embed path mappings
// for all domain-discovered files. Used by copyDomainFiles.
func (m *Manifest) domainPathMappings(manifestDir string) map[string]string {
	mappings := make(map[string]string)
	if len(m.Domains) == 0 {
		return mappings
	}
	for _, domain := range m.Domains {
		domainDir := filepath.Join(manifestDir, "domains", domain)

		for _, sub := range domainSubdirs {
			subDir := filepath.Join(domainDir, sub.Dir)
			entries, err := os.ReadDir(subDir)
			if err != nil {
				continue
			}
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				src := filepath.Join(subDir, e.Name())
				dst := sub.Dir + "/" + e.Name()
				mappings[src] = dst
			}
		}

		for _, f := range domainFiles {
			src := filepath.Join(domainDir, f)
			if _, err := os.Stat(src); err == nil {
				mappings[src] = f
			}
		}

		for _, rd := range domainRecursiveDirs {
			rdPath := filepath.Join(domainDir, rd)
			if info, err := os.Stat(rdPath); err != nil || !info.IsDir() {
				continue
			}
			filepath.Walk(rdPath, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return nil
				}
				rel, _ := filepath.Rel(domainDir, path)
				flatPath := filepath.ToSlash(rel)
				mappings[path] = flatPath
				return nil
			})
		}
	}
	return mappings
}
