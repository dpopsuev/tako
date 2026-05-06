package cerebrum

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/dpopsuev/tako/agent/reactivity"
)

type Consolidator interface {
	Consolidate(m *reactivity.Molecule, need []byte) error
}

type BookAtom struct {
	Type     string `json:"type"`
	Taxonomy string `json:"taxonomy"`
	Content  string `json:"content"`
}

func ResidualPattern(m *reactivity.Molecule) string {
	r := m.Residual()
	if r == nil {
		return ""
	}
	data, _ := json.Marshal(r)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func ExtractBook(m *reactivity.Molecule) []BookAtom {
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
	return book
}
