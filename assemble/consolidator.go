package assemble

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tako/memory"
)

type MeshConsolidator struct {
	Mesh           memory.Mesh
	PromotionCount int
	WisdomCount    int
}

func NewMeshConsolidator(mesh memory.Mesh) *MeshConsolidator {
	return &MeshConsolidator{
		Mesh:           mesh,
		PromotionCount: 3,
		WisdomCount:    5,
	}
}

var _ cerebrum.Consolidator = (*MeshConsolidator)(nil)

func (c *MeshConsolidator) Consolidate(m *reactivity.Molecule, need []byte) error {
	pattern := cerebrum.ResidualPattern(m)
	if pattern == "" {
		return nil
	}

	existing := c.findByPattern(pattern)

	if existing == nil {
		return c.createKnowledge(m, need, pattern)
	}

	if err := c.recordAccess(existing.ID); err != nil {
		return err
	}
	count := c.accessCount(existing.ID)

	switch existing.Tier {
	case memory.Knowledge:
		if count >= c.PromotionCount {
			return c.promote(existing, memory.Understanding)
		}
	case memory.Understanding:
		if count >= c.WisdomCount {
			return c.promote(existing, memory.Wisdom)
		}
	}

	return nil
}

func (c *MeshConsolidator) createKnowledge(m *reactivity.Molecule, _ []byte, pattern string) error {
	book := cerebrum.ExtractBook(m)
	data, err := json.Marshal(book)
	if err != nil {
		return err
	}

	nodeID := fmt.Sprintf("book:%s", pattern[:16])
	err = c.Mesh.AddNode(memory.KnowledgeNode{
		ID:        nodeID,
		Content:   string(data),
		Tier:      memory.Knowledge,
		CreatedAt: time.Now(),
	})
	if err != nil {
		return err
	}

	c.Mesh.AddEdge(memory.Edge{
		From:     nodeID,
		To:       "pattern:" + pattern,
		Relation: "residual_pattern",
	})

	slog.Info("consolidator.created",
		slog.String("node", nodeID),
		slog.String("tier", "knowledge"),
		slog.String("pattern", pattern[:16]),
		slog.Int("book_atoms", len(book)))

	return nil
}

func (c *MeshConsolidator) promote(node *memory.KnowledgeNode, tier memory.Tier) error {
	promoted := *node
	promoted.Tier = tier

	if err := c.Mesh.AddNode(promoted); err != nil {
		return err
	}

	tierName := "understanding"
	if tier == memory.Wisdom {
		tierName = "wisdom"
	}
	slog.Info("consolidator.promoted",
		slog.String("node", node.ID),
		slog.String("from", fmt.Sprintf("%d", node.Tier)),
		slog.String("to", tierName))

	return nil
}

func (c *MeshConsolidator) recordAccess(nodeID string) error {
	return c.Mesh.AddEdge(memory.Edge{
		From:      nodeID,
		To:        nodeID,
		Relation:  "accessed",
		CreatedAt: time.Now(),
	})
}

func (c *MeshConsolidator) accessCount(nodeID string) int {
	edges := c.Mesh.Edges()
	count := 0
	for _, e := range edges {
		if e.From == nodeID && e.Relation == "accessed" {
			count++
		}
	}
	return count
}

func (c *MeshConsolidator) findByPattern(pattern string) *memory.KnowledgeNode {
	target := "pattern:" + pattern
	for _, e := range c.Mesh.Edges() {
		if e.To == target && e.Relation == "residual_pattern" {
			if n, err := c.Mesh.Node(e.From); err == nil {
				return &n
			}
		}
	}
	return nil
}

func (c *MeshConsolidator) Decay(maxAge time.Duration) int {
	now := time.Now()
	decayed := 0
	for _, n := range c.Mesh.Nodes() {
		if n.Tier == memory.Wisdom {
			continue
		}
		lastAccess := n.CreatedAt
		for _, e := range c.Mesh.Edges() {
			if e.From == n.ID && e.Relation == "accessed" && e.CreatedAt.After(lastAccess) {
				lastAccess = e.CreatedAt
			}
		}
		idle := now.Sub(lastAccess)
		if idle > maxAge && now.Sub(n.CreatedAt) > maxAge {
			c.Mesh.AddNode(memory.KnowledgeNode{
				ID:        n.ID,
				Content:   n.Content,
				Tier:      memory.Knowledge,
				CreatedAt: n.CreatedAt,
			})
			decayed++
		}
	}
	if decayed > 0 {
		slog.Info("consolidator.decay", slog.Int("demoted", decayed), slog.Duration("max_age", maxAge))
	}
	return decayed
}
