package fold

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dpopsuev/tako/circuit"
)

const (
	useKindSchematic = "Schematic"
	wireInstance     = "instance"
)

// bridgeUsesToLegacy converts the new Uses/Bind DSL to the legacy
// Schematics/Connectors fields so the existing resolution pipeline works.
// This is a temporary bridge — once all consumers use Uses/Bind, the
// legacy fields and this function are deleted.
func bridgeUsesToLegacy(m *Manifest) {
	if m.Schematics == nil {
		m.Schematics = make(map[string]SchematicRef)
	}
	if m.Connectors == nil {
		m.Connectors = make(map[string]ConnectorRef)
	}
	for name, u := range m.Uses {
		switch u.Kind {
		case useKindSchematic:
			bindings := m.Bind[name] // may be nil
			m.Schematics[name] = SchematicRef{
				Path:     u.Module,
				Bindings: bindings,
			}
		case "Component", "":
			m.Connectors[name] = ConnectorRef{Path: u.Module}
		}
	}
}

// ResolvedConnector is a connector ready for codegen instantiation.
type ResolvedConnector struct {
	Name    string // manifest key (e.g. "datasource")
	Module  string // Go import path
	Alias   string // import alias for codegen
	Entries []ResolvedSatisfy
}

// ResolvedSatisfy is a single socket satisfaction from a connector.
type ResolvedSatisfy struct {
	Socket  string // socket name on the consuming schematic
	Factory string // factory function name (e.g. "NewSourceReader")
	Wire    string // "instance" or "factory"
}

// ResolvedSchematic is a schematic with all sockets resolved to providers.
type ResolvedSchematic struct {
	Name     string // manifest key (e.g. "beta", "alpha")
	Module   string // Go import path
	Alias    string // import alias for codegen
	Factory  string // constructor function (e.g. "NewRouter", "NewServer")
	Resolver string // circuit overlay resolver function (e.g. "SchematicResolver"), empty if none
	Options  []ResolvedOption

	// SessionFactory mode: when set, fold generates CircuitConfig inline
	// using the consumer's SessionFactory instead of calling Factory.
	SessionFactory string              // Go symbol: "alpha.Factory()" or "Factory()"
	Params         []circuit.ParamDef  // extra start_circuit parameters
	Schemas        []string            // step schema paths
	Report         string              // report template path
	Dispatch       circuit.DispatchDef // dispatch config
}

// ResolvedOption is one With* call on a schematic factory.
type ResolvedOption struct {
	OptionFunc string // With* function name (e.g. "WithGitDriver")
	Provider   string // variable or function reference (e.g. "gitDriver", "rp.NewSourceReader")
	Wire       string // "instance" or "factory"
}

// ResolvedGraph is the complete binding resolution, ordered for codegen.
// Instantiation order: Connectors (instance-mode only), then Schematics
// in dependency order, then Root.
type ResolvedGraph struct {
	Connectors []ResolvedConnector
	Schematics []ResolvedSchematic // sub-schematics in dependency order (excludes root)
	Root       ResolvedSchematic   // the primary schematic
	Imports    []ImportEntry       // deduplicated, sorted
}

// ImportEntry is a Go import for the generated binary.
type ImportEntry struct {
	Alias string
	Path  string
}

