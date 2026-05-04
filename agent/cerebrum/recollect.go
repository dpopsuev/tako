package cerebrum

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tako/memory"
)

// Recollector retrieves relevant knowledge from the Memory Mesh
// and injects it as Recollected atoms into the Molecule before
// the Think loop starts. The reactor's fuel rods.
type Recollector interface {
	Recollect(need []byte) []reactivity.Atom
}

// MeshRecollector queries a Memory Mesh. Wisdom-tier nodes are book
// moves (serialized atom sequences from prior successful runs).
// Knowledge/Understanding nodes are injected as Knowledge atoms.
type MeshRecollector struct {
	Mesh memory.Mesh
}

func (r MeshRecollector) Recollect(need []byte) []reactivity.Atom {
	nodes := r.Mesh.Nodes()
	if len(nodes) == 0 {
		return nil
	}

	// Check for book moves first (Wisdom tier = full molecule recipe)
	for _, n := range nodes {
		if n.Tier == memory.Wisdom && needMatches(need, n) {
			return decodeBook(n)
		}
	}

	// Fall back to Knowledge injection
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
	return strings.Contains(n.ID, "book:") || strings.Contains(n.Content, string(need)[:min(len(need), 50)])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// BookAtom is the serialized form of an atom in a book.
type BookAtom struct {
	Type    string `json:"type"`
	Taxonomy string `json:"taxonomy"`
	Content string `json:"content"`
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

// PersistMolecule saves a sealed Molecule's atoms to the Mesh as a
// Wisdom-tier book move. Next Recollection with matching Need replays it.
func PersistMolecule(mesh memory.Mesh, m *reactivity.Molecule, need []byte) error {
	var book []BookAtom
	for _, at := range reactivity.AllAtomTypes() {
		if at.Triad == reactivity.ReflectTriad {
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
