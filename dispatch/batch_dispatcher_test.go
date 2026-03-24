package dispatch

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dpopsuev/origami/agentport"
)

func TestBatchFileDispatcher_AllComplete(t *testing.T) {
	dir := t.TempDir()
	suiteDir := filepath.Join(dir, "suite")
	os.MkdirAll(suiteDir, 0755)

	cfg := BatchFileDispatcherConfig{
		FileConfig: FileDispatcherConfig{
			PollInterval: 50 * time.Millisecond,
			Timeout:      5 * time.Second,
		},
		SuiteDir:  suiteDir,
		BatchSize: 4,
	}
	bfd := NewBatchFileDispatcher(cfg)

	// Create 2 cases with pre-written artifacts
	cases := make([]agentport.Context, 2)
	for i := 0; i < 2; i++ {
		caseDir := filepath.Join(dir, "cases", caseID(i))
		os.MkdirAll(caseDir, 0755)
		artifactPath := filepath.Join(caseDir, "recall-result.json")

		cases[i] = agentport.Context{
			CaseID:       caseID(i),
			Step:         "F0_RECALL",
			PromptPath:   filepath.Join(caseDir, "prompt.md"),
			ArtifactPath: artifactPath,
		}
		// Write prompt
		os.WriteFile(cases[i].PromptPath, []byte("test prompt"), 0644)
	}

	// Start a goroutine that writes artifacts after a short delay
	// (simulating what an external agent would do)
	go func() {
		time.Sleep(200 * time.Millisecond)
		for i, ctx := range cases {
			// Read the signal to get the dispatch_id
			sigDir := filepath.Dir(ctx.ArtifactPath)
			sigPath := filepath.Join(sigDir, "signal.json")
			sigData, err := os.ReadFile(sigPath)
			if err != nil {
				t.Logf("case %d: signal read error (may not be ready): %v", i, err)
				// Try reading again after a bit
				time.Sleep(100 * time.Millisecond)
				sigData, err = os.ReadFile(sigPath)
				if err != nil {
					t.Logf("case %d: signal read error (retry): %v", i, err)
					continue
				}
			}

			var sig SignalFile
			if err := json.Unmarshal(sigData, &sig); err != nil {
				t.Logf("case %d: signal parse error: %v", i, err)
				continue
			}

			// Write artifact with matching dispatch_id
			wrapper := ArtifactWrapper{
				DispatchID: sig.DispatchID,
				Data:       json.RawMessage(`{"match":true,"confidence":0.95}`),
			}
			data, _ := json.Marshal(wrapper)
			os.WriteFile(ctx.ArtifactPath, data, 0644)
		}
	}()

	results, errs := bfd.DispatchBatch(context.Background(), cases, "triage", "")

	for i, err := range errs {
		if err != nil {
			t.Errorf("case %d error: %v", i, err)
		}
	}
	for i, data := range results {
		if len(data) == 0 {
			t.Errorf("case %d: empty result", i)
		}
	}

	// Verify manifest was written
	manifestPath := filepath.Join(suiteDir, "batch-manifest.json")
	m, err := ReadManifest(manifestPath)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	if m.BatchID != 1 {
		t.Errorf("batch_id: got %d, want 1", m.BatchID)
	}
	if m.Status != "done" {
		t.Errorf("status: got %q, want done", m.Status)
	}
}

