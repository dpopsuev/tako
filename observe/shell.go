package observe

import (
	"context"
	"encoding/json"

	"github.com/dpopsuev/tako/agent/organ"
	"github.com/dpopsuev/tako/ergograph"
	"go.opentelemetry.io/otel/trace"
)

// WrapCapability adds observability (tracing + ergograph recording) to a Capability's Execute.
func WrapCapability(cap organ.Func, pool ergograph.Ledger, tracer trace.Tracer) organ.Func {
	if cap.Execute == nil {
		return cap
	}
	inner := cap.Execute
	cap.Execute = func(ctx context.Context, input json.RawMessage) (organ.Result, error) {
		ctx, span := tracer.Start(ctx, "organ.exec")
		defer span.End()
		result, err := inner(ctx, input)
		spanError(span, err)
		record(ctx, pool, "organ.exec", map[string]string{"capability": cap.Name})
		return result, err
	}
	return cap
}
