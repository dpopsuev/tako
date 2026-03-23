package sumi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/kami"
	"github.com/dpopsuev/origami/view"

	tea "github.com/charmbracelet/bubbletea"
)

func windowSizeMsg(w, h int) tea.WindowSizeMsg {
	return tea.WindowSizeMsg{Width: w, Height: h}
}

func TestIntegration_RecorderCapturesFrameOnDiff(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit: "rec-test",
		Nodes: []circuit.NodeDef{
			{Name: "alpha", Approach: "rapid"},
			{Name: "beta", Approach: "analytical"},
		},
		Edges: []circuit.EdgeDef{{From: "alpha", To: "beta"}},
		Start: "alpha",
		Done:  "beta",
	}

	store := view.NewCircuitStore(def)
	defer store.Close()

	engine := &view.GridLayout{}
	layout, _ := engine.Layout(def)

	recorder := NewViewRecorder(10)

	m := New(Config{
		Def:      def,
		Store:    store,
		Layout:   layout,
		Opts:     RenderOpts{},
		Recorder: recorder,
	})

	if recorder.Len() != 0 {
		t.Fatalf("expected 0 frames before any events, got %d", recorder.Len())
	}

	// WindowSizeMsg makes model ready and records a frame.
	updated, _ := m.Update(windowSizeMsg(120, 40))
	m = updated.(Model)

	if recorder.Len() != 1 {
		t.Fatalf("expected 1 frame after WindowSizeMsg, got %d", recorder.Len())
	}

	f := recorder.Latest()
	if f.Width != 120 || f.Height != 40 {
		t.Fatalf("expected 120x40, got %dx%d", f.Width, f.Height)
	}
	if f.ViewText == "" {
		t.Fatal("expected non-empty ViewText")
	}

	// DiffMsg records another frame.
	updated, _ = m.Update(DiffMsg(view.StateDiff{Type: view.DiffNodeState, Node: "alpha"}))
	_ = updated

	if recorder.Len() != 2 {
		t.Fatalf("expected 2 frames after DiffMsg, got %d", recorder.Len())
	}

	f2 := recorder.Latest()
	if !strings.Contains(f2.ViewText, "alpha") {
		t.Fatalf("expected ViewText to mention 'alpha', got: %s", f2.ViewText[:min(100, len(f2.ViewText))])
	}
}

func TestIntegration_RecorderNoColorOutput(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit: "nocolor-test",
		Nodes:   []circuit.NodeDef{{Name: "node1"}},
		Start:   "node1",
		Done:    "node1",
	}

	store := view.NewCircuitStore(def)
	defer store.Close()

	engine := &view.GridLayout{}
	layout, _ := engine.Layout(def)

	recorder := NewViewRecorder(5)
	m := New(Config{
		Def:      def,
		Store:    store,
		Layout:   layout,
		Opts:     RenderOpts{},
		Recorder: recorder,
	})

	updated, _ := m.Update(windowSizeMsg(120, 40))
	_ = updated.(Model)

	f := recorder.Latest()
	if f == nil {
		t.Fatal("expected recorded frame")
	}
	if strings.Contains(f.ViewText, "\x1b[") {
		t.Fatal("ViewText should not contain ANSI escape sequences")
	}
}

