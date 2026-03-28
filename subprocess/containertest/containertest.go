// Package containertest provides test helpers for container-based E2E tests.
// It builds real OCI images, starts real containers via ContainerRuntime,
// and provides health-check polling and cleanup utilities.
package containertest

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/dpopsuev/origami/subprocess"
)

// RequirePodman skips the test if podman is not available or if
// testing.Short() is set.
func RequirePodman(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping container E2E in short mode")
	}
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("podman not available, skipping container E2E")
	}
}

// Env holds a set of started containers for cleanup.
type Env struct {
	t       *testing.T
	runtime *subprocess.ContainerRuntime
	ids     []string
}

// NewEnv creates a test environment with a podman runtime.
func NewEnv(t *testing.T) *Env {
	t.Helper()
	RequirePodman(t)
	e := &Env{
		t:       t,
		runtime: subprocess.NewContainerRuntime("podman"),
	}
	t.Cleanup(func() { e.StopAll() })
	return e
}

// Runtime returns the underlying ContainerRuntime.
func (e *Env) Runtime() *subprocess.ContainerRuntime {
	return e.runtime
}

// BuildImage builds an OCI image from a Dockerfile context directory.
func (e *Env) BuildImage(ctx context.Context, contextDir, tag string) {
	e.t.Helper()
	cmd := exec.CommandContext(ctx, "podman", "build", "-t", tag, contextDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		e.t.Fatalf("build image %s: %s: %v", tag, strings.TrimSpace(string(output)), err)
	}
}

// BuildImageFromSource builds an OCI image using a Dockerfile string and
// the repository root as the build context.
func (e *Env) BuildImageFromSource(ctx context.Context, dockerfile, tag, contextDir string) {
	e.t.Helper()
	cmd := exec.CommandContext(ctx, "podman", "build", "-f", "-", "-t", tag, contextDir)
	cmd.Stdin = strings.NewReader(dockerfile)
	output, err := cmd.CombinedOutput()
	if err != nil {
		e.t.Fatalf("build image %s: %s: %v", tag, strings.TrimSpace(string(output)), err)
	}
}

// BuildImageFromDockerfile builds an OCI image from a Dockerfile path
// and build context directory.
func (e *Env) BuildImageFromDockerfile(ctx context.Context, dockerfilePath, tag, contextDir string) {
	e.t.Helper()
	cmd := exec.CommandContext(ctx, "podman", "build", "-f", dockerfilePath, "-t", tag, contextDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		e.t.Fatalf("build image %s: %s: %v", tag, strings.TrimSpace(string(output)), err)
	}
}

// StartService starts a container and waits for /healthz to return 200.
func (e *Env) StartService(ctx context.Context, name, image string, port int, env []string) {
	e.t.Helper()
	e.StartServiceWithArgs(ctx, name, image, port, env, nil)
}

// ServiceConfig configures a container service start.
type ServiceConfig struct {
	Name    string
	Image   string
	Port    int
	Env     []string
	Args    []string
	Network string
}

// StartServiceWithArgs starts a container with command-line args and waits
// for /healthz to return 200.
func (e *Env) StartServiceWithArgs(ctx context.Context, name, image string, port int, env, args []string) {
	e.t.Helper()
	e.StartServiceWithConfig(ctx, &ServiceConfig{
		Name: name, Image: image, Port: port, Env: env, Args: args,
	})
}

// StartServiceWithConfig starts a container from a full ServiceConfig and
// waits for /healthz to return 200.
func (e *Env) StartServiceWithConfig(ctx context.Context, cfg *ServiceConfig) {
	e.t.Helper()
	opts := subprocess.RunOptions{
		Name:          cfg.Name,
		Image:         cfg.Image,
		HostPort:      cfg.Port,
		ContainerPort: cfg.Port,
		Env:           cfg.Env,
		Args:          cfg.Args,
		Network:       cfg.Network,
	}
	if cfg.Network == "host" {
		opts.HostPort = 0
		opts.ContainerPort = 0
	}
	id, err := e.runtime.RunWithOptions(ctx, &opts)
	if err != nil {
		e.t.Fatalf("start service %s: %v", cfg.Name, err)
	}
	e.ids = append(e.ids, id)

	if err := e.waitHealthy(ctx, cfg.Port, 30*time.Second); err != nil {
		e.t.Fatalf("service %s did not become healthy: %v", cfg.Name, err)
	}
}

// StartWorker starts a container that is not expected to serve HTTP
// (no health check polling).
func (e *Env) StartWorker(ctx context.Context, name, image string, env, args []string) string {
	e.t.Helper()
	opts := subprocess.RunOptions{
		Name:    name,
		Image:   image,
		Env:     env,
		Args:    args,
		Network: "host",
	}
	id, err := e.runtime.RunWithOptions(ctx, &opts)
	if err != nil {
		e.t.Fatalf("start worker %s: %v", name, err)
	}
	e.ids = append(e.ids, id)
	return id
}

// StopAll stops and removes all tracked containers.
func (e *Env) StopAll() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, id := range e.ids {
		_ = e.runtime.Stop(ctx, id)
	}
	e.ids = nil
}

// WaitForContainer blocks until the container exits.
func (e *Env) WaitForContainer(ctx context.Context, id string) error {
	return e.runtime.WaitContainer(ctx, id)
}

func (e *Env) waitHealthy(ctx context.Context, port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	url := fmt.Sprintf("http://127.0.0.1:%d/healthz", port)
	client := &http.Client{Timeout: 2 * time.Second}

	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	return fmt.Errorf("timeout waiting for healthz on port %d", port)
}