func TestBatchFileDispatcher_PartialFailure(t *testing.T) {
	dir := t.TempDir()
	suiteDir := filepath.Join(dir, "suite")
	os.MkdirAll(suiteDir, 0755)

	cfg := BatchFileDispatcherConfig{
		FileConfig: FileDispatcherConfig{
			PollInterval: 50 * time.Millisecond,
			Timeout:      1 * time.Second, // short timeout for failure case
		},
		SuiteDir:  suiteDir,
		BatchSize: 4,
	}
	bfd := NewBatchFileDispatcher(cfg)

	// Case 0: will succeed, Case 1: will timeout (no artifact written)
	cases := make([]agentport.Context, 2)
	for i := 0; i < 2; i++ {
		caseDir := filepath.Join(dir, "cases", caseID(i))
		os.MkdirAll(caseDir, 0755)
		cases[i] = agentport.Context{
			CaseID:       caseID(i),
			Step:         "F0_RECALL",
			PromptPath:   filepath.Join(caseDir, "prompt.md"),
			ArtifactPath: filepath.Join(caseDir, "recall-result.json"),
		}
		os.WriteFile(cases[i].PromptPath, []byte("test"), 0644)
	}

	// Write artifact for case 0 only
	go func() {
		time.Sleep(200 * time.Millisecond)
		sigDir := filepath.Dir(cases[0].ArtifactPath)
		sigPath := filepath.Join(sigDir, "signal.json")
		for i := 0; i < 20; i++ {
			data, err := os.ReadFile(sigPath)
			if err == nil {
				var sig SignalFile
				if json.Unmarshal(data, &sig) == nil && sig.Status == "waiting" {
					wrapper := ArtifactWrapper{
						DispatchID: sig.DispatchID,
						Data:       json.RawMessage(`{"match":true}`),
					}
					d, _ := json.Marshal(wrapper)
					os.WriteFile(cases[0].ArtifactPath, d, 0644)
					return
				}
			}
			time.Sleep(50 * time.Millisecond)
		}
	}()

	_, errs := bfd.DispatchBatch(context.Background(), cases, "triage", "")

	if errs[0] != nil {
		t.Errorf("case 0 should succeed, got: %v", errs[0])
	}
	if errs[1] == nil {
		t.Error("case 1 should fail with timeout")
	}

	// Manifest should still be written
	m, err := ReadManifest(filepath.Join(suiteDir, "batch-manifest.json"))
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	// Partial success -> status "done"
	if m.Status != "done" {
		t.Errorf("manifest status: got %q, want done", m.Status)
	}
}

func TestBatchFileDispatcher_ManifestLifecycle(t *testing.T) {
	dir := t.TempDir()
	suiteDir := filepath.Join(dir, "suite")

	cfg := BatchFileDispatcherConfig{
		FileConfig: FileDispatcherConfig{
			PollInterval: 50 * time.Millisecond,
			Timeout:      2 * time.Second,
		},
		SuiteDir:  suiteDir,
		BatchSize: 4,
	}
	bfd := NewBatchFileDispatcher(cfg)

	caseDir := filepath.Join(dir, "cases", "C1")
	os.MkdirAll(caseDir, 0755)
	os.WriteFile(filepath.Join(caseDir, "prompt.md"), []byte("test"), 0644)

	ctx := agentport.Context{
		CaseID:       "C1",
		Step:         "F0_RECALL",
		PromptPath:   filepath.Join(caseDir, "prompt.md"),
		ArtifactPath: filepath.Join(caseDir, "recall-result.json"),
	}

	// Race detector: dispatch and read manifest concurrently
	var ops int64
	go func() {
		time.Sleep(100 * time.Millisecond)
		// Read manifest while dispatch is running
		for i := 0; i < 10; i++ {
			mp := filepath.Join(suiteDir, "batch-manifest.json")
			if _, err := ReadManifest(mp); err == nil {
				atomic.AddInt64(&ops, 1)
			}
			time.Sleep(50 * time.Millisecond)
		}
	}()

	// Write artifact for the case
	go func() {
		time.Sleep(300 * time.Millisecond)
		sigPath := filepath.Join(caseDir, "signal.json")
		for i := 0; i < 30; i++ {
			data, err := os.ReadFile(sigPath)
			if err == nil {
				var sig SignalFile
				if json.Unmarshal(data, &sig) == nil && sig.Status == "waiting" {
					wrapper := ArtifactWrapper{
						DispatchID: sig.DispatchID,
						Data:       json.RawMessage(`{"match":true}`),
					}
					d, _ := json.Marshal(wrapper)
					os.WriteFile(ctx.ArtifactPath, d, 0644)
					return
				}
			}
			time.Sleep(50 * time.Millisecond)
		}
	}()

	_, errs := bfd.DispatchBatch(context.Background(), []agentport.Context{ctx}, "triage", "")
	if errs[0] != nil {
		t.Errorf("dispatch error: %v", errs[0])
	}
	if atomic.LoadInt64(&ops) == 0 {
		t.Log("warning: concurrent manifest reads didn't complete (timing)")
	}

	// Verify batch ID increments
	if bfd.LastBatchID() != 1 {
		t.Errorf("batch ID: got %d, want 1", bfd.LastBatchID())
	}
}

