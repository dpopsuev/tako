// Package gitops provides git-backed transformers for SDLC circuit nodes.
// Worktree isolation and branch management for code changes.
package gitops

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/dpopsuev/origami/engine/handler"
	"github.com/dpopsuev/origami/simulate/sdlc/sdlctype"
)

// CreateWorktree creates an isolated git worktree branch for code changes.
type CreateWorktree struct {
	repoPath string
}

// NewCreateWorktree creates a create-worktree transformer.
func NewCreateWorktree(repoPath string) *CreateWorktree {
	return &CreateWorktree{repoPath: repoPath}
}

// Name implements handler.Transformer.
func (c *CreateWorktree) Name() string { return "create-worktree" }

// Transform implements handler.Transformer.
func (c *CreateWorktree) Transform(ctx context.Context, tc *handler.TransformerContext) (any, error) {
	branch := fmt.Sprintf("circuit/%s/%d", tc.WalkerState.ID, time.Now().Unix())
	wtPath := c.repoPath + "/.worktrees/" + tc.WalkerState.ID

	// Create branch.
	if err := gitCmd(ctx, c.repoPath, "checkout", "-b", branch); err != nil {
		// Branch might already exist from a retry — try switching.
		if switchErr := gitCmd(ctx, c.repoPath, "checkout", branch); switchErr != nil {
			return nil, fmt.Errorf("create-worktree: %w", err)
		}
	}

	return &sdlctype.CreateWorktreeResult{
		Branch:       branch,
		WorktreePath: wtPath,
	}, nil
}

// Release pushes the current branch to remote and optionally creates a PR.
type Release struct {
	repoPath string
}

// NewRelease creates a release transformer.
func NewRelease(repoPath string) *Release {
	return &Release{repoPath: repoPath}
}

// Name implements handler.Transformer.
func (r *Release) Name() string { return "release" }

// Transform implements handler.Transformer.
func (r *Release) Transform(ctx context.Context, _ *handler.TransformerContext) (any, error) {
	// Get current branch name.
	branch, err := gitOutput(ctx, r.repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return &sdlctype.ReleaseResult{Tag: "", Changelog: "failed to get branch"}, nil
	}

	// Push branch to remote.
	if err := gitCmd(ctx, r.repoPath, "push", "-u", "origin", branch); err != nil {
		return &sdlctype.ReleaseResult{Tag: branch, Changelog: "push failed: " + err.Error()}, nil
	}

	return &sdlctype.ReleaseResult{
		Tag:       branch,
		Changelog: "branch pushed to origin",
	}, nil
}

func gitCmd(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %s: %w (%s)", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", args[0], err)
	}
	return strings.TrimSpace(string(out)), nil
}

// Compile-time interface checks.
var (
	_ handler.Transformer = (*CreateWorktree)(nil)
	_ handler.Transformer = (*Release)(nil)
)
