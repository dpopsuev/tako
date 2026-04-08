package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dpopsuev/origami/circuit"
	_ "modernc.org/sqlite" // register sqlite3 driver
)

// PersistentStore implements circuit.MemoryStore backed by SQLite.
// Data survives process restarts.
type PersistentStore struct {
	mu       sync.RWMutex
	db       *sql.DB
	embedder circuit.EmbeddingProvider
}

var _ circuit.MemoryStore = (*PersistentStore)(nil)

// PersistentStoreOption configures a PersistentStore.
type PersistentStoreOption func(*PersistentStore)

// WithPersistentEmbeddings enables vector-similarity search using the given provider.
func WithPersistentEmbeddings(p circuit.EmbeddingProvider) PersistentStoreOption {
	return func(s *PersistentStore) {
		s.embedder = p
	}
}

// NewMemoryStore creates a PersistentStore, auto-creating the DB and table.
func NewMemoryStore(dbPath string, opts ...PersistentStoreOption) (*PersistentStore, error) {
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
		embedding TEXT NOT NULL DEFAULT '',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (namespace, walker_id, key)
	)`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("create memories table: %w", err)
	}
	// Migrate: add embedding column if missing (existing databases).
	_, _ = db.Exec(`ALTER TABLE memories ADD COLUMN embedding TEXT NOT NULL DEFAULT ''`)
	s := &PersistentStore{db: db}
	for _, o := range opts {
		o(s)
	}
	return s, nil
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
	if errors.Is(err, sql.ErrNoRows) {
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
	embJSON := ""
	if s.embedder != nil {
		embJSON = s.computeEmbeddingJSON(namespace, walkerID, value)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = s.db.Exec(
		`INSERT INTO memories (namespace, walker_id, key, value, embedding, created_at) VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(namespace, walker_id, key) DO UPDATE SET value = excluded.value, embedding = excluded.embedding, created_at = excluded.created_at`,
		namespace, walkerID, key, data, embJSON, time.Now().UTC(),
	)
}

// computeEmbeddingJSON returns the JSON-encoded embedding vector for value,
// or "" on error.
func (s *PersistentStore) computeEmbeddingJSON(namespace, walkerID string, value any) string {
	text := fmt.Sprintf("%v", value)
	vec, err := s.embedder.Embed(context.Background(), text)
	if err != nil {
		slog.WarnContext(context.Background(), "embedding failed",
			slog.String(circuit.LogKeyNamespace, namespace),
			slog.String(circuit.LogKeyWalkerID, walkerID),
			slog.String(circuit.LogKeyError, err.Error()))
		return ""
	}
	b, err := json.Marshal(vec)
	if err != nil {
		return ""
	}
	return string(b)
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
	if s.embedder != nil {
		return s.searchByEmbedding(namespace, query)
	}
	return s.searchBySubstring(namespace, query)
}

func (s *PersistentStore) searchBySubstring(namespace, query string) []circuit.MemoryItem {
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

type persistentScoredItem struct {
	item  circuit.MemoryItem
	score float64
}

func (s *PersistentStore) searchByEmbedding(namespace, query string) []circuit.MemoryItem {
	qVec, err := s.embedder.Embed(context.Background(), query)
	if err != nil {
		slog.WarnContext(context.Background(), "query embedding failed, falling back to substring",
			slog.String(circuit.LogKeyNamespace, namespace),
			slog.String(circuit.LogKeyQuery, query),
			slog.String(circuit.LogKeyError, err.Error()))
		return s.searchBySubstring(namespace, query)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.Query(
		`SELECT namespace, walker_id, key, value, tags, embedding, created_at FROM memories
		 WHERE namespace = ?`,
		namespace,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var scored []persistentScoredItem
	for rows.Next() {
		var item circuit.MemoryItem
		var data []byte
		var tags, embStr string
		var created time.Time
		if rows.Scan(&item.Namespace, &item.WalkerID, &item.Key, &data, &tags, &embStr, &created) != nil {
			continue
		}
		_ = json.Unmarshal(data, &item.Value)
		if tags != "" {
			item.Tags = strings.Split(tags, ",")
		}
		item.CreatedAt = created
		if embStr == "" {
			continue
		}
		var vec []float64
		if json.Unmarshal([]byte(embStr), &vec) != nil {
			continue
		}
		sim := persistentCosineSimilarity(qVec, vec)
		scored = append(scored, persistentScoredItem{item: item, score: sim})
	}
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})
	results := make([]circuit.MemoryItem, len(scored))
	for i, si := range scored {
		results[i] = si.item
	}
	slog.DebugContext(context.Background(), "embedding search complete",
		slog.String(circuit.LogKeyNamespace, namespace),
		slog.String(circuit.LogKeyQuery, query),
		slog.Int(circuit.LogKeyResults, len(results)))
	return results
}

// persistentCosineSimilarity returns the cosine similarity between two vectors.
func persistentCosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, magA, magB float64
	for i := range a {
		dot += a[i] * b[i]
		magA += a[i] * a[i]
		magB += b[i] * b[i]
	}
	if magA == 0 || magB == 0 {
		return 0
	}
	return dot / (math.Sqrt(magA) * math.Sqrt(magB))
}

// SetNSTagged stores a value with tags.
func (s *PersistentStore) SetNSTagged(namespace, walkerID, key string, value any, tags []string) {
	data, err := json.Marshal(value)
	if err != nil {
		return
	}
	tagStr := strings.Join(tags, ",")
	embJSON := ""
	if s.embedder != nil {
		embJSON = s.computeEmbeddingJSON(namespace, walkerID, value)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = s.db.Exec(
		`INSERT INTO memories (namespace, walker_id, key, value, tags, embedding, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(namespace, walker_id, key) DO UPDATE SET value = excluded.value, tags = excluded.tags, embedding = excluded.embedding, created_at = excluded.created_at`,
		namespace, walkerID, key, data, tagStr, embJSON, time.Now().UTC(),
	)
}

// Close releases the database connection.
func (s *PersistentStore) Close() error {
	return s.db.Close()
}
