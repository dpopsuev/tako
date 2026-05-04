package organ

import (
	"context"
	"encoding/json"
)

// Shell groups multiple Functions under a single discoverable interface.
// Three levels of awareness: Names (L0), Describe (L1), Schema (L2).
type Shell interface {
	Names() []string
	Describe(name string) (string, error)
	Schema(name string) (json.RawMessage, error)
	Exec(ctx context.Context, name string, input json.RawMessage) (Result, error)
}
