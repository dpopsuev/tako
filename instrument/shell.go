package instrument

import (
	"context"
	"encoding/json"
)

// Shell is the Workstation's workbench — instruments are the tools on the bench.
// Three levels of awareness: Names (L0), Describe (L1), Schema (L2).
type Shell interface {
	Names() []string
	Describe(name string) (string, error)
	Schema(name string) (json.RawMessage, error)
	Exec(ctx context.Context, name string, input json.RawMessage) (Result, error)
}
