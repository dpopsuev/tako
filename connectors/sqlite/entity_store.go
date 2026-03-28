package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
)

const colTypeInteger = "integer"

// Row represents a database row as column-name → value pairs.
// Values are typed by their schema column type:
//
//	integer → int64, text → string, real → float64, blob → []byte
type Row map[string]any

// String returns the string value for key, or "" if absent/nil.
func (r Row) String(key string) string {
	v, ok := r[key]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// Int64 returns the int64 value for key, or 0 if absent/nil.
func (r Row) Int64(key string) int64 {
	v, ok := r[key]
	if !ok || v == nil {
		return 0
	}
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	default:
		return 0
	}
}

// Int returns the int value for key, or 0 if absent/nil.
func (r Row) Int(key string) int {
	return int(r.Int64(key))
}

// Float64 returns the float64 value for key, or 0 if absent/nil.
func (r Row) Float64(key string) float64 {
	v, ok := r[key]
	if !ok || v == nil {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return n
	case int64:
		return float64(n)
	case int:
		return float64(n)
	default:
		return 0
	}
}

// Bool returns true if the integer value for key is non-zero.
func (r Row) Bool(key string) bool {
	return r.Int64(key) != 0
}

// Bytes returns the []byte value for key, or nil if absent/nil.
func (r Row) Bytes(key string) []byte {
	v, ok := r[key]
	if !ok || v == nil {
		return nil
	}
	if b, ok := v.([]byte); ok {
		return b
	}
	return nil
}

// EntityStore provides schema-driven CRUD operations on a SQLite database.
// It derives INSERT/SELECT/UPDATE/DELETE from the parsed Schema, eliminating
// hand-written SQL for entity operations.
type EntityStore struct {
	db     *DB
	tables map[string]*Table
}

// NewEntityStore wraps a DB with schema-driven entity CRUD.
func NewEntityStore(db *DB) *EntityStore {
	tables := make(map[string]*Table)
	if db.schema != nil {
		for i := range db.schema.Tables {
			t := &db.schema.Tables[i]
			tables[t.Name] = t
		}
	}
	return &EntityStore{db: db, tables: tables}
}

// DB returns the underlying *DB for raw SQL operations.
func (e *EntityStore) DB() *DB { return e.db }

// Create inserts a row into the named table and returns the auto-generated ID.
// Autoincrement primary key columns should be omitted from the row.
func (e *EntityStore) Create(table string, row Row) (int64, error) {
	t, ok := e.tables[table]
	if !ok {
		return 0, fmt.Errorf("entity create: unknown table %q", table)
	}

	cols := make([]string, 0, len(t.Columns))
	vals := make([]any, 0, len(t.Columns))
	for _, c := range t.Columns {
		if c.PrimaryKey && c.Autoincrement {
			continue
		}
		v, exists := row[c.Name]
		if !exists {
			continue
		}
		cols = append(cols, c.Name)
		vals = append(vals, v)
	}

	if len(cols) == 0 {
		return 0, fmt.Errorf("entity create %s: no columns provided", table)
	}

	return e.db.Insert(InsertParams{
		Table:   table,
		Columns: cols,
		Values:  vals,
	})
}

// Get retrieves one row by primary key ID. Returns nil, nil if not found.
func (e *EntityStore) Get(table string, id int64) (Row, error) {
	t, ok := e.tables[table]
	if !ok {
		return nil, fmt.Errorf("entity get: unknown table %q", table)
	}

	pk := primaryKeyCol(t)
	cols := columnNames(t)
	dests := makeScanDest(t)

	query := fmt.Sprintf("SELECT %s FROM %s WHERE %s = ?",
		strings.Join(cols, ", "), table, pk)

	err := e.db.QueryRow(query, id).Scan(dests...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("entity get %s: %w", table, err)
	}
	return destToRow(t, dests), nil
}

// GetBy retrieves one row matching the given column-value conditions.
// Returns nil, nil if not found.
func (e *EntityStore) GetBy(table string, where Row) (Row, error) {
	t, ok := e.tables[table]
	if !ok {
		return nil, fmt.Errorf("entity get_by: unknown table %q", table)
	}
	if len(where) == 0 {
		return nil, fmt.Errorf("entity get_by %s: conditions required", table)
	}

	cols := columnNames(t)
	dests := makeScanDest(t)

	whereClause, args := buildWhere(where)
	query := fmt.Sprintf("SELECT %s FROM %s WHERE %s LIMIT 1",
		strings.Join(cols, ", "), table, whereClause)

	err := e.db.QueryRow(query, args...).Scan(dests...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("entity get_by %s: %w", table, err)
	}
	return destToRow(t, dests), nil
}

