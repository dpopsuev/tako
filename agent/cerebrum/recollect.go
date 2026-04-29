package cerebrum

import (
	"fmt"
	"strings"
	"time"

	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tako/memory"
)

func recollect(mesh memory.Mesh, need []byte) []reactivity.Atom {
	if mesh == nil {
		return nil
	}

	nodes := mesh.Nodes()
	if len(nodes) == 0 {
		return nil
	}

	keywords := extractKeywords(string(need))
	var matched []memory.KnowledgeNode
	for _, node := range nodes {
		for _, kw := range keywords {
			if strings.Contains(strings.ToLower(node.Content), strings.ToLower(kw)) {
				matched = append(matched, node)
				break
			}
		}
	}

	atoms := make([]reactivity.Atom, 0, len(matched))
	for i, node := range matched {
		atoms = append(atoms, reactivity.Atom{
			ID:        fmt.Sprintf("recollect-%d", i),
			Type:      reactivity.AssessmentAtom,
			Source:    reactivity.Recollected,
			Taxonomy:  "assessment.recollection.mesh",
			Content:   []byte(node.Content),
			CreatedAt: time.Now(),
		})
	}
	return atoms
}

func extractKeywords(text string) []string {
	words := strings.Fields(text)
	var keywords []string
	for _, w := range words {
		w = strings.ToLower(strings.Trim(w, ".,!?\"'"))
		if len(w) >= 3 {
			keywords = append(keywords, w)
		}
	}
	return keywords
}
