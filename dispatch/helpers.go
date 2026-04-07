package dispatch

import (
	"io"
	"log/slog"
)

// DiscardLogger returns a logger that discards all output.
func DiscardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// UnwrapFinalizer walks the dispatcher chain and returns the first Finalizer.
func UnwrapFinalizer(d Dispatcher) Finalizer {
	for d != nil {
		if f, ok := d.(Finalizer); ok {
			return f
		}
		if u, ok := d.(Unwrapper); ok {
			d = u.Inner()
			continue
		}
		return nil
	}
	return nil
}
