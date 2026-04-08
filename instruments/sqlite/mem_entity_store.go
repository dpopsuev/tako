package sqlite

import (
	"fmt"
	"sort"
	"sync"
)

type memTable struct {
	nextID int64
	rows   map[int64]Row
}

// MemEntityStore provides in-memory entity CRUD for testing.
// It uses schema metadata for table awareness but stores data in maps.
type MemEntityStore struct {
	mu     sync.Mutex
	tables map[string]*Table
	data   map[string]*memTable
}

// NewMemEntityStore creates an in-memory entity store from schema metadata.
func NewMemEntityStore(schema *Schema) *MemEntityStore {
	tables := make(map[string]*Table)
	data := make(map[string]*memTable)
	if schema != nil {
		for i := range schema.Tables {
			t := &schema.Tables[i]
			tables[t.Name] = t
			data[t.Name] = &memTable{rows: make(map[int64]Row)}
		}
	}
	return &MemEntityStore{tables: tables, data: data}
}

// Create inserts a row and returns the auto-generated ID.
func (m *MemEntityStore) Create(table string, row Row) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.tables[table]
	if !ok {
		return 0, fmt.Errorf("mem entity create: %w: %q", ErrUnknownTable, table)
	}

	mt := m.data[table]
	mt.nextID++
	id := mt.nextID

	cp := make(Row, len(row)+1)
	if pk := primaryKeyCol(t); pk != "" {
		cp[pk] = id
	}
	for k, v := range row {
		cp[k] = v
	}

	mt.rows[id] = cp
	return id, nil
}

// Get retrieves one row by primary key ID. Returns nil, nil if not found.
func (m *MemEntityStore) Get(table string, id int64) (Row, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	mt, ok := m.data[table]
	if !ok {
		return nil, fmt.Errorf("mem entity get: %w: %q", ErrUnknownTable, table)
	}

	row, ok := mt.rows[id]
	if !ok {
		return nil, nil
	}
	return copyRow(row), nil
}

// GetBy retrieves one row matching the given conditions. Returns nil, nil if not found.
func (m *MemEntityStore) GetBy(table string, where Row) (Row, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	mt, ok := m.data[table]
	if !ok {
		return nil, fmt.Errorf("mem entity get_by: %w: %q", ErrUnknownTable, table)
	}

	// Iterate in ID order for determinism.
	ids := sortedIDs(mt.rows)
	for _, id := range ids {
		row := mt.rows[id]
		if matchesWhere(row, where) {
			return copyRow(row), nil
		}
	}
	return nil, nil
}

// List retrieves rows matching the optional conditions, sorted by ID ASC.
func (m *MemEntityStore) List(table string, where Row, _ string) ([]Row, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	mt, ok := m.data[table]
	if !ok {
		return nil, fmt.Errorf("mem entity list: %w: %q", ErrUnknownTable, table)
	}

	ids := sortedIDs(mt.rows)
	var out []Row
	for _, id := range ids {
		row := mt.rows[id]
		if len(where) == 0 || matchesWhere(row, where) {
			out = append(out, copyRow(row))
		}
	}
	return out, nil
}

// Update merges set into the stored row identified by primary key.
func (m *MemEntityStore) Update(table string, id int64, set Row) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	mt, ok := m.data[table]
	if !ok {
		return fmt.Errorf("mem entity update: %w: %q", ErrUnknownTable, table)
	}

	row, ok := mt.rows[id]
	if !ok {
		return fmt.Errorf("mem entity update %s: id %d: %w", table, id, ErrNotFound)
	}

	for k, v := range set {
		row[k] = v
	}
	return nil
}

// Delete removes a row by primary key.
func (m *MemEntityStore) Delete(table string, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	mt, ok := m.data[table]
	if !ok {
		return fmt.Errorf("mem entity delete: %w: %q", ErrUnknownTable, table)
	}

	delete(mt.rows, id)
	return nil
}

// Mutate applies fn to the stored row identified by id under the lock.
// fn receives a mutable reference — changes persist directly.
func (m *MemEntityStore) Mutate(table string, id int64, fn func(Row)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	mt, ok := m.data[table]
	if !ok {
		return fmt.Errorf("mem entity mutate: %w: %q", ErrUnknownTable, table)
	}

	row, ok := mt.rows[id]
	if !ok {
		return fmt.Errorf("mem entity mutate %s: id %d: %w", table, id, ErrNotFound)
	}

	fn(row)
	return nil
}

// MutateAll calls fn for each row in the table.
// fn should return true if it mutated the row. Returns the count of mutated rows.
func (m *MemEntityStore) MutateAll(table string, fn func(Row) bool) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	mt, ok := m.data[table]
	if !ok {
		return 0, fmt.Errorf("mem entity mutate_all: %w: %q", ErrUnknownTable, table)
	}

	var count int64
	for _, row := range mt.rows {
		if fn(row) {
			count++
		}
	}
	return count, nil
}

// --- helpers ---

func copyRow(r Row) Row {
	cp := make(Row, len(r))
	for k, v := range r {
		cp[k] = v
	}
	return cp
}

func matchesWhere(row, where Row) bool {
	for col, val := range where {
		rv, ok := row[col]
		if val == nil {
			if ok && rv != nil {
				return false
			}
			continue
		}
		if !ok {
			return false
		}
		if !valuesEqual(rv, val) {
			return false
		}
	}
	return true
}

func valuesEqual(a, b any) bool {
	if ai, aok := toInt64(a); aok {
		if bi, bok := toInt64(b); bok {
			return ai == bi
		}
	}
	if af, aok := toFloat64(a); aok {
		if bf, bok := toFloat64(b); bok {
			return af == bf
		}
	}
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

func toInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case int:
		return int64(n), true
	case int32:
		return int64(n), true
	default:
		return 0, false
	}
}

func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	default:
		return 0, false
	}
}

func sortedIDs(rows map[int64]Row) []int64 {
	ids := make([]int64, 0, len(rows))
	for id := range rows {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}
