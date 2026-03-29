package subprocess

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const defaultRuntime = "podman"

// ContainerRuntime handles OCI container lifecycle via podman or docker.
// It knows nothing about MCP — it only starts, stops, and removes containers.
type ContainerRuntime struct {
	Runtime string // "podman" or "docker"; defaults to "podman"
}

// NewContainerRuntime creates a runtime with the given CLI tool name.
func NewContainerRuntime(runtime string) *ContainerRuntime {
	if runtime == "" {
		runtime = defaultRuntime
	}
	return &ContainerRuntime{Runtime: runtime}
}

// RunOptions configures a container run with environment variables,
// command arguments, and network settings beyond the basic port mapping.
type RunOptions struct {
	Name          string
	Image         string
	HostPort      int
	ContainerPort int
	Env           []string // KEY=VALUE pairs
	Args          []string // appended after the image
	Network       string   // e.g. "host" or a custom network name
}

// Run starts a container and returns its ID. The container binds to
// 127.0.0.1 only and maps hostPort to containerPort.
func (cr *ContainerRuntime) Run(ctx context.Context, name, image string, hostPort, containerPort int) (string, error) {
	return cr.RunWithOptions(ctx, &RunOptions{
		Name:          name,
		Image:         image,
		HostPort:      hostPort,
		ContainerPort: containerPort,
	})
}

// RunWithOptions starts a container with full configuration and returns its ID.
func (cr *ContainerRuntime) RunWithOptions(ctx context.Context, opts *RunOptions) (string, error) {
	args := []string{"run", "-d", "--name", opts.Name}

	if opts.HostPort > 0 && opts.ContainerPort > 0 {
		args = append(args, "-p", fmt.Sprintf("127.0.0.1:%d:%d", opts.HostPort, opts.ContainerPort))
	}

	for _, e := range opts.Env {
		args = append(args, "-e", e)
	}

	if opts.Network != "" {
		args = append(args, "--network", opts.Network)
	}

	args = append(args, opts.Image)
	args = append(args, opts.Args...)

	cmd := exec.CommandContext(ctx, cr.Runtime, args...) //nolint:gosec // args are constructed from validated config fields
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("start container %q: %s: %w", opts.Name, strings.TrimSpace(string(output)), err)
	}
	return strings.TrimSpace(string(output)), nil
}

// Stop stops and removes a container by ID.
func (cr *ContainerRuntime) Stop(ctx context.Context, id string) error {
	stop := exec.CommandContext(ctx, cr.Runtime, "stop", "-t", "5", id) //nolint:gosec // runtime path is from trusted config
	_, _ = stop.CombinedOutput()

	rm := exec.CommandContext(ctx, cr.Runtime, "rm", "-f", id) //nolint:gosec // runtime path is from trusted config
	output, err := rm.CombinedOutput()
	if err != nil {
		return fmt.Errorf("remove container %s: %s: %w", id, strings.TrimSpace(string(output)), err)
	}
	return nil
}

// WaitContainer blocks until the container exits.
func (cr *ContainerRuntime) WaitContainer(ctx context.Context, id string) error {
	cmd := exec.CommandContext(ctx, cr.Runtime, "wait", id) //nolint:gosec // runtime path is from trusted config
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("wait container %s: %s: %w", id, strings.TrimSpace(string(output)), err)
	}
	return nil
}

// ContainerBackend implements SchematicBackend for a single OCI container
// communicating via MCP over Streamable HTTP.
type ContainerBackend struct {
	Image         string
	Port          int
	ContainerPort int // port inside the container; defaults to 9100
	Name          string
	Runtime       *ContainerRuntime
	Connector     *MCPConnector

	StartupTimeout time.Duration // max wait for container port readiness; defaults to 10s
	HTTPTimeout    time.Duration // HTTP client timeout for MCP calls; defaults to 30s
	PingTimeout    time.Duration // health check timeout; defaults to 2s

	mu          sync.Mutex
	session     *sdkmcp.ClientSession
	containerID string
}

// Start launches the container and connects an MCP client over HTTP.
func (cb *ContainerBackend) Start(ctx context.Context) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.session != nil {
		return fmt.Errorf("%w: %q already started", ErrContainer, cb.Name)
	}

	rt := cb.Runtime
	if rt == nil {
		rt = NewContainerRuntime("")
	}

	containerPort := cb.ContainerPort
	if containerPort == 0 {
		containerPort = 9100
	}

	id, err := rt.Run(ctx, cb.Name, cb.Image, cb.Port, containerPort)
	if err != nil {
		return err
	}
	cb.containerID = id

	session, err := cb.connectMCP(ctx)
	if err != nil {
		_ = rt.Stop(ctx, id)
		cb.containerID = ""
		return fmt.Errorf("connect MCP to container %q: %w", cb.Name, err)
	}

	cb.session = session
	return nil
}

