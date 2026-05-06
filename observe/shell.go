package observe

import (
	"context"
	"encoding/json"

	"github.com/dpopsuev/tako/agent/capability"
	"github.com/dpopsuev/tako/ergograph"
	"go.opentelemetry.io/otel/trace"
)

// WrapCapability adds observability (tracing + ergograph recording) to a Capability's Execute.
func WrapCapability(cap capability.Capability, pool ergograph.Ledger, tracer trace.Tracer) capability.Capability {
	if cap.Execute == nil {
		return cap
	}
	inner := cap.Execute
	cap.Execute = func(ctx context.Context, input json.RawMessage) (capability.Result, error) {
		ctx, span := tracer.Start(ctx, "capability.exec")
		defer span.End()
		result, err := inner(ctx, input)
		spanError(span, err)
		record(ctx, pool, "capability.exec", map[string]string{"capability": cap.Name})
		return result, err
	}
	return cap
}
