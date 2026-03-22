package transformers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dpopsuev/origami/engine"
)

// HTTPTransformer makes HTTP requests and returns the JSON response body.
type HTTPTransformer struct {
	client       *http.Client
	allowedHosts []string // empty = all hosts allowed
}

// HTTPOption configures the HTTP transformer.
type HTTPOption func(*HTTPTransformer)

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(c *http.Client) HTTPOption {
	return func(t *HTTPTransformer) { t.client = c }
}

// WithAllowedHosts restricts requests to specific hosts (SSRF mitigation).
func WithAllowedHosts(hosts ...string) HTTPOption {
	return func(t *HTTPTransformer) { t.allowedHosts = hosts }
}

// NewHTTP creates a transformer that makes HTTP requests.
func NewHTTP(opts ...HTTPOption) *HTTPTransformer {
	t := &HTTPTransformer{
		client: &http.Client{Timeout: 30 * time.Second},
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *HTTPTransformer) Name() string        { return "http" }
func (t *HTTPTransformer) Deterministic() bool { return true }

func (t *HTTPTransformer) Transform(ctx context.Context, tc *engine.TransformerContext) (any, error) {
	url, _ := metaString(tc, "url")
	if url == "" {
		return nil, fmt.Errorf("http transformer: 'url' is required in meta")
	}

	if len(t.allowedHosts) > 0 {
		allowed := false
		for _, h := range t.allowedHosts {
			if strings.Contains(url, h) {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, fmt.Errorf("http transformer: host not in allowlist for url %q", url)
		}
	}

	method, _ := metaString(tc, "method")
	if method == "" {
		method = "GET"
	}

	var body io.Reader
	if tc.Input != nil {
		data, err := json.Marshal(tc.Input)
		if err == nil {
			body = strings.NewReader(string(data))
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("http transformer: create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if headers, ok := tc.Meta["headers"].(map[string]any); ok {
		for k, v := range headers {
			if s, ok := v.(string); ok {
				req.Header.Set(k, s)
			}
		}
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http transformer: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("http transformer: read body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("http transformer: status %d: %s", resp.StatusCode, string(respBody))
	}

	var result any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return map[string]any{"body": string(respBody), "status": resp.StatusCode}, nil
	}

	return result, nil
}

func metaString(tc *engine.TransformerContext, key string) (string, bool) {
	if tc.Meta == nil {
		return "", false
	}
	v, ok := tc.Meta[key].(string)
	return v, ok
}