func TestIntegration_FramePushToKami(t *testing.T) {
	var received view.RecordedFrame
	var gotRequest bool

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/sumi/frame" && r.Method == "POST" {
			if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			gotRequest = true
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	recorder := NewViewRecorder(5)
	recorder.Record(view.RecordedFrame{
		Timestamp:    time.Now(),
		Width:        120,
		Height:       40,
		LayoutTier:   "full",
		SelectedNode: "triage",
		WorkerCount:  1,
		EventCount:   5,
		ViewText:     "test frame",
	})

	// Manually push a frame like framePushLoop would.
	f := recorder.Latest()
	body, _ := json.Marshal(f)
	addr := strings.TrimPrefix(ts.URL, "http://")
	resp, err := http.Post("http://"+addr+"/api/sumi/frame", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	resp.Body.Close()

	if !gotRequest {
		t.Fatal("expected server to receive frame")
	}
	if received.SelectedNode != "triage" {
		t.Fatalf("expected triage, got %s", received.SelectedNode)
	}
	if received.ViewText != "test frame" {
		t.Fatalf("expected 'test frame', got %q", received.ViewText)
	}
}

func TestFramePushLoop_RepushOnIdle(t *testing.T) {
	var mu sync.Mutex
	var pushCount int

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/sumi/frame" && r.Method == "POST" {
			mu.Lock()
			pushCount++
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	recorder := NewViewRecorder(5)
	recorder.Record(view.RecordedFrame{
		Timestamp:    time.Now(),
		Width:        120,
		Height:       40,
		LayoutTier:   "full",
		SelectedNode: "triage",
		WorkerCount:  1,
		EventCount:   5,
		ViewText:     "idle frame",
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	addr := strings.TrimPrefix(ts.URL, "http://")
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	go framePushLoop(ctx, recorder, addr, log)

	// Wait for first push.
	deadline := time.After(3 * time.Second)
	for {
		mu.Lock()
		n := pushCount
		mu.Unlock()
		if n >= 1 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timeout waiting for first push")
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	// No new frames recorded (Sumi is idle). Wait for re-push.
	// With the fix, framePushLoop should re-push even when timestamp
	// hasn't changed, so pushCount should reach at least 3 within ~2s.
	time.Sleep(2 * time.Second)

	mu.Lock()
	final := pushCount
	mu.Unlock()

	if final < 3 {
		t.Fatalf("expected at least 3 pushes during idle (got %d) — framePushLoop is not re-pushing stale frames", final)
	}
	t.Logf("idle re-push: %d pushes in ~2s (interval=%v)", final, framePushInterval)
}

func TestFramePushLoop_NoPushWhenEmpty(t *testing.T) {
	var mu sync.Mutex
	var pushCount int

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/sumi/frame" && r.Method == "POST" {
			mu.Lock()
			pushCount++
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	recorder := NewViewRecorder(5)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	addr := strings.TrimPrefix(ts.URL, "http://")
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	go framePushLoop(ctx, recorder, addr, log)

	time.Sleep(1500 * time.Millisecond)

	mu.Lock()
	final := pushCount
	mu.Unlock()

	if final != 0 {
		t.Fatalf("expected 0 pushes when recorder is empty, got %d", final)
	}
}

// --- Pipeline tests (T1-T4) ---
// These test the full frame pipeline: Model.Update -> ViewRecorder -> framePushLoop -> Kami

func newPipelineModel(t *testing.T, def *circuit.CircuitDef, recorder *ViewRecorder) Model {
	t.Helper()
	store := view.NewCircuitStore(def)
	t.Cleanup(store.Close)
	engine := &view.GridLayout{}
	layout, _ := engine.Layout(def)
	return New(Config{
		Def:      def,
		Store:    store,
		Layout:   layout,
		Opts:     RenderOpts{},
		Recorder: recorder,
	})
}

func pipelineDef() *circuit.CircuitDef {
	return &circuit.CircuitDef{
		Circuit: "pipeline-test",
		Nodes: []circuit.NodeDef{
			{Name: "recall", Approach: "rapid"},
			{Name: "triage", Approach: "analytical"},
		},
		Edges: []circuit.EdgeDef{{From: "recall", To: "triage"}},
		Start: "recall",
		Done:  "triage",
	}
}

func discardLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestPipeline_ModelUpdateThroughPushLoop(t *testing.T) {
	var mu sync.Mutex
	var received view.RecordedFrame
	var gotFrame bool

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/sumi/frame" && r.Method == "POST" {
			mu.Lock()
			json.NewDecoder(r.Body).Decode(&received)
			gotFrame = true
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	recorder := NewViewRecorder(10)
	m := newPipelineModel(t, pipelineDef(), recorder)

	updated, _ := m.Update(windowSizeMsg(120, 40))
	_ = updated.(Model)

	if recorder.Len() != 1 {
		t.Fatalf("expected 1 frame after WindowSizeMsg, got %d", recorder.Len())
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	addr := strings.TrimPrefix(ts.URL, "http://")
	go framePushLoop(ctx, recorder, addr, discardLog())

	deadline := time.After(3 * time.Second)
	for {
		mu.Lock()
		ok := gotFrame
		mu.Unlock()
		if ok {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timeout: framePushLoop never delivered the frame recorded by Model.Update")
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if received.Width != 120 || received.Height != 40 {
		t.Fatalf("expected 120x40, got %dx%d", received.Width, received.Height)
	}
	if received.ViewText == "" {
		t.Fatal("expected non-empty ViewText")
	}
	t.Logf("T1 pass: frame %dx%d, ViewText=%d bytes", received.Width, received.Height, len(received.ViewText))
}

func TestPipeline_WithRealKamiServer(t *testing.T) {
	bridge := kami.NewEventBridge(nil)
	kamiSrv := kami.NewServer(kami.Config{Bridge: bridge})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	httpAddr, _, err := kamiSrv.StartOnAvailablePort(ctx)
	if err != nil {
		t.Fatalf("kami start: %v", err)
	}
	t.Cleanup(func() { bridge.Close() })

	recorder := NewViewRecorder(10)
	m := newPipelineModel(t, pipelineDef(), recorder)

	updated, _ := m.Update(windowSizeMsg(140, 45))
	_ = updated.(Model)

	if recorder.Len() < 1 {
		t.Fatal("expected at least 1 frame after WindowSizeMsg")
	}

	go framePushLoop(ctx, recorder, httpAddr, discardLog())

	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.After(3 * time.Second)
	for {
		resp, err := client.Get("http://" + httpAddr + "/api/sumi/frame")
		if err == nil {
			if resp.StatusCode == http.StatusOK {
				var got view.RecordedFrame
				json.NewDecoder(resp.Body).Decode(&got)
				resp.Body.Close()

				if got.Width != 140 || got.Height != 45 {
					t.Fatalf("expected 140x45, got %dx%d", got.Width, got.Height)
				}
				if got.ViewText == "" {
					t.Fatal("expected non-empty ViewText from Kami")
				}
				t.Logf("T2 pass: Kami served frame %dx%d, tier=%s", got.Width, got.Height, got.LayoutTier)
				return
			}
			resp.Body.Close()
		}
		select {
		case <-deadline:
			t.Fatal("timeout: frame never arrived at real Kami server")
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func TestPipeline_KamiFrameStoreReadBack(t *testing.T) {
	bridge := kami.NewEventBridge(nil)
	kamiSrv := kami.NewServer(kami.Config{Bridge: bridge})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	httpAddr, _, err := kamiSrv.StartOnAvailablePort(ctx)
	if err != nil {
		t.Fatalf("kami start: %v", err)
	}
	t.Cleanup(func() { bridge.Close() })

	recorder := NewViewRecorder(10)
	m := newPipelineModel(t, pipelineDef(), recorder)

	updated, _ := m.Update(windowSizeMsg(120, 35))
	_ = updated.(Model)

	go framePushLoop(ctx, recorder, httpAddr, discardLog())

	deadline := time.After(3 * time.Second)
	for {
		f := kamiSrv.FrameStore().Latest()
		if f != nil {
			if f.Width != 120 || f.Height != 35 {
				t.Fatalf("expected 120x35, got %dx%d", f.Width, f.Height)
			}
			if f.ViewText == "" {
				t.Fatal("FrameStore returned frame with empty ViewText")
			}
			if !strings.Contains(f.LayoutTier, "standard") && !strings.Contains(f.LayoutTier, "compact") {
				t.Logf("layout tier: %s (120x35 may be standard or compact)", f.LayoutTier)
			}
			t.Logf("T3 pass: FrameStore.Latest() = %dx%d tier=%s, ViewText=%d bytes",
				f.Width, f.Height, f.LayoutTier, len(f.ViewText))
			return
		}
		select {
		case <-deadline:
			t.Fatal("timeout: FrameStore never received frame from push loop")
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func TestPipeline_EmptyWatchCircuit(t *testing.T) {
	emptyDef := &circuit.CircuitDef{Circuit: "watch"}

	var mu sync.Mutex
	var received view.RecordedFrame
	var gotFrame bool

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/sumi/frame" && r.Method == "POST" {
			mu.Lock()
			json.NewDecoder(r.Body).Decode(&received)
			gotFrame = true
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	recorder := NewViewRecorder(10)
	m := newPipelineModel(t, emptyDef, recorder)

	updated, _ := m.Update(windowSizeMsg(120, 40))
	_ = updated.(Model)

	if recorder.Len() != 1 {
		t.Fatalf("expected 1 frame after WindowSizeMsg on empty circuit, got %d", recorder.Len())
	}

	f := recorder.Latest()
	if f.ViewText == "" {
		t.Fatal("expected non-empty ViewText even on empty circuit")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	addr := strings.TrimPrefix(ts.URL, "http://")
	go framePushLoop(ctx, recorder, addr, discardLog())

	deadline := time.After(3 * time.Second)
	for {
		mu.Lock()
		ok := gotFrame
		mu.Unlock()
		if ok {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timeout: push loop never delivered frame for empty circuit")
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if received.Width != 120 || received.Height != 40 {
		t.Fatalf("expected 120x40, got %dx%d", received.Width, received.Height)
	}
	if received.ViewText == "" {
		t.Fatal("received empty ViewText for empty circuit")
	}
	t.Logf("T4 pass: empty circuit frame delivered, tier=%s, ViewText=%d bytes",
		received.LayoutTier, len(received.ViewText))
}

func TestIntegration_RecorderOnlyOnStateChange(t *testing.T) {
	def := &circuit.CircuitDef{
		Circuit: "dirty-test",
		Nodes:   []circuit.NodeDef{{Name: "a"}},
		Start:   "a",
		Done:    "a",
	}

	store := view.NewCircuitStore(def)
	defer store.Close()

	engine := &view.GridLayout{}
	layout, _ := engine.Layout(def)

	recorder := NewViewRecorder(20)
	m := New(Config{
		Def:      def,
		Store:    store,
		Layout:   layout,
		Opts:     RenderOpts{},
		Recorder: recorder,
	})

	// WindowSizeMsg records one frame.
	updated, _ := m.Update(windowSizeMsg(120, 40))
	m = updated.(Model)
	if recorder.Len() != 1 {
		t.Fatalf("expected 1 frame after WindowSizeMsg, got %d", recorder.Len())
	}

	// Calling View() without further state change does not record.
	m.View()
	m.View()
	if recorder.Len() != 1 {
		t.Fatalf("expected still 1 frame (no state change), got %d", recorder.Len())
	}

	// Another DiffMsg adds a second frame.
	updated, _ = m.Update(DiffMsg(view.StateDiff{Type: view.DiffNodeState, Node: "a"}))
	_ = updated
	if recorder.Len() != 2 {
		t.Fatalf("expected 2 frames after DiffMsg, got %d", recorder.Len())
	}
}

