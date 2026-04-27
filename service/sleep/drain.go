package sleep

import "github.com/dpopsuev/origami/memory"

// Drain performs the two-sweep memory lifecycle.
// Sweep 1: append working memory to saved memory.
// Sweep 2: consolidate (fusion/fission/decay).
type Drain interface {
	Sweep(mesh memory.Mesh) error
	Consolidate(mesh memory.Mesh) error
}
