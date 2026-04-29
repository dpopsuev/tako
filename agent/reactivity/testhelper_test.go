package reactivity

import "time"

func mkAtom(id string, t AtomType, taxonomy string, source AtomSource, targets ...string) Atom {
	return Atom{
		ID:        id,
		Type:      t,
		Source:    source,
		Taxonomy:  taxonomy,
		Content:   []byte(id),
		Targets:   targets,
		CreatedAt: time.Now(),
	}
}
