package agentport

import "github.com/dpopsuev/troupe/arsenal"

// Arsenal type aliases — model selection catalog.
type (
	Arsenal       = arsenal.Arsenal
	ResolvedAgent = arsenal.ResolvedAgent
	Preferences   = arsenal.Preferences
	TraitVector   = arsenal.TraitVector
	ModelEntry    = arsenal.ModelEntry
	SourceEntry   = arsenal.SourceEntry
	CostEntry     = arsenal.CostEntry
	Filter        = arsenal.Filter
)

// Constructors.
var NewArsenal = arsenal.NewArsenal

// Sentinel errors.
var (
	ErrArsenalNotFound    = arsenal.ErrNotFound
	ErrArsenalNoCandidate = arsenal.ErrNoCandidate
	ErrArsenalBadPin      = arsenal.ErrBadPin
)
