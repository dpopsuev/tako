package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

const labelPaused = "PAUSED"

// KamiBridge connects to a Kami server's SSE stream and maintains
// live circuit state for inlay hint overlays.
type KamiBridge struct {
	mu      sync.RWMutex
	enabled bool
	port    int
	baseURL string

	cancel context.CancelFunc
	done   chan struct{}

	state CircuitState
}

// CircuitState tracks live node/edge state from Kami events.
type CircuitState struct {
	ActiveNode  string            `json:"active_node,omitempty"`
	ActiveAgent string            `json:"active_agent,omitempty"`
	Paused      bool              `json:"paused,omitempty"`
	Visited     map[string]VisitInfo `json:"visited,omitempty"`
	Transitions map[string]time.Time `json:"transitions,omitempty"`
}

// VisitInfo records when a node was last visited and by whom.
type VisitInfo struct {
	Agent     string    `json:"agent"`
	Timestamp time.Time `json:"ts"`
}

// NewKamiBridge creates a bridge with given port. Not connected until Start().
func NewKamiBridge(port int) *KamiBridge {
	return &KamiBridge{
		port:    port,
		baseURL: fmt.Sprintf("http://localhost:%d", port),
		state: CircuitState{
			Visited:     make(map[string]VisitInfo),
			Transitions: make(map[string]time.Time),
		},
	}
}

// Start begins SSE consumption in a background goroutine.
func (kb *KamiBridge) Start(ctx context.Context) {
	kb.mu.Lock()
	if kb.enabled {
		kb.mu.Unlock()
		return
	}
	kb.enabled = true
	kb.done = make(chan struct{})
	innerCtx, cancel := context.WithCancel(ctx)
	kb.cancel = cancel
	kb.mu.Unlock()

	go kb.connectLoop(innerCtx)
}

// Stop disconnects from Kami.
func (kb *KamiBridge) Stop() {
	kb.mu.Lock()
	if !kb.enabled {
		kb.mu.Unlock()
		return
	}
	kb.enabled = false
	kb.cancel()
	kb.mu.Unlock()
	<-kb.done
}

// State returns a snapshot of the current circuit state.
func (kb *KamiBridge) State() CircuitState {
	kb.mu.RLock()
	defer kb.mu.RUnlock()

	visited := make(map[string]VisitInfo, len(kb.state.Visited))
	for k, v := range kb.state.Visited {
		visited[k] = v
	}
	transitions := make(map[string]time.Time, len(kb.state.Transitions))
	for k, v := range kb.state.Transitions {
		transitions[k] = v
	}

	return CircuitState{
		ActiveNode:  kb.state.ActiveNode,
		ActiveAgent: kb.state.ActiveAgent,
		Paused:      kb.state.Paused,
		Visited:     visited,
		Transitions: transitions,
	}
}

// Connected reports whether the bridge is actively receiving events.
func (kb *KamiBridge) Connected() bool {
	kb.mu.RLock()
	defer kb.mu.RUnlock()
	return kb.enabled
}

func (kb *KamiBridge) connectLoop(ctx context.Context) {
	defer close(kb.done)

	backoff := 1 * time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		err := kb.consumeSSE(ctx)
		if ctx.Err() != nil {
			return
		}

		log.Printf("kami-bridge: disconnected: %v, reconnecting in %s", err, backoff)

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}

		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

func (kb *KamiBridge) consumeSSE(ctx context.Context) error {
	url := kb.baseURL + "/events/stream"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		kb.processEvent(payload)
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read: %w", err)
	}
	return fmt.Errorf("stream ended")
}

