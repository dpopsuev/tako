package depo

import (
	"database/sql"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/dpopsuev/tako/artifact"
	"github.com/jmoiron/sqlx"
)

// DoltDepo is a Dolt-backed Depo.
type DoltDepo struct {
	mu      sync.Mutex
	db      *sqlx.DB
	name    string
	shelves map[string]*DoltShelf
}

var _ Depo = (*DoltDepo)(nil)

func NewDoltDepo(db *sqlx.DB, name string) *DoltDepo {
	return &DoltDepo{
		db:      db,
		name:    name,
		shelves: make(map[string]*DoltShelf),
	}
}

func (d *DoltDepo) Shelf(name string) Shelf {
	d.mu.Lock()
	defer d.mu.Unlock()
	s, ok := d.shelves[name]
	if !ok {
		s = &DoltShelf{db: d.db, name: name, dbmu: &d.mu}
		d.shelves[name] = s
	}
	return s
}

func (d *DoltDepo) Shelves() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]string, 0, len(d.shelves))
	for name := range d.shelves {
		out = append(out, name)
	}
	return out
}

// DoltShelf is a Dolt-backed Shelf.
type DoltShelf struct {
	mu       sync.Mutex
	dbmu     *sync.Mutex
	db       *sqlx.DB
	name     string
	watchers []chan artifact.Envelope
}

var _ Shelf = (*DoltShelf)(nil)

func (s *DoltShelf) Push(envelope artifact.Envelope) error {
	labelsJSON, err := json.Marshal(envelope.Labels)
	if err != nil {
		return err
	}
	s.dbmu.Lock()
	_, err = s.db.Exec(
		`INSERT INTO envelopes (id, shelf_name, origin, payload, labels, hash, state, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, 'UNCLAIMED', ?)`,
		envelope.ID, s.name, envelope.Origin, envelope.Payload, string(labelsJSON),
		envelope.Hash, envelope.CreatedAt,
	)
	s.dbmu.Unlock()
	if err != nil {
		return err
	}
	s.mu.Lock()
	for _, w := range s.watchers {
		select {
		case w <- envelope:
		default:
		}
	}
	s.mu.Unlock()
	return nil
}

func (s *DoltShelf) Pull(agentID string) (artifact.Envelope, error) {
	s.dbmu.Lock()
	defer s.dbmu.Unlock()

	now := time.Now()
	expiresAt := now.Add(5 * time.Minute)

	var row struct {
		ID        string         `db:"id"`
		Origin    string         `db:"origin"`
		Payload   []byte         `db:"payload"`
		LabelsRaw string         `db:"labels"`
		Hash      string         `db:"hash"`
		CreatedAt time.Time      `db:"created_at"`
		ClaimedBy sql.NullString `db:"claimed_by"`
	}

	err := s.db.Get(&row,
		`SELECT id, origin, payload, labels, hash, created_at, claimed_by
		 FROM envelopes
		 WHERE shelf_name = ? AND state = 'UNCLAIMED'
		 LIMIT 1`,
		s.name,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return artifact.Envelope{}, ErrShelfEmpty
		}
		return artifact.Envelope{}, err
	}

	_, err = s.db.Exec(
		`UPDATE envelopes SET state = 'CLAIMED', claimed_by = ?, claimed_at = ?, expires_at = ? WHERE id = ?`,
		agentID, now, expiresAt, row.ID,
	)
	if err != nil {
		return artifact.Envelope{}, err
	}

	env := artifact.Envelope{
		ID:        row.ID,
		Origin:    row.Origin,
		Payload:   row.Payload,
		Hash:      row.Hash,
		CreatedAt: row.CreatedAt,
	}
	env.Labels = make(map[string]string)
	if row.LabelsRaw != "" {
		_ = json.Unmarshal([]byte(row.LabelsRaw), &env.Labels)
	}
	return env, nil
}

func (s *DoltShelf) Peek() []artifact.Envelope {
	s.dbmu.Lock()
	defer s.dbmu.Unlock()

	var rows []struct {
		ID        string    `db:"id"`
		Origin    string    `db:"origin"`
		Payload   []byte    `db:"payload"`
		LabelsRaw string    `db:"labels"`
		Hash      string    `db:"hash"`
		CreatedAt time.Time `db:"created_at"`
	}
	err := s.db.Select(&rows,
		`SELECT id, origin, payload, labels, hash, created_at
		 FROM envelopes
		 WHERE shelf_name = ? AND state = 'UNCLAIMED'`,
		s.name,
	)
	if err != nil {
		return nil
	}
	out := make([]artifact.Envelope, 0, len(rows))
	for _, row := range rows {
		env := artifact.Envelope{
			ID:        row.ID,
			Origin:    row.Origin,
			Payload:   row.Payload,
			Hash:      row.Hash,
			CreatedAt: row.CreatedAt,
		}
		env.Labels = make(map[string]string)
		if row.LabelsRaw != "" {
			_ = json.Unmarshal([]byte(row.LabelsRaw), &env.Labels)
		}
		out = append(out, env)
	}
	return out
}

func (s *DoltShelf) Watch() <-chan artifact.Envelope {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch := make(chan artifact.Envelope, 16)
	s.watchers = append(s.watchers, ch)
	return ch
}
