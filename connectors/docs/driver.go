// Package docs implements the HTTP documentation GND driver.
// It searches docs.redhat.com (or any similarly structured doc site),
// fetches individual pages, and converts HTML to plain text. Results
// are cached locally with a configurable TTL.
package docs

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dpopsuev/origami/schematics/toolkit"
)

var _ toolkit.Driver = (*DocsDriver)(nil)

// DocsDriver searches and fetches web-based documentation.
type DocsDriver struct {
	client   *http.Client
	cacheDir string
	cacheTTL time.Duration

	mu    sync.RWMutex
	cache map[string]cacheEntry
}

type cacheEntry struct {
	data      []byte
	fetchedAt time.Time
}

// Option configures a DocsDriver.
type Option func(*DocsDriver)

// WithCacheDir sets the local cache directory.
func WithCacheDir(dir string) Option {
	return func(d *DocsDriver) { d.cacheDir = dir }
}

// WithCacheTTL sets the cache time-to-live.
func WithCacheTTL(ttl time.Duration) Option {
	return func(d *DocsDriver) { d.cacheTTL = ttl }
}

// WithHTTPClient sets a custom HTTP client (useful for testing).
func WithHTTPClient(c *http.Client) Option {
	return func(d *DocsDriver) { d.client = c }
}

// DefaultDocsDriver creates a DocsDriver with default settings.
// This zero-arg factory is used by codegen for secondary schematic
// binding construction. Returns (driver, nil) for uniform error handling.
func DefaultDocsDriver() (*DocsDriver, error) {
	return NewDocsDriver(), nil
}

// NewDocsDriver creates a DocsDriver with the given options.
func NewDocsDriver(opts ...Option) *DocsDriver {
	d := &DocsDriver{
		client:   &http.Client{Timeout: 30 * time.Second},
		cacheDir: defaultCacheDir(),
		cacheTTL: 1 * time.Hour,
		cache:    make(map[string]cacheEntry),
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

func (d *DocsDriver) Handles() toolkit.SourceKind {
	return toolkit.SourceKindDoc
}

// Ensure validates the documentation endpoint is reachable.
func (d *DocsDriver) Ensure(ctx context.Context, src toolkit.Source) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, src.URI, nil)
	if err != nil {
		return fmt.Errorf("docs ensure: %w", err)
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("docs ensure %s: %w", src.URI, err)
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("docs ensure %s: HTTP %d", src.URI, resp.StatusCode)
	}
	return nil
}

// Search queries the documentation search endpoint.
// The source URI should be the base URL (e.g. "https://docs.redhat.com").
// Source tags can include "product" and "version" for filtering.
func (d *DocsDriver) Search(ctx context.Context, src toolkit.Source, query string, maxResults int) ([]toolkit.SearchResult, error) {
	searchURL, err := buildSearchURL(src, query)
	if err != nil {
		return nil, err
	}

	cacheKey := searchURL
	if cached, ok := d.getCached(cacheKey); ok {
		return parseSearchResults(src.Name, string(cached), maxResults), nil
	}

	body, err := d.fetchURL(ctx, searchURL)
	if err != nil {
		return nil, fmt.Errorf("docs search: %w", err)
	}

	d.putCache(cacheKey, body)
	return parseSearchResults(src.Name, string(body), maxResults), nil
}

// Read fetches a documentation page and returns it as plain text.
func (d *DocsDriver) Read(ctx context.Context, src toolkit.Source, path string) ([]byte, error) {
	pageURL := src.URI
	if path != "" && path != "/" {
		pageURL = strings.TrimRight(src.URI, "/") + "/" + strings.TrimLeft(path, "/")
	}

	cacheKey := pageURL
	if cached, ok := d.getCached(cacheKey); ok {
		return cached, nil
	}

	body, err := d.fetchURL(ctx, pageURL)
	if err != nil {
		return nil, fmt.Errorf("docs read %s: %w", pageURL, err)
	}

	text := htmlToText(string(body))
	textBytes := []byte(text)
	d.putCache(cacheKey, textBytes)
	return textBytes, nil
}

// List is a no-op for documentation sources — search is the discovery mechanism.
func (d *DocsDriver) List(_ context.Context, _ toolkit.Source, _ string, _ int) ([]toolkit.ContentEntry, error) {
	return nil, nil
}

func (d *DocsDriver) fetchURL(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "origami-docs-driver/0.1")

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, rawURL)
	}

	return io.ReadAll(resp.Body)
}

