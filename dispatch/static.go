package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dpopsuev/origami/agentport"
)

var ErrNoArtifact = errors.New("static dispatcher: no artifact")

// StaticDispatcher returns pre-authored artifact data by looking up
// (CaseID, Step) in a directory of JSON files or in-memory map.
type StaticDispatcher struct {
	dir       string
	artifacts map[string]json.RawMessage
}

func NewStaticDispatcher(dir string) *StaticDispatcher {
	return &StaticDispatcher{dir: dir, artifacts: make(map[string]json.RawMessage)}
}

func (d *StaticDispatcher) Set(caseID, step string, data json.RawMessage) {
	d.artifacts[staticKey(caseID, step)] = data
}

func (d *StaticDispatcher) Dispatch(_ context.Context, ctx agentport.Context) ([]byte, error) {
	key := staticKey(ctx.CaseID, ctx.Step)

	if data, ok := d.artifacts[key]; ok {
		return data, nil
	}

	if d.dir != "" {
		path := filepath.Join(d.dir, ctx.CaseID, ctx.Step+".json")
		data, err := os.ReadFile(path)
		if err == nil {
			return data, nil
		}
		path = filepath.Join(d.dir, ctx.CaseID, strings.ToLower(ctx.Step)+".json")
		data, err = os.ReadFile(path)
		if err == nil {
			return data, nil
		}
	}

	return nil, fmt.Errorf("%w for %s/%s", ErrNoArtifact, ctx.CaseID, ctx.Step)
}

func staticKey(caseID, step string) string {
	return caseID + ":" + step
}
