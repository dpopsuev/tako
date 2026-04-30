package motor

import (
	"context"
	"encoding/json"

	"github.com/dpopsuev/tako/instrument"
)

type stubShell struct{}

func (stubShell) Names() []string { return []string{"grep"} }

func (stubShell) Exec(_ context.Context, name string, _ json.RawMessage) (instrument.Result, error) {
	return instrument.TextResult(name + " result"), nil
}

func (stubShell) Describe(name string) (string, error) { return name, nil }

func (stubShell) Schema(name string) (json.RawMessage, error) { return nil, nil }
