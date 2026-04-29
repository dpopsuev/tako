package organ

import "github.com/dpopsuev/tako/artifact"

// OrganName is a typed identifier for an Organ.
type OrganName string

const (
	Dialog      OrganName = "dialog"
	Kanban      OrganName = "kanban"
	Andon       OrganName = "andon"
	Workstation OrganName = "workstation"
)

// Organ is a functional part attached to an agent's Corpus.
// The Uniform declares which Organs attach. Tangled assembles.
type Organ interface {
	Name() OrganName
	Receive(wire artifact.Wire) error
}
