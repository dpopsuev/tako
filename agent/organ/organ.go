package organ

import "github.com/dpopsuev/tako/artifact"

// OrganName is a typed identifier for an Organ.
type OrganName string

const (
	CerebrumOrgan OrganName = "cerebrum"
	Dialog        OrganName = "dialog"
	Kanban        OrganName = "kanban"
	Andon         OrganName = "andon"
)

// Kind classifies an organ's role in the agent.
type Kind int

const (
	Cognitive Kind = iota // the brain — Cerebrum
	Sensory              // involuntary input — events arrive without asking
	Signal               // involuntary output — telemetry, alerting
	Motor                // voluntary actions — each action declares its Mode
)

// ActionMode labels whether an individual action reads or writes.
type ActionMode int

const (
	ReadAction  ActionMode = iota // observation — look, status, check
	WriteAction                   // mutation — take, cook, move
)

// Organ is a functional part attached to an agent's Corpus.
// The Uniform declares which Organs attach. Tangled assembles.
type Organ interface {
	Name() OrganName
	Kind() Kind
	Receive(wire artifact.Wire) error
}
