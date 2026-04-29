package observe

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type RunRecord struct {
	ID          string    `json:"id"`
	TraceID     string    `json:"trace_id,omitempty"`
	Scenario    string    `json:"scenario"`
	Backend     string    `json:"backend,omitempty"`
	Parallel    int       `json:"parallel"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
	DurationMs  int64     `json:"duration_ms"`
	CaseCount   int       `json:"case_count"`
	ErrorCount  int       `json:"error_count"`
	TraceEvents int       `json:"trace_events"`
}

func SaveRunRecord(dir string, rec *RunRecord) error {
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "run.json"), data, 0o600)
}

func LoadRunRecord(dir string) (*RunRecord, error) {
	data, err := os.ReadFile(filepath.Join(dir, "run.json"))
	if err != nil {
		return nil, err
	}
	var rec RunRecord
	return &rec, json.Unmarshal(data, &rec)
}