// Resolve reads component.yaml files for all declared schematics and
// connectors, matches socket bindings, validates completeness, detects
// cycles, and returns a topologically ordered instantiation plan.
func Resolve(m *Manifest, takoRoot string, resolver ModuleResolver) (*ResolvedGraph, error) {
	if !m.HasBindings() {
		return nil, ErrManifestHasNoSchematicsOrUsesSection
	}

	// Bridge: if Uses/Bind are set, populate legacy Schematics/Connectors
	// so the rest of the resolution pipeline works unchanged.
	if len(m.Uses) > 0 {
		bridgeUsesToLegacy(m)
	}

	connManifests := make(map[string]*circuit.ComponentManifest)
	schemManifests := make(map[string]*circuit.ComponentManifest)

	for name, ref := range m.Connectors {
		cmPath := resolveComponentPath(ref.Path, takoRoot, resolver)
		cm, err := circuit.LoadComponentManifest(cmPath)
		if err != nil {
			return nil, fmt.Errorf("connector %q: %w", name, err)
		}
		connManifests[name] = cm
	}

	for name, ref := range m.Schematics {
		cmPath := resolveComponentPath(ref.Path, takoRoot, resolver)
		cm, err := circuit.LoadComponentManifest(cmPath)
		if err != nil {
			return nil, fmt.Errorf("schematic %q: %w", name, err)
		}
		if cm.Factory == "" && cm.SessionFactory == "" && cm.Resolver == "" {
			return nil, fmt.Errorf("%w: %q: component.yaml must declare factory, session_factory, or resolver", ErrSchematic, name)
		}
		// Validate that declared symbols exist as exported Go functions.
		moduleDir := filepath.Dir(cmPath)
		if err := ValidateExports(cm, moduleDir); err != nil {
			return nil, fmt.Errorf("schematic %q: %w", name, err)
		}
		schemManifests[name] = cm
	}

	if err := detectCycles(m); err != nil {
		return nil, err
	}

	root, depOrder, err := topoSort(m)
	if err != nil {
		return nil, err
	}

	connIndex := buildConnectorIndex(connManifests)
	schemIndex := buildSchematicIndex(schemManifests)

	resolvedSchematics := make([]ResolvedSchematic, 0, len(depOrder))
	varNames := make(map[string]string) // schematic name -> variable name

	for _, name := range depOrder {
		rs, err := resolveSchematic(name, m, schemManifests[name], connIndex, schemIndex, varNames)
		if err != nil {
			return nil, err
		}
		varNames[name] = rs.Alias + "Instance"
		resolvedSchematics = append(resolvedSchematics, *rs)
	}

	rootRS, err := resolveSchematic(root, m, schemManifests[root], connIndex, schemIndex, varNames)
	if err != nil {
		return nil, err
	}

	resolvedConns := buildResolvedConnectors(connIndex)
	imports := collectImports(resolvedConns, resolvedSchematics, rootRS)

	return &ResolvedGraph{
		Connectors: resolvedConns,
		Schematics: resolvedSchematics,
		Root:       *rootRS,
		Imports:    imports,
	}, nil
}

type connectorEntry struct {
	name     string
	manifest *circuit.ComponentManifest
}

type schematicEntry struct {
	name     string
	manifest *circuit.ComponentManifest
}

func buildConnectorIndex(cms map[string]*circuit.ComponentManifest) map[string]connectorEntry {
	idx := make(map[string]connectorEntry, len(cms))
	for name, cm := range cms {
		idx[name] = connectorEntry{name: name, manifest: cm}
	}
	return idx
}

func buildSchematicIndex(cms map[string]*circuit.ComponentManifest) map[string]schematicEntry {
	idx := make(map[string]schematicEntry, len(cms))
	for name, cm := range cms {
		idx[name] = schematicEntry{name: name, manifest: cm}
	}
	return idx
}

func resolveSchematic(
	name string,
	m *Manifest,
	cm *circuit.ComponentManifest,
	connIdx map[string]connectorEntry,
	schemIdx map[string]schematicEntry,
	varNames map[string]string,
) (*ResolvedSchematic, error) {
	ref := m.Schematics[name]
	bindings := ref.Bindings

	options := make([]ResolvedOption, 0, len(cm.Needs.Transports)+len(cm.Needs.Sources)+len(cm.Needs.Storage))
	// Iterate all typed socket sections: transports, sources, storage.
	allSockets := make([]circuit.SocketDef, 0, len(cm.Needs.Transports)+len(cm.Needs.Sources)+len(cm.Needs.Storage))
	allSockets = append(allSockets, cm.Needs.Transports...)
	allSockets = append(allSockets, cm.Needs.Sources...)
	allSockets = append(allSockets, cm.Needs.Storage...)
	for _, sock := range allSockets {
		if sock.Option == "" {
			continue
		}

		boundTo, hasBound := bindings[sock.Name]
		if !hasBound {
			if sock.Optional {
				continue
			}
			return nil, fmt.Errorf("%w: %q: socket %q has no binding and is not optional", ErrSchematic, name, sock.Name)
		}

		opt, err := resolveSocketBinding(name, &sock, boundTo, connIdx, schemIdx, varNames)
		if err != nil {
			return nil, err
		}
		options = append(options, opt)
	}

	return &ResolvedSchematic{
		Name:           name,
		Module:         cm.Module,
		Alias:          importAlias(cm.Module),
		Factory:        cm.Factory,
		Resolver:       cm.Resolver,
		Options:        options,
		SessionFactory: cm.SessionFactory,
		Params:         cm.Params,
		Schemas:        cm.Schemas,
		Report:         cm.Report,
		Dispatch:       cm.Dispatch,
	}, nil
}

