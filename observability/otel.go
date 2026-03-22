package observability

import (
	"context"
	"sync"

	"github.com/dpopsuev/origami/circuit"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// OTelObserver translates WalkEvents into OpenTelemetry spans.
// Walk start creates a root span; node entries create child spans.
type OTelObserver struct {
	tracer trace.Tracer

	mu        sync.Mutex
	walkCtx   context.Context
	walkSpan  trace.Span
	nodeSpans map[string]trace.Span
}

// NewOTelObserver creates an observer backed by the given tracer.
func NewOTelObserver(tracer trace.Tracer) *OTelObserver {
	return &OTelObserver{
		tracer:    tracer,
		nodeSpans: make(map[string]trace.Span),
	}
}

func (o *OTelObserver) OnEvent(e circuit.WalkEvent) {
	switch e.Type {
	case circuit.EventNodeEnter:
		o.onNodeEnter(e)
	case circuit.EventNodeExit:
		o.onNodeExit(e)
	case circuit.EventTransition:
		o.onTransition(e)
	case circuit.EventWalkComplete:
		o.onWalkComplete(e)
	case circuit.EventWalkError:
		o.onWalkError(e)
	}
}

func (o *OTelObserver) onNodeEnter(e circuit.WalkEvent) {
	o.mu.Lock()
	defer o.mu.Unlock()

	parentCtx := o.walkCtx
	if parentCtx == nil {
		parentCtx = context.Background()
	}

	attrs := []attribute.KeyValue{
		attribute.String("node", e.Node),
	}
	if e.Walker != "" {
		attrs = append(attrs, attribute.String("walker", e.Walker))
	}

	_, span := o.tracer.Start(parentCtx, "node.visit", trace.WithAttributes(attrs...))
	o.nodeSpans[e.Node] = span
}

func (o *OTelObserver) onNodeExit(e circuit.WalkEvent) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if span, ok := o.nodeSpans[e.Node]; ok {
		span.SetAttributes(attribute.Int64("duration_ms", e.Elapsed.Milliseconds()))
		span.End()
		delete(o.nodeSpans, e.Node)
	}
}

func (o *OTelObserver) onTransition(e circuit.WalkEvent) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.walkSpan != nil {
		o.walkSpan.AddEvent("edge.transition", trace.WithAttributes(
			attribute.String("edge", e.Edge),
			attribute.String("node", e.Node),
		))
	}
}

func (o *OTelObserver) onWalkComplete(_ circuit.WalkEvent) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.walkSpan != nil {
		o.walkSpan.SetStatus(codes.Ok, "walk completed")
		o.walkSpan.End()
		o.walkSpan = nil
		o.walkCtx = nil
	}
}

func (o *OTelObserver) onWalkError(e circuit.WalkEvent) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.walkSpan != nil {
		o.walkSpan.SetStatus(codes.Error, "walk failed")
		if e.Error != nil {
			o.walkSpan.RecordError(e.Error)
		}
		o.walkSpan.End()
		o.walkSpan = nil
		o.walkCtx = nil
	}
}

// StartWalk initializes the root span for a circuit walk.
func (o *OTelObserver) StartWalk(circuit string, attrs ...attribute.KeyValue) {
	o.mu.Lock()
	defer o.mu.Unlock()

	all := append([]attribute.KeyValue{attribute.String("circuit", circuit)}, attrs...)
	ctx, span := o.tracer.Start(context.Background(), "circuit.walk", trace.WithAttributes(all...))
	o.walkCtx = ctx
	o.walkSpan = span
}
