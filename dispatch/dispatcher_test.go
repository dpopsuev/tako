package dispatch

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileDispatcher_HappyPath(t *testing.T) {
	dir := t.TempDir()
	artifactPath := filepath.Join(dir, "artifact.json")
	promptPath := filepath.Join(dir, "prompt.md")
	signalPath := filepath.Join(dir, "signal.json")

	if err := os.WriteFile(promptPath, []byte("# Prompt"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := FileDispatcherConfig{
		PollInterval: 50 * time.Millisecond,
		Timeout:      5 * time.Second,
		SignalDir:    dir,
	}
	d := NewFileDispatcher(cfg)

	ctx := DispatchContext{
		CaseID:       "C1",
		Step:         "F0_RECALL",
		PromptPath:   promptPath,
		ArtifactPath: artifactPath,
	}

	// Write artifact in background after signal appears
	go func() {
		// Wait for signal to be written with dispatch_id
		var did int64
		for i := 0; i < 100; i++ {
			time.Sleep(20 * time.Millisecond)
			data, err := os.ReadFile(signalPath)
			if err != nil {
				continue
			}
			var sig SignalFile
			if json.Unmarshal(data, &sig) == nil && sig.Status == "waiting" {
				did = sig.DispatchID
				break
			}
		}
		artifact := map[string]any{"match": true, "confidence": 0.95}
		inner, _ := json.Marshal(artifact)
		wrapper := ArtifactWrapper{DispatchID: did, Data: json.RawMessage(inner)}
		data, _ := json.MarshalIndent(wrapper, "", "  ")
		_ = os.WriteFile(artifactPath, data, 0644)
	}()

	data, err := d.Dispatch(context.Background(), ctx)
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	// Verify we got the inner data
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal artifact: %v", err)
	}
	if result["match"] != true {
		t.Errorf("expected match=true, got %v", result["match"])
	}

	// Verify signal.json was updated to "processing"
	sigData, err := os.ReadFile(signalPath)
	if err != nil {
		t.Fatalf("read signal: %v", err)
	}
	var sig SignalFile
	if err := json.Unmarshal(sigData, &sig); err != nil {
		t.Fatalf("unmarshal signal: %v", err)
	}
	if sig.Status != "processing" {
		t.Errorf("expected signal status=processing, got %q", sig.Status)
	}
	if sig.DispatchID != 1 {
		t.Errorf("expected dispatch_id=1, got %d", sig.DispatchID)
	}

	// MarkDone should update to "done"
	d.MarkDone(artifactPath)
	sigData, _ = os.ReadFile(signalPath)
	_ = json.Unmarshal(sigData, &sig)
	if sig.Status != "done" {
		t.Errorf("expected signal status=done after MarkDone, got %q", sig.Status)
	}
}

func TestFileDispatcher_Timeout(t *testing.T) {
	dir := t.TempDir()
	artifactPath := filepath.Join(dir, "artifact.json")
	promptPath := filepath.Join(dir, "prompt.md")
	_ = os.WriteFile(promptPath, []byte("# Prompt"), 0644)

	cfg := FileDispatcherConfig{
		PollInterval: 10 * time.Millisecond,
		Timeout:      100 * time.Millisecond,
		SignalDir:    dir,
	}
	d := NewFileDispatcher(cfg)

	ctx := DispatchContext{
		CaseID:       "C1",
		Step:         "F1_TRIAGE",
		PromptPath:   promptPath,
		ArtifactPath: artifactPath,
	}

	// No artifact written — should timeout
	_, err := d.Dispatch(context.Background(), ctx)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}

	// Verify signal.json shows error status
	signalPath := filepath.Join(dir, "signal.json")
	sigData, err := os.ReadFile(signalPath)
	if err != nil {
		t.Fatalf("read signal: %v", err)
	}
	var sig SignalFile
	if err := json.Unmarshal(sigData, &sig); err != nil {
		t.Fatalf("unmarshal signal: %v", err)
	}
	if sig.Status != "error" {
		t.Errorf("expected signal status=error on timeout, got %q", sig.Status)
	}
	if sig.Error == "" {
		t.Error("expected non-empty error in signal after timeout")
	}
}

