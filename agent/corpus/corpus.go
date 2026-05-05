package corpus

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	agentshell "github.com/dpopsuev/tako/agent/shell"
	"github.com/dpopsuev/tako/artifact"
)

var (
	ErrHandlerNotFound = errors.New("corpus: handler not found")
)

// Handler is a named wire receiver. Stations, services, listeners
// implement this to receive routed wires.
type Handler interface {
	Name() string
	Receive(wire artifact.Wire) error
}

// Corpus is the agent's body — registers all capabilities (built-in + environment),
// wires buses, enforces gating. The composition root.
type Corpus struct {
	mu            sync.RWMutex
	handlers      map[string]Handler
	capabilities  *agentshell.CapabilitySet
	subscriptions map[string][]string
}

func New() *Corpus {
	return &Corpus{
		handlers:      make(map[string]Handler),
		capabilities:  agentshell.NewCapabilitySet(),
		subscriptions: make(map[string][]string),
	}
}

// Register adds a Capability to the Corpus. The unified path —
// no distinction between organ, instrument, or shell.
func (c *Corpus) Register(cap agentshell.Capability) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.capabilities.Register(cap)
}

// Capability returns a registered capability by name.
func (c *Corpus) Capability(name string) (agentshell.Capability, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.capabilities.Get(name)
}

// Capabilities returns the full set.
func (c *Corpus) Capabilities() *agentshell.CapabilitySet {
	return c.capabilities
}

func (c *Corpus) Attach(h Handler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handlers[h.Name()] = h
}

func (c *Corpus) Handler(name string) (Handler, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	h, ok := c.handlers[name]
	if !ok {
		return nil, ErrHandlerNotFound
	}
	return h, nil
}

func (c *Corpus) Handlers() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]string, 0, len(c.handlers))
	for name := range c.handlers {
		out = append(out, name)
	}
	return out
}

func (c *Corpus) Subscribe(kind string, name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.subscriptions[kind] = append(c.subscriptions[kind], name)
}

func (c *Corpus) Route(wire artifact.Wire) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	subs := c.subscriptions[wire.Kind]
	if len(subs) > 0 {
		var firstErr error
		for _, name := range subs {
			if h, ok := c.handlers[name]; ok {
				if err := h.Receive(wire); err != nil && firstErr == nil {
					firstErr = err
				}
			}
		}
		return firstErr
	}

	h, ok := c.handlers[wire.Kind]
	if !ok {
		return ErrHandlerNotFound
	}
	return h.Receive(wire)
}

// AttachShell registers a Shell as a handler for each of its action names.
// Motor.go's type assertion to shell.Shell works because shellHandler
// implements both Handler and Shell by delegation.
func (c *Corpus) AttachShell(sh agentshell.Shell) {
	for _, name := range sh.Names() {
		c.Attach(&shellHandler{actionName: name, shell: sh})
	}
}

type shellHandler struct {
	actionName string
	shell      agentshell.Shell
}

func (h *shellHandler) Name() string                  { return h.actionName }
func (h *shellHandler) Receive(_ artifact.Wire) error { return nil }

func (h *shellHandler) Names() []string                        { return h.shell.Names() }
func (h *shellHandler) Describe(name string) (string, error)   { return h.shell.Describe(name) }
func (h *shellHandler) Schema(name string) (json.RawMessage, error) { return h.shell.Schema(name) }
func (h *shellHandler) Mode(name string) agentshell.ActionMode { return h.shell.Mode(name) }
func (h *shellHandler) Approval(name string) agentshell.ActionApproval {
	return h.shell.Approval(name)
}
func (h *shellHandler) Risk(name string) float64 { return h.shell.Risk(name) }
func (h *shellHandler) Exec(ctx context.Context, name string, input json.RawMessage) (agentshell.Result, error) {
	return h.shell.Exec(ctx, name, input)
}
