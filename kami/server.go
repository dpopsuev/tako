package kami

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/ouroboros"
	"github.com/dpopsuev/origami/ouroboros/review"
	"github.com/dpopsuev/origami/view"
)

// ReviewConfig configures the ouroboros transcript review subsystem.
type ReviewConfig struct {
	Dir string // directory for transcript storage (empty = disabled)
}

// Config controls the KamiServer behavior.
type Config struct {
	Port         int
	Bind         string // default "127.0.0.1"
	Debug        bool   // enable debug API endpoints
	Logger       *slog.Logger
	Bridge       *EventBridge
	Store        *view.CircuitStore         // circuit state source for SSE streaming
	SPA          http.FileSystem            // embedded frontend (nil = no SPA)
	Theme        Theme                      // consumer theme (nil = default)
	Kabuki       KabukiConfig               // Kabuki presentation sections (nil = debugger-only mode)
	Vocab        circuit.RichVocabulary   // rich vocabulary for node tooltips (nil = Theme only)
	MetricsHandler http.Handler             // Prometheus /metrics handler (nil = no metrics)
	Review       ReviewConfig               // ouroboros review subsystem (empty Dir = disabled)
}

func (c *Config) addr() string {
	bind := c.Bind
	if bind == "" {
		bind = "127.0.0.1"
	}
	return fmt.Sprintf("%s:%d", bind, c.Port)
}

func (c *Config) wsAddr() string {
	bind := c.Bind
	if bind == "" {
		bind = "127.0.0.1"
	}
	return fmt.Sprintf("%s:%d", bind, c.Port+1)
}

func (c *Config) logger() *slog.Logger {
	if c.Logger != nil {
		return c.Logger
	}
	return slog.Default()
}

// Server is the triple-homed Kami debugger process.
// It runs HTTP (SSE + SPA + browser events) and WS (AI commands to browser)
// on adjacent ports.
type Server struct {
	cfg    Config
	http   *http.Server
	ws     *http.Server
	bridge *EventBridge
	store  *view.CircuitStore
	log    *slog.Logger

	mu      sync.Mutex
	wsConns map[int]*wsConn
	nextWS  int

	selMu     sync.RWMutex
	selection map[string]any

	frameStore  *FrameStore
	reviewStore *review.TranscriptStore
}

// NewServer creates a KamiServer. Call Start to begin serving.
func NewServer(cfg Config) *Server {
	s := &Server{
		cfg:        cfg,
		bridge:     cfg.Bridge,
		store:      cfg.Store,
		log:        cfg.logger(),
		wsConns:    make(map[int]*wsConn),
		frameStore: NewFrameStore(),
	}

	if cfg.Review.Dir != "" {
		rs, err := review.NewTranscriptStore(cfg.Review.Dir)
		if err != nil {
			cfg.logger().Warn("review store disabled", "err", err)
		} else {
			s.reviewStore = rs
		}
	}

	return s
}

// ReviewStore returns the review transcript store, or nil if not configured.
func (s *Server) ReviewStore() *review.TranscriptStore {
	return s.reviewStore
}

// Start begins serving HTTP and WS on the configured ports.
// Both ports are bound synchronously before any goroutine launches,
// so a port conflict is returned immediately. Blocks until ctx is
// cancelled or a serve error occurs.
func (s *Server) Start(ctx context.Context) error {
	mux := s.buildHTTPMux()

	httpLn, err := net.Listen("tcp", s.cfg.addr())
	if err != nil {
		return fmt.Errorf("HTTP listen %s: %w", s.cfg.addr(), err)
	}

	wsMux := http.NewServeMux()
	wsMux.HandleFunc("/", s.handleWS)

	wsLn, err := net.Listen("tcp", s.cfg.wsAddr())
	if err != nil {
		httpLn.Close()
		return fmt.Errorf("WS listen %s: %w", s.cfg.wsAddr(), err)
	}

	s.http = &http.Server{Handler: mux}
	s.ws = &http.Server{Handler: wsMux}

	errCh := make(chan error, 2)

	go func() {
		s.log.Info("kami HTTP server started", "addr", httpLn.Addr())
		if err := s.http.Serve(httpLn); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("HTTP: %w", err)
		}
	}()

	go func() {
		s.log.Info("kami WS server started", "addr", wsLn.Addr())
		if err := s.ws.Serve(wsLn); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("WS: %w", err)
		}
	}()

	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.http.Shutdown(shutCtx)
		s.ws.Shutdown(shutCtx)
		return ctx.Err()
	case err := <-errCh:
		shutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		s.http.Shutdown(shutCtx)
		s.ws.Shutdown(shutCtx)
		return err
	}
}

