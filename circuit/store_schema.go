package circuit

// Category: DSL & Build — store schema definitions.

import "fmt"

// StoreWiring declares how store engines are wired to schematics.
// Consumers declare this in origami.yaml to assign engines per schematic
// or per named store.
type StoreWiring struct {
	Default string                  `yaml:"default,omitempty"` // default engine (e.g. "sqlite")
	Stores  map[string]StoreBinding `yaml:"stores,omitempty"`  // named store -> engine binding
}

// StoreBinding maps a named store to a specific engine and optional config.
type StoreBinding struct {
	Engine string            `yaml:"engine"`           // e.g. "sqlite", "memory", "postgres"
	Config map[string]string `yaml:"config,omitempty"` // engine-specific config
}

// StoreLifecycle controls creation and disposal of a named store.
type StoreLifecycle string

const (
	LifecycleSession    StoreLifecycle = "session"    // created per circuit walk, disposed after
	LifecyclePersistent StoreLifecycle = "persistent" // survives across circuit walks
)

// StoreDeclaration is a named store with lifecycle and schema reference.
type StoreDeclaration struct {
	Name      string         `yaml:"name"`
	Lifecycle StoreLifecycle `yaml:"lifecycle"`
	Schema    string         `yaml:"schema,omitempty"` // reference to a store-schema
}

// StoreEngine is the generic interface that storage adapters implement.
// Each engine (sqlite, memory, etc.) provides this interface.
type StoreEngine interface {
	Name() string
	Open(config map[string]string) error
	Close() error
	Migrate(schema *StoreSchema) error
}

// StoreSchema is parsed from kind: store-schema YAML files.
type StoreSchema struct {
	Kind          string                   `yaml:"kind,omitempty"`
	Version       string                   `yaml:"version"`
	SchemaVersion int                      `yaml:"schema_version"`
	Import        string                   `yaml:"import,omitempty"`
	Extend        bool                     `yaml:"extend,omitempty"`
	Tables        map[string]StoreTableDef `yaml:"tables"`
	Stores        []StoreDeclaration       `yaml:"stores,omitempty"`
}

// StoreTableDef defines a table in the store schema.
type StoreTableDef struct {
	Columns    []StoreColumnDef `yaml:"columns"`
	PrimaryKey []string         `yaml:"primary_key,omitempty"`
	Indexes    []StoreIndexDef  `yaml:"indexes,omitempty"`
}

// StoreColumnDef defines a column in a table.
type StoreColumnDef struct {
	Name     string `yaml:"name"`
	Type     string `yaml:"type"`
	Required bool   `yaml:"required,omitempty"`
	Default  string `yaml:"default,omitempty"`
}

// StoreIndexDef defines an index on a table.
type StoreIndexDef struct {
	Name    string   `yaml:"name"`
	Columns []string `yaml:"columns"`
	Unique  bool     `yaml:"unique,omitempty"`
}

// SchemaProvider supplies a store schema.
type SchemaProvider interface {
	StoreSchema() (*StoreSchema, error)
}

// LoadStoreSchema parses YAML bytes into a StoreSchema.
func LoadStoreSchema(data []byte) (*StoreSchema, error) {
	var s StoreSchema
	if err := yamlUnmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse store schema: %w", err)
	}
	if s.Tables == nil {
		s.Tables = make(map[string]StoreTableDef)
	}
	return &s, nil
}

