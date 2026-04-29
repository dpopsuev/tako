package minitako

import (
	"context"

	"github.com/dpopsuev/tako/discourse"
	"github.com/dpopsuev/tako/memory"
	"github.com/dpopsuev/tako/service/depo"
	"github.com/dpopsuev/tako/service/sleep"
)

type Minitako struct {
	cfg SessionConfig
}

type Builder struct {
	cfg SessionConfig
}

func NewBuilder() *Builder {
	return &Builder{
		cfg: SessionConfig{
			Player:    RandomPlayer{},
			Inspector: StubInspector{},
			MaxDays:   7,
		},
	}
}

func (b *Builder) WithShelf(s depo.Shelf) *Builder      { b.cfg.Shelf = s; return b }
func (b *Builder) WithDrain(d sleep.Drain) *Builder      { b.cfg.Drain = d; return b }
func (b *Builder) WithMesh(m memory.Mesh) *Builder       { b.cfg.Mesh = m; return b }
func (b *Builder) WithMonolog(m discourse.Monolog) *Builder { b.cfg.Monolog = m; return b }
func (b *Builder) WithPlayer(p Player) *Builder          { b.cfg.Player = p; return b }
func (b *Builder) WithInspector(i GameInspector) *Builder { b.cfg.Inspector = i; return b }
func (b *Builder) WithMaxDays(n int) *Builder            { b.cfg.MaxDays = n; return b }

func (b *Builder) Build() *Minitako {
	return &Minitako{cfg: b.cfg}
}

func (m *Minitako) Run(ctx context.Context) RunResult {
	if m.cfg.Shelf != nil {
		return RunSession(ctx, m.cfg)
	}
	return RunGame(m.cfg.Player, m.cfg.Inspector)
}
