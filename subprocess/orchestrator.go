package subprocess

import (
	"context"
	"fmt"
	"log"
	"sync"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Orchestrator manages multiple named schematic backends with
// lifecycle operations including hot-swap. It accepts any
// SchematicBackend implementation (Server, ContainerBackend, etc.).
type Orchestrator struct {
	mu         sync.RWMutex
	schematics map[string]SchematicBackend
}

// NewOrchestrator creates an empty Orchestrator.
func NewOrchestrator() *Orchestrator {
	return &Orchestrator{
		schematics: make(map[string]SchematicBackend),
	}
}

// Register adds a named schematic backend. It does not start it.
func (o *Orchestrator) Register(name string, backend SchematicBackend) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.schematics[name] = backend
}

// Start launches a named schematic.
func (o *Orchestrator) Start(ctx context.Context, name string) error {
	o.mu.RLock()
	backend, ok := o.schematics[name]
	o.mu.RUnlock()
	if !ok {
		return fmt.Errorf("unknown schematic %q", name)
	}
	return backend.Start(ctx)
}

// Stop shuts down a named schematic.
func (o *Orchestrator) Stop(ctx context.Context, name string) error {
	o.mu.RLock()
	backend, ok := o.schematics[name]
	o.mu.RUnlock()
	if !ok {
		return fmt.Errorf("unknown schematic %q", name)
	}
	return backend.Stop(ctx)
}

// Swap replaces a running schematic with a new backend. The old backend
// is gracefully stopped before the new one starts.
func (o *Orchestrator) Swap(ctx context.Context, name string, newBackend SchematicBackend) error {
	o.mu.Lock()
	old, ok := o.schematics[name]
	if !ok {
		o.mu.Unlock()
		return fmt.Errorf("unknown schematic %q", name)
	}

	o.schematics[name] = newBackend
	o.mu.Unlock()

	if err := old.Stop(ctx); err != nil {
		log.Printf("subprocess: warning: stop %q for swap: %v (proceeding)", name, err)
	}

	return newBackend.Start(ctx)
}

// CallTool calls a tool on a named schematic.
func (o *Orchestrator) CallTool(ctx context.Context, name string, tool string, args map[string]any) (*sdkmcp.CallToolResult, error) {
	o.mu.RLock()
	backend, ok := o.schematics[name]
	o.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown schematic %q", name)
	}
	return backend.CallTool(ctx, tool, args)
}

// Healthy checks if a named schematic is healthy.
func (o *Orchestrator) Healthy(ctx context.Context, name string) bool {
	o.mu.RLock()
	backend, ok := o.schematics[name]
	o.mu.RUnlock()
	if !ok {
		return false
	}
	return backend.Healthy(ctx)
}

// StopAll stops all registered schematics.
func (o *Orchestrator) StopAll(ctx context.Context) {
	o.mu.RLock()
	names := make([]string, 0, len(o.schematics))
	for name := range o.schematics {
		names = append(names, name)
	}
	o.mu.RUnlock()

	for _, name := range names {
		_ = o.Stop(ctx, name)
	}
}