func TestBatchFileDispatcher_EmptyBatch(t *testing.T) {
	bfd := NewBatchFileDispatcher(BatchFileDispatcherConfig{
		SuiteDir: t.TempDir(),
	})
	data, errs := bfd.DispatchBatch(context.Background(), nil, "triage", "")
	if data != nil || errs != nil {
		t.Errorf("empty batch should return nil, nil; got %v, %v", data, errs)
	}
}

func TestBatchFileDispatcher_SingleDispatchInterface(t *testing.T) {
	dir := t.TempDir()
	suiteDir := filepath.Join(dir, "suite")

	cfg := BatchFileDispatcherConfig{
		FileConfig: FileDispatcherConfig{
			PollInterval: 50 * time.Millisecond,
			Timeout:      2 * time.Second,
		},
		SuiteDir: suiteDir,
	}
	bfd := NewBatchFileDispatcher(cfg)

	caseDir := filepath.Join(dir, "cases", "C1")
	os.MkdirAll(caseDir, 0755)
	os.WriteFile(filepath.Join(caseDir, "prompt.md"), []byte("test"), 0644)

	ctx := agentport.Context{
		CaseID:       "C1",
		Step:         "F0_RECALL",
		PromptPath:   filepath.Join(caseDir, "prompt.md"),
		ArtifactPath: filepath.Join(caseDir, "recall-result.json"),
	}

	// Write artifact asynchronously
	go func() {
		time.Sleep(200 * time.Millisecond)
		sigPath := filepath.Join(caseDir, "signal.json")
		for i := 0; i < 30; i++ {
			data, err := os.ReadFile(sigPath)
			if err == nil {
				var sig SignalFile
				if json.Unmarshal(data, &sig) == nil && sig.Status == "waiting" {
					wrapper := ArtifactWrapper{
						DispatchID: sig.DispatchID,
						Data:       json.RawMessage(`{"ok":true}`),
					}
					d, _ := json.Marshal(wrapper)
					os.WriteFile(ctx.ArtifactPath, d, 0644)
					return
				}
			}
			time.Sleep(50 * time.Millisecond)
		}
	}()

	// Use the Dispatcher interface
	var d agentport.Dispatcher = bfd
	data, err := d.Dispatch(context.Background(), ctx)
	if err != nil {
		t.Fatalf("Dispatch error: %v", err)
	}
	if len(data) == 0 {
		t.Error("Dispatch returned empty data")
	}
}

func TestBatchFileDispatcher_WriteBriefing(t *testing.T) {
	dir := t.TempDir()
	bfd := NewBatchFileDispatcher(BatchFileDispatcherConfig{
		SuiteDir: dir,
	})

	path, err := bfd.WriteBriefing("# Test Briefing\n\nSome context.")
	if err != nil {
		t.Fatalf("WriteBriefing: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read briefing: %v", err)
	}
	if string(data) != "# Test Briefing\n\nSome context." {
		t.Errorf("briefing content mismatch: got %q", string(data))
	}
}

func caseID(i int) string {
	return "C" + string(rune('1'+i))
}
