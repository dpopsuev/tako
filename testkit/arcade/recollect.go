package arcade

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tako/memory"
)

type BookAtom struct {
	Type     string `json:"type"`
	Taxonomy string `json:"taxonomy"`
	Content  string `json:"content"`
}

type MeshRecollector struct {
	Mesh memory.Mesh
}

func (r MeshRecollector) Recollect(need []byte) []reactivity.Atom {
	nodes := r.Mesh.Nodes()
	if len(nodes) == 0 {
		return nil
	}

	for _, n := range nodes {
		if n.Tier == memory.Wisdom && needMatches(need, n) {
			return decodeBook(n)
		}
	}

	var atoms []reactivity.Atom
	for i, n := range nodes {
		atoms = append(atoms, reactivity.Atom{
			ID:        fmt.Sprintf("recollect-%d", i),
			Type:      reactivity.KnowledgeAtom,
			Source:    reactivity.Recollected,
			Taxonomy:  "knowledge.recollected",
			Content:   []byte(n.Content),
			CreatedAt: time.Now(),
		})
	}
	return atoms
}

func needMatches(need []byte, n memory.KnowledgeNode) bool {
	limit := len(need)
	if limit > 50 {
		limit = 50
	}
	return strings.Contains(n.ID, "book:") || strings.Contains(n.Content, string(need)[:limit])
}

func decodeBook(n memory.KnowledgeNode) []reactivity.Atom {
	var book []BookAtom
	if err := json.Unmarshal([]byte(n.Content), &book); err != nil {
		return nil
	}
	atoms := make([]reactivity.Atom, 0, len(book))
	for i, b := range book {
		atomType := atomTypeFromString(b.Type)
		atoms = append(atoms, reactivity.Atom{
			ID:        fmt.Sprintf("book-%d", i),
			Type:      atomType,
			Source:    reactivity.Recollected,
			Taxonomy:  b.Taxonomy,
			Content:   []byte(b.Content),
			CreatedAt: time.Now(),
		})
	}
	return atoms
}

func atomTypeFromString(s string) reactivity.AtomType {
	for _, at := range reactivity.AllAtomTypes() {
		if at.String() == s {
			return at
		}
	}
	return reactivity.KnowledgeAtom
}

func PersistMolecule(mesh memory.Mesh, m *reactivity.Molecule, need []byte) error {
	var book []BookAtom
	for _, at := range reactivity.AllAtomTypes() {
		if at.Triad == reactivity.ImplementTriad || at.Triad == reactivity.ReflectTriad {
			continue
		}
		for _, atom := range m.Atoms(at) {
			if atom.Source == reactivity.Recollected {
				continue
			}
			book = append(book, BookAtom{
				Type:     at.String(),
				Taxonomy: atom.Taxonomy,
				Content:  string(atom.Content),
			})
		}
	}
	data, err := json.Marshal(book)
	if err != nil {
		return err
	}
	return mesh.AddNode(memory.KnowledgeNode{
		ID:        fmt.Sprintf("book:%d", time.Now().UnixNano()),
		Content:   string(data),
		Tier:      memory.Wisdom,
		CreatedAt: time.Now(),
	})
}
