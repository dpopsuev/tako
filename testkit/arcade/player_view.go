package arcade

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dpopsuev/tako/agent/organ"
)

// PlayerView restricts a Game's Shell to a subset of instruments.
// Each agent in a multi-agent game gets its own PlayerView of the shared Game.
type PlayerView struct {
	game        *Game
	playerID    string
	instruments []string
}


func NewPlayerView(game *Game, playerID string, instruments []string) *PlayerView {
	return &PlayerView{game: game, playerID: playerID, instruments: instruments}
}

func (v *PlayerView) Names() []string {
	return append([]string(nil), v.instruments...)
}

func (v *PlayerView) Describe(name string) (string, error) {
	if !v.has(name) {
		return "", fmt.Errorf("instrument %s not available to player %s", name, v.playerID)
	}
	return v.game.Describe(name)
}

func (v *PlayerView) Schema(name string) (json.RawMessage, error) {
	if !v.has(name) {
		return nil, fmt.Errorf("instrument %s not available to player %s", name, v.playerID)
	}
	return v.game.Schema(name)
}

func (v *PlayerView) Mode(name string) organ.ActionMode {
	return v.game.Mode(name)
}

func (v *PlayerView) Approval(name string) organ.ActionApproval {
	return v.game.Approval(name)
}

func (v *PlayerView) Risk(name string) float64 {
	return v.game.Risk(name)
}

func (v *PlayerView) Exec(ctx context.Context, name string, input json.RawMessage) (organ.Result, error) {
	if !v.has(name) {
		return organ.ErrorResult(fmt.Sprintf("instrument %s not available to player %s", name, v.playerID)), nil
	}
	return v.game.Exec(ctx, name, input)
}

// Organs returns the filtered subset.
func (v *PlayerView) Organs() []organ.Func {
	all := v.game.Organs()
	var filtered []organ.Func
	for _, cap := range all {
		if v.has(cap.Name) {
			filtered = append(filtered, cap)
		}
	}
	return filtered
}

func (v *PlayerView) has(name string) bool {
	for _, n := range v.instruments {
		if n == name {
			return true
		}
	}
	return false
}
