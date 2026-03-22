package circuit

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestLoadStoreSchema_BasicParsing(t *testing.T) {
	yaml := `
kind: store-schema
version: "1.0"
schema_version: 1
tables:
  users:
    columns:
      - name: id
        type: integer
        required: true
      - name: email
        type: text
        required: true
    primary_key: [id]
    indexes:
      - name: idx_users_email
        columns: [email]
        unique: true
`
	s, err := LoadStoreSchema([]byte(yaml))
	if err != nil {
		t.Fatalf("LoadStoreSchema: %v", err)
	}
	if s.Kind != "store-schema" {
		t.Errorf("Kind = %q, want store-schema", s.Kind)
	}
	if s.Version != "1.0" {
		t.Errorf("Version = %q, want 1.0", s.Version)
	}
	if s.SchemaVersion != 1 {
		t.Errorf("SchemaVersion = %d, want 1", s.SchemaVersion)
	}
	tbl, ok := s.Tables["users"]
	if !ok {
		t.Fatal("table users not found")
	}
	if len(tbl.Columns) != 2 {
		t.Errorf("columns len = %d, want 2", len(tbl.Columns))
	}
	if tbl.Columns[0].Name != "id" || tbl.Columns[0].Type != "integer" || !tbl.Columns[0].Required {
		t.Errorf("first column = %+v", tbl.Columns[0])
	}
	if len(tbl.PrimaryKey) != 1 || tbl.PrimaryKey[0] != "id" {
		t.Errorf("primary_key = %v", tbl.PrimaryKey)
	}
	if len(tbl.Indexes) != 1 || tbl.Indexes[0].Name != "idx_users_email" || !tbl.Indexes[0].Unique {
		t.Errorf("indexes = %+v", tbl.Indexes)
	}
}

func TestMergeStoreSchemas_ExtendAddsColumns(t *testing.T) {
	base := &StoreSchema{
		Version:       "1.0",
		SchemaVersion: 1,
		Tables: map[string]StoreTableDef{
			"users": {
				Columns: []StoreColumnDef{
					{Name: "id", Type: "integer", Required: true},
					{Name: "email", Type: "text", Required: true},
				},
				PrimaryKey: []string{"id"},
			},
		},
	}
	overlay := &StoreSchema{
		Version:       "1.1",
		SchemaVersion: 2,
		Import:        "base",
		Extend:        true,
		Tables: map[string]StoreTableDef{
			"users": {
				Columns: []StoreColumnDef{
					{Name: "name", Type: "text", Required: false},
					{Name: "created_at", Type: "text", Required: false, Default: "''"},
				},
			},
		},
	}
	merged, err := MergeStoreSchemas(base, overlay)
	if err != nil {
		t.Fatalf("MergeStoreSchemas: %v", err)
	}
	tbl, ok := merged.Tables["users"]
	if !ok {
		t.Fatal("users table missing")
	}
	colNames := make(map[string]bool)
	for _, c := range tbl.Columns {
		colNames[c.Name] = true
	}
	for _, want := range []string{"id", "email", "name", "created_at"} {
		if !colNames[want] {
			t.Errorf("column %q missing from merged table", want)
		}
	}
	if merged.SchemaVersion != 2 {
		t.Errorf("SchemaVersion = %d, want 2 (overlay wins)", merged.SchemaVersion)
	}
}

func TestMergeStoreSchemas_ExtendAddsNewTables(t *testing.T) {
	base := &StoreSchema{
		Version:       "1.0",
		SchemaVersion: 1,
		Tables: map[string]StoreTableDef{
			"users": {
				Columns: []StoreColumnDef{
					{Name: "id", Type: "integer", Required: true},
					{Name: "email", Type: "text", Required: true},
				},
				PrimaryKey: []string{"id"},
			},
		},
	}
	overlay := &StoreSchema{
		Version:       "1.1",
		SchemaVersion: 2,
		Import:        "base",
		Extend:        true,
		Tables: map[string]StoreTableDef{
			"users": {
				Columns: []StoreColumnDef{},
			},
			"posts": {
				Columns: []StoreColumnDef{
					{Name: "id", Type: "integer", Required: true},
					{Name: "user_id", Type: "integer", Required: true},
					{Name: "title", Type: "text", Required: false},
				},
				PrimaryKey: []string{"id"},
			},
		},
	}
	merged, err := MergeStoreSchemas(base, overlay)
	if err != nil {
		t.Fatalf("MergeStoreSchemas: %v", err)
	}
	if _, ok := merged.Tables["users"]; !ok {
		t.Fatal("users table missing")
	}
	if len(merged.Tables["users"].Columns) != 2 {
		t.Errorf("users should keep base columns, got %d", len(merged.Tables["users"].Columns))
	}
	posts, ok := merged.Tables["posts"]
	if !ok {
		t.Fatal("posts table missing")
	}
	if len(posts.Columns) != 3 {
		t.Errorf("posts columns = %d, want 3", len(posts.Columns))
	}
}

