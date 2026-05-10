package arcade

import (
	"github.com/dpopsuev/tako/agent/organ"
)

type PlayerView struct {
	game        *Game
	playerID    string
	instruments []string
}

func NewPlayerView(game *Game, playerID string, instruments []string) *PlayerView {
	return &PlayerView{game: game, playerID: playerID, instruments: instruments}
}

func (v *PlayerView) Organs() []organ.Func {
	all := v.game.Organs()
	var filtered []organ.Func
	for _, o := range all {
		if v.has(o.Name) {
			filtered = append(filtered, o)
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
