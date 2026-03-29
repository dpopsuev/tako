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

const kindBoard = "Board"

// Manifest is the top-level origami.yaml schema.
// YAML format follows K8s pattern: apiVersion/kind/metadata/spec.
// Go struct keeps flat fields for internal convenience.
type Manifest struct {
	// Parsed directly from YAML.
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`

	// Flat fields — populated by ParseManifest from the nested YAML.
	Name        string
	Description string
	Version     string
	Domains     []string
	DomainServe *DomainServeConfig
	Uses        map[string]UsesRef
	Bind        map[string]map[string]string
	Params      []ParamDef

	// Bridge fields — populated by bridgeUsesToLegacy from Uses/Bind.
	Schematics map[string]SchematicRef
	Connectors map[string]ConnectorRef
}

// manifestYAML is the K8s-style YAML structure for unmarshaling.
type manifestYAML struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description,omitempty"`
	} `yaml:"metadata"`
	Spec struct {
		Domains     []string                     `yaml:"domains,omitempty"`
		DomainServe *DomainServeConfig           `yaml:"domain_serve,omitempty"`
		Uses        map[string]UsesRef           `yaml:"uses,omitempty"`
		Bind        map[string]map[string]string `yaml:"bind,omitempty"`
		Params      []ParamDef                   `yaml:"params,omitempty"`
	} `yaml:"spec"`
}

// UsesRef declares a schematic or component used by this board.
type UsesRef struct {
	Kind   string `yaml:"kind"`
	Module string `yaml:"module"`
}

// SchematicRef is a resolved schematic with socket bindings.
type SchematicRef struct {
	Path     string
	Bindings map[string]string
}

// ConnectorRef is a resolved connector.
type ConnectorRef struct {
	Path string
}

// ParamDef declares an extra parameter for the circuit start tool.
type ParamDef struct {
	Name        string   `yaml:"name"`
	Type        string   `yaml:"type"`
	Description string   `yaml:"description,omitempty"`
	Enum        []string `yaml:"enum,omitempty"`
}

// DomainServeConfig controls generation of a domain data MCP server binary.
type DomainServeConfig struct {
	Port   int          `yaml:"port"`
	Assets *AssetMap    `yaml:"assets,omitempty"`
	Store  *StoreConfig `yaml:"store,omitempty"`
}

// StoreConfig declares the storage backend.
type StoreConfig struct {
	Engine string `yaml:"engine"`
	Schema string `yaml:"schema"`
}

// AssetMap declares domain files by section and key.
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

// AllPaths returns a deduplicated, sorted list of every file path.
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

// Sections returns named map sections.
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

// ScalarFiles returns singleton asset entries.
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

// ParseManifest parses K8s-style YAML into a flat Manifest.
func ParseManifest(data []byte) (*Manifest, error) {
	var raw manifestYAML
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	if raw.APIVersion != "origami/v1" {
		return nil, fmt.Errorf("%w: %q", ErrManifestApiVersionMustBeOrigamiV1Got, raw.APIVersion)
	}
	if raw.Kind != kindBoard {
		return nil, fmt.Errorf("%w: %q", ErrManifestKindMustBeBoardGot, raw.Kind)
	}
	if raw.Metadata.Name == "" {
		return nil, ErrManifestMetadataNameIsRequired
	}

	m := &Manifest{
		APIVersion:  raw.APIVersion,
		Kind:        raw.Kind,
		Name:        raw.Metadata.Name,
		Description: raw.Metadata.Description,
		Domains:     raw.Spec.Domains,
		DomainServe: raw.Spec.DomainServe,
		Uses:        raw.Spec.Uses,
		Bind:        raw.Spec.Bind,
		Params:      raw.Spec.Params,
	}

	if ds := m.DomainServe; ds != nil {
		if ds.Assets == nil {
			return nil, ErrDomainServeAssetsIsRequired
		}
	}
	for name, u := range m.Uses {
		if u.Module == "" {
			return nil, fmt.Errorf("%w: %q: module is required", ErrUses, name)
		}
	}
	for schematic, bindings := range m.Bind {
		if _, ok := m.Uses[schematic]; !ok {
			return nil, fmt.Errorf("%w: %q: not found in uses", ErrBind, schematic)
		}
		for _, component := range bindings {
			if _, ok := m.Uses[component]; !ok {
				return nil, fmt.Errorf("%w: %q: component %q not found in uses", ErrBind, schematic, component)
			}
		}
	}
	return m, nil
}

// HasBindings returns true when the manifest has uses or schematics.
func (m *Manifest) HasBindings() bool {
	return len(m.Schematics) > 0 || len(m.Uses) > 0
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

var domainRecursiveDirs = []string{"offline"}
var domainFiles = []string{"heuristics.yaml"}

// MergeDiscoveredAssets scans each domain directory and merges discovered files.
//
//nolint:gocyclo // walks multiple directory types per domain — inherently branchy
func (m *Manifest) MergeDiscoveredAssets(manifestDir string) error {
	if len(m.Domains) == 0 || m.DomainServe == nil || m.DomainServe.Assets == nil {
		return nil
	}
	a := m.DomainServe.Assets

	for _, domain := range m.Domains {
		domainDir := filepath.Join(manifestDir, "domains", domain)
		info, err := os.Stat(domainDir)
		if err != nil || !info.IsDir() {
			return fmt.Errorf("%w: %q: directory domains/%s/ not found", ErrDomain, domain, domain)
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
			_ = filepath.Walk(rdPath, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return nil
				}
				rel, _ := filepath.Rel(domainDir, path)
				flatPath := filepath.ToSlash(rel)
				if a.Files == nil {
					a.Files = make(map[string]string)
				}
				a.Files[flatPath] = flatPath
				return nil
			})
		}
	}
	return nil
}

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
			_ = filepath.Walk(rdPath, func(path string, info os.FileInfo, err error) error {
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
