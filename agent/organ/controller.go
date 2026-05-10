package organ

import (
	"context"
	"encoding/json"
)

type Controller interface {
	Name() string
	Description() string
	Schema() json.RawMessage
	Handle(ctx context.Context, input json.RawMessage) (Result, error)
	IsResponse() bool
}

func ControllerFunc(c Controller) Func {
	return Func{
		Name:        c.Name(),
		Description: c.Description(),
		Schema:      c.Schema(),
		Mode:        WriteAction,
		Source:      BuiltIn,
		Response:    c.IsResponse(),
		Execute:     c.Handle,
	}
}