// kamiEvent mirrors kami.Event for JSON deserialization.
type kamiEvent struct {
	Type      string         `json:"type"`
	Timestamp time.Time      `json:"ts"`
	Agent     string         `json:"agent,omitempty"`
	Node      string         `json:"node,omitempty"`
	Edge      string         `json:"edge,omitempty"`
	ElapsedMs int64          `json:"elapsed_ms,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
}

func (kb *KamiBridge) processEvent(payload string) {
	var evt kamiEvent
	if err := json.Unmarshal([]byte(payload), &evt); err != nil {
		return
	}

	kb.mu.Lock()
	defer kb.mu.Unlock()

	ts := evt.Timestamp
	if ts.IsZero() {
		ts = time.Now().UTC()
	}

	switch evt.Type {
	case "node_enter":
		kb.state.ActiveNode = evt.Node
		kb.state.ActiveAgent = evt.Agent
		kb.state.Paused = false
		kb.state.Visited[evt.Node] = VisitInfo{Agent: evt.Agent, Timestamp: ts}

	case "node_exit":
		if kb.state.ActiveNode == evt.Node {
			kb.state.ActiveNode = ""
		}

	case "transition":
		kb.state.Transitions[evt.Edge] = ts

	case "paused", "breakpoint_hit":
		kb.state.Paused = true

	case "resumed":
		kb.state.Paused = false

	case "walk_complete", "walk_error":
		kb.state.ActiveNode = ""
		kb.state.ActiveAgent = ""
		kb.state.Paused = false
	}
}

// LiveInlayHints returns inlay hints for live circuit state overlaid on
// the document. These augment the static hints from computeInlayHints.
func (kb *KamiBridge) LiveInlayHints(doc *document) []InlayHint {
	if doc == nil || doc.Def == nil {
		return nil
	}

	state := kb.State()
	lines := strings.Split(doc.Content, "\n")
	var hints []InlayHint

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "- name:") && !strings.HasPrefix(trimmed, "name:") {
			continue
		}

		name := strings.TrimSpace(strings.TrimPrefix(trimmed, "- name:"))
		if name == "" {
			name = strings.TrimSpace(strings.TrimPrefix(trimmed, "name:"))
		}
		if name == "" {
			continue
		}

		if name == state.ActiveNode {
			label := "ACTIVE"
			if state.ActiveAgent != "" {
				label = fmt.Sprintf("ACTIVE [%s]", state.ActiveAgent)
			}
			if state.Paused {
				label = labelPaused
			}
			hints = append(hints, InlayHint{
				Position:    Position{Line: uint32(i), Character: uint32(len(line))},
				Label:       label,
				Kind:        1,
				PaddingLeft: true,
			})
		} else if visit, ok := state.Visited[name]; ok {
			ago := time.Since(visit.Timestamp).Truncate(time.Second)
			label := fmt.Sprintf("visited (%s ago)", ago)
			hints = append(hints, InlayHint{
				Position:    Position{Line: uint32(i), Character: uint32(len(line))},
				Label:       label,
				Kind:        1,
				PaddingLeft: true,
			})
		}
	}

	hints = append(hints, kb.lastTransitionHints(doc, state, lines)...)

	return hints
}

// lastTransitionHints emits an inlay hint on the most recently traversed edge.
func (kb *KamiBridge) lastTransitionHints(doc *document, state CircuitState, lines []string) []InlayHint {
	if len(state.Transitions) == 0 || doc.Def == nil {
		return nil
	}

	var latestEdge string
	var latestTime time.Time
	for edgeID, ts := range state.Transitions {
		if ts.After(latestTime) {
			latestTime = ts
			latestEdge = edgeID
		}
	}
	if latestEdge == "" {
		return nil
	}

	line := findEdgeIDLine(lines, latestEdge)
	if line < 0 {
		return nil
	}

	ago := time.Since(latestTime).Truncate(time.Second)
	return []InlayHint{{
		Position:    Position{Line: uint32(line), Character: uint32(len(lines[line]))},
		Label:       fmt.Sprintf("last transition (%s ago)", ago),
		Kind:        1,
		PaddingLeft: true,
	}}
}

// configureKami starts or stops the Kami bridge based on configuration.
func (s *Server) configureKami(enabled bool, port int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch {
	case enabled && s.kamiBridge == nil:
		s.kamiBridge = NewKamiBridge(port)
		s.kamiBridge.Start(context.Background())
	case enabled && s.kamiBridge != nil:
		s.kamiBridge.Stop()
		s.kamiBridge = NewKamiBridge(port)
		s.kamiBridge.Start(context.Background())
	case !enabled && s.kamiBridge != nil:
		s.kamiBridge.Stop()
		s.kamiBridge = nil
	}
}
