package sqlite

import (
	"database/sql"
	"fmt"
	"strings"
)

// InsertParams holds the parameters for an INSERT operation.
type InsertParams struct {
	Table   string
	Columns []string
	Values  []any
}

// Insert executes a parameterized INSERT and returns the last inserted row ID.
func (d *DB) Insert(p InsertParams) (int64, error) {
	if p.Table == "" {
		return 0, fmt.Errorf("insert: table name is required")
	}
	if len(p.Columns) == 0 {
		return 0, fmt.Errorf("insert into %s: columns are required", p.Table)
	}
	if len(p.Columns) != len(p.Values) {
		return 0, fmt.Errorf("insert into %s: columns/values count mismatch (%d vs %d)",
			p.Table, len(p.Columns), len(p.Values))
	}

	placeholders := make([]string, len(p.Values))
	for i := range placeholders {
		placeholders[i] = "?"
	}

	query := fmt.Sprintf("INSERT INTO %s(%s) VALUES(%s)",
		p.Table,
		strings.Join(p.Columns, ", "),
		strings.Join(placeholders, ", "))

	res, err := d.Exec(query, p.Values...)
	if err != nil {
		return 0, fmt.Errorf("insert into %s: %w", p.Table, err)
	}
	return res.LastInsertId()
}

// QueryParams holds the parameters for a SELECT operation.
type QueryParams struct {
	Table   string
	Columns []string
	Where   string
	Args    []any
	OrderBy string
	Limit   int
}

// QueryRows executes a parameterized SELECT and returns the rows.
// Caller is responsible for closing the rows.
func (d *DB) QueryRows(p *QueryParams) (*sql.Rows, error) {
	if p.Table == "" {
		return nil, fmt.Errorf("query: table name is required")
	}
	cols := "*"
	if len(p.Columns) > 0 {
		cols = strings.Join(p.Columns, ", ")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "SELECT %s FROM %s", cols, p.Table)
	if p.Where != "" {
		fmt.Fprintf(&b, " WHERE %s", p.Where)
	}
	if p.OrderBy != "" {
		fmt.Fprintf(&b, " ORDER BY %s", p.OrderBy)
	}
	if p.Limit > 0 {
		fmt.Fprintf(&b, " LIMIT %d", p.Limit)
	}

	return d.Query(b.String(), p.Args...)
}

// QueryRow executes a parameterized SELECT expecting a single row.
func (d *DB) QueryOne(p *QueryParams) *sql.Row {
	p.Limit = 1
	cols := "*"
	if len(p.Columns) > 0 {
		cols = strings.Join(p.Columns, ", ")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "SELECT %s FROM %s", cols, p.Table)
	if p.Where != "" {
		fmt.Fprintf(&b, " WHERE %s", p.Where)
	}
	if p.OrderBy != "" {
		fmt.Fprintf(&b, " ORDER BY %s", p.OrderBy)
	}
	fmt.Fprintf(&b, " LIMIT 1")

	return d.DB.QueryRow(b.String(), p.Args...)
}

// UpdateParams holds the parameters for an UPDATE operation.
type UpdateParams struct {
	Table   string
	Set     map[string]any
	Where   string
	Args    []any
}

// Update executes a parameterized UPDATE and returns the number of affected rows.
func (d *DB) Update(p UpdateParams) (int64, error) {
	if p.Table == "" {
		return 0, fmt.Errorf("update: table name is required")
	}
	if len(p.Set) == 0 {
		return 0, fmt.Errorf("update %s: set columns are required", p.Table)
	}

	setClauses := make([]string, 0, len(p.Set))
	args := make([]any, 0, len(p.Set)+len(p.Args))
	for col, val := range p.Set {
		setClauses = append(setClauses, col+" = ?")
		args = append(args, val)
	}
	args = append(args, p.Args...)

	var b strings.Builder
	fmt.Fprintf(&b, "UPDATE %s SET %s", p.Table, strings.Join(setClauses, ", "))
	if p.Where != "" {
		fmt.Fprintf(&b, " WHERE %s", p.Where)
	}

	res, err := d.Exec(b.String(), args...)
	if err != nil {
		return 0, fmt.Errorf("update %s: %w", p.Table, err)
	}
	return res.RowsAffected()
}

// ExecSQL executes a raw parameterized SQL statement.
// Use for complex queries (JOINs, subqueries) that don't fit the helpers.
func (d *DB) ExecSQL(query string, args ...any) (sql.Result, error) {
	return d.Exec(query, args...)
}

// QuerySQL executes a raw parameterized SQL query and returns rows.
// Caller is responsible for closing the rows.
func (d *DB) QuerySQL(query string, args ...any) (*sql.Rows, error) {
	return d.Query(query, args...)
}

// QueryRowSQL executes a raw parameterized SQL query expecting a single row.
func (d *DB) QueryRowSQL(query string, args ...any) *sql.Row {
	return d.DB.QueryRow(query, args...)
}
