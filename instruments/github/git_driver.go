package github

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/dpopsuev/origami/toolkit"
)

// GitDriver implements toolkit.Driver for git repositories. It wraps the
// existing RepoCache for shallow cloning and the ripgrep-based search.
type GitDriver struct {
	cache *RepoCache

	mu         sync.RWMutex
	localPaths map[string]string // uri -> local path
}

var _ toolkit.Driver = (*GitDriver)(nil)

// GitDriverOption configures a GitDriver.
type GitDriverOption func(*GitDriver)

// WithRepoCache injects a pre-built RepoCache, useful for testing
// without requiring real git/ripgrep binaries.
func WithRepoCache(c *RepoCache) GitDriverOption {
	return func(d *GitDriver) { d.cache = c }
}

// DefaultGitDriver creates a GitDriver with default token resolution
// ($GITHUB_TOKEN env var, then .github-token file). This zero-arg factory
// is used by codegen for secondary schematic binding construction.
func DefaultGitDriver() (*GitDriver, error) {
	return NewGitDriver("")
}

// NewGitDriver creates a GitDriver backed by a shallow-clone cache.
// tokenSource is the path to a GitHub token file; if empty, standard
// resolution ($GITHUB_TOKEN, then .github-token) is used.
// Options are applied after default construction.
func NewGitDriver(tokenSource string, opts ...GitDriverOption) (*GitDriver, error) {
	token, err := ResolveToken(tokenSource)
	if err != nil {
		return nil, err
	}
	d := &GitDriver{
		cache:      NewRepoCache(DefaultCacheDir(), token),
		localPaths: make(map[string]string),
	}
	for _, opt := range opts {
		opt(d)
	}
	return d, nil
}

func (d *GitDriver) Handles() toolkit.SourceKind {
	return toolkit.SourceKindRepo
}

func (d *GitDriver) Ensure(ctx context.Context, src *toolkit.Source) error {
	org, repo, err := parseGitURI(src.URI)
	if err != nil {
		return err
	}

	localPath, err := d.cache.EnsureCloned(ctx, org, repo, src.Branch)
	if err != nil {
		return err
	}

	d.mu.Lock()
	d.localPaths[src.URI] = localPath
	d.mu.Unlock()
	return nil
}

func (d *GitDriver) Search(ctx context.Context, src *toolkit.Source, query string, maxResults int) ([]toolkit.SearchResult, error) {
	localPath, err := d.resolvePath(ctx, src)
	if err != nil {
		return nil, err
	}

	keywords := strings.Fields(query)
	if len(keywords) == 0 {
		return nil, nil
	}

	localResults, err := SearchCode(ctx, localPath, keywords)
	if err != nil {
		return nil, err
	}

	results := make([]toolkit.SearchResult, 0, len(localResults))
	for _, r := range localResults {
		if len(results) >= maxResults {
			break
		}
		results = append(results, toolkit.SearchResult{
			Source:  src.Name,
			Path:    r.File,
			Line:    r.Line,
			Snippet: r.Snippet,
		})
	}
	return results, nil
}

func (d *GitDriver) Read(ctx context.Context, src *toolkit.Source, path string) ([]byte, error) {
	localPath, err := d.resolvePath(ctx, src)
	if err != nil {
		return nil, err
	}
	return ReadFile(ctx, localPath, path)
}

func (d *GitDriver) List(ctx context.Context, src *toolkit.Source, root string, maxDepth int) ([]toolkit.ContentEntry, error) {
	localPath, err := d.resolvePath(ctx, src)
	if err != nil {
		return nil, err
	}

	localEntries, err := ListTree(ctx, localPath, maxDepth)
	if err != nil {
		return nil, err
	}

	entries := make([]toolkit.ContentEntry, 0, len(localEntries))
	for _, e := range localEntries {
		if root != "" && root != "." && !strings.HasPrefix(e.Path, root) {
			continue
		}
		entries = append(entries, toolkit.ContentEntry{
			Path:  e.Path,
			IsDir: e.IsDir,
		})
	}
	return entries, nil
}

func (d *GitDriver) resolvePath(ctx context.Context, src *toolkit.Source) (string, error) {
	d.mu.RLock()
	lp, ok := d.localPaths[src.URI]
	d.mu.RUnlock()
	if ok {
		return lp, nil
	}

	if err := d.Ensure(ctx, src); err != nil {
		return "", err
	}

	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.localPaths[src.URI], nil
}

// parseGitURI extracts org and repo from a GitHub URI.
// Handles "https://github.com/org/repo" and "https://github.com/org/repo.git".
func parseGitURI(uri string) (org, repo string, err error) {
	uri = strings.TrimSuffix(uri, gitDir)
	parts := strings.Split(uri, "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("%w: %q: expected at least org/repo", ErrCannotParseGitURI, uri)
	}
	return parts[len(parts)-2], parts[len(parts)-1], nil
}
