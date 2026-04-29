package adapter

import (
	"fmt"

	"github.com/dpopsuev/tako/render"
	"github.com/dpopsuev/tako/testkit/minitako"
)

type CanvasRenderer struct {
	canvas render.Canvas
}

func NewCanvasRenderer(canvas render.Canvas) *CanvasRenderer {
	return &CanvasRenderer{canvas: canvas}
}

func (r *CanvasRenderer) Render(state minitako.GameState) {
	face := petFace(state)
	stats := formatStats(state)

	r.canvas.Post(render.Panel{
		ID:     "minitako-pet",
		Source: "minitako",
		Data:   []byte(face),
	})

	r.canvas.Post(render.Panel{
		ID:       "minitako-stats",
		Source:   "minitako",
		Priority: 1,
		Data:     []byte(stats),
	})
}

func petFace(state minitako.GameState) string {
	if !state.Alive {
		return "(X_X)"
	}
	if state.Pet.Health < 20 {
		return "(x_x)"
	}
	if state.Pet.Hunger < 20 {
		return "(>_<)"
	}
	if state.Pet.Energy < 20 {
		return "(-_-)zzz"
	}
	if state.Pet.Hygiene < 30 {
		return "(~_~)"
	}
	if state.Pet.Happiness < 30 {
		return "(;_;)"
	}
	return "(^_^)"
}

func formatStats(state minitako.GameState) string {
	return fmt.Sprintf(
		"Day %d %s | %s | Coins: %d\nHunger: %s %d\nEnergy: %s %d\nHappy:  %s %d\nHealth: %s %d\nHygiene:%s %d",
		state.Day, state.Stage, fmtTOD(state.Hour), state.Wallet,
		bar(state.Pet.Hunger), state.Pet.Hunger,
		bar(state.Pet.Energy), state.Pet.Energy,
		bar(state.Pet.Happiness), state.Pet.Happiness,
		bar(state.Pet.Health), state.Pet.Health,
		bar(state.Pet.Hygiene), state.Pet.Hygiene,
	)
}

func bar(val int) string {
	filled := val / 10
	empty := 10 - filled
	b := "["
	for i := 0; i < filled; i++ {
		b += "#"
	}
	for i := 0; i < empty; i++ {
		b += "."
	}
	return b + "]"
}

func fmtTOD(hour int) string {
	return fmt.Sprintf("%02d:00", hour)
}
