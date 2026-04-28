package instrument

import (
	"context"
	"errors"
	"testing"
)

func TestStubCompleterReturnsResponse(t *testing.T) {
	c := &StubCompleter{Response: []byte("hello")}
	resp, err := c.Complete(context.Background(), []byte("prompt"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(resp) != "hello" {
		t.Errorf("expected 'hello', got %q", resp)
	}
}

func TestStubCompleterReturnsError(t *testing.T) {
	c := &StubCompleter{Err: errors.New("fail")}
	_, err := c.Complete(context.Background(), []byte("prompt"))
	if err == nil {
		t.Fatal("expected error")
	}
}
