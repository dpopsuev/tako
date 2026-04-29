package observe

import (
	"context"
	"sync"

	"github.com/dpopsuev/tako/service/andon"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type OTelObserver struct {
	tracer trace.Tracer

	mu        sync.Mutex
	walkCtx   context.Context
	walkSpan  trace.Span
	nodeSpans map[string]trace.Span
}

var _ andon.Observer = (*OTelObserver)(nil)

func NewOTelObserver(tracer trace.Tracer) *OTelObserver {
	return &OTelObserver{
		tracer:    tracer,
		nodeSpans: make(map[string]trace.Span),
	}
}

func (o *OTelObserver) OnEvent(e *andon.Event) {
	switch e.Type {
	case andon.NodeEnter:
		o.onNodeEnter(e)
	case andon.NodeExit:
		o.onNodeExit(e)
	case andon.Transition:
		o.onTransition(e)
	case andon.WalkComplete:
		o.onWalkComplete()
	case andon.WalkError:
		o.onWalkError(e)
	}
}

func (o *OTelObserver) onNodeEnter(e *andon.Event) {
	o.mu.Lock()
	defer o.mu.Unlock()

	parentCtx := o.walkCtx
	if parentCtx == nil {
		parentCtx = context.Background()
	}

	attrs := []attribute.KeyValue{
		attribute.String("node", e.Node),
	}
	if e.Agent != "" {
		attrs = append(attrs, attribute.String("agent", e.Agent))
	}

	_, span := o.tracer.Start(parentCtx, "station.visit", trace.WithAttributes(attrs...))
	o.nodeSpans[e.Node] = span
}

func (o *OTelObserver) onNodeExit(e *andon.Event) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if span, ok := o.nodeSpans[e.Node]; ok {
		span.SetAttributes(attribute.Int64("duration_ms", e.Elapsed.Milliseconds()))
		span.End()
		delete(o.nodeSpans, e.Node)
	}
}

func (o *OTelObserver) onTransition(e *andon.Event) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.walkSpan != nil {
		o.walkSpan.AddEvent("contract.transition", trace.WithAttributes(
			attribute.String("edge", e.Edge),
			attribute.String("node", e.Node),
		))
	}
}

func (o *OTelObserver) onWalkComplete() {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.walkSpan != nil {
		o.walkSpan.SetStatus(codes.Ok, "fab completed")
		o.walkSpan.End()
		o.walkSpan = nil
		o.walkCtx = nil
	}
}

func (o *OTelObserver) onWalkError(e *andon.Event) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.walkSpan != nil {
		o.walkSpan.SetStatus(codes.Error, "fab failed")
		if e.Error != nil {
			o.walkSpan.RecordError(e.Error)
		}
		o.walkSpan.End()
		o.walkSpan = nil
		o.walkCtx = nil
	}
}

func (o *OTelObserver) StartFab(fabName string, attrs ...attribute.KeyValue) {
	o.mu.Lock()
	defer o.mu.Unlock()

	all := append([]attribute.KeyValue{attribute.String("fab", fabName)}, attrs...)
	ctx, span := o.tracer.Start(context.Background(), "fab.walk", trace.WithAttributes(all...))
	o.walkCtx = ctx
	o.walkSpan = span
}