// MergeStoreSchemas merges a base schema with an overlay.
// When overlay.Extend is true, the overlay may add new columns to existing tables
// and add entirely new tables. The overlay cannot remove or redefine existing columns.
// SchemaVersion from the overlay wins if set (non-zero).
// When overlay.Import is empty, the overlay is returned as-is (no merge).
func MergeStoreSchemas(base, overlay *StoreSchema) (*StoreSchema, error) {
	if overlay == nil {
		return base, nil
	}
	if overlay.Import == "" {
		return overlay, nil
	}
	if !overlay.Extend {
		return overlay, nil
	}
	if base == nil {
		return overlay, nil
	}

	merged := &StoreSchema{
		Version:       overlay.Version,
		SchemaVersion: overlay.SchemaVersion,
		Import:        overlay.Import,
		Extend:        overlay.Extend,
		Tables:        make(map[string]StoreTableDef),
		Stores:        mergeStores(base.Stores, overlay.Stores),
	}
	if merged.SchemaVersion == 0 {
		merged.SchemaVersion = base.SchemaVersion
	}
	if merged.Version == "" {
		merged.Version = base.Version
	}

	// Copy base tables
	for name, tbl := range base.Tables {
		merged.Tables[name] = copyTableDef(tbl)
	}

	// Apply overlay
	for name, overlayTbl := range overlay.Tables {
		baseTbl, exists := merged.Tables[name]
		if !exists {
			merged.Tables[name] = copyTableDef(overlayTbl)
			continue
		}
		mergedTbl, err := mergeTable(baseTbl, overlayTbl)
		if err != nil {
			return nil, fmt.Errorf("table %q: %w", name, err)
		}
		merged.Tables[name] = mergedTbl
	}

	return merged, nil
}

func mergeStores(base, overlay []StoreDeclaration) []StoreDeclaration {
	byName := make(map[string]StoreDeclaration)
	for _, s := range base {
		byName[s.Name] = s
	}
	for _, s := range overlay {
		byName[s.Name] = s
	}
	if len(byName) == 0 {
		return nil
	}
	// Preserve order: base first, then overlay (overlay overwrites)
	seen := make(map[string]bool)
	out := make([]StoreDeclaration, 0, len(byName))
	for _, s := range base {
		out = append(out, byName[s.Name])
		seen[s.Name] = true
	}
	for _, s := range overlay {
		if !seen[s.Name] {
			out = append(out, s)
			seen[s.Name] = true
		}
	}
	return out
}

func copyTableDef(t StoreTableDef) StoreTableDef {
	cp := StoreTableDef{
		Columns:    make([]StoreColumnDef, len(t.Columns)),
		PrimaryKey: append([]string{}, t.PrimaryKey...),
		Indexes:    make([]StoreIndexDef, len(t.Indexes)),
	}
	copy(cp.Columns, t.Columns)
	copy(cp.Indexes, t.Indexes)
	return cp
}

func mergeTable(base, overlay StoreTableDef) (StoreTableDef, error) {
	baseCols := make(map[string]StoreColumnDef)
	for _, c := range base.Columns {
		baseCols[c.Name] = c
	}

	for _, oc := range overlay.Columns {
		bc, exists := baseCols[oc.Name]
		if exists {
			if bc.Type != oc.Type || bc.Required != oc.Required || bc.Default != oc.Default {
				return StoreTableDef{}, fmt.Errorf("%w: %q", ErrCannotRedefineExistingColumn, oc.Name)
			}
			continue
		}
		baseCols[oc.Name] = oc
	}

	cols := make([]StoreColumnDef, 0, len(baseCols))
	seen := make(map[string]bool)
	for _, c := range base.Columns {
		cols = append(cols, c)
		seen[c.Name] = true
	}
	for _, c := range overlay.Columns {
		if !seen[c.Name] {
			cols = append(cols, c)
			seen[c.Name] = true
		}
	}

	// PrimaryKey: overlay wins if set, else base
	pk := base.PrimaryKey
	if len(overlay.PrimaryKey) > 0 {
		pk = overlay.PrimaryKey
	}

	// Indexes: merge (base + overlay new indexes)
	idxMap := make(map[string]StoreIndexDef)
	for _, i := range base.Indexes {
		idxMap[i.Name] = i
	}
	for _, i := range overlay.Indexes {
		idxMap[i.Name] = i
	}
	indexes := make([]StoreIndexDef, 0, len(idxMap))
	for _, i := range idxMap {
		indexes = append(indexes, i)
	}

	return StoreTableDef{
		Columns:    cols,
		PrimaryKey: pk,
		Indexes:    indexes,
	}, nil
}
