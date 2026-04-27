package ergograph

import (
	"testing"
	"time"
)

func TestStubPoolAppendAndVerify(t *testing.T) {
	pool := &StubPool{}
	err := pool.Append(Record{
		Identity:  "agent-1",
		Action:    "exec",
		Timestamp: time.Now(),
		Payload:   []byte("result"),
	})
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}
	if pool.Len() != 1 {
		t.Errorf("expected 1 record, got %d", pool.Len())
	}
	if err := pool.VerifyChain(); err != nil {
		t.Errorf("chain verification failed: %v", err)
	}
}

func TestStubPoolHashChain(t *testing.T) {
	pool := &StubPool{}
	for i := range 5 {
		_ = pool.Append(Record{
			Identity:  "agent-1",
			Action:    "exec",
			Timestamp: time.Now(),
			Labels:    map[string]string{"i": string(rune('0' + i))},
			Payload:   []byte("data"),
		})
	}
	if pool.Len() != 5 {
		t.Fatalf("expected 5 records, got %d", pool.Len())
	}
	if err := pool.VerifyChain(); err != nil {
		t.Errorf("chain verification failed after 5 appends: %v", err)
	}

	records := pool.Records()
	for i := 1; i < len(records); i++ {
		if records[i].PrevHash != records[i-1].Hash {
			t.Errorf("record %d PrevHash doesn't match record %d Hash", i, i-1)
		}
	}
}

func TestStubInspectorScore(t *testing.T) {
	pool := &StubPool{}
	_ = pool.Append(Record{Identity: "a", Action: "x", Timestamp: time.Now()})
	inspector := StubInspector{}
	oae, err := inspector.Score(pool)
	if err != nil {
		t.Fatalf("Score failed: %v", err)
	}
	if oae.Score() != 1.0 {
		t.Errorf("expected OAE 1.0, got %f", oae.Score())
	}
}

func TestStubInspectorVerify(t *testing.T) {
	pool := &StubPool{}
	_ = pool.Append(Record{Identity: "a", Action: "x", Timestamp: time.Now()})
	inspector := StubInspector{}
	if err := inspector.Verify(pool); err != nil {
		t.Errorf("Verify failed: %v", err)
	}
}
