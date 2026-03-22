package sqlite

import (
	"context"
	"testing"

	"github.com/dpopsuev/origami/engine"

)

type hookArtifact struct {
	data map[string]any
}

func (a hookArtifact) Type() string        { return "hook-test" }
func (a hookArtifact) Confidence() float64 { return 1.0 }
func (a hookArtifact) Raw() any            { return a.data }

func TestNewExecHook(t *testing.T) {
	db, err := OpenMemory(nil)
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer db.Close()

	hook := NewExecHook(db, nil)
	if hook == nil {
		t.Fatal("NewExecHook returned nil")
	}
	if hook.Name() != BuiltinHookSQLiteExec {
		t.Errorf("Name() = %q, want %q", hook.Name(), BuiltinHookSQLiteExec)
	}
}

func TestExecHook_NoMetaSkips(t *testing.T) {
	db, err := OpenMemory(nil)
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer db.Close()

	hook := NewExecHook(db, nil)
	art := hookArtifact{data: map[string]any{"id": "1"}}

	if err := hook.Run(context.Background(), "unknown", art); err != nil {
		t.Errorf("Run with no meta should return nil, got: %v", err)
	}
}

func TestExecHook_NoQuerySkips(t *testing.T) {
	db, err := OpenMemory(nil)
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer db.Close()

	meta := map[string]map[string]any{
		"step": {"other_key": "value"},
	}
	hook := NewExecHook(db, meta)
	art := hookArtifact{data: map[string]any{"id": "1"}}

	if err := hook.Run(context.Background(), "step", art); err != nil {
		t.Errorf("Run with no sqlite_query should return nil, got: %v", err)
	}
}

func TestExecHook_ExecQuery(t *testing.T) {
	db, err := OpenMemory(nil)
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer db.Close()

	_, err = db.Exec("CREATE TABLE cases (id TEXT, status TEXT)")
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	_, err = db.Exec("INSERT INTO cases VALUES ('C1', 'open')")
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	meta := map[string]map[string]any{
		"update-status": {
			"sqlite_query":  "UPDATE cases SET status = ? WHERE id = ?",
			"sqlite_params": []any{"closed", "C1"},
		},
	}

	hook := NewExecHook(db, meta)
	art := hookArtifact{data: map[string]any{"case_id": "C1"}}

	if err := hook.Run(context.Background(), "update-status", art); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var status string
	if err := db.QueryRow("SELECT status FROM cases WHERE id = 'C1'").Scan(&status); err != nil {
		t.Fatalf("query: %v", err)
	}
	if status != "closed" {
		t.Errorf("status = %q, want closed", status)
	}
}

func TestExecHook_TemplateRendering(t *testing.T) {
	db, err := OpenMemory(nil)
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer db.Close()

	_, err = db.Exec("CREATE TABLE results (node TEXT, value TEXT)")
	if err != nil {
		t.Fatalf("create table: %v", err)
	}

	meta := map[string]map[string]any{
		"store": {
			"sqlite_query":  "INSERT INTO results VALUES ('{{ .NodeName }}', '{{ .finding }}')",
			"sqlite_params": []any{},
		},
	}

	hook := NewExecHook(db, meta)
	art := hookArtifact{data: map[string]any{"finding": "bug-123"}}

	if err := hook.Run(context.Background(), "store", art); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var node, value string
	if err := db.QueryRow("SELECT node, value FROM results").Scan(&node, &value); err != nil {
		t.Fatalf("query: %v", err)
	}
	if node != "store" {
		t.Errorf("node = %q, want store", node)
	}
	if value != "bug-123" {
		t.Errorf("value = %q, want bug-123", value)
	}
}

var _ engine.Hook = (*ExecHook)(nil)
