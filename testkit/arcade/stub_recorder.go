package arcade

import (
	"sync"

	"github.com/dpopsuev/tako/agent/cerebrum"
)

type StubRecorder struct {
	mu      sync.Mutex
	records []cerebrum.Record
}

func (r *StubRecorder) Append(rec cerebrum.Record) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, rec)
	return nil
}

func (r *StubRecorder) Records() []cerebrum.Record {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]cerebrum.Record(nil), r.records...)
}

func (r *StubRecorder) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.records)
}
