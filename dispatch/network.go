package dispatch

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/dpopsuev/bugle/resilience"
	"github.com/dpopsuev/origami/agentport"
)

// NetworkServer wraps an agentport.ExternalDispatcher and exposes it over HTTP.
// Agents connect and poll for work via GET /next, then submit results via POST /submit.
// Optionally exposes signal bus endpoints (POST /signal, GET /signals) when a
// SignalBus is provided via WithSignalBus.
type NetworkServer struct {
	dispatcher agentport.ExternalDispatcher
	bus        agentport.Bus
	server     *http.Server
	log        *slog.Logger
	addr       string
	authToken  string
	rateLimit  float64
	mu         sync.Mutex
	started    bool
}

// NetworkServerOption configures a NetworkServer.
type NetworkServerOption func(*NetworkServer)

// WithTLS configures TLS for the network server.
func WithTLS(cfg *tls.Config) NetworkServerOption {
	return func(s *NetworkServer) {
		s.server.TLSConfig = cfg
	}
}

// WithServerLogger sets the logger for the network server.
func WithServerLogger(l *slog.Logger) NetworkServerOption {
	return func(s *NetworkServer) { s.log = l }
}

// WithSignalBus enables signal bus endpoints (POST /signal, GET /signals).
// When nil, signal endpoints return 404.
func WithSignalBus(bus agentport.Bus) NetworkServerOption {
	return func(s *NetworkServer) { s.bus = bus }
}

// WithAuthToken sets a bearer token required for all requests.
// When empty (default), no authentication is enforced.
func WithAuthToken(token string) NetworkServerOption {
	return func(s *NetworkServer) { s.authToken = token }
}

// WithRateLimit sets the maximum requests per second for all endpoints.
// Zero (default) disables rate limiting.
func WithRateLimit(rps float64) NetworkServerOption {
	return func(s *NetworkServer) { s.rateLimit = rps }
}

// NewNetworkServer creates an HTTP server that exposes an agentport.ExternalDispatcher.
func NewNetworkServer(dispatcher agentport.ExternalDispatcher, addr string, opts ...NetworkServerOption) *NetworkServer {
	s := &NetworkServer{
		dispatcher: dispatcher,
		log:        discardLogger(),
		addr:       addr,
		server:     &http.Server{Addr: addr},
	}
	for _, opt := range opts {
		opt(s)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /next", s.handleGetNext)
	mux.HandleFunc("POST /submit", s.handleSubmit)
	mux.HandleFunc("POST /signal", s.handleEmitSignal)
	mux.HandleFunc("GET /signals", s.handleGetSignals)

	var handler http.Handler = mux
	if s.rateLimit > 0 {
		rl := resilience.NewRateLimiter(resilience.RateLimitConfig{
			Rate:  s.rateLimit,
			Burst: max(int(s.rateLimit), 10),
		})
		handler = rateLimitMiddleware(rl, handler)
	}
	if s.authToken != "" {
		handler = s.authMiddleware(handler)
	}
	s.server.Handler = handler

	return s
}

// Serve starts the HTTP server and blocks until the context is cancelled or
// the server encounters a fatal error.
func (s *NetworkServer) Serve(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("network server listen: %w", err)
	}

	s.mu.Lock()
	s.addr = ln.Addr().String()
	s.started = true
	s.mu.Unlock()

	s.log.Info("network server started", slog.String("addr", s.addr))

	go func() {
		<-ctx.Done()
		s.server.Close()
	}()

	if s.server.TLSConfig != nil {
		tlsLn := tls.NewListener(ln, s.server.TLSConfig)
		err = s.server.Serve(tlsLn)
	} else {
		err = s.server.Serve(ln)
	}

	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Addr returns the address the server is listening on.
// Only valid after Serve has been called.
func (s *NetworkServer) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.addr
}

type nextResponse struct {
	DispatchID   int64  `json:"dispatch_id"`
	CaseID       string `json:"case_id"`
	Step         string `json:"step"`
	PromptPath   string `json:"prompt_path"`
	ArtifactPath string `json:"artifact_path"`
}

