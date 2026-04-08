package sqlite

import (
	"bytes"
	"testing"
)

var entityTestSchema = &Schema{
	Version: 1,
	Tables: []Table{
		{
			Name: "items",
			Columns: []Column{
				{Name: "id", Type: "integer", PrimaryKey: true, Autoincrement: true},
				{Name: "name", Type: "text", NotNull: true},
				{Name: "description", Type: "text"},
				{Name: "score", Type: "real"},
				{Name: "count", Type: "integer"},
				{Name: "payload", Type: "blob"},
			},
		},
	},
}

func openTestEntityStore(t *testing.T) *EntityStore {
	t.Helper()
	db, err := OpenMemory(entityTestSchema)
	if err != nil {
		t.Fatalf("open memory: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewEntityStore(db)
}

func TestEntityStore_CreateAndGet(t *testing.T) {
	es := openTestEntityStore(t)

	id, err := es.Create("items", Row{
		"name":        "widget",
		"description": "a fine widget",
		"score":       3.14,
		"count":       int64(42),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if id != 1 {
		t.Fatalf("expected id=1, got %d", id)
	}

	row, err := es.Get("items", id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if row == nil {
		t.Fatal("get: expected row, got nil")
	}
	if row.Int64("id") != 1 {
		t.Errorf("id: got %d", row.Int64("id"))
	}
	if row.String("name") != "widget" {
		t.Errorf("name: got %q", row.String("name"))
	}
	if row.String("description") != "a fine widget" {
		t.Errorf("description: got %q", row.String("description"))
	}
	if row.Float64("score") != 3.14 {
		t.Errorf("score: got %f", row.Float64("score"))
	}
	if row.Int64("count") != 42 {
		t.Errorf("count: got %d", row.Int64("count"))
	}
}

func TestEntityStore_CreateNullable(t *testing.T) {
	es := openTestEntityStore(t)

	id, err := es.Create("items", Row{
		"name": "minimal",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	row, err := es.Get("items", id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if row.String("description") != "" {
		t.Errorf("expected empty description, got %q", row.String("description"))
	}
	if row.Float64("score") != 0 {
		t.Errorf("expected zero score, got %f", row.Float64("score"))
	}
	if row.Int64("count") != 0 {
		t.Errorf("expected zero count, got %d", row.Int64("count"))
	}
}

func TestEntityStore_GetNotFound(t *testing.T) {
	es := openTestEntityStore(t)

	row, err := es.Get("items", 999)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if row != nil {
		t.Fatal("expected nil for missing row")
	}
}

func TestEntityStore_GetBy(t *testing.T) {
	es := openTestEntityStore(t)

	es.Create("items", Row{"name": "alpha", "count": int64(10)})
	es.Create("items", Row{"name": "beta", "count": int64(20)})

	row, err := es.GetBy("items", Row{"name": "beta"})
	if err != nil {
		t.Fatalf("get_by: %v", err)
	}
	if row == nil {
		t.Fatal("expected row, got nil")
	}
	if row.String("name") != "beta" {
		t.Errorf("name: got %q", row.String("name"))
	}
	if row.Int64("count") != 20 {
		t.Errorf("count: got %d", row.Int64("count"))
	}
}

func TestEntityStore_GetByNotFound(t *testing.T) {
	es := openTestEntityStore(t)

	row, err := es.GetBy("items", Row{"name": "nonexistent"})
	if err != nil {
		t.Fatalf("get_by: %v", err)
	}
	if row != nil {
		t.Fatal("expected nil for no match")
	}
}

func TestEntityStore_List(t *testing.T) {
	es := openTestEntityStore(t)

	es.Create("items", Row{"name": "first", "count": int64(1)})
	es.Create("items", Row{"name": "second", "count": int64(2)})
	es.Create("items", Row{"name": "third", "count": int64(1)})

	t.Run("all", func(t *testing.T) {
		rows, err := es.List("items", nil, "")
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(rows) != 3 {
			t.Fatalf("expected 3 rows, got %d", len(rows))
		}
		if rows[0].String("name") != "first" {
			t.Errorf("first: got %q", rows[0].String("name"))
		}
	})

	t.Run("filtered", func(t *testing.T) {
		rows, err := es.List("items", Row{"count": int64(1)}, "")
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(rows) != 2 {
			t.Fatalf("expected 2 rows, got %d", len(rows))
		}
	})

	t.Run("no_match", func(t *testing.T) {
		rows, err := es.List("items", Row{"count": int64(99)}, "")
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(rows) != 0 {
			t.Fatalf("expected 0 rows, got %d", len(rows))
		}
	})
}

func TestEntityStore_Update(t *testing.T) {
	es := openTestEntityStore(t)

	id, _ := es.Create("items", Row{"name": "original", "count": int64(5)})

	err := es.Update("items", id, Row{"name": "updated", "count": int64(10)})
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	row, _ := es.Get("items", id)
	if row.String("name") != "updated" {
		t.Errorf("name: got %q", row.String("name"))
	}
	if row.Int64("count") != 10 {
		t.Errorf("count: got %d", row.Int64("count"))
	}
}

func TestEntityStore_UpdateNotFound(t *testing.T) {
	es := openTestEntityStore(t)

	err := es.Update("items", 999, Row{"name": "x"})
	if err == nil {
		t.Fatal("expected error for missing row")
	}
}

func TestEntityStore_Delete(t *testing.T) {
	es := openTestEntityStore(t)

	id, _ := es.Create("items", Row{"name": "doomed"})

	err := es.Delete("items", id)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}

	row, _ := es.Get("items", id)
	if row != nil {
		t.Fatal("expected nil after delete")
	}
}

func TestEntityStore_UnknownTable(t *testing.T) {
	es := openTestEntityStore(t)

	_, err := es.Create("nonexistent", Row{"x": 1})
	if err == nil {
		t.Fatal("expected error for unknown table")
	}

	_, err = es.Get("nonexistent", 1)
	if err == nil {
		t.Fatal("expected error for unknown table")
	}
}

func TestEntityStore_BlobRoundTrip(t *testing.T) {
	es := openTestEntityStore(t)

	data := []byte(`{"key":"value"}`)
	id, err := es.Create("items", Row{"name": "blob-test", "payload": data})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	row, err := es.Get("items", id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	got := row.Bytes("payload")
	if !bytes.Equal(got, data) {
		t.Errorf("payload: got %q, want %q", got, data)
	}
}

// --- MemEntityStore tests ---

func TestMemEntityStore_CreateAndGet(t *testing.T) {
	mes := NewMemEntityStore(entityTestSchema)

	id, err := mes.Create("items", Row{
		"name":  "widget",
		"score": 3.14,
		"count": int64(42),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if id != 1 {
		t.Fatalf("expected id=1, got %d", id)
	}

	row, err := mes.Get("items", id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if row == nil {
		t.Fatal("expected row, got nil")
	}
	if row.Int64("id") != 1 {
		t.Errorf("id: got %d", row.Int64("id"))
	}
	if row.String("name") != "widget" {
		t.Errorf("name: got %q", row.String("name"))
	}
}

func TestMemEntityStore_GetNotFound(t *testing.T) {
	mes := NewMemEntityStore(entityTestSchema)

	row, err := mes.Get("items", 999)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if row != nil {
		t.Fatal("expected nil for missing row")
	}
}

func TestMemEntityStore_GetReturnsACopy(t *testing.T) {
	mes := NewMemEntityStore(entityTestSchema)

	id, _ := mes.Create("items", Row{"name": "original"})
	row, _ := mes.Get("items", id)
	row["name"] = "mutated"

	row2, _ := mes.Get("items", id)
	if row2.String("name") != "original" {
		t.Errorf("get should return a copy; name was mutated to %q", row2.String("name"))
	}
}

func TestMemEntityStore_GetBy(t *testing.T) {
	mes := NewMemEntityStore(entityTestSchema)

	mes.Create("items", Row{"name": "alpha", "count": int64(10)})
	mes.Create("items", Row{"name": "beta", "count": int64(20)})

	row, err := mes.GetBy("items", Row{"name": "beta"})
	if err != nil {
		t.Fatalf("get_by: %v", err)
	}
	if row == nil || row.String("name") != "beta" {
		t.Fatalf("expected beta, got %v", row)
	}
}

func TestMemEntityStore_List(t *testing.T) {
	mes := NewMemEntityStore(entityTestSchema)

	mes.Create("items", Row{"name": "first", "count": int64(1)})
	mes.Create("items", Row{"name": "second", "count": int64(2)})
	mes.Create("items", Row{"name": "third", "count": int64(1)})

	t.Run("all", func(t *testing.T) {
		rows, err := mes.List("items", nil, "")
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(rows) != 3 {
			t.Fatalf("expected 3 rows, got %d", len(rows))
		}
	})

	t.Run("filtered", func(t *testing.T) {
		rows, err := mes.List("items", Row{"count": int64(1)}, "")
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(rows) != 2 {
			t.Fatalf("expected 2 rows, got %d", len(rows))
		}
	})
}

func TestMemEntityStore_Update(t *testing.T) {
	mes := NewMemEntityStore(entityTestSchema)

	id, _ := mes.Create("items", Row{"name": "original", "count": int64(5)})

	err := mes.Update("items", id, Row{"name": "updated"})
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	row, _ := mes.Get("items", id)
	if row.String("name") != "updated" {
		t.Errorf("name: got %q", row.String("name"))
	}
	if row.Int64("count") != 5 {
		t.Errorf("count should be unchanged: got %d", row.Int64("count"))
	}
}

func TestMemEntityStore_Delete(t *testing.T) {
	mes := NewMemEntityStore(entityTestSchema)

	id, _ := mes.Create("items", Row{"name": "doomed"})
	mes.Delete("items", id)

	row, _ := mes.Get("items", id)
	if row != nil {
		t.Fatal("expected nil after delete")
	}
}

func TestMemEntityStore_Mutate(t *testing.T) {
	mes := NewMemEntityStore(entityTestSchema)

	id, _ := mes.Create("items", Row{"name": "counter", "count": int64(0)})

	err := mes.Mutate("items", id, func(r Row) {
		r["count"] = r.Int64("count") + 1
	})
	if err != nil {
		t.Fatalf("mutate: %v", err)
	}

	row, _ := mes.Get("items", id)
	if row.Int64("count") != 1 {
		t.Errorf("count: got %d, want 1", row.Int64("count"))
	}
}

func TestMemEntityStore_MutateAll(t *testing.T) {
	mes := NewMemEntityStore(entityTestSchema)

	mes.Create("items", Row{"name": "a", "count": int64(1)})
	mes.Create("items", Row{"name": "b", "count": int64(2)})
	mes.Create("items", Row{"name": "c", "count": int64(3)})

	n, err := mes.MutateAll("items", func(r Row) bool {
		if r.Int64("count") < 3 {
			r["count"] = r.Int64("count") * 10
			return true
		}
		return false
	})
	if err != nil {
		t.Fatalf("mutate_all: %v", err)
	}
	if n != 2 {
		t.Errorf("expected 2 mutations, got %d", n)
	}

	rows, _ := mes.List("items", nil, "")
	for _, r := range rows {
		switch r.String("name") {
		case "a":
			if r.Int64("count") != 10 {
				t.Errorf("a: got %d", r.Int64("count"))
			}
		case "b":
			if r.Int64("count") != 20 {
				t.Errorf("b: got %d", r.Int64("count"))
			}
		case "c":
			if r.Int64("count") != 3 {
				t.Errorf("c: got %d", r.Int64("count"))
			}
		}
	}
}

// --- Row accessor tests ---

func TestRow_Accessors(t *testing.T) {
	r := Row{
		"s":    "hello",
		"i":    int64(42),
		"f":    3.14,
		"b":    int64(1),
		"blob": []byte("data"),
		"nil":  nil,
		"int":  5,
	}

	if r.String("s") != "hello" {
		t.Errorf("String: got %q", r.String("s"))
	}
	if r.String("missing") != "" {
		t.Errorf("String missing: got %q", r.String("missing"))
	}
	if r.String("nil") != "" {
		t.Errorf("String nil: got %q", r.String("nil"))
	}

	if r.Int64("i") != 42 {
		t.Errorf("Int64: got %d", r.Int64("i"))
	}
	if r.Int64("int") != 5 {
		t.Errorf("Int64 from int: got %d", r.Int64("int"))
	}
	if r.Int64("missing") != 0 {
		t.Errorf("Int64 missing: got %d", r.Int64("missing"))
	}
	if r.Int("i") != 42 {
		t.Errorf("Int: got %d", r.Int("i"))
	}

	if r.Float64("f") != 3.14 {
		t.Errorf("Float64: got %f", r.Float64("f"))
	}
	if r.Float64("missing") != 0 {
		t.Errorf("Float64 missing: got %f", r.Float64("missing"))
	}

	if !r.Bool("b") {
		t.Error("Bool: expected true")
	}
	if r.Bool("missing") {
		t.Error("Bool missing: expected false")
	}

	if string(r.Bytes("blob")) != "data" {
		t.Errorf("Bytes: got %q", r.Bytes("blob"))
	}
	if r.Bytes("missing") != nil {
		t.Error("Bytes missing: expected nil")
	}
}
