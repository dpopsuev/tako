package circuit

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"strings"
)

// MaskMode controls how sensitive values are displayed.
type MaskMode int

const (
	// MaskFull replaces the entire value with the redacted string.
	MaskFull MaskMode = iota
	// MaskPartial shows first 4 + last 4 characters (e.g., "sk-l...7z9").
	// Falls back to MaskFull if the value is shorter than 8 characters.
	MaskPartial
	// MaskHash shows a deterministic SHA-256 prefix (e.g., "sha256:a3f2c8d91e04").
	// Same input always produces the same output for correlation.
	MaskHash
)

// SafeHandlerOption configures a SafeHandler.
type SafeHandlerOption func(*safeHandlerConfig)

type safeHandlerConfig struct {
	maxStringLen  int
	keyLimits     map[string]int
	sensitiveKeys map[string]bool
	redactedValue string
	truncSuffix   string
	maskMode      MaskMode
}

func defaultSafeConfig() *safeHandlerConfig {
	return &safeHandlerConfig{
		maxStringLen: 1024,
		keyLimits:    make(map[string]int),
		sensitiveKeys: map[string]bool{
			"token":      true,
			"secret":     true,
			"password":   true,
			"credential": true,
			"api_key":    true,
			"bearer":     true,
		},
		redactedValue: "[REDACTED]",
		truncSuffix:   "...[truncated]",
		maskMode:      MaskFull,
	}
}

// WithMaxStringLen sets the maximum string value length before truncation.
// Default: 1024. Set 0 to disable truncation.
func WithMaxStringLen(n int) SafeHandlerOption {
	return func(c *safeHandlerConfig) { c.maxStringLen = n }
}

// WithKeyLimit sets a per-key max string length, overriding the global default.
func WithKeyLimit(key string, maxLen int) SafeHandlerOption {
	return func(c *safeHandlerConfig) { c.keyLimits[key] = maxLen }
}

// WithSensitiveKeys adds keys whose values should be redacted.
// These are matched case-insensitively by exact key name.
func WithSensitiveKeys(keys ...string) SafeHandlerOption {
	return func(c *safeHandlerConfig) {
		for _, k := range keys {
			c.sensitiveKeys[strings.ToLower(k)] = true
		}
	}
}

// WithRedactedValue sets the replacement string for redacted values.
// Default: "[REDACTED]".
func WithRedactedValue(v string) SafeHandlerOption {
	return func(c *safeHandlerConfig) { c.redactedValue = v }
}

// WithMaskMode sets how sensitive values are displayed.
// Default: MaskFull.
func WithMaskMode(m MaskMode) SafeHandlerOption {
	return func(c *safeHandlerConfig) { c.maskMode = m }
}

// SafeHandler wraps an slog.Handler with truncation and redaction.
type SafeHandler struct {
	inner slog.Handler
	cfg   *safeHandlerConfig
}

// NewSafeHandler wraps an existing slog.Handler with truncation and redaction.
func NewSafeHandler(inner slog.Handler, opts ...SafeHandlerOption) *SafeHandler {
	cfg := defaultSafeConfig()
	for _, opt := range opts {
		opt(cfg)
	}
	return &SafeHandler{inner: inner, cfg: cfg}
}

// Enabled delegates to the inner handler.
func (h *SafeHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

// Handle processes the record's attributes for truncation/redaction,
// then delegates to the inner handler.
//
//nolint:gocritic // slog.Handler interface requires value receiver for Record
func (h *SafeHandler) Handle(ctx context.Context, r slog.Record) error {
	safe := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		safe.AddAttrs(h.processAttr(a))
		return true
	})
	return h.inner.Handle(ctx, safe)
}

// WithAttrs processes the attrs for truncation/redaction, then passes
// them to the inner handler.
func (h *SafeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	processed := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		processed[i] = h.processAttr(a)
	}
	return &SafeHandler{
		inner: h.inner.WithAttrs(processed),
		cfg:   h.cfg,
	}
}

// WithGroup delegates to the inner handler.
func (h *SafeHandler) WithGroup(name string) slog.Handler {
	return &SafeHandler{
		inner: h.inner.WithGroup(name),
		cfg:   h.cfg,
	}
}

func (h *SafeHandler) processAttr(a slog.Attr) slog.Attr {
	// Redact sensitive keys.
	if h.cfg.sensitiveKeys[strings.ToLower(a.Key)] {
		return slog.String(a.Key, h.mask(a.Value.String()))
	}

	// Recurse into groups.
	if a.Value.Kind() == slog.KindGroup {
		attrs := a.Value.Group()
		processed := make([]slog.Attr, len(attrs))
		for i, ga := range attrs {
			processed[i] = h.processAttr(ga)
		}
		return slog.Group(a.Key, attrsToAny(processed)...)
	}

	// Truncate long strings (per-key limit overrides global).
	if a.Value.Kind() == slog.KindString {
		limit := h.cfg.maxStringLen
		if kl, ok := h.cfg.keyLimits[a.Key]; ok {
			limit = kl
		}
		if limit > 0 {
			s := a.Value.String()
			if len(s) > limit {
				return slog.String(a.Key, s[:limit]+h.cfg.truncSuffix)
			}
		}
	}

	return a
}

func (h *SafeHandler) mask(value string) string {
	switch h.cfg.maskMode {
	case MaskPartial:
		if len(value) >= 8 {
			return value[:4] + "..." + value[len(value)-4:]
		}
		return h.cfg.redactedValue
	case MaskHash:
		hash := sha256.Sum256([]byte(value))
		return fmt.Sprintf("sha256:%x", hash[:6])
	default:
		return h.cfg.redactedValue
	}
}

func attrsToAny(attrs []slog.Attr) []any {
	result := make([]any, len(attrs))
	for i, a := range attrs {
		result[i] = a
	}
	return result
}
