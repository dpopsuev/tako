package sqlite

import (
	"testing"
)

func setupCRUD(t *testing.T) *DB {
	t.Helper()
	db, err := OpenMemory(testSchema)
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestInsert_and_QueryOne(t *testing.T) {
	db := setupCRUD(t)

	id, err := db.Insert(InsertParams{
		Table:   "users",
		Columns: []string{"name", "email"},
		Values:  []any{"Alice", "alice@test.com"},
	})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if id != 1 {
		t.Errorf("id = %d, want 1", id)
	}

	var name, email string
	row := db.QueryOne(&QueryParams{
		Table:   "users",
		Columns: []string{"name", "email"},
		Where:   "id = ?",
		Args:    []any{id},
	})
	if err := row.Scan(&name, &email); err != nil {
		t.Fatalf("QueryOne: %v", err)
	}
	if name != "Alice" || email != "alice@test.com" {
		t.Errorf("got name=%q email=%q", name, email)
	}
}

func TestQueryRows(t *testing.T) {
	db := setupCRUD(t)

	for _, u := range []struct{ name, email string }{
		{"Alice", "alice@test.com"},
		{"Bob", "bob@test.com"},
		{"Charlie", "charlie@test.com"},
	} {
		_, err := db.Insert(InsertParams{
			Table:   "users",
			Columns: []string{"name", "email"},
			Values:  []any{u.name, u.email},
		})
		if err != nil {
			t.Fatalf("Insert %s: %v", u.name, err)
		}
	}

	rows, err := db.QueryRows(&QueryParams{
		Table:   "users",
		Columns: []string{"name"},
		OrderBy: "name",
	})
	if err != nil {
		t.Fatalf("QueryRows: %v", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatalf("scan: %v", err)
		}
		names = append(names, n)
	}
	if len(names) != 3 {
		t.Fatalf("count = %d, want 3", len(names))
	}
	if names[0] != "Alice" || names[1] != "Bob" || names[2] != "Charlie" {
		t.Errorf("names = %v", names)
	}
}

func TestQueryRows_WithLimit(t *testing.T) {
	db := setupCRUD(t)

	for _, u := range []struct{ name, email string }{
		{"Alice", "a@test.com"},
		{"Bob", "b@test.com"},
		{"Charlie", "c@test.com"},
	} {
		db.Insert(InsertParams{
			Table:   "users",
			Columns: []string{"name", "email"},
			Values:  []any{u.name, u.email},
		})
	}

	rows, err := db.QueryRows(&QueryParams{
		Table:   "users",
		Columns: []string{"name"},
		OrderBy: "name",
		Limit:   2,
	})
	if err != nil {
		t.Fatalf("QueryRows: %v", err)
	}
	defer rows.Close()

	var count int
	for rows.Next() {
		count++
		var n string
		rows.Scan(&n)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
}

func TestUpdate(t *testing.T) {
	db := setupCRUD(t)

	id, _ := db.Insert(InsertParams{
		Table:   "users",
		Columns: []string{"name", "email"},
		Values:  []any{"Alice", "alice@test.com"},
	})

	affected, err := db.Update(UpdateParams{
		Table: "users",
		Set:   map[string]any{"name": "Alice Updated"},
		Where: "id = ?",
		Args:  []any{id},
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if affected != 1 {
		t.Errorf("affected = %d, want 1", affected)
	}

	var name string
	db.QueryOne(&QueryParams{
		Table:   "users",
		Columns: []string{"name"},
		Where:   "id = ?",
		Args:    []any{id},
	}).Scan(&name)
	if name != "Alice Updated" {
		t.Errorf("name = %q, want Alice Updated", name)
	}
}

func TestInsert_Errors(t *testing.T) {
	db := setupCRUD(t)

	tests := []struct {
		name   string
		params InsertParams
		want   string
	}{
		{"no table", InsertParams{Columns: []string{"a"}, Values: []any{1}}, "table name"},
		{"no columns", InsertParams{Table: "users"}, "columns are required"},
		{"mismatch", InsertParams{Table: "users", Columns: []string{"a", "b"}, Values: []any{1}}, "mismatch"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := db.Insert(tt.params)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestUpdate_Errors(t *testing.T) {
	db := setupCRUD(t)

	_, err := db.Update(UpdateParams{Set: map[string]any{"a": 1}})
	if err == nil {
		t.Fatal("expected error for no table")
	}
	_, err = db.Update(UpdateParams{Table: "users"})
	if err == nil {
		t.Fatal("expected error for no set")
	}
}

func TestExecSQL_and_QuerySQL(t *testing.T) {
	db := setupCRUD(t)

	_, err := db.ExecSQL("INSERT INTO users(name, email) VALUES(?, ?)", "Alice", "alice@test.com")
	if err != nil {
		t.Fatalf("ExecSQL: %v", err)
	}

	rows, err := db.QuerySQL("SELECT name FROM users WHERE email = ?", "alice@test.com")
	if err != nil {
		t.Fatalf("QuerySQL: %v", err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Fatal("no rows")
	}
	var name string
	rows.Scan(&name)
	if name != "Alice" {
		t.Errorf("name = %q", name)
	}
}

func TestQueryRowSQL(t *testing.T) {
	db := setupCRUD(t)

	db.ExecSQL("INSERT INTO users(name, email) VALUES(?, ?)", "Bob", "bob@test.com")
	var name string
	err := db.QueryRowSQL("SELECT name FROM users WHERE email = ?", "bob@test.com").Scan(&name)
	if err != nil {
		t.Fatalf("QueryRowSQL: %v", err)
	}
	if name != "Bob" {
		t.Errorf("name = %q", name)
	}
}

func TestForeignKey_Insert(t *testing.T) {
	db := setupCRUD(t)

	userID, err := db.Insert(InsertParams{
		Table:   "users",
		Columns: []string{"name", "email"},
		Values:  []any{"Alice", "alice@test.com"},
	})
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	postID, err := db.Insert(InsertParams{
		Table:   "posts",
		Columns: []string{"user_id", "title", "body"},
		Values:  []any{userID, "Hello", "World"},
	})
	if err != nil {
		t.Fatalf("insert post: %v", err)
	}

	var title string
	db.QueryOne(&QueryParams{
		Table:   "posts",
		Columns: []string{"title"},
		Where:   "id = ?",
		Args:    []any{postID},
	}).Scan(&title)
	if title != "Hello" {
		t.Errorf("title = %q, want Hello", title)
	}
}
