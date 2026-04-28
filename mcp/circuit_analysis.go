package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/dpopsuev/tako/circuit"
	"github.com/dpopsuev/tako/engine"
)

// --- HITL handlers ---

type inspectOutput struct {
	WalkerID      string               `json:"walker_id"`
	CurrentNode   string               `json:"current_node"`
	Status        string               `json:"status"`
	InterruptData map[string]any       `json:"interrupt_data,omitempty"`
	History       []circuit.StepRecord `json:"history"`
	LoopCounts    map[string]int       `json:"loop_counts,omitempty"`
}

type resumeOutput struct {
	Status string `json:"status"` // "resumed"
}

func (s *CircuitServer) handleInspectCircuit(ctx context.Context, input *circuitInput) (inspectOutput, error) {
	logger := slog.Default().With(slog.Any(circuit.LogKeyComponent, circuit.LogComponentCircuitSession))

	if s.Config.Checkpointer == nil {
		return inspectOutput{}, ErrCheckpointerNotConfigured
	}
	if input.WalkerID == "" {
		return inspectOutput{}, ErrWalkerIDRequired
	}

	inspection, err := engine.InspectCheckpoint(s.Config.Checkpointer, input.WalkerID)
	if err != nil {
		return inspectOutput{}, err
	}

	logger.InfoContext(ctx, circuit.LogInspectCheckpoint,
		slog.Any(circuit.LogKeyWalkerID, input.WalkerID),
		slog.Any(circuit.LogKeyNode, inspection.CurrentNode),
		slog.Any(circuit.LogKeyStatus, inspection.Status))

	return inspectOutput{
		WalkerID:      inspection.WalkerID,
		CurrentNode:   inspection.CurrentNode,
		Status:        inspection.Status,
		InterruptData: inspection.InterruptData,
		History:       inspection.History,
		LoopCounts:    inspection.LoopCounts,
	}, nil
}

func (s *CircuitServer) handleResumeCircuit(ctx context.Context, input *circuitInput) (resumeOutput, error) {
	logger := slog.Default().With(slog.Any(circuit.LogKeyComponent, circuit.LogComponentCircuitSession))

	if s.Config.Checkpointer == nil {
		return resumeOutput{}, ErrCheckpointerNotConfigured
	}
	if input.WalkerID == "" {
		return resumeOutput{}, ErrWalkerIDRequired
	}

	// Verify checkpoint exists and walker is interrupted.
	inspection, err := engine.InspectCheckpoint(s.Config.Checkpointer, input.WalkerID)
	if err != nil {
		return resumeOutput{}, err
	}
	if inspection.Status != "interrupted" {
		return resumeOutput{}, fmt.Errorf("%w: status is %q", engine.ErrWalkerNotInterrupted, inspection.Status)
	}

	sess, err := s.getSession(input.SessionID)
	if err != nil {
		return resumeOutput{}, err
	}

	// Inject resume input and restart the walk via the session.
	if err := sess.ResumeWalk(s.Config.Checkpointer, input.WalkerID, input.ResumeInput); err != nil {
		return resumeOutput{}, fmt.Errorf("resume walk: %w", err)
	}

	logger.InfoContext(ctx, circuit.LogResumeWalk,
		slog.Any(circuit.LogKeySessionID, input.SessionID),
		slog.Any(circuit.LogKeyWalkerID, input.WalkerID))

	return resumeOutput{Status: "resumed"}, nil
}

// --- Post-mortem handlers ---

// handleRunSummary returns a compact summary of the circuit run result.
// Extracts metrics and per-case one-liners, excluding verbose fields like
// actual_rca_message, evidence_refs, evidence_gaps. Target: <4KB response.
func (s *CircuitServer) handleRunSummary(sess *CircuitSession) (any, error) {
	result := sess.Result()
	if result == nil {
		return nil, ErrNoResultAvailable
	}
	data, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}
	var full map[string]any
	if err := json.Unmarshal(data, &full); err != nil {
		return nil, fmt.Errorf("parse result: %w", err)
	}

	summary := make(map[string]any)

	// Extract metrics (compact)
	if metrics, ok := full["metrics"]; ok {
		summary["metrics"] = metrics
	}

	// Extract case one-liners
	if oneLiners := extractCaseOneLiners(full); len(oneLiners) > 0 {
		summary["cases"] = oneLiners
	}

	// If no structured data was extracted, return the full result as-is
	// (domain may not use metrics/case_results keys)
	if len(summary) == 0 {
		summary["result"] = full
	}

	return summary, nil
}

var caseOneLinerKeys = []string{"case_id", "defect_type", "category", "component", "convergence", "step_count"}