// List retrieves rows matching the optional conditions, ordered by orderBy.
// Pass nil/empty where for all rows. Pass empty orderBy for default (PK ASC).
func (e *EntityStore) List(table string, where Row, orderBy string) ([]Row, error) {
	t, ok := e.tables[table]
	if !ok {
		return nil, fmt.Errorf("entity list: unknown table %q", table)
	}

	cols := columnNames(t)

	var b strings.Builder
	fmt.Fprintf(&b, "SELECT %s FROM %s", strings.Join(cols, ", "), table)

	var args []any
	if len(where) > 0 {
		whereClause, whereArgs := buildWhere(where)
		fmt.Fprintf(&b, " WHERE %s", whereClause)
		args = whereArgs
	}

	if orderBy == "" {
		orderBy = primaryKeyCol(t) + " ASC"
	}
	fmt.Fprintf(&b, " ORDER BY %s", orderBy)

	rows, err := e.db.Query(b.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("entity list %s: %w", table, err)
	}
	defer rows.Close()

	var out []Row
	for rows.Next() {
		dests := makeScanDest(t)
		if err := rows.Scan(dests...); err != nil {
			return nil, fmt.Errorf("entity list %s scan: %w", table, err)
		}
		out = append(out, destToRow(t, dests))
	}
	return out, rows.Err()
}

// Update sets columns on the row identified by primary key.
func (e *EntityStore) Update(table string, id int64, set Row) error {
	t, ok := e.tables[table]
	if !ok {
		return fmt.Errorf("entity update: unknown table %q", table)
	}

	pk := primaryKeyCol(t)
	n, err := e.db.Update(UpdateParams{
		Table: table,
		Set:   map[string]any(set),
		Where: pk + " = ?",
		Args:  []any{id},
	})
	if err != nil {
		return fmt.Errorf("entity update %s: %w", table, err)
	}
	if n == 0 {
		return fmt.Errorf("entity update %s: id %d not found", table, id)
	}
	return nil
}

// Delete removes a row by primary key.
func (e *EntityStore) Delete(table string, id int64) error {
	t, ok := e.tables[table]
	if !ok {
		return fmt.Errorf("entity delete: unknown table %q", table)
	}

	pk := primaryKeyCol(t)
	_, err := e.db.ExecSQL(
		fmt.Sprintf("DELETE FROM %s WHERE %s = ?", table, pk), id)
	if err != nil {
		return fmt.Errorf("entity delete %s: %w", table, err)
	}
	return nil
}

// --- helpers ---

func primaryKeyCol(t *Table) string {
	for _, c := range t.Columns {
		if c.PrimaryKey {
			return c.Name
		}
	}
	return ""
}

func columnNames(t *Table) []string {
	cols := make([]string, len(t.Columns))
	for i, c := range t.Columns {
		cols[i] = c.Name
	}
	return cols
}

func makeScanDest(t *Table) []any {
	dests := make([]any, len(t.Columns))
	for i, c := range t.Columns {
		switch strings.ToLower(c.Type) {
		case colTypeInteger:
			dests[i] = new(sql.NullInt64)
		case "real":
			dests[i] = new(sql.NullFloat64)
		case "blob":
			dests[i] = new([]byte)
		default:
			dests[i] = new(sql.NullString)
		}
	}
	return dests
}

func destToRow(t *Table, dests []any) Row {
	row := make(Row, len(t.Columns))
	for i, c := range t.Columns {
		switch v := dests[i].(type) {
		case *sql.NullInt64:
			if v.Valid {
				row[c.Name] = v.Int64
			} else {
				row[c.Name] = int64(0)
			}
		case *sql.NullString:
			if v.Valid {
				row[c.Name] = v.String
			} else {
				row[c.Name] = ""
			}
		case *sql.NullFloat64:
			if v.Valid {
				row[c.Name] = v.Float64
			} else {
				row[c.Name] = float64(0)
			}
		case *[]byte:
			row[c.Name] = *v
		}
	}
	return row
}

func buildWhere(where Row) (clause string, args []any) {
	keys := make([]string, 0, len(where))
	for k := range where {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, col := range keys {
		val := where[col]
		if val == nil {
			parts = append(parts, col+" IS NULL")
		} else {
			parts = append(parts, col+" = ?")
			args = append(args, val)
		}
	}
	return strings.Join(parts, " AND "), args
}
