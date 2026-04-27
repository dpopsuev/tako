package vision

import "time"

// Frame is a single rendered snapshot of a Canvas subtree.
type Frame struct {
	SubtreeID string
	Data      []byte
	Timestamp time.Time
}

// Stream provides on-demand unicast rendering per client per subtree.
type Stream interface {
	Subscribe(subtreeID string) <-chan Frame
	Unsubscribe(subtreeID string)
}
