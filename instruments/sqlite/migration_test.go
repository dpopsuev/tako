package sqlite

import (
	"strings"
	"testing"
)

func TestParseMigration_Valid(t *testing.T) {
	yaml := `
from: 1
to: 2
operations:
  - rename_table:
      from: old_items
      to: _v1_items
  - create_table:
      name: items
      columns:
        - name: id
          type: integer
          primary_key: true
          autoincrement: true
        - name: label
          type: text
          not_null: true
  - add_column:
      table: items
      column:
        name: status
        type: text
        default: "'new'"
  - create_index:
      name: idx_items_label
      table: items
      columns: [label]
  - raw_sql: "INSERT INTO items(label) SELECT name FROM _v1_items"
  - drop_table:
      name: _v1_items
`
	m, err := ParseMigration([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseMigration: %v", err)
	}
	if m.From != 1 || m.To != 2 {
		t.Errorf("from=%d to=%d, want 1→2", m.From, m.To)
	}
	if len(m.Operations) != 6 {
		t.Fatalf("operations = %d, want 6", len(m.Operations))
	}
}

func TestParseMigration_Errors(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want string
	}{
		{"missing from", "to: 2\noperations:\n  - raw_sql: 'x'", "from and to"},
		{"bad direction", "from: 3\nto: 1\noperations:\n  - raw_sql: 'x'", "must be greater"},
		{"no ops", "from: 1\nto: 2\noperations: []", "no operations"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseMigration([]byte(tt.yaml))
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want %q", err, tt.want)
			}
		})
	}
}

func TestMigration_GenerateSQL(t *testing.T) {
	m := &Migration{
		From: 1,
		To:   2,
		Operations: []Operation{
			{RenameTable: &RenameTable{From: "old", To: "new"}},
			{CreateTable: &Table{
				Name: "items",
				Columns: []Column{
					{Name: "id", Type: "integer", PrimaryKey: true},
					{Name: "name", Type: "text"},
				},
			}},
			{AddColumn: &AddColumn{
				Table:  "items",
				Column: Column{Name: "status", Type: "text", Default: "'active'"},
			}},
			{DropTable: &DropTable{Name: "old_backup"}},
			{CreateIndex: &Index{Name: "idx_items_name", Table: "items", Columns: []string{"name"}}},
			{RawSQL: "INSERT INTO items(name) VALUES('seed')"},
		},
	}

	sql := m.GenerateSQL()
	expectations := []string{
		"ALTER TABLE old RENAME TO new;",
		"CREATE TABLE IF NOT EXISTS items",
		"ALTER TABLE items ADD COLUMN status TEXT DEFAULT 'active';",
		"DROP TABLE IF EXISTS old_backup;",
		"CREATE INDEX IF NOT EXISTS idx_items_name ON items(name);",
		"INSERT INTO items(name) VALUES('seed');",
	}
	for _, exp := range expectations {
		if !strings.Contains(sql, exp) {
			t.Errorf("SQL missing %q\nGot:\n%s", exp, sql)
		}
	}
}

func TestRunMigrations(t *testing.T) {
	v1Schema := &Schema{
		Version: 1,
		Tables: []Table{
			{
				Name: "items",
				Columns: []Column{
					{Name: "id", Type: "integer", PrimaryKey: true, Autoincrement: true},
					{Name: "name", Type: "text", NotNull: true},
				},
			},
		},
	}

	db, err := OpenMemory(v1Schema)
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer db.Close()

	_, err = db.Insert(InsertParams{
		Table:   "items",
		Columns: []string{"name"},
		Values:  []any{"seed-item"},
	})
	if err != nil {
		t.Fatalf("seed insert: %v", err)
	}

	migration := &Migration{
		From: 1,
		To:   2,
		Operations: []Operation{
			{AddColumn: &AddColumn{
				Table:  "items",
				Column: Column{Name: "status", Type: "text", Default: "'active'"},
			}},
			{CreateTable: &Table{
				Name: "tags",
				Columns: []Column{
					{Name: "id", Type: "integer", PrimaryKey: true, Autoincrement: true},
					{Name: "item_id", Type: "integer", NotNull: true, References: "items(id)"},
					{Name: "label", Type: "text", NotNull: true},
				},
			}},
			{CreateIndex: &Index{Name: "idx_tags_item", Table: "tags", Columns: []string{"item_id"}}},
		},
	}

	if err := db.Migrate([]*Migration{migration}); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	var version int
	if err := db.QueryRow("SELECT version FROM schema_version").Scan(&version); err != nil {
		t.Fatalf("read version: %v", err)
	}
	if version != 2 {
		t.Errorf("version = %d, want 2", version)
	}

	var name, status string
	err = db.QueryRow("SELECT name, status FROM items WHERE id = 1").Scan(&name, &status)
	if err != nil {
		t.Fatalf("query migrated item: %v", err)
	}
	if name != "seed-item" {
		t.Errorf("name = %q, want seed-item", name)
	}
	if status != "active" {
		t.Errorf("status = %q, want active", status)
	}

	_, err = db.Insert(InsertParams{
		Table:   "tags",
		Columns: []string{"item_id", "label"},
		Values:  []any{1, "important"},
	})
	if err != nil {
		t.Fatalf("insert tag: %v", err)
	}
}

func TestRunMigrations_SkipsApplied(t *testing.T) {
	schema := &Schema{
		Version: 2,
		Tables: []Table{
			{
				Name: "data",
				Columns: []Column{
					{Name: "id", Type: "integer", PrimaryKey: true},
					{Name: "val", Type: "text"},
				},
			},
		},
	}

	db, err := OpenMemory(schema)
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer db.Close()

	v1to2 := &Migration{
		From: 1, To: 2,
		Operations: []Operation{{RawSQL: "SELECT 1"}},
	}
	if err := db.Migrate([]*Migration{v1to2}); err != nil {
		t.Fatalf("Migrate should skip: %v", err)
	}

	var version int
	db.QueryRow("SELECT version FROM schema_version").Scan(&version)
	if version != 2 {
		t.Errorf("version should still be 2, got %d", version)
	}
}
