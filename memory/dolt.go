package memory

import (
	"database/sql"
	"errors"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
)

// DoltMesh is a Dolt-backed Mesh.
type DoltMesh struct {
	mu sync.Mutex
	db *sqlx.DB
}

var _ Mesh = (*DoltMesh)(nil)

func NewDoltMesh(db *sqlx.DB) *DoltMesh {
	return &DoltMesh{db: db}
}

func (m *DoltMesh) AddNode(node KnowledgeNode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, err := m.db.Exec(
		`INSERT INTO knowledge_nodes (id, content, tier, created_at) VALUES (?, ?, ?, ?)`,
		node.ID, node.Content, int(node.Tier), node.CreatedAt,
	)
	return err
}

func (m *DoltMesh) AddEdge(edge Edge) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, err := m.db.Exec(
		`INSERT INTO knowledge_edges (from_id, to_id, relation, weight, created_at) VALUES (?, ?, ?, ?, ?)`,
		edge.From, edge.To, edge.Relation, edge.Weight, edge.CreatedAt,
	)
	return err
}

func (m *DoltMesh) Node(id string) (KnowledgeNode, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var row struct {
		ID        string    `db:"id"`
		Content   string    `db:"content"`
		Tier      int       `db:"tier"`
		CreatedAt time.Time `db:"created_at"`
	}
	err := m.db.Get(&row, `SELECT id, content, tier, created_at FROM knowledge_nodes WHERE id = ?`, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return KnowledgeNode{}, ErrNodeNotFound
		}
		return KnowledgeNode{}, err
	}
	return KnowledgeNode{ID: row.ID, Content: row.Content, Tier: Tier(row.Tier), CreatedAt: row.CreatedAt}, nil
}

func (m *DoltMesh) Neighbors(id string) ([]KnowledgeNode, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var ids []string
	err := m.db.Select(&ids, `SELECT to_id FROM knowledge_edges WHERE from_id = ?`, id)
	if err != nil {
		return nil, err
	}
	var out []KnowledgeNode
	for _, nid := range ids {
		var row struct {
			ID        string    `db:"id"`
			Content   string    `db:"content"`
			Tier      int       `db:"tier"`
			CreatedAt time.Time `db:"created_at"`
		}
		if err := m.db.Get(&row, `SELECT id, content, tier, created_at FROM knowledge_nodes WHERE id = ?`, nid); err == nil {
			out = append(out, KnowledgeNode{ID: row.ID, Content: row.Content, Tier: Tier(row.Tier), CreatedAt: row.CreatedAt})
		}
	}
	return out, nil
}

func (m *DoltMesh) Walk(startID string, fn WalkFunc) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	visited := make(map[string]bool)
	return m.walk(startID, fn, visited, 0)
}

func (m *DoltMesh) walk(id string, fn WalkFunc, visited map[string]bool, depth int) error {
	if visited[id] {
		return nil
	}
	visited[id] = true
	var row struct {
		ID        string    `db:"id"`
		Content   string    `db:"content"`
		Tier      int       `db:"tier"`
		CreatedAt time.Time `db:"created_at"`
	}
	err := m.db.Get(&row, `SELECT id, content, tier, created_at FROM knowledge_nodes WHERE id = ?`, id)
	if err != nil {
		return err
	}
	node := KnowledgeNode{ID: row.ID, Content: row.Content, Tier: Tier(row.Tier), CreatedAt: row.CreatedAt}
	if !fn(node, depth) {
		return nil
	}
	var neighbors []string
	if err := m.db.Select(&neighbors, `SELECT to_id FROM knowledge_edges WHERE from_id = ?`, id); err != nil {
		return err
	}
	for _, nid := range neighbors {
		if err := m.walk(nid, fn, visited, depth+1); err != nil {
			return err
		}
	}
	return nil
}

func (m *DoltMesh) Nodes() []KnowledgeNode {
	m.mu.Lock()
	defer m.mu.Unlock()
	var rows []struct {
		ID        string    `db:"id"`
		Content   string    `db:"content"`
		Tier      int       `db:"tier"`
		CreatedAt time.Time `db:"created_at"`
	}
	if err := m.db.Select(&rows, `SELECT id, content, tier, created_at FROM knowledge_nodes`); err != nil {
		return nil
	}
	out := make([]KnowledgeNode, 0, len(rows))
	for _, row := range rows {
		out = append(out, KnowledgeNode{ID: row.ID, Content: row.Content, Tier: Tier(row.Tier), CreatedAt: row.CreatedAt})
	}
	return out
}

func (m *DoltMesh) Edges() []Edge {
	m.mu.Lock()
	defer m.mu.Unlock()
	var rows []struct {
		From      string    `db:"from_id"`
		To        string    `db:"to_id"`
		Relation  string    `db:"relation"`
		Weight    float64   `db:"weight"`
		CreatedAt time.Time `db:"created_at"`
	}
	if err := m.db.Select(&rows, `SELECT from_id, to_id, relation, weight, created_at FROM knowledge_edges`); err != nil {
		return nil
	}
	out := make([]Edge, 0, len(rows))
	for _, row := range rows {
		out = append(out, Edge{From: row.From, To: row.To, Relation: row.Relation, Weight: row.Weight, CreatedAt: row.CreatedAt})
	}
	return out
}
