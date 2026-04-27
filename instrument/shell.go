package instrument

import "context"

// Shell is the Workstation's workbench — instruments are the tools on the bench.
// Three levels of awareness: Names (L0), Signature (L1), Manual (L2).
type Shell interface {
	Names() []string
	Signature(name string) (string, error)
	Manual(name string) (string, error)
	Exec(ctx context.Context, name string, input []byte) (Result, error)
}
