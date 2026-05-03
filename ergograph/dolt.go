package ergograph

import (
	"encoding/json"
	"sync"

	"github.com/jmoiron/sqlx"
)

// DoltLedger is a Dolt-backed append-only pool with hash chain verification.
type DoltLedger struct {
	mu   sync.Mutex
	db   *sqlx.DB
	seq  uint64
	prev string
}

var _ Ledger = (*DoltLedger)(nil)

func NewDoltLedger(db *sqlx.DB) *DoltLedger {
	return &DoltLedger{db: db}
}

func (p *DoltLedger) Append(record Record) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	record.Sequence = p.seq
	record.PrevHash = p.prev
	record.ComputeHash()

	labelsJSON, _ := json.Marshal(record.Labels)

	_, err := p.db.Exec(
		`INSERT INTO ergograph_records (identity, action, timestamp, sequence, labels, payload, hash, prev_hash)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		record.Identity, record.Action, record.Timestamp, record.Sequence,
		string(labelsJSON), record.Payload, record.Hash, record.PrevHash,
	)
	if err != nil {
		return err
	}

	p.prev = record.Hash
	p.seq++
	return nil
}

func (p *DoltLedger) Records() []Record {
	p.mu.Lock()
	defer p.mu.Unlock()

	var rows []struct {
		Identity  string `db:"identity"`
		Action    string `db:"action"`
		Sequence  uint64 `db:"sequence"`
		LabelsRaw string `db:"labels"`
		Payload   []byte `db:"payload"`
		Hash      string `db:"hash"`
		PrevHash  string `db:"prev_hash"`
	}

	err := p.db.Select(&rows,
		`SELECT identity, action, sequence, labels, payload, hash, prev_hash
		 FROM ergograph_records ORDER BY sequence ASC`)
	if err != nil {
		return nil
	}

	out := make([]Record, 0, len(rows))
	for _, row := range rows {
		rec := Record{
			Identity: row.Identity,
			Action:   row.Action,
			Sequence: row.Sequence,
			Payload:  row.Payload,
			Hash:     row.Hash,
			PrevHash: row.PrevHash,
		}
		if row.LabelsRaw != "" {
			_ = json.Unmarshal([]byte(row.LabelsRaw), &rec.Labels)
		}
		out = append(out, rec)
	}
	return out
}

func (p *DoltLedger) VerifyChain() error {
	records := p.Records()
	for i := 1; i < len(records); i++ {
		if records[i].PrevHash != records[i-1].Hash {
			return ErrChainBroken
		}
	}
	return nil
}

func (p *DoltLedger) Len() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	var count int
	err := p.db.Get(&count, "SELECT COUNT(*) FROM ergograph_records")
	if err != nil {
		return 0
	}
	return count
}
