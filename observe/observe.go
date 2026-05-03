package observe

import (
	"context"
	"log/slog"
	"time"

	"github.com/dpopsuev/tako/ergograph"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	logKeyAction = "action"
	logKeyError  = "error"
)

func record(ctx context.Context, pool ergograph.Ledger, action string, labels map[string]string) {
	rec := ergograph.Record{
		Action:    action,
		Timestamp: time.Now(),
		Labels:    labels,
	}
	if err := pool.Append(rec); err != nil {
		slog.WarnContext(ctx, "observe: record append failed",
			slog.String(logKeyAction, action),
			slog.Any(logKeyError, err),
		)
	}
}

func spanError(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
}
