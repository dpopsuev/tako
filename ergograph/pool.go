package ergograph

import "errors"

var (
	ErrChainBroken = errors.New("ergograph: hash chain verification failed")
	ErrPoolEmpty   = errors.New("ergograph: pool is empty")
)

// Pool is a label-scoped, append-only collection of Records.
type Pool interface {
	Append(record Record) error
	Records() []Record
	VerifyChain() error
	Len() int
}

// Inspector scores agent effectiveness from Ergograph records.
type Inspector interface {
	Verify(pool Pool) error
	Score(pool Pool) (OAE, error)
}
