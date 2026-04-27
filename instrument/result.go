package instrument

import "time"

// Structure describes the shape of instrument output.
type Structure int

const (
	Blob   Structure = iota // raw bytes
	List                    // ordered items
	Tree                    // hierarchical
	Graph                   // nodes + edges
	Table                   // rows + columns
	Record                  // key-value pairs
)

// Result is the structured output of an instrument execution.
type Result struct {
	Content   []byte
	Structure Structure
	ExitCode  int
	Duration  time.Duration
}
