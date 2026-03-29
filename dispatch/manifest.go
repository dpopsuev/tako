package dispatch

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// BatchManifest is the coordination file between the Go CLI and the Cursor
// skill for multi-subagent batch dispatch.
type BatchManifest struct {
	BatchID      int64              `json:"batch_id"`
	Status       string             `json:"status"`
	Phase        string             `json:"phase"`
	CreatedAt    string             `json:"created_at"`
	UpdatedAt    string             `json:"updated_at"`
	Total        int                `json:"total"`
	BriefingPath string             `json:"briefing_path"`
	Signals      []BatchSignalEntry `json:"signals"`
}

// BatchSignalEntry is one per-case signal reference in a batch manifest.
type BatchSignalEntry struct {
	CaseID     string `json:"case_id"`
	SignalPath string `json:"signal_path"`
	Status     string `json:"status"`
}

// BudgetStatus is written alongside the batch manifest to inform the skill
// parent about token budget consumption.
type BudgetStatus struct {
	TotalBudget int     `json:"total_budget"`
	Used        int     `json:"used"`
	Remaining   int     `json:"remaining"`
	PercentUsed float64 `json:"percent_used"`
}

// NewBatchManifest creates a manifest in pending state.
func NewBatchManifest(batchID int64, phase, briefingPath string, signals []BatchSignalEntry) *BatchManifest {
	now := time.Now().UTC().Format(time.RFC3339)
	return &BatchManifest{
		BatchID:      batchID,
		Status:       "pending",
		Phase:        phase,
		CreatedAt:    now,
		UpdatedAt:    now,
		Total:        len(signals),
		BriefingPath: briefingPath,
		Signals:      signals,
	}
}

// WriteManifest atomically writes a BatchManifest to disk.
func WriteManifest(path string, m *BatchManifest) error {
	m.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write manifest tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		defer os.Remove(tmp)
		return os.WriteFile(path, data, 0o600)
	}
	return nil
}

// ReadManifest reads a BatchManifest from disk.
func ReadManifest(path string) (*BatchManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	var m BatchManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("unmarshal manifest: %w", err)
	}
	return &m, nil
}

// WriteBudgetStatus atomically writes a BudgetStatus to disk.
func WriteBudgetStatus(path string, totalBudget, used int) error {
	remaining := totalBudget - used
	pct := 0.0
	if totalBudget > 0 {
		pct = float64(used) / float64(totalBudget) * 100.0
	}
	bs := BudgetStatus{
		TotalBudget: totalBudget,
		Used:        used,
		Remaining:   remaining,
		PercentUsed: pct,
	}
	data, err := json.MarshalIndent(bs, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal budget status: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write budget tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		defer os.Remove(tmp)
		return os.WriteFile(path, data, 0o600)
	}
	return nil
}

// ReadBudgetStatus reads a BudgetStatus from disk.
func ReadBudgetStatus(path string) (*BudgetStatus, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read budget status: %w", err)
	}
	var bs BudgetStatus
	if err := json.Unmarshal(data, &bs); err != nil {
		return nil, fmt.Errorf("unmarshal budget status: %w", err)
	}
	return &bs, nil
}
