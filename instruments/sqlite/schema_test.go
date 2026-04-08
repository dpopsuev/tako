package sqlite

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseSchema_Valid(t *testing.T) {
	yaml := `
version: 2
tables:
  - name: users
    columns:
      - name: id
        type: integer
        primary_key: true
        autoincrement: true
      - name: email
        type: text
        not_null: true
        unique: true
      - name: name
        type: text
        not_null: true
      - name: status
        type: text
        not_null: true
        default: "'active'"
  - name: posts
    columns:
      - name: id
        type: integer
        primary_key: true
        autoincrement: true
      - name: user_id
        type: integer
        not_null: true
        references: "users(id)"
      - name: title
        type: text
        not_null: true
      - name: body
        type: text
    unique:
      - [user_id, title]
indexes:
  - name: idx_posts_user
    table: posts
    columns: [user_id]
`
	s, err := ParseSchema([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseSchema: %v", err)
	}
	if s.Version != 2 {
		t.Errorf("version = %d, want 2", s.Version)
	}
	if len(s.Tables) != 2 {
		t.Fatalf("tables count = %d, want 2", len(s.Tables))
	}
	if s.Tables[0].Name != "users" {
		t.Errorf("table[0].Name = %q, want users", s.Tables[0].Name)
	}
	if len(s.Tables[0].Columns) != 4 {
		t.Errorf("users columns = %d, want 4", len(s.Tables[0].Columns))
	}
	if len(s.Tables[1].Unique) != 1 {
		t.Errorf("posts unique = %d, want 1", len(s.Tables[1].Unique))
	}
	if len(s.Indexes) != 1 {
		t.Errorf("indexes = %d, want 1", len(s.Indexes))
	}
}

func TestParseSchema_Errors(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want string
	}{
		{"no version", "tables:\n  - name: t\n    columns:\n      - name: c\n        type: text", "version is required"},
		{"no table name", "version: 1\ntables:\n  - columns:\n      - name: c\n        type: text", "table name is required"},
		{"no columns", "version: 1\ntables:\n  - name: t", "has no columns"},
		{"no column name", "version: 1\ntables:\n  - name: t\n    columns:\n      - type: text", "column name is required"},
		{"no column type", "version: 1\ntables:\n  - name: t\n    columns:\n      - name: c", "type is required"},
		{"duplicate table", "version: 1\ntables:\n  - name: t\n    columns:\n      - name: c\n        type: text\n  - name: t\n    columns:\n      - name: c\n        type: text", "duplicate table"},
		{"duplicate column", "version: 1\ntables:\n  - name: t\n    columns:\n      - name: c\n        type: text\n      - name: c\n        type: int", "duplicate column"},
		{"bad index ref", "version: 1\ntables:\n  - name: t\n    columns:\n      - name: c\n        type: text\nindexes:\n  - name: idx\n    table: missing\n    columns: [c]", "unknown table"},
		{"bad unique ref", "version: 1\ntables:\n  - name: t\n    columns:\n      - name: c\n        type: text\n    unique:\n      - [missing]", "unknown column"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSchema([]byte(tt.yaml))
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.want)
			}
		})
	}
}

func TestGenerateDDL(t *testing.T) {
	s := &Schema{
		Version: 1,
		Tables: []Table{
			{
				Name: "items",
				Columns: []Column{
					{Name: "id", Type: "integer", PrimaryKey: true, Autoincrement: true},
					{Name: "name", Type: "text", NotNull: true},
					{Name: "value", Type: "real", Default: "0.0"},
					{Name: "category_id", Type: "integer", References: "categories(id)"},
				},
				Unique: [][]string{{"name", "category_id"}},
			},
		},
		Indexes: []Index{
			{Name: "idx_items_cat", Table: "items", Columns: []string{"category_id"}},
		},
	}

	ddl := s.GenerateDDL()

	expectations := []string{
		"CREATE TABLE IF NOT EXISTS items",
		"id INTEGER PRIMARY KEY AUTOINCREMENT",
		"name TEXT NOT NULL",
		"value REAL DEFAULT 0.0",
		"category_id INTEGER REFERENCES categories(id)",
		"UNIQUE(name, category_id)",
		"CREATE INDEX IF NOT EXISTS idx_items_cat ON items(category_id)",
	}
	for _, exp := range expectations {
		if !strings.Contains(ddl, exp) {
			t.Errorf("DDL missing %q\nGot:\n%s", exp, ddl)
		}
	}
}

