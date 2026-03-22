package circuit

// Category: Execution

// Checkpointer persists and restores WalkerState between nodes, enabling
// resume-from-failure and crash recovery. Implementations must be safe
// for concurrent use by multiple walkers with distinct IDs.
type Checkpointer interface {
	Save(state *WalkerState) error
	Load(id string) (*WalkerState, error)
	Remove(id string) error
	List() ([]string, error)
}
