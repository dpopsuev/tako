package organ

import "github.com/dpopsuev/tako/artifact"

// Organ is a functional part attached to an agent's Corpus.
// The Uniform declares which Organs attach. Tangled assembles.
type Organ interface {
	Name() string
	Receive(wire artifact.Wire) error
}