// --- Shorthand column parsing ---

func TestParseSchema_ShorthandColumns(t *testing.T) {
	input := `
version: 1
tables:
  - name: users
    columns:
      - name: id
        type: integer
        primary_key: true
        autoincrement: true
      - email: text not_null unique
      - name: text not_null
      - status: text not_null default='active'
      - score: real
      - avatar: blob
`
	s, err := ParseSchema([]byte(input))
	if err != nil {
		t.Fatalf("ParseSchema: %v", err)
	}
	if len(s.Tables) != 1 {
		t.Fatalf("tables = %d, want 1", len(s.Tables))
	}
	cols := s.Tables[0].Columns
	if len(cols) != 6 {
		t.Fatalf("columns = %d, want 6", len(cols))
	}

	want := []Column{
		{Name: "id", Type: "integer", PrimaryKey: true, Autoincrement: true},
		{Name: "email", Type: "text", NotNull: true, Unique: true},
		{Name: "name", Type: "text", NotNull: true},
		{Name: "status", Type: "text", NotNull: true, Default: "'active'"},
		{Name: "score", Type: "real"},
		{Name: "avatar", Type: "blob"},
	}
	for i, w := range want {
		if !reflect.DeepEqual(cols[i], w) {
			t.Errorf("column[%d]:\n  got  %+v\n  want %+v", i, cols[i], w)
		}
	}
}

func TestParseSchema_ShorthandFKArrow(t *testing.T) {
	input := `
version: 1
tables:
  - name: parent
    columns:
      - name: id
        type: integer
        primary_key: true
      - label: text
  - name: child
    columns:
      - name: id
        type: integer
        primary_key: true
      - parent_id: integer not_null -> parent
      - custom_ref: integer -> parent(custom_col)
`
	s, err := ParseSchema([]byte(input))
	if err != nil {
		t.Fatalf("ParseSchema: %v", err)
	}
	child := s.Tables[1]
	if child.Columns[1].References != "parent(id)" {
		t.Errorf("parent_id references = %q, want parent(id)", child.Columns[1].References)
	}
	if child.Columns[2].References != "parent(custom_col)" {
		t.Errorf("custom_ref references = %q, want parent(custom_col)", child.Columns[2].References)
	}
}

func TestParseSchema_ShorthandReferencesKeyword(t *testing.T) {
	input := `
version: 1
tables:
  - name: parent
    columns:
      - name: id
        type: integer
        primary_key: true
      - label: text
  - name: child
    columns:
      - name: id
        type: integer
        primary_key: true
      - parent_id: integer not_null references parent
      - custom_ref: integer references parent(custom_col)
`
	s, err := ParseSchema([]byte(input))
	if err != nil {
		t.Fatalf("ParseSchema: %v", err)
	}
	child := s.Tables[1]
	if child.Columns[1].References != "parent(id)" {
		t.Errorf("parent_id references = %q, want parent(id)", child.Columns[1].References)
	}
	if child.Columns[2].References != "parent(custom_col)" {
		t.Errorf("custom_ref references = %q, want parent(custom_col)", child.Columns[2].References)
	}
}

func TestParseSchema_EnvelopeFields(t *testing.T) {
	input := `
kind: store-schema
version: 1
metadata:
  name: test-schema
  description: "Test schema with envelope"
tables:
  - name: items
    columns:
      - label: text not_null
`
	s, err := ParseSchema([]byte(input))
	if err != nil {
		t.Fatalf("ParseSchema: %v", err)
	}
	if len(s.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(s.Tables))
	}
	if s.Tables[0].Name != "items" {
		t.Errorf("table name = %q, want items", s.Tables[0].Name)
	}
}

