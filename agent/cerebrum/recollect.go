package cerebrum

import (
	"fmt"
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

// MeshRecollector queries a Memory Mesh and returns all nodes as
// Recollected Knowledge atoms.
type MeshRecollector struct {
	Mesh memory.Mesh
}

func (r MeshRecollector) Recollect(need []byte) []reactivity.Atom {
	nodes := r.Mesh.Nodes()
	if len(nodes) == 0 {
		return nil
	}
	atoms := make([]reactivity.Atom, 0, len(nodes))
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