type submitRequest struct {
	DispatchID int64  `json:"dispatch_id"`
	Data       []byte `json:"data"`
}

func (s *NetworkServer) handleGetNext(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	hints := agentport.PullHints{
		PreferredCaseID: q.Get("preferred_case_id"),
		PreferredZone:   q.Get("preferred_zone"),
	}
	if v := q.Get("stickiness"); v != "" {
		_, _ = fmt.Sscanf(v, "%d", &hints.Stickiness)
	}
	if v := q.Get("consecutive_misses"); v != "" {
		_, _ = fmt.Sscanf(v, "%d", &hints.ConsecutiveMisses)
	}

	dc, err := s.dispatcher.GetNextStepWithHints(r.Context(), hints)
	if err != nil {
		s.log.Error("get next step failed", "error", err)
		http.Error(w, "dispatch unavailable", http.StatusServiceUnavailable)
		return
	}

	resp := nextResponse{
		DispatchID:   dc.DispatchID,
		CaseID:       dc.CaseID,
		Step:         dc.Step,
		PromptPath:   dc.PromptPath,
		ArtifactPath: dc.ArtifactPath,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *NetworkServer) handleSubmit(w http.ResponseWriter, r *http.Request) {
	var req submitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := s.dispatcher.SubmitArtifact(r.Context(), req.DispatchID, req.Data); err != nil {
		s.log.Error("submit artifact failed", "dispatch_id", req.DispatchID, "error", err)
		http.Error(w, "submit failed", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type emitSignalRequest struct {
	Event  string            `json:"event"`
	Agent  string            `json:"agent"`
	CaseID string            `json:"case_id,omitempty"`
	Step   string            `json:"step,omitempty"`
	Meta   map[string]string `json:"meta,omitempty"`
}

func (s *NetworkServer) handleEmitSignal(w http.ResponseWriter, r *http.Request) {
	if s.bus == nil {
		http.Error(w, "signal bus not configured", http.StatusNotFound)
		return
	}

	var req emitSignalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Event == "" {
		http.Error(w, "event is required", http.StatusBadRequest)
		return
	}

	s.bus.Emit(&agentport.Signal{
		Event:  req.Event,
		Agent:  req.Agent,
		CaseID: req.CaseID,
		Step:   req.Step,
		Meta:   req.Meta,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *NetworkServer) handleGetSignals(w http.ResponseWriter, r *http.Request) {
	if s.bus == nil {
		http.Error(w, "signal bus not configured", http.StatusNotFound)
		return
	}

	since := 0
	if v := r.URL.Query().Get("since"); v != "" {
		_, _ = fmt.Sscanf(v, "%d", &since)
	}

	sigs := s.bus.Since(since)
	if sigs == nil {
		sigs = []agentport.Signal{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(sigs)
}

func rateLimitMiddleware(rl *resilience.RateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !rl.Allow() {
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *NetworkServer) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer "+s.authToken {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// NetworkClient implements agentport.ExternalDispatcher by calling a remote NetworkServer
// over HTTP. This is the agent-side counterpart to NetworkServer.
type NetworkClient struct {
	baseURL string
	client  *http.Client
	log     *slog.Logger
}

// NetworkClientOption configures a NetworkClient.
type NetworkClientOption func(*NetworkClient)

// WithNetworkHTTPClient sets a custom HTTP client (for auth middleware, TLS, timeouts).
func WithNetworkHTTPClient(c *http.Client) NetworkClientOption {
	return func(nc *NetworkClient) { nc.client = c }
}

// WithClientLogger sets the logger for the network client.
func WithClientLogger(l *slog.Logger) NetworkClientOption {
	return func(nc *NetworkClient) { nc.log = l }
}

// NewNetworkClient creates an agentport.ExternalDispatcher that connects to a remote
// NetworkServer. The baseURL should be like "http://localhost:8080".
func NewNetworkClient(baseURL string, opts ...NetworkClientOption) *NetworkClient {
	nc := &NetworkClient{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 5 * time.Minute},
		log:     discardLogger(),
	}
	for _, opt := range opts {
		opt(nc)
	}
	return nc
}

// GetNextStep polls the server for the next dispatch context (no hints).
func (c *NetworkClient) GetNextStep(ctx context.Context) (agentport.Context, error) {
	return c.GetNextStepWithHints(ctx, agentport.PullHints{})
}

// GetNextStepWithHints polls the server for the next dispatch context,
// passing pull hints as query parameters for server-side matching.
func (c *NetworkClient) GetNextStepWithHints(ctx context.Context, hints agentport.PullHints) (agentport.Context, error) {
	url := c.baseURL + "/next"
	sep := "?"
	if hints.PreferredCaseID != "" {
		url += sep + "preferred_case_id=" + hints.PreferredCaseID
		sep = "&"
	}
	if hints.PreferredZone != "" {
		url += sep + "preferred_zone=" + hints.PreferredZone
		sep = "&"
	}
	if hints.Stickiness > 0 {
		url += sep + fmt.Sprintf("stickiness=%d", hints.Stickiness)
		sep = "&"
	}
	if hints.ConsecutiveMisses > 0 {
		url += fmt.Sprintf("%sconsecutive_misses=%d", sep, hints.ConsecutiveMisses)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return agentport.Context{}, fmt.Errorf("network client: create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return agentport.Context{}, fmt.Errorf("network client: GET /next: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return agentport.Context{}, fmt.Errorf("network client: GET /next: status %d: %s",
			resp.StatusCode, string(body))
	}

	var nr nextResponse
	if err := json.NewDecoder(resp.Body).Decode(&nr); err != nil {
		return agentport.Context{}, fmt.Errorf("network client: decode response: %w", err)
	}

	return agentport.Context{
		DispatchID:   nr.DispatchID,
		CaseID:       nr.CaseID,
		Step:         nr.Step,
		PromptPath:   nr.PromptPath,
		ArtifactPath: nr.ArtifactPath,
	}, nil
}

// SubmitArtifact sends artifact data to the server for the given dispatch ID.
func (c *NetworkClient) SubmitArtifact(ctx context.Context, dispatchID int64, data []byte) error {
	body, err := json.Marshal(submitRequest{DispatchID: dispatchID, Data: data})
	if err != nil {
		return fmt.Errorf("network client: marshal submit: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/submit",
		bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("network client: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("network client: POST /submit: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("network client: POST /submit: status %d: %s",
			resp.StatusCode, string(respBody))
	}

	return nil
}

// EmitSignal sends a signal to the server's signal bus.
func (c *NetworkClient) EmitSignal(ctx context.Context, event, agent, caseID, step string, meta map[string]string) error {
	body, err := json.Marshal(emitSignalRequest{
		Event:  event,
		Agent:  agent,
		CaseID: caseID,
		Step:   step,
		Meta:   meta,
	})
	if err != nil {
		return fmt.Errorf("network client: marshal signal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/signal",
		bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("network client: create signal request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("network client: POST /signal: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("network client: POST /signal: status %d: %s",
			resp.StatusCode, string(respBody))
	}

	return nil
}

// GetSignals retrieves signals from the server's signal bus starting at the given index.
func (c *NetworkClient) GetSignals(ctx context.Context, since int) ([]agentport.Signal, error) {
	url := fmt.Sprintf("%s/signals?since=%d", c.baseURL, since)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("network client: create signals request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network client: GET /signals: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("network client: GET /signals: status %d: %s",
			resp.StatusCode, string(body))
	}

	var sigs []agentport.Signal
	if err := json.NewDecoder(resp.Body).Decode(&sigs); err != nil {
		return nil, fmt.Errorf("network client: decode signals: %w", err)
	}

	return sigs, nil
}

var _ agentport.ExternalDispatcher = (*NetworkClient)(nil)
