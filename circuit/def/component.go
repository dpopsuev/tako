package def

// Category: DSL & Build — component manifest (YAML-level types only).
// The live Component struct with runtime registries stays in the root package.

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

const (
	kindSchematic = "Schematic" // K8s-style kind value (capitalized)
	kindComponent = "Component" // K8s-style kind value (capitalized)
	wireFactory   = "factory"
)

// SocketDef declares a typed dependency slot that a schematic requires.
// Connectors satisfy sockets by declaring a matching factory in their manifest.
type SocketDef struct {
	Name        string `yaml:"name"`
	Type        string `yaml:"type"`
	Option      string `yaml:"option,omitempty"` // With* function name on the server
	Description string `yaml:"description,omitempty"`
	Schematic   string `yaml:"schematic,omitempty"` // if set, satisfied by another schematic
	Optional    bool   `yaml:"optional,omitempty"`  // true if socket has a built-in default
}

// GivesDef declares that a component provides a factory for a named socket.
type GivesDef struct {
	Socket  string `yaml:"socket"`
	Factory string `yaml:"factory"`
	Wire    string `yaml:"wire,omitempty"` // "instance" (default): call factory, pass result; "factory": pass function reference
}

// WireMode returns the effective wire mode, defaulting to "instance".
func (g GivesDef) WireMode() string {
	if g.Wire == wireFactory {
		return wireFactory
	}
	return "instance"
}

// ComponentManifest is the YAML schema for component.yaml files.
type ComponentManifest struct {
	Kind        string `yaml:"kind"`
	Component   string `yaml:"component"`
	Module      string `yaml:"module"` // Go import path (e.g. github.com/dpopsuev/origami/connectors/rp)
	Namespace   string `yaml:"namespace"`
	Version     string `yaml:"version"`
	Description string `yaml:"description,omitempty"`
	Factory     string `yaml:"factory,omitempty"`  // schematic constructor (e.g. NewRouter, NewServer)
	Resolver    string `yaml:"resolver,omitempty"` // circuit overlay resolver function (e.g. SchematicResolver)
	Adapter     string `yaml:"adapter,omitempty"`  // optional adapter for subprocess mode
	Serve       string `yaml:"serve,omitempty"`    // path to serve command for subprocess mode
	Provides    struct {
		Transformers []string `yaml:"transformers,omitempty"`
		Extractors   []string `yaml:"extractors,omitempty"`
		Hooks        []string `yaml:"hooks,omitempty"`
	} `yaml:"provides"`
	Needs struct {
		Origami    string      `yaml:"origami,omitempty"`
		Transports []SocketDef `yaml:"transports,omitempty"` // Transport, Trigger
		Sources    []SocketDef `yaml:"sources,omitempty"`    // SourceReader, SourceCatalog
		Storage    []SocketDef `yaml:"storage,omitempty"`    // Driver
	} `yaml:"needs,omitempty"`
	Gives []GivesDef `yaml:"gives,omitempty"`

	// MCP server configuration — fold reads these to generate CircuitConfig.
	Params         []ParamDef  `yaml:"params,omitempty"`          // extra start_circuit parameters
	Schemas        []string    `yaml:"schemas,omitempty"`         // step schema paths (relative to domain FS)
	Report         string      `yaml:"report,omitempty"`          // report template YAML path
	Dispatch       DispatchDef `yaml:"dispatch,omitempty"`        // dispatch provider config
	SessionFactory string          `yaml:"session_factory,omitempty"` // Go symbol: "rca.Factory()"
	CustomKinds    []CustomKindDef `yaml:"custom_kinds,omitempty"`    // CRDs registered by this schematic
}