func extractCaseOneLiners(full map[string]any) []map[string]any {
	caseResults, ok := full["case_results"]
	if !ok {
		return nil
	}
	cases, ok := caseResults.([]any)
	if !ok {
		return nil
	}
	var oneLiners []map[string]any
	for _, c := range cases {
		cm, ok := c.(map[string]any)
		if !ok {
			continue
		}
		oneLiner := make(map[string]any)
		for _, key := range caseOneLinerKeys {
			if v, exists := cm[key]; exists {
				oneLiner[key] = v
			}
		}
		if len(oneLiner) > 0 {
			oneLiners = append(oneLiners, oneLiner)
		}
	}
	return oneLiners
}

// handleCaseDetail returns the full case_result for a single case_id.
func (s *CircuitServer) handleCaseDetail(sess *CircuitSession, caseID string) (any, error) {
	if caseID == "" {
		return nil, ErrCaseIdIsRequiredForDetailAction
	}
	result := sess.Result()
	if result == nil {
		return nil, ErrNoResultAvailable
	}
	data, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}
	var full map[string]any
	if err := json.Unmarshal(data, &full); err != nil {
		return nil, fmt.Errorf("parse result: %w", err)
	}

	caseResults, ok := full["case_results"]
	if !ok {
		return nil, fmt.Errorf("%w: %q not found in results", ErrCaseId, caseID)
	}
	cases, ok := caseResults.([]any)
	if !ok {
		return nil, fmt.Errorf("%w: %q not found in results", ErrCaseId, caseID)
	}
	for _, c := range cases {
		cm, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if cm["case_id"] == caseID {
			return cm, nil
		}
	}
	return nil, fmt.Errorf("%w: %q not found in results", ErrCaseId, caseID)
}

// handleFailingMetrics returns only the metrics where pass=false.
func (s *CircuitServer) handleFailingMetrics(sess *CircuitSession) (any, error) {
	result := sess.Result()
	if result == nil {
		return nil, ErrNoResultAvailable
	}
	data, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}
	var full map[string]any
	if err := json.Unmarshal(data, &full); err != nil {
		return nil, fmt.Errorf("parse result: %w", err)
	}

	metricsRaw, ok := full["metrics"]
	if !ok {
		return map[string]any{"failing": []any{}}, nil
	}
	metricsMap, ok := metricsRaw.(map[string]any)
	if !ok {
		return map[string]any{"failing": []any{}}, nil
	}
	metricsList, ok := metricsMap["metrics"]
	if !ok {
		return map[string]any{"failing": []any{}}, nil
	}
	metrics, ok := metricsList.([]any)
	if !ok {
		return map[string]any{"failing": []any{}}, nil
	}

	var failing []any
	for _, m := range metrics {
		mm, ok := m.(map[string]any)
		if !ok {
			continue
		}
		if pass, ok := mm["pass"].(bool); ok && !pass {
			failing = append(failing, mm)
		}
	}
	return map[string]any{"failing": failing}, nil
}

// handleWeakCases returns cases where convergence < threshold.
func (s *CircuitServer) handleWeakCases(sess *CircuitSession, threshold float64) (any, error) {
	result := sess.Result()
	if result == nil {
		return nil, ErrNoResultAvailable
	}
	data, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}
	var full map[string]any
	if err := json.Unmarshal(data, &full); err != nil {
		return nil, fmt.Errorf("parse result: %w", err)
	}

	caseResults, ok := full["case_results"]
	if !ok {
		return map[string]any{"weak": []any{}, "threshold": threshold}, nil
	}
	cases, ok := caseResults.([]any)
	if !ok {
		return map[string]any{"weak": []any{}, "threshold": threshold}, nil
	}

	var weak []any
	for _, c := range cases {
		cm, ok := c.(map[string]any)
		if !ok {
			continue
		}
		conv, ok := cm["convergence"].(float64)
		if ok && conv < threshold {
			weak = append(weak, cm)
		}
	}
	return map[string]any{"weak": weak, "threshold": threshold}, nil
}

