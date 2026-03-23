package subprocess

import (
	"context"
	"fmt"
	"sync"
)

// WorkerPoolConfig configures a pool of worker containers.
type WorkerPoolConfig struct {
	Image    string
	Replicas int
	Env      []string // KEY=VALUE pairs passed to each container
	Args     []string // command arguments appended after the image
	Network  string   // container network (e.g. "host")
	Runtime  *ContainerRuntime
}

// WorkerPool manages N identical worker containers. It is the local
// equivalent of a K8s Deployment with replicas: N.
type WorkerPool struct {
	config WorkerPoolConfig

	mu      sync.Mutex
	workers []workerEntry
}

type workerEntry struct {
	id   string
	name string
}

// NewWorkerPool creates a pool that is not yet started.
func NewWorkerPool(cfg WorkerPoolConfig) *WorkerPool {
	if cfg.Runtime == nil {
		cfg.Runtime = NewContainerRuntime("")
	}
	return &WorkerPool{config: cfg}
}

// Start launches cfg.Replicas worker containers.
func (wp *WorkerPool) Start(ctx context.Context) error {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	for i := range wp.config.Replicas {
		name := fmt.Sprintf("worker-%d", i)
		id, err := wp.config.Runtime.RunWithOptions(ctx, RunOptions{
			Name:    name,
			Image:   wp.config.Image,
			Env:     wp.config.Env,
			Args:    wp.config.Args,
			Network: wp.config.Network,
		})
		if err != nil {
			return fmt.Errorf("start worker %d: %w", i, err)
		}
		wp.workers = append(wp.workers, workerEntry{id: id, name: name})
	}
	return nil
}

// Scale adjusts the pool to n workers. If n > current, new workers are
// started. If n < current, excess workers are stopped.
func (wp *WorkerPool) Scale(ctx context.Context, n int) error {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	current := len(wp.workers)
	if n > current {
		for i := current; i < n; i++ {
			name := fmt.Sprintf("worker-%d", i)
			id, err := wp.config.Runtime.RunWithOptions(ctx, RunOptions{
				Name:    name,
				Image:   wp.config.Image,
				Env:     wp.config.Env,
				Args:    wp.config.Args,
				Network: wp.config.Network,
			})
			if err != nil {
				return fmt.Errorf("scale up worker %d: %w", i, err)
			}
			wp.workers = append(wp.workers, workerEntry{id: id, name: name})
		}
	} else if n < current {
		for i := current - 1; i >= n; i-- {
			_ = wp.config.Runtime.Stop(ctx, wp.workers[i].id)
		}
		wp.workers = wp.workers[:n]
	}
	return nil
}

// StopAll stops and removes all worker containers.
func (wp *WorkerPool) StopAll(ctx context.Context) {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	for _, w := range wp.workers {
		_ = wp.config.Runtime.Stop(ctx, w.id)
	}
	wp.workers = nil
}

// Len returns the current number of workers.
func (wp *WorkerPool) Len() int {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	return len(wp.workers)
}

// Wait blocks until all containers exit. It polls container status via
// `podman wait`. This is a best-effort implementation for local use.
func (wp *WorkerPool) Wait(ctx context.Context) error {
	wp.mu.Lock()
	ids := make([]string, len(wp.workers))
	for i, w := range wp.workers {
		ids[i] = w.id
	}
	wp.mu.Unlock()

	for _, id := range ids {
		if err := wp.config.Runtime.WaitContainer(ctx, id); err != nil {
			return err
		}
	}
	return nil
}
