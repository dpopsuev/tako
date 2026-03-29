package circuit_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/dpopsuev/origami/circuit"
)

func newTestHandler(buf *bytes.Buffer, opts ...circuit.SafeHandlerOption) *circuit.SafeHandler {
	inner := slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	return circuit.NewSafeHandler(inner, opts...)
}

func parseLogLine(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("parse log JSON: %v\nraw: %s", err, buf.String())
	}
	return m
}

func TestSafeHandler_Truncation(t *testing.T) {
	var buf bytes.Buffer
	h := newTestHandler(&buf, circuit.WithMaxStringLen(10))
	logger := slog.New(h)

	logger.InfoContext(context.Background(), "test", slog.String("body", "short"))
	m := parseLogLine(t, &buf)
	if m["body"] != "short" {
		t.Errorf("expected short string passthrough, got %q", m["body"])
	}

	buf.Reset()
	logger.InfoContext(context.Background(), "test", slog.String("body", "this is a very long string that exceeds the limit"))
	m = parseLogLine(t, &buf)
	body := m["body"].(string)
	if body != "this is a ...[truncated]" {
		t.Errorf("unexpected truncation: %q", body)
	}
}

func TestSafeHandler_Redaction(t *testing.T) {
	var buf bytes.Buffer
	h := newTestHandler(&buf)
	logger := slog.New(h)

	logger.InfoContext(context.Background(), "test",
		slog.String("token", "abc123"),
		slog.String("password", "secret"),
		slog.String("name", "visible"),
	)
	m := parseLogLine(t, &buf)

	if m["token"] != "[REDACTED]" {
		t.Errorf("token not redacted: %v", m["token"])
	}
	if m["password"] != "[REDACTED]" {
		t.Errorf("password not redacted: %v", m["password"])
	}
	if m["name"] != "visible" {
		t.Errorf("name should not be redacted: %v", m["name"])
	}
}

func TestSafeHandler_CustomSensitiveKeys(t *testing.T) {
	var buf bytes.Buffer
	h := newTestHandler(&buf, circuit.WithSensitiveKeys("ssn", "dob"))
	logger := slog.New(h)

	logger.InfoContext(context.Background(), "test",
		slog.String("ssn", "123-45-6789"),
		slog.String("token", "also-secret"),
		slog.String("name", "visible"),
	)
	m := parseLogLine(t, &buf)

	if m["ssn"] != "[REDACTED]" {
		t.Errorf("ssn not redacted: %v", m["ssn"])
	}
	if m["token"] != "[REDACTED]" {
		t.Errorf("token not redacted: %v", m["token"])
	}
}

func TestSafeHandler_CustomRedactedValue(t *testing.T) {
	var buf bytes.Buffer
	h := newTestHandler(&buf, circuit.WithRedactedValue("***"))
	logger := slog.New(h)

	logger.InfoContext(context.Background(), "test", slog.String("secret", "mysecret"))
	m := parseLogLine(t, &buf)
	if m["secret"] != "***" {
		t.Errorf("expected custom redacted value, got %v", m["secret"])
	}
}

func TestSafeHandler_TruncationDisabled(t *testing.T) {
	var buf bytes.Buffer
	h := newTestHandler(&buf, circuit.WithMaxStringLen(0))
	logger := slog.New(h)

	long := string(make([]byte, 5000))
	logger.InfoContext(context.Background(), "test", slog.String("body", long))
	m := parseLogLine(t, &buf)
	if len(m["body"].(string)) != 5000 {
		t.Errorf("expected no truncation when disabled, got len=%d", len(m["body"].(string)))
	}
}

func TestSafeHandler_Passthrough(t *testing.T) {
	var buf bytes.Buffer
	h := newTestHandler(&buf)
	logger := slog.New(h)

	logger.InfoContext(context.Background(), "test",
		slog.Int("count", 42),
		slog.Bool("ok", true),
		slog.String("msg", "hello"),
	)
	m := parseLogLine(t, &buf)
	if m["count"].(float64) != 42 {
		t.Errorf("int passthrough failed: %v", m["count"])
	}
	if m["ok"].(bool) != true {
		t.Errorf("bool passthrough failed: %v", m["ok"])
	}
	if m["msg"] != "hello" {
		t.Errorf("string passthrough failed: %v", m["msg"])
	}
}

func TestSafeHandler_GroupRedaction(t *testing.T) {
	var buf bytes.Buffer
	h := newTestHandler(&buf)
	logger := slog.New(h)

	logger.InfoContext(context.Background(), "test",
		slog.Group("auth",
			slog.String("token", "secret-token"),
			slog.String("user", "alice"),
		),
	)
	m := parseLogLine(t, &buf)
	auth := m["auth"].(map[string]any)
	if auth["token"] != "[REDACTED]" {
		t.Errorf("nested token not redacted: %v", auth["token"])
	}
	if auth["user"] != "alice" {
		t.Errorf("nested user should not be redacted: %v", auth["user"])
	}
}

