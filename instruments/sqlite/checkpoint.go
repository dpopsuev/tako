package sqlite

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/dpopsuev/origami/circuit"
	_ "modernc.org/sqlite" // register sqlite3 driver
)

// SQLiteCheckpointer implements circuit.Checkpointer backed by a SQLite
// database. Each walker ID maps to one row. Thread-safe for concurrent
// walkers with distinct IDs.
type SQLiteCheckpointer struct {
	mu sync.Mutex
	db *sql.DB
}

var _ circuit.Checkpointer = (*SQLiteCheckpointer)(nil)

// NewCheckpointer creates a SQLiteCheckpointer, auto-creating the DB and table.
func NewCheckpointer(dbPath string) (*SQLiteCheckpointer, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", dbPath, err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS checkpoints (
		id TEXT PRIMARY KEY,
		state BLOB NOT NULL,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("create checkpoints table: %w", err)
	}
	return &SQLiteCheckpointer{db: db}, nil
}

func (c *SQLiteCheckpointer) Save(state *circuit.WalkerState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	_, err = c.db.Exec(
		`INSERT INTO checkpoints (id, state, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(id) DO UPDATE SET state = excluded.state, updated_at = CURRENT_TIMESTAMP`,
		state.ID, data,
	)
	if err != nil {
		return fmt.Errorf("save checkpoint %s: %w", state.ID, err)
	}
	return nil
}

func (c *SQLiteCheckpointer) Load(id string) (*circuit.WalkerState, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var data []byte
	err := c.db.QueryRow("SELECT state FROM checkpoints WHERE id = ?", id).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load checkpoint %s: %w", id, err)
	}
	var state circuit.WalkerState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("unmarshal checkpoint %s: %w", id, err)
	}
	if state.LoopCounts == nil {
		state.LoopCounts = make(map[string]int)
	}
	if state.Context == nil {
		state.Context = make(map[string]any)
	}
	if state.Outputs == nil {
		state.Outputs = make(map[string]circuit.Artifact)
	}
	return &state, nil
}

func (c *SQLiteCheckpointer) Remove(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, err := c.db.Exec("DELETE FROM checkpoints WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("remove checkpoint %s: %w", id, err)
	}
	return nil
}

func (c *SQLiteCheckpointer) List() ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	rows, err := c.db.Query("SELECT id FROM checkpoints ORDER BY updated_at")
	if err != nil {
		return nil, fmt.Errorf("list checkpoints: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan checkpoint id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// Close releases the database connection.
func (c *SQLiteCheckpointer) Close() error {
	return c.db.Close()
}
