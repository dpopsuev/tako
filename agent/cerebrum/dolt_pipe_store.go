package cerebrum

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
)

type DoltPipeStore struct {
	mu sync.Mutex
	db *sqlx.DB
}

var _ ReflexStore = (*DoltPipeStore)(nil)

func NewDoltPipeStore(db *sqlx.DB) *DoltPipeStore {
	return &DoltPipeStore{db: db}
}

type pipeRow struct {
	Name        string  `db:"name"`
	Description string  `db:"description"`
	Embedding   string  `db:"embedding"`
	Replays     int     `db:"replays"`
	UsageCount  int     `db:"usage_count"`
	LastPlayed  *string `db:"last_played"`
}

type stepRow struct {
	PipeName     string  `db:"pipe_name"`
	StepID       string  `db:"step_id"`
	CallName     string  `db:"call_name"`
	Args         string  `db:"args"`
	DependsOn    string  `db:"depends_on"`
	ExpectedHash []byte  `db:"expected_hash"`
	Confidence   float64 `db:"confidence"`
	StepOrder    int     `db:"step_order"`
}

func (s *DoltPipeStore) Match(embedding []float64) (*Pipe, float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(embedding) == 0 {
		return nil, 0
	}

	var rows []pipeRow
	if err := s.db.Select(&rows,
		`SELECT name, description, embedding, replays, usage_count, last_played FROM pipes`); err != nil {
		return nil, 0
	}

	var best *Pipe
	var bestSim float64

	for _, row := range rows {
		var emb []float64
		if err := json.Unmarshal([]byte(row.Embedding), &emb); err != nil {
			continue
		}
		sim := CosineSimilarity(embedding, emb)
		if sim > bestSim {
			bestSim = sim
			p := s.rowToPipe(row, emb)
			best = &p
		}
	}

	if best != nil {
		steps, _ := s.loadSteps(best.Name)
		best.Steps = steps
	}

	return best, bestSim
}

func (s *DoltPipeStore) Add(pipe Pipe) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var count int
	s.db.Get(&count, "SELECT COUNT(*) FROM pipes WHERE name = ?", pipe.Name)
	if count > 0 {
		return fmt.Errorf("%w: %s", ErrPipeExists, pipe.Name)
	}

	embJSON, _ := json.Marshal(pipe.Embedding)

	tx, err := s.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		`INSERT INTO pipes (name, description, embedding, replays, usage_count, last_played)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		pipe.Name, pipe.Description, string(embJSON),
		pipe.Replays, pipe.Usage, timePtr(pipe.LastPlayed),
	)
	if err != nil {
		return err
	}

	for i, step := range pipe.Steps {
		argsJSON, _ := json.Marshal(step.Args)
		depsJSON, _ := json.Marshal(step.DependsOn)
		_, err = tx.Exec(
			`INSERT INTO pipe_steps (pipe_name, step_id, call_name, args, depends_on, expected_hash, confidence, step_order)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			pipe.Name, step.ID, step.Call, string(argsJSON), string(depsJSON),
			step.Expected[:], step.Confidence, i,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *DoltPipeStore) Merge(embedding []float64, steps []PipeStep) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	var rows []pipeRow
	if err := s.db.Select(&rows,
		`SELECT name, description, embedding, replays, usage_count, last_played FROM pipes`); err != nil {
		return false
	}

	for _, row := range rows {
		var emb []float64
		if err := json.Unmarshal([]byte(row.Embedding), &emb); err != nil {
			continue
		}
		if CosineSimilarity(embedding, emb) > 0.9 {
			existing, _ := s.loadSteps(row.Name)
			merged := mergeStepSlices(existing, steps)
			s.replaceSteps(row.Name, merged)
			return true
		}
	}
	return false
}

func (s *DoltPipeStore) Prune(minScore float64) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	var rows []pipeRow
	if err := s.db.Select(&rows,
		`SELECT name, description, embedding, replays, usage_count, last_played FROM pipes`); err != nil {
		return 0
	}

	pruned := 0
	for _, row := range rows {
		p := s.rowToPipe(row, nil)
		if p.Score() < minScore {
			s.db.Exec("DELETE FROM pipe_steps WHERE pipe_name = ?", row.Name)
			s.db.Exec("DELETE FROM pipes WHERE name = ?", row.Name)
			pruned++
		}
	}
	return pruned
}

func (s *DoltPipeStore) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.db.Exec("CALL dolt_add('.')")
	_, err := s.db.Exec("CALL dolt_commit('-m', 'reflex update')")
	if err != nil {
		_, err = s.db.Exec("CALL dolt_commit('--allow-empty', '-m', 'reflex update')")
	}
	return err
}

func (s *DoltPipeStore) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	var count int
	if err := s.db.Get(&count, "SELECT COUNT(*) FROM pipes"); err != nil {
		return 0
	}
	return count
}

func (s *DoltPipeStore) loadSteps(pipeName string) ([]PipeStep, error) {
	var rows []stepRow
	if err := s.db.Select(&rows,
		`SELECT pipe_name, step_id, call_name, args, depends_on, expected_hash, confidence, step_order
		 FROM pipe_steps WHERE pipe_name = ? ORDER BY step_order`, pipeName); err != nil {
		return nil, err
	}

	steps := make([]PipeStep, len(rows))
	for i, row := range rows {
		steps[i] = PipeStep{
			ID:         row.StepID,
			Call:       row.CallName,
			Confidence: row.Confidence,
		}
		if row.Args != "" && row.Args != "null" {
			json.Unmarshal([]byte(row.Args), &steps[i].Args)
		}
		if row.DependsOn != "" && row.DependsOn != "null" {
			json.Unmarshal([]byte(row.DependsOn), &steps[i].DependsOn)
		}
		if len(row.ExpectedHash) == 32 {
			copy(steps[i].Expected[:], row.ExpectedHash)
		}
	}
	return steps, nil
}

func (s *DoltPipeStore) replaceSteps(pipeName string, steps []PipeStep) {
	s.db.Exec("DELETE FROM pipe_steps WHERE pipe_name = ?", pipeName)
	for i, step := range steps {
		argsJSON, _ := json.Marshal(step.Args)
		depsJSON, _ := json.Marshal(step.DependsOn)
		s.db.Exec(
			`INSERT INTO pipe_steps (pipe_name, step_id, call_name, args, depends_on, expected_hash, confidence, step_order)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			pipeName, step.ID, step.Call, string(argsJSON), string(depsJSON),
			step.Expected[:], step.Confidence, i,
		)
	}
}

func (s *DoltPipeStore) rowToPipe(row pipeRow, emb []float64) Pipe {
	p := Pipe{
		Name:        row.Name,
		Description: row.Description,
		Embedding:   emb,
		Replays:     row.Replays,
		Usage:       row.UsageCount,
	}
	if row.LastPlayed != nil {
		p.LastPlayed, _ = time.Parse("2006-01-02 15:04:05", *row.LastPlayed)
	}
	return p
}

func mergeStepSlices(existing, incoming []PipeStep) []PipeStep {
	i := 0
	for i < len(existing) && i < len(incoming) {
		if existing[i].Call == incoming[i].Call {
			existing[i].Confidence = existing[i].Confidence*0.9 + 0.1
			i++
		} else {
			break
		}
	}
	for j := i; j < len(incoming); j++ {
		existing = append(existing, incoming[j])
	}
	return existing
}

func timePtr(t time.Time) *string {
	if t.IsZero() {
		return nil
	}
	s := t.Format("2006-01-02 15:04:05")
	return &s
}
