package observe

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dpopsuev/tako/ergograph"
)

type failLedger struct{}

var errLedgerFail = errors.New("pool failure")

func (*failLedger) Append(_ ergograph.Record) error { return errLedgerFail }
func (*failLedger) Records() []ergograph.Record     { return nil }
func (*failLedger) VerifyChain() error              { return nil }
func (*failLedger) Len() int                        { return 0 }

func TestRecordAppendsToLedger(t *testing.T) {
	pool := &ergograph.StubLedger{}
	record(context.Background(), pool, "test.action", map[string]string{"key": "val"})

	if pool.Len() != 1 {
		t.Fatalf("expected 1 record, got %d", pool.Len())
	}
	recs := pool.Records()
	if recs[0].Action != "test.action" {
		t.Errorf("expected action 'test.action', got %q", recs[0].Action)
	}
	if recs[0].Labels["key"] != "val" {
		t.Errorf("expected label key=val, got %q", recs[0].Labels["key"])
	}
	if recs[0].Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

func TestRecordTimestampIsRecent(t *testing.T) {
	pool := &ergograph.StubLedger{}
	before := time.Now()
	record(context.Background(), pool, "test.timing", nil)
	after := time.Now()

	rec := pool.Records()[0]
	if rec.Timestamp.Before(before) || rec.Timestamp.After(after) {
		t.Errorf("timestamp %v not between %v and %v", rec.Timestamp, before, after)
	}
}

func TestRecordLedgerErrorDoesNotPanic(t *testing.T) {
	pool := &failLedger{}
	record(context.Background(), pool, "test.fail", nil)
}