// handleConfusion groups failing cases by (actual, expected) pairs for a given metric field.
// Enables data-driven tuning: shows which misclassification patterns are most frequent.
func (s *CircuitServer) handleConfusion(sess *CircuitSession, metric string) (any, error) {
	if metric == "" {
		metric = "component"
	}

	result := sess.Result()
	if result == nil {
		return nil, ErrNoResultAvailable
	}
	data, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}
	var full map[string]any
	if err := json.Unmarshal(data, &full); err != nil {
		return nil, fmt.Errorf("parse result: %w", err)
	}

	caseResults, ok := full["case_results"]
	if !ok {
		return map[string]any{"metric": metric, "confusion": []any{}, "total": 0, "correct": 0, "incorrect": 0}, nil
	}
	cases, ok := caseResults.([]any)
	if !ok {
		return map[string]any{"metric": metric, "confusion": []any{}, "total": 0, "correct": 0, "incorrect": 0}, nil
	}

	// Field name mapping: metric → (actual field, expected field, correct field)
	type fieldSpec struct{ actual, expected, correct string }
	fields := map[string]fieldSpec{
		"component":   {"actual_component", "expected_component", "component_correct"},
		"category":    {"actual_category", "expected_category", "category_correct"},
		"defect_type": {"actual_defect_type", "expected_defect_type", "defect_type_correct"},
	}
	spec, ok := fields[metric]
	if !ok {
		return nil, fmt.Errorf("%w: %q; valid: component, category, defect_type", ErrUnknownMetric, metric)
	}

	// Group incorrect cases by (actual, expected) pair
	type confusionKey struct{ actual, expected string }
	groups := make(map[confusionKey][]string)
	total, correct := 0, 0

	for _, c := range cases {
		cm, ok := c.(map[string]any)
		if !ok {
			continue
		}
		total++
		isCorrect, _ := cm[spec.correct].(bool)
		if isCorrect {
			correct++
			continue
		}
		actual, _ := cm[spec.actual].(string)
		expected, _ := cm[spec.expected].(string)
		caseID, _ := cm["case_id"].(string)
		if actual == "" && expected == "" {
			continue
		}
		key := confusionKey{actual, expected}
		groups[key] = append(groups[key], caseID)
	}

	// Build sorted result (highest count first)
	type confusionEntry struct {
		Actual   string   `json:"actual"`
		Expected string   `json:"expected"`
		Cases    []string `json:"cases"`
		Count    int      `json:"count"`
	}
	entries := make([]confusionEntry, 0, len(groups))
	for key, caseIDs := range groups {
		entries = append(entries, confusionEntry{
			Actual:   key.actual,
			Expected: key.expected,
			Cases:    caseIDs,
			Count:    len(caseIDs),
		})
	}
	// Sort by count descending
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].Count > entries[i].Count {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	return map[string]any{
		"metric":    metric,
		"total":     total,
		"correct":   correct,
		"incorrect": total - correct,
		"confusion": entries,
	}, nil
}

// handleDiff compares metrics between two runs by loading their report.json files.
func (s *CircuitServer) handleDiff(sessionA, sessionB string) (any, error) {
	if sessionA == "" || sessionB == "" {
		return nil, fmt.Errorf("%w: diff requires session_id (run A) and against (run B)", ErrSessionId)
	}
	stateDir := s.Config.StateDir
	if stateDir == "" {
		return nil, fmt.Errorf("%w: StateDir not configured", ErrNoResultAvailable)
	}

	loadReport := func(sessionID string) (map[string]any, error) {
		path := filepath.Join(stateDir, "runs", sessionID, "report.json")
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read report for %s: %w", sessionID, err)
		}
		var report map[string]any
		if err := json.Unmarshal(data, &report); err != nil {
			return nil, fmt.Errorf("parse report for %s: %w", sessionID, err)
		}
		return report, nil
	}

	reportA, err := loadReport(sessionA)
	if err != nil {
		return nil, err
	}
	reportB, err := loadReport(sessionB)
	if err != nil {
		return nil, err
	}

	// Extract metrics from both reports
	extractMetrics := func(report map[string]any) map[string]map[string]any {
		result := make(map[string]map[string]any)
		metricsObj, _ := report["metrics"].(map[string]any)
		metricsList, _ := metricsObj["metrics"].([]any)
		for _, m := range metricsList {
			mm, ok := m.(map[string]any)
			if !ok {
				continue
			}
			id, _ := mm["id"].(string)
			if id != "" {
				result[id] = mm
			}
		}
		return result
	}

	metricsA := extractMetrics(reportA)
	metricsB := extractMetrics(reportB)

	// Compute metric deltas
	type metricDelta struct {
		ID           string  `json:"id"`
		Name         string  `json:"name"`
		Before       float64 `json:"before"`
		After        float64 `json:"after"`
		Delta        float64 `json:"delta"`
		StatusChange string  `json:"status_change,omitempty"` // "fail→pass", "pass→fail", etc.
	}

	deltas := make([]metricDelta, 0, len(metricsA))
	for id, ma := range metricsA {
		valA, _ := ma["value"].(float64)
		passA, _ := ma["pass"].(bool)
		name, _ := ma["name"].(string)
		d := metricDelta{ID: id, Name: name, Before: valA}
		if mb, ok := metricsB[id]; ok {
			valB, _ := mb["value"].(float64)
			passB, _ := mb["pass"].(bool)
			d.After = valB
			d.Delta = valB - valA
			if !passA && passB {
				d.StatusChange = "fail→pass"
			} else if passA && !passB {
				d.StatusChange = "pass→fail"
			}
		}
		deltas = append(deltas, d)
	}

	return map[string]any{
		"session_a": sessionA,
		"session_b": sessionB,
		"metrics":   deltas,
	}, nil
}