func TestSafeHandler_CaseInsensitiveKeys(t *testing.T) {
	var buf bytes.Buffer
	h := newTestHandler(&buf, circuit.WithSensitiveKeys("API_KEY"))
	logger := slog.New(h)

	logger.InfoContext(context.Background(), "test",
		slog.String("api_key", "key123"),
	)
	m := parseLogLine(t, &buf)
	if m["api_key"] != "[REDACTED]" {
		t.Errorf("api_key not redacted: %v", m["api_key"])
	}
}

// --- Mask modes ---

func TestSafeHandler_MaskPartial(t *testing.T) {
	var buf bytes.Buffer
	h := newTestHandler(&buf, circuit.WithMaskMode(circuit.MaskPartial))
	logger := slog.New(h)

	logger.InfoContext(context.Background(), "test",
		slog.String("token", "sk-live-abc123xyz789"),
	)
	m := parseLogLine(t, &buf)
	val := m["token"].(string)
	// Should show first 4 + ... + last 4
	if val != "sk-l...z789" {
		t.Errorf("expected partial mask, got %q", val)
	}
}

func TestSafeHandler_MaskPartialShortValue(t *testing.T) {
	var buf bytes.Buffer
	h := newTestHandler(&buf, circuit.WithMaskMode(circuit.MaskPartial))
	logger := slog.New(h)

	logger.InfoContext(context.Background(), "test",
		slog.String("token", "short"),
	)
	m := parseLogLine(t, &buf)
	// Shorter than 8 chars — falls back to full redaction
	if m["token"] != "[REDACTED]" {
		t.Errorf("short value should fall back to full redaction, got %q", m["token"])
	}
}

func TestSafeHandler_MaskHash(t *testing.T) {
	var buf bytes.Buffer
	h := newTestHandler(&buf, circuit.WithMaskMode(circuit.MaskHash))
	logger := slog.New(h)

	logger.InfoContext(context.Background(), "test",
		slog.String("token", "sk-live-abc123xyz789"),
	)
	m := parseLogLine(t, &buf)
	val := m["token"].(string)
	if !strings.HasPrefix(val, "sha256:") {
		t.Errorf("expected sha256: prefix, got %q", val)
	}
	// 6 bytes = 12 hex chars
	hash := strings.TrimPrefix(val, "sha256:")
	if len(hash) != 12 {
		t.Errorf("expected 12 hex chars, got %d: %q", len(hash), hash)
	}

	// Deterministic — same input produces same output
	buf.Reset()
	logger.InfoContext(context.Background(), "test",
		slog.String("token", "sk-live-abc123xyz789"),
	)
	m2 := parseLogLine(t, &buf)
	if m2["token"] != val {
		t.Errorf("hash should be deterministic: %q != %q", m2["token"], val)
	}
}

// --- Per-key truncation limits ---

func TestSafeHandler_PerKeyLimit(t *testing.T) {
	var buf bytes.Buffer
	h := newTestHandler(&buf,
		circuit.WithMaxStringLen(20),
		circuit.WithKeyLimit("body", 5),
		circuit.WithKeyLimit("error", 50),
	)
	logger := slog.New(h)

	logger.InfoContext(context.Background(), "test",
		slog.String("body", "this is the body content that is long"),
		slog.String("error", "this is a short error"),
		slog.String("other", "this string uses the global limit of twenty chars"),
	)
	m := parseLogLine(t, &buf)

	body := m["body"].(string)
	if body != "this ...[truncated]" {
		t.Errorf("body should be truncated at 5: %q", body)
	}

	errVal := m["error"].(string)
	if errVal != "this is a short error" {
		t.Errorf("error should not be truncated (under 50): %q", errVal)
	}

	other := m["other"].(string)
	if other != "this string uses the...[truncated]" {
		t.Errorf("other should use global limit of 20: %q", other)
	}
}

func TestSafeHandler_PerKeyLimitZeroDisables(t *testing.T) {
	var buf bytes.Buffer
	h := newTestHandler(&buf,
		circuit.WithMaxStringLen(10),
		circuit.WithKeyLimit("body", 0),
	)
	logger := slog.New(h)

	long := strings.Repeat("x", 500)
	logger.InfoContext(context.Background(), "test",
		slog.String("body", long),
	)
	m := parseLogLine(t, &buf)
	if len(m["body"].(string)) != 500 {
		t.Errorf("per-key limit 0 should disable truncation, got len=%d", len(m["body"].(string)))
	}
}