func TestMergeStoreSchemas_RejectsColumnRedefinition(t *testing.T) {
	base := &StoreSchema{
		Version:       "1.0",
		SchemaVersion: 1,
		Tables: map[string]StoreTableDef{
			"users": {
				Columns: []StoreColumnDef{
					{Name: "id", Type: "integer", Required: true},
					{Name: "email", Type: "text", Required: true},
				},
				PrimaryKey: []string{"id"},
			},
		},
	}
	overlay := &StoreSchema{
		Version:    "1.1",
		Import:     "base",
		Extend:     true,
		Tables: map[string]StoreTableDef{
			"users": {
				Columns: []StoreColumnDef{
					{Name: "email", Type: "varchar", Required: false},
				},
			},
		},
	}
	_, err := MergeStoreSchemas(base, overlay)
	if err == nil {
		t.Fatal("expected error for column redefinition")
	}
	if err.Error() == "" {
		t.Error("error message should not be empty")
	}
}

func TestMergeStoreSchemas_WithoutImportReturnsAsIs(t *testing.T) {
	base := &StoreSchema{
		Version:       "1.0",
		SchemaVersion: 1,
		Tables: map[string]StoreTableDef{
			"users": {
				Columns: []StoreColumnDef{
					{Name: "id", Type: "integer", Required: true},
				},
				PrimaryKey: []string{"id"},
			},
		},
	}
	overlay := &StoreSchema{
		Version:       "2.0",
		SchemaVersion: 2,
		Import:        "",
		Extend:        true,
		Tables: map[string]StoreTableDef{
			"standalone": {
				Columns: []StoreColumnDef{
					{Name: "x", Type: "integer", Required: true},
				},
				PrimaryKey: []string{"x"},
			},
		},
	}
	merged, err := MergeStoreSchemas(base, overlay)
	if err != nil {
		t.Fatalf("MergeStoreSchemas: %v", err)
	}
	// Overlay returned as-is: no users table, only standalone
	if _, ok := merged.Tables["users"]; ok {
		t.Error("overlay without import should not merge users from base")
	}
	if _, ok := merged.Tables["standalone"]; !ok {
		t.Error("overlay without import should retain standalone table")
	}
	if merged.Version != "2.0" {
		t.Errorf("Version = %q, want 2.0 (overlay as-is)", merged.Version)
	}
}

func TestStoreWiring_Parsing(t *testing.T) {
	raw := `
default: sqlite
stores:
  main:
    engine: sqlite
    config:
      path: /tmp/main.db
  cache:
    engine: memory
`
	var w StoreWiring
	if err := yaml.Unmarshal([]byte(raw), &w); err != nil {
		t.Fatalf("Unmarshal StoreWiring: %v", err)
	}
	if w.Default != "sqlite" {
		t.Errorf("Default = %q, want sqlite", w.Default)
	}
	if len(w.Stores) != 2 {
		t.Fatalf("Stores len = %d, want 2", len(w.Stores))
	}
	main, ok := w.Stores["main"]
	if !ok {
		t.Fatal("store main not found")
	}
	if main.Engine != "sqlite" {
		t.Errorf("main.Engine = %q, want sqlite", main.Engine)
	}
	if main.Config["path"] != "/tmp/main.db" {
		t.Errorf("main.Config[path] = %q, want /tmp/main.db", main.Config["path"])
	}
	cache, ok := w.Stores["cache"]
	if !ok {
		t.Fatal("store cache not found")
	}
	if cache.Engine != "memory" {
		t.Errorf("cache.Engine = %q, want memory", cache.Engine)
	}
}

func TestStoreDeclaration_LifecycleParsing(t *testing.T) {
	raw := `
- name: session_store
  lifecycle: session
  schema: rca-session
- name: persistent_store
  lifecycle: persistent
  schema: rca-persistent
`
	var decls []StoreDeclaration
	if err := yaml.Unmarshal([]byte(raw), &decls); err != nil {
		t.Fatalf("Unmarshal StoreDeclaration: %v", err)
	}
	if len(decls) != 2 {
		t.Fatalf("len = %d, want 2", len(decls))
	}
	if decls[0].Name != "session_store" || decls[0].Lifecycle != LifecycleSession || decls[0].Schema != "rca-session" {
		t.Errorf("decls[0] = %+v, want session_store/session/rca-session", decls[0])
	}
	if decls[1].Name != "persistent_store" || decls[1].Lifecycle != LifecyclePersistent || decls[1].Schema != "rca-persistent" {
		t.Errorf("decls[1] = %+v, want persistent_store/persistent/rca-persistent", decls[1])
	}
}

func TestStoreSchema_WithStoresField(t *testing.T) {
	yaml := `
kind: store-schema
version: "1.0"
schema_version: 1
tables:
  events:
    columns:
      - name: id
        type: integer
        required: true
    primary_key: [id]
stores:
  - name: main
    lifecycle: persistent
    schema: rca-main
  - name: session
    lifecycle: session
    schema: rca-session
`
	s, err := LoadStoreSchema([]byte(yaml))
	if err != nil {
		t.Fatalf("LoadStoreSchema: %v", err)
	}
	if len(s.Stores) != 2 {
		t.Fatalf("Stores len = %d, want 2", len(s.Stores))
	}
	if s.Stores[0].Name != "main" || s.Stores[0].Lifecycle != LifecyclePersistent || s.Stores[0].Schema != "rca-main" {
		t.Errorf("Stores[0] = %+v, want main/persistent/rca-main", s.Stores[0])
	}
	if s.Stores[1].Name != "session" || s.Stores[1].Lifecycle != LifecycleSession || s.Stores[1].Schema != "rca-session" {
		t.Errorf("Stores[1] = %+v, want session/session/rca-session", s.Stores[1])
	}
}