func TestFileDispatcher_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	artifactPath := filepath.Join(dir, "artifact.json")
	promptPath := filepath.Join(dir, "prompt.md")
	_ = os.WriteFile(promptPath, []byte("# Prompt"), 0644)

	cfg := FileDispatcherConfig{
		PollInterval: 10 * time.Millisecond,
		Timeout:      500 * time.Millisecond,
		SignalDir:    dir,
	}
	d := NewFileDispatcher(cfg)

	ctx := DispatchContext{
		CaseID:       "C2",
		Step:         "F3_INVESTIGATE",
		PromptPath:   promptPath,
		ArtifactPath: artifactPath,
	}

	// Write invalid JSON immediately
	if err := os.WriteFile(artifactPath, []byte("not valid json {{{"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := d.Dispatch(context.Background(), ctx)
	if err == nil {
		t.Fatal("expected invalid JSON error, got nil")
	}

	// Verify signal.json shows error status
	signalPath := filepath.Join(dir, "signal.json")
	sigData, _ := os.ReadFile(signalPath)
	var sig SignalFile
	_ = json.Unmarshal(sigData, &sig)
	if sig.Status != "error" {
		t.Errorf("expected signal status=error for invalid JSON, got %q", sig.Status)
	}
}

func TestFileDispatcher_SignalLifecycle(t *testing.T) {
	dir := t.TempDir()
	artifactPath := filepath.Join(dir, "artifact.json")
	promptPath := filepath.Join(dir, "prompt.md")
	signalPath := filepath.Join(dir, "signal.json")
	_ = os.WriteFile(promptPath, []byte("# Prompt"), 0644)

	cfg := FileDispatcherConfig{
		PollInterval: 10 * time.Millisecond,
		Timeout:      2 * time.Second,
		SignalDir:    dir,
	}
	d := NewFileDispatcher(cfg)

	ctx := DispatchContext{
		CaseID:       "C3",
		Step:         "F2_RESOLVE",
		PromptPath:   promptPath,
		ArtifactPath: artifactPath,
	}

	// Dispatch in background; goroutine watches for signal and echoes dispatch_id
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 100; i++ {
			sigData, err := os.ReadFile(signalPath)
			if err == nil {
				var sig SignalFile
				if json.Unmarshal(sigData, &sig) == nil && sig.Status == "waiting" {
					payload := map[string]any{"selected_repos": []string{"linuxptp-daemon"}}
					inner, _ := json.Marshal(payload)
					wrapper := ArtifactWrapper{DispatchID: sig.DispatchID, Data: json.RawMessage(inner)}
					data, _ := json.MarshalIndent(wrapper, "", "  ")
					_ = os.WriteFile(artifactPath, data, 0644)
					return
				}
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	data, err := d.Dispatch(context.Background(), ctx)
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	<-done

	// Verify we got valid inner data
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal artifact: %v", err)
	}

	// Full lifecycle: waiting → processing → done
	d.MarkDone(artifactPath)

	sigData, _ := os.ReadFile(signalPath)
	var sig SignalFile
	_ = json.Unmarshal(sigData, &sig)
	if sig.Status != "done" {
		t.Errorf("expected signal status=done, got %q", sig.Status)
	}
}

func TestFileDispatcher_RejectsStaleArtifactByDispatchID(t *testing.T) {
	dir := t.TempDir()
	artifactPath := filepath.Join(dir, "artifact.json")
	promptPath := filepath.Join(dir, "prompt.md")
	signalPath := filepath.Join(dir, "signal.json")
	_ = os.WriteFile(promptPath, []byte("# Prompt"), 0644)

	// Pre-create a stale artifact with dispatch_id=0 (no dispatch_id)
	stale := ArtifactWrapper{DispatchID: 0, Data: json.RawMessage(`{"stale": true}`)}
	staleData, _ := json.MarshalIndent(stale, "", "  ")
	if err := os.WriteFile(artifactPath, staleData, 0644); err != nil {
		t.Fatal(err)
	}

	cfg := FileDispatcherConfig{
		PollInterval:    10 * time.Millisecond,
		Timeout:         2 * time.Second,
		MaxStaleRejects: 10, // allow enough stale reads for the goroutine to replace it
		SignalDir:       dir,
	}
	d := NewFileDispatcher(cfg)

	ctx := DispatchContext{
		CaseID:       "C4",
		Step:         "F0_RECALL",
		PromptPath:   promptPath,
		ArtifactPath: artifactPath,
	}

	// After a short delay, overwrite with a correctly wrapped artifact
	go func() {
		for i := 0; i < 100; i++ {
			data, err := os.ReadFile(signalPath)
			if err == nil {
				var sig SignalFile
				if json.Unmarshal(data, &sig) == nil && sig.Status == "waiting" {
					time.Sleep(30 * time.Millisecond) // let dispatcher see the stale one a few times
					fresh := map[string]any{"match": false, "confidence": 0.1}
					inner, _ := json.Marshal(fresh)
					wrapper := ArtifactWrapper{DispatchID: sig.DispatchID, Data: json.RawMessage(inner)}
					out, _ := json.MarshalIndent(wrapper, "", "  ")
					_ = os.WriteFile(artifactPath, out, 0644)
					return
				}
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	data, err := d.Dispatch(context.Background(), ctx)
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	// Should get the fresh artifact, not the stale one
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result["stale"] != nil {
		t.Errorf("got stale artifact — dispatch_id rejection failed")
	}
	if result["match"] != false {
		t.Errorf("expected fresh artifact with match=false, got %v", result["match"])
	}
}

func TestFileDispatcher_StaleToleranceExceeded(t *testing.T) {
	dir := t.TempDir()
	artifactPath := filepath.Join(dir, "artifact.json")
	promptPath := filepath.Join(dir, "prompt.md")
	_ = os.WriteFile(promptPath, []byte("# Prompt"), 0644)

	cfg := FileDispatcherConfig{
		PollInterval:    10 * time.Millisecond,
		Timeout:         5 * time.Second,
		MaxStaleRejects: 3,
		SignalDir:       dir,
	}
	d := NewFileDispatcher(cfg)

	ctx := DispatchContext{
		CaseID:       "C6",
		Step:         "F0_RECALL",
		PromptPath:   promptPath,
		ArtifactPath: artifactPath,
	}

	// Write stale artifacts in a goroutine to simulate a malfunctioning
	// responder that keeps producing artifacts with the wrong dispatch_id.
	// The goroutine waits briefly so the dispatcher's pre-cleanup runs first,
	// then continuously re-writes stale artifacts during polling.
	// Writes use atomic rename to avoid partial-read noise in this test —
	// we're testing stale dispatch_id rejection, not partial-write handling.
	done := make(chan struct{})
	go func() {
		defer close(done)
		time.Sleep(5 * time.Millisecond) // let dispatch start and remove any pre-existing artifact
		stale := ArtifactWrapper{DispatchID: 999, Data: json.RawMessage(`{"stale": true}`)}
		staleData, _ := json.MarshalIndent(stale, "", "  ")
		tmp := artifactPath + ".tmp"
		for i := 0; i < 20; i++ {
			_ = os.WriteFile(tmp, staleData, 0644)
			_ = os.Rename(tmp, artifactPath)
			time.Sleep(5 * time.Millisecond)
		}
	}()
	defer func() { <-done }()

	start := time.Now()
	_, err := d.Dispatch(context.Background(), ctx)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected stale tolerance error, got nil")
	}
	if elapsed > 2*time.Second {
		t.Errorf("expected fail-fast but took %v (timeout is 5s)", elapsed)
	}
	if !contains(err.Error(), "stale artifact tolerance exceeded") {
		t.Errorf("expected 'stale artifact tolerance exceeded' in message, got: %v", err)
	}

	// Verify signal.json shows error status
	signalPath := filepath.Join(dir, "signal.json")
	sigData, _ := os.ReadFile(signalPath)
	var sig SignalFile
	_ = json.Unmarshal(sigData, &sig)
	if sig.Status != "error" {
		t.Errorf("expected signal status=error for stale tolerance, got %q", sig.Status)
	}
}

func TestFileDispatcher_ResponderErrorFailsFast(t *testing.T) {
	dir := t.TempDir()
	artifactPath := filepath.Join(dir, "artifact.json")
	promptPath := filepath.Join(dir, "prompt.md")
	signalPath := filepath.Join(dir, "signal.json")
	_ = os.WriteFile(promptPath, []byte("# Prompt"), 0644)

	cfg := FileDispatcherConfig{
		PollInterval: 10 * time.Millisecond,
		Timeout:      5 * time.Second, // long timeout — but we should fail fast
		SignalDir:    dir,
	}
	d := NewFileDispatcher(cfg)

	ctx := DispatchContext{
		CaseID:       "C5",
		Step:         "F1_TRIAGE",
		PromptPath:   promptPath,
		ArtifactPath: artifactPath,
	}

	// Simulate a responder that immediately writes an error to signal.json
	go func() {
		for i := 0; i < 100; i++ {
			data, err := os.ReadFile(signalPath)
			if err == nil {
				var sig SignalFile
				if json.Unmarshal(data, &sig) == nil && sig.Status == "waiting" {
					sig.Status = "error"
					sig.Error = "responder crashed: out of memory"
					out, _ := json.MarshalIndent(sig, "", "  ")
					_ = os.WriteFile(signalPath, out, 0644)
					return
				}
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	start := time.Now()
	_, err := d.Dispatch(context.Background(), ctx)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error from responder, got nil")
	}
	if elapsed > 2*time.Second {
		t.Errorf("expected fail-fast but took %v (timeout is 5s)", elapsed)
	}
	if !contains(err.Error(), "responder error") {
		t.Errorf("expected 'responder error' in message, got: %v", err)
	}
}

func TestFileDispatcher_MonotonicDispatchID(t *testing.T) {
	dir := t.TempDir()
	signalPath := filepath.Join(dir, "signal.json")

	cfg := FileDispatcherConfig{
		PollInterval: 10 * time.Millisecond,
		Timeout:      1 * time.Second,
		SignalDir:    dir,
	}
	d := NewFileDispatcher(cfg)

	// Run two dispatches; second should get dispatch_id=2
	for i := int64(1); i <= 2; i++ {
		promptPath := filepath.Join(dir, "prompt.md")
		artifactPath := filepath.Join(dir, "artifact.json")
		_ = os.WriteFile(promptPath, []byte("# Prompt"), 0644)

		ctx := DispatchContext{
			CaseID:       "C1",
			Step:         "F0_RECALL",
			PromptPath:   promptPath,
			ArtifactPath: artifactPath,
		}

		go func(expectedID int64) {
			for j := 0; j < 100; j++ {
				data, err := os.ReadFile(signalPath)
				if err == nil {
					var sig SignalFile
					if json.Unmarshal(data, &sig) == nil && sig.Status == "waiting" && sig.DispatchID == expectedID {
						payload := map[string]any{"iteration": expectedID}
						inner, _ := json.Marshal(payload)
						wrapper := ArtifactWrapper{DispatchID: expectedID, Data: json.RawMessage(inner)}
						out, _ := json.MarshalIndent(wrapper, "", "  ")
						_ = os.WriteFile(artifactPath, out, 0644)
						return
					}
				}
				time.Sleep(10 * time.Millisecond)
			}
		}(i)

		data, err := d.Dispatch(context.Background(), ctx)
		if err != nil {
			t.Fatalf("dispatch #%d: %v", i, err)
		}

		var result map[string]any
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatalf("unmarshal #%d: %v", i, err)
		}

		got := int64(result["iteration"].(float64))
		if got != i {
			t.Errorf("dispatch #%d: expected iteration=%d, got %d", i, i, got)
		}

		if d.CurrentDispatchID() != i {
			t.Errorf("dispatch #%d: CurrentDispatchID()=%d", i, d.CurrentDispatchID())
		}

		d.MarkDone(artifactPath)
	}
}

func TestNewFileDispatcher_DefaultConfig(t *testing.T) {
	d := NewFileDispatcher(FileDispatcherConfig{})
	if d.cfg.PollInterval != 500*time.Millisecond {
		t.Errorf("expected default poll=500ms, got %v", d.cfg.PollInterval)
	}
	if d.cfg.Timeout != 10*time.Minute {
		t.Errorf("expected default timeout=10m, got %v", d.cfg.Timeout)
	}
	if d.cfg.MaxStaleRejects != 10 {
		t.Errorf("expected default MaxStaleRejects=10, got %d", d.cfg.MaxStaleRejects)
	}
}

func TestDispatchContext_Fields(t *testing.T) {
	ctx := DispatchContext{
		CaseID:       "C1",
		Step:         "F0_RECALL",
		PromptPath:   "/tmp/prompt.md",
		ArtifactPath: "/tmp/artifact.json",
	}
	if ctx.CaseID != "C1" || ctx.Step != "F0_RECALL" {
		t.Errorf("unexpected context: %+v", ctx)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsBytes(s, substr))
}

func containsBytes(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
