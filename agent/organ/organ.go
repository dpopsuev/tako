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

// ActionMode labels whether an individual action reads or writes.
type ActionMode int

const (
	ReadAction  ActionMode = iota // observation — look, status, check
	WriteAction                   // mutation — take, cook, move, deploy
)

// ActionApproval controls whether an action needs human sign-off.
type ActionApproval int

const (
	Auto ActionApproval = iota // agent decides
	HITL                       // human must approve
)

// Organ is a functional part attached to an agent's Corpus.
// Organs connect to buses (sensory, motor, signal) at assembly time.
type Organ interface {
	Name() OrganName
	Receive(wire artifact.Wire) error
}