func (s *Server) buildHTTPMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /events/stream", s.handleSSE)
	mux.HandleFunc("POST /events/click", s.handleBrowserEvent("click"))
	mux.HandleFunc("POST /events/hover", s.handleBrowserEvent("hover"))
	mux.HandleFunc("POST /events/selection", s.handleBrowserEvent("selection"))
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/snapshot", s.handleSnapshotAPI)
	mux.HandleFunc("GET /api/theme", s.handleThemeAPI)
	mux.HandleFunc("GET /api/circuit", s.handleCircuitAPI)
	mux.HandleFunc("GET /api/kabuki", s.handleKabukiAPI)
	mux.HandleFunc("POST /api/sumi/frame", s.handleStoreFrame)
	mux.HandleFunc("GET /api/sumi/frame", s.handleGetFrame)
	mux.HandleFunc("POST /api/store/reset", s.handleStoreReset)

	if s.reviewStore != nil {
		mux.HandleFunc("GET /api/review", s.handleReviewList)
		mux.HandleFunc("GET /api/review/{id}", s.handleReviewGet)
		mux.HandleFunc("POST /api/review/{id}/score", s.handleReviewScore)
	}

	if s.cfg.MetricsHandler != nil {
		mux.Handle("GET /metrics", s.cfg.MetricsHandler)
	}

	if s.cfg.SPA != nil {
		mux.Handle("GET /", http.FileServer(s.cfg.SPA))
	} else {
		mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/" {
				http.NotFound(w, r)
				return
			}
			fmt.Fprintf(w, "Kami debugger running. Frontend not embedded.")
		})
	}
	return mux
}

// SetStore replaces the active CircuitStore. The old store (if any) is
// closed, which terminates all SSE connections subscribed to it. SSE
// clients are expected to reconnect and will pick up the new store.
// Thread-safe; may be called while SSE handlers are active.
func (s *Server) SetStore(store *view.CircuitStore) {
	s.mu.Lock()
	old := s.store
	s.store = store
	s.mu.Unlock()

	if old != nil {
		old.Close()
	}
}

// ResetStore clears the active CircuitStore. All node states, walkers,
// and completion status are removed. SSE clients receive a DiffReset.
// Safe to call when no store is set.
func (s *Server) ResetStore() {
	s.mu.Lock()
	store := s.store
	s.mu.Unlock()
	if store != nil {
		store.Reset(nil)
	}
}

