package ergograph

import "sync"

// StubPool is an append-only in-memory pool with hash chain verification.
type StubPool struct {
	mu      sync.Mutex
	records []Record
}

var _ Pool = (*StubPool)(nil)

func (p *StubPool) Append(record Record) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	record.Sequence = uint64(len(p.records))
	if len(p.records) > 0 {
		record.PrevHash = p.records[len(p.records)-1].Hash
	}
	record.ComputeHash()
	p.records = append(p.records, record)
	return nil
}

func (p *StubPool) Records() []Record {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]Record(nil), p.records...)
}

func (p *StubPool) VerifyChain() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := 1; i < len(p.records); i++ {
		if p.records[i].PrevHash != p.records[i-1].Hash {
			return ErrChainBroken
		}
	}
	return nil
}

func (p *StubPool) Len() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.records)
}

// StubInspector always returns perfect OAE scores.
type StubInspector struct{}

var _ Inspector = StubInspector{}

func (StubInspector) Verify(pool Pool) error {
	return pool.VerifyChain()
}

func (StubInspector) Score(_ Pool) (OAE, error) {
	return OAE{Availability: 1.0, Performance: 1.0, Quality: 1.0}, nil
}