func (d *DocsDriver) getCached(key string) ([]byte, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	e, ok := d.cache[key]
	if !ok {
		return nil, false
	}
	if time.Since(e.fetchedAt) > d.cacheTTL {
		return nil, false
	}
	return e.data, true
}

func (d *DocsDriver) putCache(key string, data []byte) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.cache[key] = cacheEntry{data: data, fetchedAt: time.Now()}

	if d.cacheDir != "" {
		d.persistToDisk(key, data)
	}
}

func (d *DocsDriver) persistToDisk(key string, data []byte) {
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(key)))
	path := filepath.Join(d.cacheDir, hash[:2], hash)
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, data, 0o644)
}

func buildSearchURL(src toolkit.Source, query string) (string, error) {
	base := strings.TrimRight(src.URI, "/")
	u, err := url.Parse(base + "/search/")
	if err != nil {
		return "", fmt.Errorf("build search URL: %w", err)
	}

	q := u.Query()
	q.Set("q", query)
	q.Set("documentKind", "Documentation")

	if product, ok := src.Tags["product"]; ok {
		q.Set("product", product)
	}
	if version, ok := src.Tags["version"]; ok {
		q.Set("documentation_version", version)
	}

	u.RawQuery = q.Encode()
	return u.String(), nil
}

// parseSearchResults extracts doc links and snippets from the HTML response.
// This is a simple heuristic parser — it looks for common doc page patterns.
func parseSearchResults(sourceName, html string, maxResults int) []toolkit.SearchResult {
	var results []toolkit.SearchResult

	lines := strings.Split(html, "\n")
	for _, line := range lines {
		if len(results) >= maxResults {
			break
		}
		line = strings.TrimSpace(line)

		// Look for links to documentation pages
		hrefIdx := strings.Index(line, `href="`)
		if hrefIdx < 0 {
			continue
		}
		hrefStart := hrefIdx + 6
		hrefEnd := strings.Index(line[hrefStart:], `"`)
		if hrefEnd < 0 {
			continue
		}
		href := line[hrefStart : hrefStart+hrefEnd]

		if !strings.Contains(href, "/documentation/") && !strings.Contains(href, "/docs/") {
			continue
		}

		snippet := extractTextFromTag(line)
		if snippet == "" {
			snippet = href
		}

		results = append(results, toolkit.SearchResult{
			Source:  sourceName,
			Path:    href,
			Snippet: snippet,
		})
	}
	return results
}

// htmlToText strips HTML tags and decodes common entities.
func htmlToText(html string) string {
	var b strings.Builder
	inTag := false
	inScript := false

	for i := 0; i < len(html); i++ {
		ch := html[i]
		switch {
		case ch == '<':
			inTag = true
			tagEnd := strings.IndexByte(html[i:], '>')
			if tagEnd > 0 {
				tagContent := strings.ToLower(html[i+1 : i+tagEnd])
				if strings.HasPrefix(tagContent, "script") || strings.HasPrefix(tagContent, "style") {
					inScript = true
				}
				if strings.HasPrefix(tagContent, "/script") || strings.HasPrefix(tagContent, "/style") {
					inScript = false
				}
				if tagContent == "br" || tagContent == "br/" || strings.HasPrefix(tagContent, "p") ||
					strings.HasPrefix(tagContent, "/p") || strings.HasPrefix(tagContent, "div") ||
					strings.HasPrefix(tagContent, "/div") || strings.HasPrefix(tagContent, "li") {
					b.WriteByte('\n')
				}
			}
		case ch == '>':
			inTag = false
		case !inTag && !inScript:
			b.WriteByte(ch)
		}
	}

	text := b.String()
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", `"`)
	text = strings.ReplaceAll(text, "&#39;", "'")
	text = strings.ReplaceAll(text, "&nbsp;", " ")

	// Collapse excessive whitespace
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}

	return strings.TrimSpace(text)
}

func extractTextFromTag(line string) string {
	start := strings.Index(line, ">")
	if start < 0 {
		return ""
	}
	end := strings.LastIndex(line, "<")
	if end <= start {
		return ""
	}
	text := htmlToText(line[start+1 : end])
	if len(text) > 200 {
		text = text[:200] + "..."
	}
	return text
}

func defaultCacheDir() string {
	home, _ := os.UserHomeDir()
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".cache", "origami", "docs")
}
