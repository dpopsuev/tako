package cerebrum

import (
	"encoding/json"
	"testing"
)

func TestDebouncer_AllowsFirstCall(t *testing.T) {
	d := NewDebouncer(3, 2)
	if d.Check("ping", json.RawMessage(`{}`)) {
		t.Error("first call should not be blocked")
	}
}

func TestDebouncer_AllowsDifferentCalls(t *testing.T) {
	d := NewDebouncer(3, 2)
	d.Check("ping", json.RawMessage(`{}`))
	d.Check("pong", json.RawMessage(`{}`))
	d.Check("ping", json.RawMessage(`{"x":1}`))
	if d.Check("other", json.RawMessage(`{}`)) {
		t.Error("different calls should not be blocked")
	}
}

func TestDebouncer_BlocksRepeatedCalls(t *testing.T) {
	d := NewDebouncer(3, 2)
	d.Check("ping", json.RawMessage(`{}`))
	d.Check("ping", json.RawMessage(`{}`))
	if !d.Check("ping", json.RawMessage(`{}`)) {
		t.Error("third identical call should be blocked (threshold=2)")
	}
}

func TestDebouncer_WindowSlides(t *testing.T) {
	d := NewDebouncer(3, 2)
	d.Check("ping", json.RawMessage(`{}`))
	d.Check("ping", json.RawMessage(`{}`))
	d.Check("other", json.RawMessage(`{}`))
	d.Check("other2", json.RawMessage(`{}`))
	if d.Check("ping", json.RawMessage(`{}`)) {
		t.Error("old calls should slide out of window")
	}
}

func TestDebouncer_DifferentArgsDifferentFingerprint(t *testing.T) {
	d := NewDebouncer(3, 2)
	d.Check("file.read", json.RawMessage(`{"path":"a.go"}`))
	d.Check("file.read", json.RawMessage(`{"path":"b.go"}`))
	if d.Check("file.read", json.RawMessage(`{"path":"c.go"}`)) {
		t.Error("same tool with different args should not be blocked")
	}
}

func TestDebouncer_Reset(t *testing.T) {
	d := NewDebouncer(3, 2)
	d.Check("ping", json.RawMessage(`{}`))
	d.Check("ping", json.RawMessage(`{}`))
	d.Reset()
	if d.Check("ping", json.RawMessage(`{}`)) {
		t.Error("after reset, calls should not be blocked")
	}
}