func resolveSocketBinding(
	name string, sock *circuit.SocketDef, boundTo string,
	connIdx map[string]connectorEntry, schemIdx map[string]schematicEntry, varNames map[string]string,
) (ResolvedOption, error) {
	if conn, ok := connIdx[boundTo]; ok {
		sat := findSatisfy(conn.manifest, sock.Name)
		if sat == nil {
			return ResolvedOption{}, fmt.Errorf("%w: %q socket %q: connector %q does not satisfy socket %q", ErrSchematic, name, sock.Name, boundTo, sock.Name)
		}
		alias := importAlias(conn.manifest.Module)
		provider := alias + "." + sat.Factory
		if sat.WireMode() == wireInstance {
			provider = varName(boundTo, sock.Name)
		}
		return ResolvedOption{
			OptionFunc: sock.Option,
			Provider:   provider,
			Wire:       sat.WireMode(),
		}, nil
	}
	if _, ok := schemIdx[boundTo]; ok {
		vn, exists := varNames[boundTo]
		if !exists {
			return ResolvedOption{}, fmt.Errorf("%w: %q socket %q: bound schematic %q not yet resolved (cycle?)", ErrSchematic, name, sock.Name, boundTo)
		}
		return ResolvedOption{
			OptionFunc: sock.Option,
			Provider:   vn,
			Wire:       wireInstance,
		}, nil
	}
	return ResolvedOption{}, fmt.Errorf("%w: %q socket %q: binding %q is neither a connector nor a schematic", ErrSchematic, name, sock.Name, boundTo)
}

func findSatisfy(cm *circuit.ComponentManifest, socketName string) *circuit.GivesDef {
	for i := range cm.Gives {
		if cm.Gives[i].Socket == socketName {
			return &cm.Gives[i]
		}
	}
	return nil
}

// topoSort finds the root schematic (not depended on by others) and
// returns the dependency order for the remaining sub-schematics.
func topoSort(m *Manifest) (root string, order []string, err error) {
	depended := make(map[string]bool)
	deps := make(map[string][]string)

	for name, ref := range m.Schematics {
		for _, boundTo := range ref.Bindings {
			if _, isSch := m.Schematics[boundTo]; isSch {
				deps[name] = append(deps[name], boundTo)
				depended[boundTo] = true
			}
		}
	}

	var roots []string
	for name := range m.Schematics {
		if !depended[name] {
			roots = append(roots, name)
		}
	}
	if len(roots) == 0 {
		return "", nil, ErrNoRootSchematicFoundAllSchematicsAreDependenciesOfOt
	}
	if len(roots) > 1 {
		// Board declares entry point via entry: true on a uses entry.
		root = pickEntrySchematic(roots, m.Uses)
		if root == "" {
			sort.Strings(roots)
			return "", nil, fmt.Errorf("%w: %s (set entry: true on one schematic in uses)", ErrMultipleRootSchematics, strings.Join(roots, ", "))
		}
	} else {
		root = roots[0]
	}

	visited := make(map[string]bool)
	var sorted []string
	var visit func(string) error
	visit = func(name string) error {
		if name == root {
			return nil
		}
		if visited[name] {
			return nil
		}
		visited[name] = true
		for _, dep := range deps[name] {
			if err := visit(dep); err != nil {
				return err
			}
		}
		sorted = append(sorted, name)
		return nil
	}

	for name := range m.Schematics {
		if name != root {
			if err := visit(name); err != nil {
				return "", nil, err
			}
		}
	}
	return root, sorted, nil
}

// pickEntrySchematic returns the single schematic with entry: true in the
// board's uses section. Returns "" if zero or multiple have it.
func pickEntrySchematic(roots []string, uses map[string]UsesRef) string {
	var entry string
	for _, name := range roots {
		if u, ok := uses[name]; ok && u.Entry {
			if entry != "" {
				return "" // multiple entry: true — ambiguous
			}
			entry = name
		}
	}
	return entry
}