// handleSSE streams circuit state diffs as Server-Sent Events.
// The store reference is captured at handler entry so that subscribe
// and unsubscribe always operate on the same object, even if SetStore
// swaps the server's store mid-stream.
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	store := s.store
	s.mu.Unlock()

	if store == nil {
		http.Error(w, "CircuitStore not configured", http.StatusServiceUnavailable)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	flusher.Flush()

	id, ch := store.Subscribe()
	defer store.Unsubscribe(id)

	for {
		select {
		case <-r.Context().Done():
			return
		case diff, ok := <-ch:
			if !ok {
				return
			}
			evt := diffToEvent(diff)
			data, err := json.Marshal(evt)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

// diffToEvent converts a view.StateDiff into a kami.Event for SSE output.
func diffToEvent(d view.StateDiff) Event {
	e := Event{
		Timestamp: d.Timestamp,
		Node:      d.Node,
		Agent:     d.Walker,
	}
	switch d.Type {
	case view.DiffNodeState:
		switch d.State {
		case view.NodeActive:
			e.Type = EventNodeEnter
		case view.NodeCompleted:
			e.Type = EventNodeExit
		case view.NodeError:
			e.Type = EventWalkError
			e.Error = d.Error
		default:
			e.Type = EventType(string(d.Type))
		}
	case view.DiffWalkerMoved:
		e.Type = EventTransition
	case view.DiffWalkerAdded:
		e.Type = EventFanOutStart
	case view.DiffWalkerRemoved:
		e.Type = EventFanOutEnd
	case view.DiffCompleted:
		e.Type = EventWalkComplete
	case view.DiffError:
		e.Type = EventWalkError
		e.Error = d.Error
	case view.DiffPaused:
		e.Type = EventPaused
	case view.DiffResumed:
		e.Type = EventResumed
	case view.DiffBreakpointSet, view.DiffBreakpointCleared:
		e.Type = EventType(string(d.Type))
	default:
		e.Type = EventType(string(d.Type))
	}
	return e
}

// handleBrowserEvent receives browser interaction events and emits them
// to the bridge so MCP tools can observe user interaction.
// Selection events are additionally stored for retrieval via GetSelection.
func (s *Server) handleBrowserEvent(eventType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if eventType == "selection" {
			s.SetSelection(payload)
		}
		s.bridge.Emit(Event{
			Type: EventType("browser_" + eventType),
			Data: payload,
		})
		w.WriteHeader(http.StatusNoContent)
	}
}

// GetSelection returns the most recent browser selection payload, or nil.
func (s *Server) GetSelection() map[string]any {
	s.selMu.RLock()
	defer s.selMu.RUnlock()
	return s.selection
}

// SetSelection stores a browser selection payload for MCP tool retrieval.
func (s *Server) SetSelection(sel map[string]any) {
	s.selMu.Lock()
	defer s.selMu.Unlock()
	s.selection = sel
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleSnapshotAPI returns the current CircuitSnapshot as JSON.
// Sumi uses this to bootstrap its local CircuitStore with the correct
// circuit definition and current state before streaming SSE diffs.
func (s *Server) handleSnapshotAPI(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	store := s.store
	s.mu.Unlock()

	if store == nil {
		http.Error(w, "CircuitStore not configured", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(store.Snapshot())
}

// handleStoreReset clears the server's CircuitStore, removing all node
// states, walkers, and completion status. SSE clients will receive a
// DiffReset and can re-bootstrap. Safe to call when no store is set.
func (s *Server) handleStoreReset(w http.ResponseWriter, _ *http.Request) {
	s.ResetStore()
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"ok":"store reset"}`)
}

// ListenAddr returns the HTTP listener address after the server starts.
// Useful for tests that use port 0.
func (s *Server) ListenAddr() string {
	return s.cfg.addr()
}

// StartOnAvailablePort starts on OS-assigned ports and returns them.
// This is primarily for testing.
func (s *Server) StartOnAvailablePort(ctx context.Context) (httpAddr, wsAddr string, err error) {
	mux := s.buildHTTPMux()

	httpLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", "", fmt.Errorf("HTTP listen: %w", err)
	}

	wsMux := http.NewServeMux()
	wsMux.HandleFunc("/", s.handleWS)
	wsLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		httpLn.Close()
		return "", "", fmt.Errorf("WS listen: %w", err)
	}

	s.http = &http.Server{Handler: mux}
	s.ws = &http.Server{Handler: wsMux}

	go s.http.Serve(httpLn)
	go s.ws.Serve(wsLn)

	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		s.http.Shutdown(shutCtx)
		s.ws.Shutdown(shutCtx)
	}()

	return httpLn.Addr().String(), wsLn.Addr().String(), nil
}

// FrameStore returns the server's frame store for MCP tool registration.
func (s *Server) FrameStore() *FrameStore {
	return s.frameStore
}

func (s *Server) handleStoreFrame(w http.ResponseWriter, r *http.Request) {
	var f view.RecordedFrame
	if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
		http.Error(w, "invalid frame JSON", http.StatusBadRequest)
		return
	}
	s.frameStore.Store(f)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleGetFrame(w http.ResponseWriter, r *http.Request) {
	f := s.frameStore.Latest()
	if f == nil {
		http.Error(w, "no frame available", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(f)
}

// --- Review handlers ---

var reviewIDPattern = review.ValidIDPattern()

func (s *Server) handleReviewList(w http.ResponseWriter, _ *http.Request) {
	grouped, err := s.reviewStore.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(grouped)
}

func (s *Server) handleReviewGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !reviewIDPattern.MatchString(id) {
		http.Error(w, "invalid transcript ID", http.StatusBadRequest)
		return
	}

	t, err := s.reviewStore.Get(id)
	if err != nil {
		http.Error(w, "transcript not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(t)
}

func (s *Server) handleReviewScore(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !reviewIDPattern.MatchString(id) {
		http.Error(w, "invalid transcript ID", http.StatusBadRequest)
		return
	}

	var rev ouroboros.HumanReview
	if err := json.NewDecoder(r.Body).Decode(&rev); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.reviewStore.Score(id, &rev); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"ok":"review saved"}`)
}
