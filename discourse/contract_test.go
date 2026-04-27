package discourse

import (
	"testing"
	"time"
)

func TestStubMonologue(t *testing.T) {
	m := &StubMonologue{}
	m.Pin("work")
	m.Focus("planning")
	m.Write(Letter{From: "self", To: "self", Subject: "thinking", Body: "what next?", CreatedAt: time.Now()})

	letters := m.Letters()
	if len(letters) != 1 {
		t.Errorf("expected 1 letter, got %d", len(letters))
	}
	if letters[0].Subject != "thinking" {
		t.Errorf("expected subject 'thinking', got %q", letters[0].Subject)
	}
}

func TestStubDialog(t *testing.T) {
	d := &StubDialog{}

	_, ok := d.Receive()
	if ok {
		t.Error("expected no message in empty inbox")
	}

	_ = d.Send(Letter{From: "a", To: "b", Subject: "hello", CreatedAt: time.Now()})

	d.mu.Lock()
	d.inbox = append(d.inbox, Letter{From: "b", To: "a", Subject: "reply", CreatedAt: time.Now()})
	d.mu.Unlock()

	letter, ok := d.Receive()
	if !ok {
		t.Fatal("expected message in inbox")
	}
	if letter.Subject != "reply" {
		t.Errorf("expected subject 'reply', got %q", letter.Subject)
	}
}