// detectCycles checks for circular schematic dependencies.
func detectCycles(m *Manifest) error {
	const (
		white = 0
		grey  = 1
		black = 2
	)
	color := make(map[string]int)

	var dfs func(string) error
	dfs = func(name string) error {
		color[name] = grey
		ref, ok := m.Schematics[name]
		if !ok {
			color[name] = black
			return nil
		}
		for _, boundTo := range ref.Bindings {
			if _, isSch := m.Schematics[boundTo]; !isSch {
				continue
			}
			switch color[boundTo] {
			case grey:
				return fmt.Errorf("%w: %s -> %s", ErrCycleDetected, name, boundTo)
			case white:
				if err := dfs(boundTo); err != nil {
					return err
				}
			}
		}
		color[name] = black
		return nil
	}

	for name := range m.Schematics {
		if color[name] == white {
			if err := dfs(name); err != nil {
				return err
			}
		}
	}
	return nil
}

func buildResolvedConnectors(connIdx map[string]connectorEntry) []ResolvedConnector {
	var result []ResolvedConnector
	names := make([]string, 0, len(connIdx))
	for n := range connIdx {
		names = append(names, n)
	}
	sort.Strings(names)

	for _, name := range names {
		entry := connIdx[name]
		cm := entry.manifest
		var entries []ResolvedSatisfy
		for _, sat := range cm.Gives {
			if sat.WireMode() == wireInstance {
				entries = append(entries, ResolvedSatisfy{
					Socket:  sat.Socket,
					Factory: sat.Factory,
					Wire:    wireInstance,
				})
			}
		}
		if len(entries) > 0 {
			result = append(result, ResolvedConnector{
				Name:    name,
				Module:  cm.Module,
				Alias:   importAlias(cm.Module),
				Entries: entries,
			})
		}
	}
	return result
}

func collectImports(
	connectors []ResolvedConnector,
	schematics []ResolvedSchematic,
	root *ResolvedSchematic,
) []ImportEntry {
	seen := make(map[string]ImportEntry)
	add := func(mod string) {
		if mod == "" {
			return
		}
		if _, ok := seen[mod]; !ok {
			seen[mod] = ImportEntry{Alias: importAlias(mod), Path: mod}
		}
	}

	for i := range connectors {
		add(connectors[i].Module)
	}
	for i := range schematics {
		add(schematics[i].Module)
	}
	add(root.Module)

	result := make([]ImportEntry, 0, len(seen))
	for _, e := range seen {
		result = append(result, e)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Path < result[j].Path })
	return result
}

func importAlias(modulePath string) string {
	base := filepath.Base(modulePath)
	base = strings.ReplaceAll(base, "-", "")
	return base
}

func varName(connector, socket string) string {
	return sanitize(connector) + strings.ToUpper(socket[:1]) + socket[1:]
}

func sanitize(s string) string {
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, ".", "")
	return s
}

// resolveComponentPath resolves a SchematicRef/ConnectorRef path to the
// filesystem location of component.yaml. Supports both relative paths
// (e.g. "schematics/alpha") and module-qualified paths
// (e.g. "github.com/example/schematic-a").
func resolveComponentPath(refPath, takoRoot string, resolver ModuleResolver) string {
	// Module path: first segment contains a dot (e.g. "github.com").
	if parts := strings.SplitN(refPath, "/", 2); strings.Contains(parts[0], ".") && resolver != nil {
		// Try the full path as a module, then walk up to find the module root.
		// For "github.com/example/schematic-a/connectors/rp":
		//   try schematic-a/connectors/rp → no go.mod
		//   try schematic-a → has go.mod → subpath = connectors/rp
		segments := strings.Split(refPath, "/")
		for i := len(segments); i >= 3; i-- { // minimum: host/org/repo
			candidate := strings.Join(segments[:i], "/")
			if root := resolver.FindLocalModule(candidate); root != "" {
				subpath := ""
				if i < len(segments) {
					subpath = strings.Join(segments[i:], "/")
				}
				return filepath.Join(root, subpath, "component.yaml")
			}
		}
	}
	// Relative path: join with takoRoot (backward compatible).
	return filepath.Join(takoRoot, refPath, "component.yaml")
}
