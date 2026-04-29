package cerebrum

import (
	"github.com/dpopsuev/tako/discourse"
	"github.com/dpopsuev/tako/instrument"
	"github.com/dpopsuev/tako/memory"
)

type Option func(*Cerebrum)

func WithShell(s instrument.Shell) Option {
	return func(cb *Cerebrum) { cb.shell = s }
}

func WithMesh(m memory.Mesh) Option {
	return func(cb *Cerebrum) { cb.mesh = m }
}

func WithMonolog(m discourse.Monolog) Option {
	return func(cb *Cerebrum) { cb.monolog = m }
}

func WithMaxTurns(n int) Option {
	return func(cb *Cerebrum) { cb.maxTurns = n }
}
