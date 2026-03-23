package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dpopsuev/origami/circuit"
	_ "modernc.org/sqlite"
)

// PersistentStore implements circuit.MemoryStore backed by SQLite.
// Data survives process restarts.
type PersistentStore struct {
	mu sync.RWMutex
	db *sql.DB
}

var _ circuit.MemoryStore = (*PersistentStore)(nil)

// NewMemoryStore creates a PersistentStore, auto-creating the DB and table.
func NewMemoryStore(dbPath string) (*PersistentStore, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", dbPath, err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS memories (
		namespace TEXT NOT NULL DEFAULT '',
		walker_id TEXT NOT NULL,
		key TEXT NOT NULL,
		value BLOB NOT NULL,
		tags TEXT NOT NULL DEFAULT '',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (namespace, walker_id, key)
	)`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("create memories table: %w", err)
	}
	return &PersistentStore{db: db}, nil
}

func (s *PersistentStore) Get(walkerID, key string) (any, bool) {
	return s.GetNS("", walkerID, key)
}

func (s *PersistentStore) Set(walkerID, key string, value any) {
	s.SetNS("", walkerID, key, value)
}

func (s *PersistentStore) Keys(walkerID string) []string {
	return s.KeysNS("", walkerID)
}

func (s *PersistentStore) GetNS(namespace, walkerID, key string) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var data []byte
	err := s.db.QueryRow(
		"SELECT value FROM memories WHERE namespace = ? AND walker_id = ? AND key = ?",
		namespace, walkerID, key,
	).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, false
	}
	if err != nil {
		return nil, false
	}
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, false
	}
	return v, true
}

func (s *PersistentStore) SetNS(namespace, walkerID, key string, value any) {
	data, err := json.Marshal(value)
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = s.db.Exec(
		`INSERT INTO memories (namespace, walker_id, key, value, created_at) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(namespace, walker_id, key) DO UPDATE SET value = excluded.value, created_at = excluded.created_at`,
		namespace, walkerID, key, data, time.Now().UTC(),
	)
}

func (s *PersistentStore) KeysNS(namespace, walkerID string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.Query(
		"SELECT key FROM memories WHERE namespace = ? AND walker_id = ? ORDER BY key",
		namespace, walkerID,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var keys []string
	for rows.Next() {
		var k string
		if rows.Scan(&k) == nil {
			keys = append(keys, k)
		}
	}
	return keys
}

func (s *PersistentStore) Search(namespace, query string) []circuit.MemoryItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	pattern := "%" + query + "%"
	rows, err := s.db.Query(
		`SELECT namespace, walker_id, key, value, tags, created_at FROM memories
		 WHERE namespace = ? AND (key LIKE ? OR value LIKE ? OR tags LIKE ?)`,
		namespace, pattern, pattern, pattern,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var items []circuit.MemoryItem
	for rows.Next() {
		var item circuit.MemoryItem
		var data []byte
		var tags string
		var created time.Time
		if rows.Scan(&item.Namespace, &item.WalkerID, &item.Key, &data, &tags, &created) != nil {
			continue
		}
		_ = json.Unmarshal(data, &item.Value)
		if tags != "" {
			item.Tags = strings.Split(tags, ",")
		}
		item.CreatedAt = created
		items = append(items, item)
	}
	return items
}

// SetNSTagged stores a value with tags.
func (s *PersistentStore) SetNSTagged(namespace, walkerID, key string, value any, tags []string) {
	data, err := json.Marshal(value)
	if err != nil {
		return
	}
	tagStr := strings.Join(tags, ",")
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = s.db.Exec(
		`INSERT INTO memories (namespace, walker_id, key, value, tags, created_at) VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(namespace, walker_id, key) DO UPDATE SET value = excluded.value, tags = excluded.tags, created_at = excluded.created_at`,
		namespace, walkerID, key, data, tagStr, time.Now().UTC(),
	)
}

// Close releases the database connection.
func (s *PersistentStore) Close() error {
	return s.db.Close()
}
