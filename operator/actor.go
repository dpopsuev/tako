package operator

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

// InProcessActor runs the SDLC circuit in-process using simulate/sdlc.Run().
// No container needed — fastest path for development and testing.
type InProcessActor struct {
	runFunc func(ctx context.Context, drift DriftResult) (*RunResult, error)
}

// NewInProcessActor creates an actor that runs the circuit in-process.
// The provided function executes the circuit and returns the result.
func NewInProcessActor(fn func(ctx context.Context, drift DriftResult) (*RunResult, error)) *InProcessActor {
	return &InProcessActor{runFunc: fn}
}

// Act implements Actor.
func (a *InProcessActor) Act(drift DriftResult) (*RunResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	return a.runFunc(ctx, drift)
}

var _ Actor = (*InProcessActor)(nil)

// ContainerActor runs the SDLC circuit by spawning an ephemeral container.
// Uses docker or podman to run the fold-generated circuit binary.
type ContainerActor struct {
	runtime  string   // "docker" or "podman"
	image    string   // container image name
	repoPath string   // host path to mount as /workspace
	envVars  []string // additional env vars to pass
}

// ContainerActorOption configures the container actor.
type ContainerActorOption func(*ContainerActor)

// WithRuntime sets the container runtime. Default "docker".
func WithRuntime(rt string) ContainerActorOption {
	return func(a *ContainerActor) { a.runtime = rt }
}

// WithEnvVars adds environment variables to pass to the container.
func WithEnvVars(vars ...string) ContainerActorOption {
	return func(a *ContainerActor) { a.envVars = append(a.envVars, vars...) }
}

// NewContainerActor creates an actor that spawns ephemeral containers.
func NewContainerActor(image, repoPath string, opts ...ContainerActorOption) *ContainerActor {
	a := &ContainerActor{
		runtime:  "docker",
		image:    image,
		repoPath: repoPath,
	}
	for _, o := range opts {
		o(a)
	}
	return a
}

// Act implements Actor by running: docker run --rm -v repoPath:/workspace image
func (a *ContainerActor) Act(_ DriftResult) (*RunResult, error) {
	start := time.Now()

	rt := a.runtime
	if rt != "docker" && rt != "podman" {
		return &RunResult{Success: false, Duration: 0, Error: fmt.Sprintf("invalid runtime: %s", rt)}, nil
	}

	args := []string{
		"run", "--rm",
		"-v", a.repoPath + ":/workspace",
		"-e", "SDLC_REPO_PATH=/workspace",
		"-e", "SDLC_MODE=real",
	}
	for _, env := range a.envVars {
		args = append(args, "-e", env)
	}
	args = append(args, a.image)

	cmd := exec.Command(rt, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	elapsed := time.Since(start)

	if err != nil {
		return &RunResult{
			Success:  false,
			Duration: elapsed,
			Error:    fmt.Sprintf("%s\n%s", err.Error(), stderr.String()),
		}, nil // return nil error — the operator handles failure via RunResult
	}

	return &RunResult{
		Success:  true,
		Duration: elapsed,
	}, nil
}

var _ Actor = (*ContainerActor)(nil)
