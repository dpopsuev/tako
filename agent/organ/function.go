package organ

import (
	"context"
	"encoding/json"
)

// Function is the interface every instrument implements.
// Self-executing: Name + Description + Schema for discovery, Execute for dispatch.
type Function interface {
	Name() string
	Description() string
	InputSchema() json.RawMessage
	Execute(ctx context.Context, input json.RawMessage) (Result, error)
}