// Stop closes the MCP session and removes the container.
func (cb *ContainerBackend) Stop(ctx context.Context) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.session != nil {
		cb.session.Close()
		cb.session = nil
	}

	if cb.containerID == "" {
		return nil
	}

	rt := cb.Runtime
	if rt == nil {
		rt = NewContainerRuntime("")
	}

	err := rt.Stop(ctx, cb.containerID)
	cb.containerID = ""
	return err
}

// CallTool invokes a tool on the container's MCP server.
func (cb *ContainerBackend) CallTool(ctx context.Context, name string, args map[string]any) (*sdkmcp.CallToolResult, error) {
	cb.mu.Lock()
	session := cb.session
	cb.mu.Unlock()

	if session == nil {
		return nil, fmt.Errorf("%w: %q not started", ErrContainer, cb.Name)
	}
	return session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
}

// Healthy returns true if the MCP session responds to a ping.
func (cb *ContainerBackend) Healthy(ctx context.Context) bool {
	cb.mu.Lock()
	session := cb.session
	cb.mu.Unlock()

	if session == nil {
		return false
	}

	pingTimeout := cb.PingTimeout
	if pingTimeout == 0 {
		pingTimeout = 2 * time.Second
	}

	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	return session.Ping(pingCtx, nil) == nil
}

func (cb *ContainerBackend) connectMCP(ctx context.Context) (*sdkmcp.ClientSession, error) {
	addr := "127.0.0.1:" + strconv.Itoa(cb.Port)

	startupTimeout := cb.StartupTimeout
	if startupTimeout == 0 {
		startupTimeout = 10 * time.Second
	}

	deadline := time.Now().Add(startupTimeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			conn.Close()
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}

	httpTimeout := cb.HTTPTimeout
	if httpTimeout == 0 {
		httpTimeout = 30 * time.Second
	}

	endpoint := "http://" + addr + "/mcp"

	transport := &sdkmcp.StreamableClientTransport{
		Endpoint: endpoint,
		HTTPClient: &http.Client{
			Timeout: httpTimeout,
		},
	}

	c := cb.Connector
	if c == nil {
		c = DefaultConnector()
	}
	return c.Connect(ctx, transport)
}

// --- Legacy ContainerManager (thin wrapper for backward compatibility) ---

// ContainerManager manages multiple named OCI containers. It is a thin
// registry on top of ContainerRuntime + ContainerBackend. New code should
// prefer using Orchestrator with ContainerBackend directly.
type ContainerManager struct {
	Runtime string // "podman" or "docker"; defaults to "podman"

	mu         sync.RWMutex
	containers map[string]*containerEntry
}

type containerEntry struct {
	backend *ContainerBackend
}

// NewContainerManager creates a ContainerManager with the given runtime.
func NewContainerManager(runtime string) *ContainerManager {
	if runtime == "" {
		runtime = defaultRuntime
	}
	return &ContainerManager{
		Runtime:    runtime,
		containers: make(map[string]*containerEntry),
	}
}

// Start starts a named container.
func (cm *ContainerManager) Start(ctx context.Context, name, image string, port int) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if _, exists := cm.containers[name]; exists {
		return fmt.Errorf("%w: %q already running", ErrContainer, name)
	}

	backend := &ContainerBackend{
		Image:   image,
		Port:    port,
		Name:    name,
		Runtime: NewContainerRuntime(cm.Runtime),
	}

	if err := backend.Start(ctx); err != nil {
		return err
	}

	cm.containers[name] = &containerEntry{backend: backend}
	return nil
}

// Stop stops and removes a named container.
func (cm *ContainerManager) Stop(ctx context.Context, name string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	entry, ok := cm.containers[name]
	if !ok {
		return nil
	}

	err := entry.backend.Stop(ctx)
	delete(cm.containers, name)
	return err
}

// Swap replaces a running container with a new image.
func (cm *ContainerManager) Swap(ctx context.Context, name, newImage string) error {
	cm.mu.RLock()
	entry, ok := cm.containers[name]
	port := 9100
	if ok {
		port = entry.backend.Port
	}
	cm.mu.RUnlock()

	if err := cm.Stop(ctx, name); err != nil {
		log.Printf("subprocess: warning: stop %q for swap: %v (proceeding)", name, err)
	}

	return cm.Start(ctx, name, newImage, port)
}

// CallTool calls a tool on a named container's MCP server.
func (cm *ContainerManager) CallTool(ctx context.Context, name, tool string, args map[string]any) (*sdkmcp.CallToolResult, error) {
	cm.mu.RLock()
	entry, ok := cm.containers[name]
	cm.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownContainer, name)
	}
	return entry.backend.CallTool(ctx, tool, args)
}
