package sqlite

import (
	"testing"
)

var testSchema = &Schema{
	Version: 1,
	Tables: []Table{
		{
			Name: "users",
			Columns: []Column{
				{Name: "id", Type: "integer", PrimaryKey: true, Autoincrement: true},
				{Name: "name", Type: "text", NotNull: true},
				{Name: "email", Type: "text", NotNull: true, Unique: true},
			},
		},
		{
			Name: "posts",
			Columns: []Column{
				{Name: "id", Type: "integer", PrimaryKey: true, Autoincrement: true},
				{Name: "user_id", Type: "integer", NotNull: true, References: "users(id)"},
				{Name: "title", Type: "text", NotNull: true},
				{Name: "body", Type: "text"},
			},
		},
	},
	Indexes: []Index{
		{Name: "idx_posts_user", Table: "posts", Columns: []string{"user_id"}},
	},
}

func TestOpenMemory(t *testing.T) {
	db, err := OpenMemory(testSchema)
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer db.Close()

	var count int
	err = db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='users'",
	).Scan(&count)
	if err != nil {
		t.Fatalf("check users table: %v", err)
	}
	if count != 1 {
		t.Errorf("users table count = %d, want 1", count)
	}

	err = db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='posts'",
	).Scan(&count)
	if err != nil {
		t.Fatalf("check posts table: %v", err)
	}
	if count != 1 {
		t.Errorf("posts table count = %d, want 1", count)
	}

	var version int
	err = db.QueryRow("SELECT version FROM schema_version LIMIT 1").Scan(&version)
	if err != nil {
		t.Fatalf("read version: %v", err)
	}
	if version != 1 {
		t.Errorf("version = %d, want 1", version)
	}
}

func TestOpenMemory_NilSchema(t *testing.T) {
	db, err := OpenMemory(nil)
	if err != nil {
		t.Fatalf("OpenMemory(nil): %v", err)
	}
	defer db.Close()

	if db.Schema() != nil {
		t.Error("Schema() should be nil for nil-schema DB")
	}
}

func TestOpenMemory_IdempotentApply(t *testing.T) {
	db, err := OpenMemory(testSchema)
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer db.Close()

	_, err = db.Insert(InsertParams{
		Table:   "users",
		Columns: []string{"name", "email"},
		Values:  []any{"Alice", "alice@example.com"},
	})
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	// Calling applySchema again should not destroy existing data.
	if err := db.applySchema(); err != nil {
		t.Fatalf("second applySchema: %v", err)
	}

	var name string
	err = db.QueryRow("SELECT name FROM users WHERE email = 'alice@example.com'").Scan(&name)
	if err != nil {
		t.Fatalf("query after reapply: %v", err)
	}
	if name != "Alice" {
		t.Errorf("name = %q, want Alice", name)
	}
}

func TestOpen_FileDB(t *testing.T) {
	path := t.TempDir() + "/test.db"
	db, err := Open(path, testSchema)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	_, err = db.Insert(InsertParams{
		Table:   "users",
		Columns: []string{"name", "email"},
		Values:  []any{"Bob", "bob@example.com"},
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	db.Close()

	db2, err := Open(path, testSchema)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer db2.Close()

	var name string
	err = db2.QueryRow("SELECT name FROM users WHERE email = 'bob@example.com'").Scan(&name)
	if err != nil {
		t.Fatalf("query after reopen: %v", err)
	}
	if name != "Bob" {
		t.Errorf("name = %q, want Bob", name)
	}
}