func TestParseSchema_ShorthandPKAuto(t *testing.T) {
	input := `
version: 1
tables:
  - name: manual_pk
    auto_id: false
    columns:
      - row_id: integer pk auto
      - value: text
`
	s, err := ParseSchema([]byte(input))
	if err != nil {
		t.Fatalf("ParseSchema: %v", err)
	}
	col := s.Tables[0].Columns[0]
	if col.Name != "row_id" || !col.PrimaryKey || !col.Autoincrement {
		t.Errorf("row_id = %+v, want pk + auto", col)
	}
}

// --- Implicit id ---

func TestParseSchema_ImplicitID(t *testing.T) {
	input := `
version: 1
tables:
  - name: things
    columns:
      - label: text not_null
      - count: integer default=0
`
	s, err := ParseSchema([]byte(input))
	if err != nil {
		t.Fatalf("ParseSchema: %v", err)
	}
	cols := s.Tables[0].Columns
	if len(cols) != 3 {
		t.Fatalf("columns = %d, want 3 (implicit id + 2)", len(cols))
	}
	id := cols[0]
	if id.Name != "id" || id.Type != "integer" || !id.PrimaryKey || !id.Autoincrement {
		t.Errorf("implicit id = %+v", id)
	}
	if cols[1].Name != "label" {
		t.Errorf("cols[1] = %q, want label", cols[1].Name)
	}
}

func TestParseSchema_ExplicitIDSuppressesAuto(t *testing.T) {
	input := `
version: 1
tables:
  - name: users
    columns:
      - name: id
        type: integer
        primary_key: true
        autoincrement: true
      - email: text
`
	s, err := ParseSchema([]byte(input))
	if err != nil {
		t.Fatalf("ParseSchema: %v", err)
	}
	if len(s.Tables[0].Columns) != 2 {
		t.Errorf("columns = %d, want 2 (no duplicate id)", len(s.Tables[0].Columns))
	}
}

func TestParseSchema_AutoIDFalse(t *testing.T) {
	input := `
version: 1
tables:
  - name: schema_version
    auto_id: false
    columns:
      - version: integer not_null
`
	s, err := ParseSchema([]byte(input))
	if err != nil {
		t.Fatalf("ParseSchema: %v", err)
	}
	cols := s.Tables[0].Columns
	if len(cols) != 1 {
		t.Fatalf("columns = %d, want 1 (no id)", len(cols))
	}
	if cols[0].Name != "version" {
		t.Errorf("cols[0] = %q, want version", cols[0].Name)
	}
}

// --- Table-local indexes ---

func TestParseSchema_TableLocalIndexes(t *testing.T) {
	input := `
version: 1
tables:
  - name: cases
    columns:
      - name: id
        type: integer
        primary_key: true
      - symptom_id: integer
      - rca_id: integer
      - launch_id: integer
    indexes:
      - [symptom_id]
      - [rca_id]
      - [launch_id, symptom_id]
`
	s, err := ParseSchema([]byte(input))
	if err != nil {
		t.Fatalf("ParseSchema: %v", err)
	}
	if len(s.Indexes) != 3 {
		t.Fatalf("indexes = %d, want 3", len(s.Indexes))
	}
	wantNames := []string{
		"idx_cases_symptom_id",
		"idx_cases_rca_id",
		"idx_cases_launch_id_symptom_id",
	}
	for i, name := range wantNames {
		if s.Indexes[i].Name != name {
			t.Errorf("index[%d].Name = %q, want %q", i, s.Indexes[i].Name, name)
		}
		if s.Indexes[i].Table != "cases" {
			t.Errorf("index[%d].Table = %q, want cases", i, s.Indexes[i].Table)
		}
	}
}

func TestParseSchema_MixedTopAndLocalIndexes(t *testing.T) {
	input := `
version: 1
tables:
  - name: items
    columns:
      - name: id
        type: integer
        primary_key: true
      - category_id: integer
    indexes:
      - [category_id]
indexes:
  - name: idx_items_custom
    table: items
    columns: [category_id]
    unique: true
`
	s, err := ParseSchema([]byte(input))
	if err != nil {
		t.Fatalf("ParseSchema: %v", err)
	}
	if len(s.Indexes) != 2 {
		t.Fatalf("indexes = %d, want 2 (1 top-level + 1 local)", len(s.Indexes))
	}
	if s.Indexes[0].Name != "idx_items_custom" {
		t.Errorf("top-level index = %q", s.Indexes[0].Name)
	}
	if s.Indexes[1].Name != "idx_items_category_id" {
		t.Errorf("local index = %q", s.Indexes[1].Name)
	}
}

