package kami

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/coder/websocket"
)

// wsConn tracks a single WebSocket client.
type wsConn struct {
	conn *websocket.Conn
	ctx  context.Context
}

// handleWS upgrades to WebSocket and relays visualization commands
// from the AI agent to the connected browser.
func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // localhost only
	})
	if err != nil {
		s.log.Warn("ws accept failed", "error", err)
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "bye")

	ctx := r.Context()
	wc := &wsConn{conn: conn, ctx: ctx}

	s.mu.Lock()
	id := s.nextWS
	s.nextWS++
	s.wsConns[id] = wc
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.wsConns, id)
		s.mu.Unlock()
	}()

	s.log.Info("ws client connected", "id", id)

	for {
		_, msg, err := conn.Read(ctx)
		if err != nil {
			return
		}
		var payload map[string]any
		if json.Unmarshal(msg, &payload) == nil {
			s.bridge.Emit(Event{
				Type: EventType("ws_message"),
				Data: payload,
			})
		}
	}
}

// BroadcastWS sends a JSON message to all connected WebSocket clients.
// Used by MCP visualization tools to push commands to the browser.
func (s *Server) BroadcastWS(ctx context.Context, msg any) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	s.mu.Lock()
	conns := make([]*wsConn, 0, len(s.wsConns))
	for _, wc := range s.wsConns {
		conns = append(conns, wc)
	}
	s.mu.Unlock()

	var mu sync.Mutex
	var errs []error
	for _, wc := range conns {
		if err := wc.conn.Write(ctx, websocket.MessageText, data); err != nil {
			mu.Lock()
			errs = append(errs, err)
			mu.Unlock()
		}
	}
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}