// CustomKindDef declares a custom kind that this schematic registers.
// The kind is scoped to the schematic's apiVersion namespace.
type CustomKindDef struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`
}

// ParamDef declares an extra parameter for MCP start_circuit.
type ParamDef struct {
	Name     string   `yaml:"name"`
	Type     string   `yaml:"type"`
	Desc     string   `yaml:"description,omitempty"`
	Required bool     `yaml:"required,omitempty"`
	Enum     []string `yaml:"enum,omitempty"`
}

// DispatchDef declares how the schematic dispatches LLM prompts.
type DispatchDef struct {
	Provider string `yaml:"provider,omitempty"` // "cli" (default), "http"
	Workers  int    `yaml:"workers,omitempty"`  // parallel workers (default 1)
	Timeout  string `yaml:"timeout,omitempty"`  // per-dispatch timeout (e.g. "5m")
}

// Forbidden socket types per needs section. Enforced — errors, not warnings.
// S40: transports must NOT contain SourceReader, SourceCatalog, Driver.
// S41: sources must NOT contain Transport, Trigger, Driver.
// S42: storage must NOT contain Transport, Trigger, SourceReader, SourceCatalog.
// Custom domain types (RunDiscoverer, DefectWriter, store.Store) are allowed
// in any section — the enforcement prevents obvious misplacement only.
var forbiddenSocketTypes = map[string]map[string]bool{
	"transports": {"SourceReader": true, "SourceCatalog": true, "Driver": true},
	"sources":    {"Transport": true, "Trigger": true, "Driver": true},
	"storage":    {"Transport": true, "Trigger": true, "SourceReader": true, "SourceCatalog": true},
}

func validateSocketTypes(path, section string, sockets []SocketDef) error {
	forbidden := forbiddenSocketTypes[section]
	for _, s := range sockets {
		if s.Type == "" {
			continue
		}
		if forbidden[s.Type] {
			return fmt.Errorf("%w: %s: socket %q has type %q which is not allowed in %s: section", ErrComponentManifest, path, s.Name, s.Type, section)
		}
	}
	return nil
}

// componentManifestYAML is the K8s-style YAML structure for component.yaml.
type componentManifestYAML struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name        string `yaml:"name"`
		Module      string `yaml:"module"`
		Namespace   string `yaml:"namespace"`
		Description string `yaml:"description,omitempty"`
	} `yaml:"metadata"`
	Spec struct {
		Version        string `yaml:"version,omitempty"`
		SessionFactory string `yaml:"session_factory,omitempty"`
		Hooks          string `yaml:"hooks,omitempty"` // deprecated: use session_factory
		Resolver       string `yaml:"resolver,omitempty"`
		Provides       struct {
			Transformers []string `yaml:"transformers,omitempty"`
			Extractors   []string `yaml:"extractors,omitempty"`
			Hooks        []string `yaml:"hooks,omitempty"`
		} `yaml:"provides,omitempty"`
		Needs struct {
			Origami    string      `yaml:"origami,omitempty"`
			Transports []SocketDef `yaml:"transports,omitempty"`
			Sources    []SocketDef `yaml:"sources,omitempty"`
			Storage    []SocketDef `yaml:"storage,omitempty"`
		} `yaml:"needs,omitempty"`
		Gives    []GivesDef  `yaml:"gives,omitempty"`
		Params   []ParamDef  `yaml:"params,omitempty"`
		Schemas  []string    `yaml:"schemas,omitempty"`
		Report   string      `yaml:"report,omitempty"`
		Dispatch    DispatchDef     `yaml:"dispatch,omitempty"`
		CustomKinds []CustomKindDef `yaml:"custom_kinds,omitempty"`
	} `yaml:"spec"`
}

// LoadComponentManifest reads and parses a component.yaml file.
// Accepts K8s-style format: apiVersion/kind/metadata/spec.
func LoadComponentManifest(path string) (*ComponentManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read component manifest %s: %w", path, err)
	}
	var raw componentManifestYAML
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse component manifest %s: %w", path, err)
	}
	if raw.APIVersion == "" {
		return nil, fmt.Errorf("%w: %s: apiVersion is required", ErrComponentManifest, path)
	}
	if raw.Kind != kindSchematic && raw.Kind != kindComponent {
		return nil, fmt.Errorf("%w: %s: kind must be 'Schematic' or 'Component', got %q", ErrComponentManifest, path, raw.Kind)
	}
	if raw.Metadata.Namespace == "" {
		return nil, fmt.Errorf("%w: %s: metadata.namespace is required", ErrComponentManifest, path)
	}

	// Prefer session_factory; fall back to deprecated hooks field.
	sf := raw.Spec.SessionFactory
	if sf == "" {
		sf = raw.Spec.Hooks
	}

	m := &ComponentManifest{
		Kind:           raw.Kind,
		Component:      raw.Metadata.Name,
		Module:         raw.Metadata.Module,
		Namespace:      raw.Metadata.Namespace,
		Version:        raw.Spec.Version,
		Description:    raw.Metadata.Description,
		Resolver:       raw.Spec.Resolver,
		Provides:       raw.Spec.Provides,
		Needs:          raw.Spec.Needs,
		Gives:          raw.Spec.Gives,
		Params:         raw.Spec.Params,
		Schemas:        raw.Spec.Schemas,
		Report:         raw.Spec.Report,
		Dispatch:       raw.Spec.Dispatch,
		SessionFactory: sf,
		CustomKinds:    raw.Spec.CustomKinds,
	}

	// S40-S42: enforce socket type constraints per needs section.
	for section, sockets := range map[string][]SocketDef{
		"transports": m.Needs.Transports,
		"sources":    m.Needs.Sources,
		"storage":    m.Needs.Storage,
	} {
		if err := validateSocketTypes(path, section, sockets); err != nil {
			return nil, err
		}
	}
	return m, nil
}
