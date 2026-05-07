package organ

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
