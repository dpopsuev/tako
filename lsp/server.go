// Package lsp implements a Language Server for Origami circuit YAML.
// It embeds the origami-lint engine for diagnostics and provides
// completion, hover, and go-to-definition for the circuit DSL.
package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sync"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/lint"
)

// Server is the Origami LSP server.
type Server struct {
	conn jsonrpc2.Conn

	mu         sync.Mutex
	docs       map[uri.URI]*document
	ready      bool
	kamiBridge *KamiBridge
	vocab      circuit.RichVocabulary
}

type document struct {
	URI     uri.URI
	Content string
	Def     *circuit.CircuitDef
	LintCtx *lint.LintContext
}

// NewServer creates a new LSP server.
func NewServer() *Server {
	return &Server{
		docs: make(map[uri.URI]*document),
	}
}

// Handler returns a jsonrpc2.Handler for the server.
func (s *Server) Handler() jsonrpc2.Handler {
	return jsonrpc2.Handler(func(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
		switch req.Method() {
		case "initialize":
			return s.handleInitialize(ctx, reply, req)
		case "initialized":
			return reply(ctx, nil, nil)
		case "shutdown":
			return reply(ctx, nil, nil)
		case "exit":
			return reply(ctx, nil, nil)
		case "textDocument/didOpen":
			return s.handleDidOpen(ctx, reply, req)
		case "textDocument/didChange":
			return s.handleDidChange(ctx, reply, req)
		case "textDocument/didClose":
			return s.handleDidClose(ctx, reply, req)
		case "textDocument/completion":
			return s.handleCompletion(ctx, reply, req)
		case "textDocument/hover":
			return s.handleHover(ctx, reply, req)
		case "textDocument/definition":
			return s.handleDefinition(ctx, reply, req)
		case "textDocument/semanticTokens/full":
			return s.handleSemanticTokensFull(ctx, reply, req)
		case "textDocument/inlayHint":
			return s.handleInlayHint(ctx, reply, req)
		case "workspace/didChangeConfiguration":
			return s.handleDidChangeConfiguration(ctx, reply, req)
		default:
			return reply(ctx, nil, jsonrpc2.ErrMethodNotFound)
		}
	})
}

// initializeResult extends protocol.InitializeResult with LSP 3.17 fields
// not present in go.lsp.dev/protocol v0.12.
type initializeResult struct {
	Capabilities initializeCapabilities `json:"capabilities"`
	ServerInfo   *protocol.ServerInfo   `json:"serverInfo,omitempty"`
}

type initializeCapabilities struct {
	protocol.ServerCapabilities
	InlayHintProvider bool `json:"inlayHintProvider,omitempty"`
}

const defaultKamiPort = 9800

func (s *Server) handleInitialize(ctx context.Context, reply jsonrpc2.Replier, _ jsonrpc2.Request) error {
	s.mu.Lock()
	s.ready = true
	s.mu.Unlock()

	result := initializeResult{
		Capabilities: initializeCapabilities{
			ServerCapabilities: protocol.ServerCapabilities{
				TextDocumentSync: protocol.TextDocumentSyncOptions{
					OpenClose: true,
					Change:    protocol.TextDocumentSyncKindFull,
				},
				CompletionProvider: &protocol.CompletionOptions{
					TriggerCharacters: []string{":", " ", "-"},
				},
				HoverProvider:          true,
				DefinitionProvider:     true,
				SemanticTokensProvider: SemanticTokensProvider(),
			},
			InlayHintProvider: true,
		},
		ServerInfo: &protocol.ServerInfo{
			Name:    "origami-lsp",
			Version: "0.1.0",
		},
	}

	go s.probeKami(defaultKamiPort)

	return reply(ctx, result, nil)
}

// probeKami attempts to connect to a Kami server on the given port.
// Best-effort: silently ignored if Kami is not running.
func (s *Server) probeKami(port int) {
	client := &http.Client{Timeout: 2 * time.Second}
	url := fmt.Sprintf("http://localhost:%d/events/stream", port)
	resp, err := client.Head(url)
	if err != nil {
		return
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusMethodNotAllowed {
		s.configureKami(true, port)
	}
}

func (s *Server) handleDidOpen(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DidOpenTextDocumentParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	doc := s.updateDocument(params.TextDocument.URI, params.TextDocument.Text)
	s.publishDiagnostics(ctx, doc)
	return reply(ctx, nil, nil)
}

func (s *Server) handleDidChange(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DidChangeTextDocumentParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	if len(params.ContentChanges) == 0 {
		return reply(ctx, nil, nil)
	}
	content := params.ContentChanges[len(params.ContentChanges)-1].Text

	doc := s.updateDocument(params.TextDocument.URI, content)
	s.publishDiagnostics(ctx, doc)
	return reply(ctx, nil, nil)
}

func (s *Server) handleDidClose(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DidCloseTextDocumentParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	s.mu.Lock()
	delete(s.docs, params.TextDocument.URI)
	s.mu.Unlock()

	return reply(ctx, nil, nil)
}

func (s *Server) updateDocument(docURI uri.URI, content string) *document {
	raw := []byte(content)
	file := string(docURI)

	lintCtx, _ := lint.NewLintContext(raw, file)

	var def *circuit.CircuitDef
	if lintCtx != nil {
		def = lintCtx.Def
	}

	doc := &document{
		URI:     docURI,
		Content: content,
		Def:     def,
		LintCtx: lintCtx,
	}

	s.mu.Lock()
	s.docs[docURI] = doc
	s.mu.Unlock()

	return doc
}

func (s *Server) getDocument(docURI uri.URI) *document {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.docs[docURI]
}

func (s *Server) publishDiagnostics(ctx context.Context, doc *document) {
	if s.conn == nil {
		return
	}

	diagnostics := computeDiagnostics(doc)

	params := protocol.PublishDiagnosticsParams{
		URI:         doc.URI,
		Diagnostics: diagnostics,
	}

	raw, _ := json.Marshal(params)
	_ = s.conn.Notify(ctx, "textDocument/publishDiagnostics", json.RawMessage(raw))
}

// SetConn sets the JSON-RPC connection for sending notifications.
func (s *Server) SetConn(conn jsonrpc2.Conn) {
	s.conn = conn
}

func (s *Server) handleDidChangeConfiguration(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params struct {
		Settings json.RawMessage `json:"settings"`
	}
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, nil)
	}

	var settings struct {
		Origami struct {
			Kami struct {
				Enabled bool `json:"enabled"`
				Port    int  `json:"port"`
			} `json:"kami"`
		} `json:"origami"`
	}
	if json.Unmarshal(params.Settings, &settings) == nil && settings.Origami.Kami.Port > 0 {
		s.configureKami(settings.Origami.Kami.Enabled, settings.Origami.Kami.Port)
	}

	return reply(ctx, nil, nil)
}

// KamiBridge returns the server's Kami bridge (may be nil).
func (s *Server) KamiBridge() *KamiBridge {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.kamiBridge
}

// SetVocab sets the rich vocabulary for hover enrichment.
func (s *Server) SetVocab(v circuit.RichVocabulary) {
	s.mu.Lock()
	s.vocab = v
	s.mu.Unlock()
}

// safeUint32 converts an int to uint32, clamping negative values to 0
// and values exceeding math.MaxUint32 to math.MaxUint32.
func safeUint32(v int) uint32 {
	if v < 0 {
		return 0
	}
	if v > math.MaxUint32 {
		return math.MaxUint32
	}
	return uint32(v)
}
