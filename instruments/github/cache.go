package github

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// RepoCache manages shallow clones of repositories on the local filesystem.
// Clones are keyed by (org, repo, branch) and shared across parallel walkers.
type RepoCache struct {
	baseDir       string
	token         string            // default token (from $GITHUB_TOKEN)
	privateTokens map[string]string // org or org/repo → token (from $GITHUB_PRIVATE_REPO_TOKENS)
	mu            sync.Mutex
	cloning       map[string]*cloneState
}

type cloneState struct {
	done chan struct{}
	err  error
}

// NewRepoCache creates a cache rooted at baseDir.
func NewRepoCache(baseDir, token string) *RepoCache {
	return &RepoCache{
		baseDir:       baseDir,
		token:         token,
		privateTokens: ResolvePrivateRepoTokens(),
		cloning:       make(map[string]*cloneState),
	}
}

// EnsureCloned returns the local path for the given repo@branch.
// If the repo is not yet cloned, it performs a shallow clone. Concurrent
// callers for the same key block until the first clone completes.
func (c *RepoCache) EnsureCloned(ctx context.Context, org, repo, branch string) (string, error) {
	localPath := c.repoPath(org, repo, branch)

	if info, err := os.Stat(filepath.Join(localPath, gitDir)); err == nil && info.IsDir() {
		return localPath, nil
	}

	key := org + "/" + repo + "@" + branch

	c.mu.Lock()
	if state, ok := c.cloning[key]; ok {
		c.mu.Unlock()
		select {
		case <-state.done:
			return localPath, state.err
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	state := &cloneState{done: make(chan struct{})}
	c.cloning[key] = state
	c.mu.Unlock()

	state.err = c.doClone(ctx, org, repo, branch, localPath)
	close(state.done)

	return localPath, state.err
}

func (c *RepoCache) repoPath(org, repo, branch string) string {
	return filepath.Join(c.baseDir, org, repo, branch)
}

func (c *RepoCache) doClone(ctx context.Context, org, repo, branch, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}
	tok := TokenForOrg(c.privateTokens, org, repo, c.token)
	url := cloneURL(org, repo, "") // never embed token in URL
	return shallowClone(ctx, url, branch, dest, tok)
}

// Clear removes all cached clones.
func (c *RepoCache) Clear() error {
	return os.RemoveAll(c.baseDir)
}

// DefaultCacheDir returns the default cache directory path.
func DefaultCacheDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".asterisk", "cache", "repos")
}
