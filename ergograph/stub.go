package ergograph

import "sync"

// StubLedger is an append-only in-memory pool with hash chain verification.
type StubLedger struct {
	mu      sync.Mutex
	records []Record
}

var _ Ledger = (*StubLedger)(nil)

func (p *StubLedger) Append(record Record) error {
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

func (p *StubLedger) Records() []Record {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]Record(nil), p.records...)
}

func (p *StubLedger) VerifyChain() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := 1; i < len(p.records); i++ {
		if p.records[i].PrevHash != p.records[i-1].Hash {
			return ErrChainBroken
		}
	}
	return nil
}

func (p *StubLedger) Len() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.records)
}

// StubInspector always returns perfect OAE scores.
type StubInspector struct{}

var _ Inspector = StubInspector{}

func (StubInspector) Verify(ledger Ledger) error {
	return ledger.VerifyChain()
}

func (StubInspector) Score(_ Ledger) (OAE, error) {
	return OAE{Availability: 1.0, Performance: 1.0, Quality: 1.0}, nil
}