// --- Shorthand DDL end-to-end ---

func TestParseSchema_ShorthandProducesCorrectDDL(t *testing.T) {
	input := `
version: 1
tables:
  - name: orders
    columns:
      - customer_id: integer not_null -> customers
      - total: real not_null default=0.0
      - status: text not_null default='pending'
    indexes:
      - [customer_id]
`
	s, err := ParseSchema([]byte(input))
	if err != nil {
		t.Fatalf("ParseSchema: %v", err)
	}
	ddl := s.GenerateDDL()
	expectations := []string{
		"id INTEGER PRIMARY KEY AUTOINCREMENT",
		"customer_id INTEGER NOT NULL REFERENCES customers(id)",
		"total REAL NOT NULL DEFAULT 0.0",
		"status TEXT NOT NULL DEFAULT 'pending'",
		"CREATE INDEX IF NOT EXISTS idx_orders_customer_id ON orders(customer_id)",
	}
	for _, exp := range expectations {
		if !strings.Contains(ddl, exp) {
			t.Errorf("DDL missing %q\nGot:\n%s", exp, ddl)
		}
	}
}

// --- Shorthand error cases ---

func TestParseSchema_ShorthandErrors(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want string
	}{
		{
			"unknown modifier",
			"version: 1\ntables:\n  - name: t\n    columns:\n      - col: text bogus",
			"unknown modifier",
		},
		{
			"arrow without table",
			"version: 1\ntables:\n  - name: t\n    columns:\n      - col: integer ->",
			"-> requires a table",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSchema([]byte(tt.yaml))
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.want)
			}
		})
	}
}

// --- Equivalence: verbose and shorthand parse to same Schema ---

func TestParseSchema_VerboseShorthandEquivalence(t *testing.T) {
	verbose := `
version: 1
tables:
  - name: items
    columns:
      - name: id
        type: integer
        primary_key: true
        autoincrement: true
      - name: owner_id
        type: integer
        not_null: true
        references: "users(id)"
      - name: label
        type: text
        not_null: true
        unique: true
      - name: score
        type: real
        default: "0.0"
    indexes:
      - [owner_id]
`
	shorthand := `
version: 1
tables:
  - name: items
    columns:
      - name: id
        type: integer
        primary_key: true
        autoincrement: true
      - owner_id: integer not_null -> users
      - label: text not_null unique
      - score: real default=0.0
    indexes:
      - [owner_id]
`
	sv, err := ParseSchema([]byte(verbose))
	if err != nil {
		t.Fatalf("verbose: %v", err)
	}
	ss, err := ParseSchema([]byte(shorthand))
	if err != nil {
		t.Fatalf("shorthand: %v", err)
	}
	if !reflect.DeepEqual(sv, ss) {
		t.Errorf("verbose and shorthand produce different schemas:\nverbose:   %+v\nshorthand: %+v", sv, ss)
	}
}

func TestGenerateDDL_ForeignKeys(t *testing.T) {
	s := &Schema{
		Version: 1,
		Tables: []Table{
			{
				Name: "links",
				Columns: []Column{
					{Name: "id", Type: "integer", PrimaryKey: true},
					{Name: "a_id", Type: "integer", NotNull: true},
					{Name: "b_id", Type: "integer", NotNull: true},
				},
				ForeignKeys: []ForeignKey{
					{Columns: []string{"a_id"}, References: "a(id)"},
					{Columns: []string{"b_id"}, References: "b(id)"},
				},
			},
		},
	}

	ddl := s.GenerateDDL()
	if !strings.Contains(ddl, "FOREIGN KEY (a_id) REFERENCES a(id)") {
		t.Errorf("DDL missing FK for a_id\nGot:\n%s", ddl)
	}
	if !strings.Contains(ddl, "FOREIGN KEY (b_id) REFERENCES b(id)") {
		t.Errorf("DDL missing FK for b_id\nGot:\n%s", ddl)
	}
}
