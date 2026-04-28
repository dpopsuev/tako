package observe

import (
	"context"

	"github.com/dpopsuev/tako/ergograph"
	"github.com/dpopsuev/tako/render"
	"go.opentelemetry.io/otel/trace"
)

type Canvas struct {
	inner  render.Canvas
	pool   ergograph.Pool
	tracer trace.Tracer
	name   string
}

var _ render.Canvas = (*Canvas)(nil)

func NewCanvas(inner render.Canvas, pool ergograph.Pool, tracer trace.Tracer, name string) *Canvas {
	return &Canvas{inner: inner, pool: pool, tracer: tracer, name: name}
}

func (c *Canvas) Post(panel render.Panel) {
	ctx, span := c.tracer.Start(context.Background(), "canvas.post")
	defer span.End()
	c.inner.Post(panel)
	record(ctx, c.pool, "canvas.post", map[string]string{"canvas": c.name, "panel": panel.ID})
}

func (c *Canvas) Retract(id string) {
	ctx, span := c.tracer.Start(context.Background(), "canvas.retract")
	defer span.End()
	c.inner.Retract(id)
	record(ctx, c.pool, "canvas.retract", map[string]string{"canvas": c.name, "panel": id})
}

func (c *Canvas) Panels() []render.Panel {
	_, span := c.tracer.Start(context.Background(), "canvas.panels")
	defer span.End()
	return c.inner.Panels()
}

func (c *Canvas) Subscribe() <-chan render.Event {
	_, span := c.tracer.Start(context.Background(), "canvas.subscribe")
	defer span.End()
	return c.inner.Subscribe()
}
