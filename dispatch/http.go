package dispatch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/dpopsuev/origami/agentport"
)

const (
	logKeyHTTPCaseID       = "case_id"
	logKeyHTTPStep         = "step"
	logKeyHTTPURL          = "url"
	logKeyHTTPResponseSize = "response_bytes"
)

var (
	ErrHTTPSRequired = errors.New("dispatch/http: base URL must use HTTPS")
	ErrAPIKeyMissing = errors.New("dispatch/http: API key environment variable not set")
	ErrNoChoices     = errors.New("dispatch/http: response has no choices")
)

// HTTPDispatcher sends prompts to an OpenAI-compatible chat completions endpoint.
type HTTPDispatcher struct {
	BaseURL    string
	Model      string
	HTTPClient *http.Client
	Logger     *slog.Logger
	apiKeyEnv  string
}

type HTTPOption func(*HTTPDispatcher)

func WithHTTPClient(c *http.Client) HTTPOption { return func(d *HTTPDispatcher) { d.HTTPClient = c } }
func WithModel(model string) HTTPOption        { return func(d *HTTPDispatcher) { d.Model = model } }
func WithAPIKeyEnv(env string) HTTPOption      { return func(d *HTTPDispatcher) { d.apiKeyEnv = env } }
func WithHTTPLogger(l *slog.Logger) HTTPOption { return func(d *HTTPDispatcher) { d.Logger = l } }

func NewHTTPDispatcher(baseURL string, opts ...HTTPOption) (*HTTPDispatcher, error) {
	if !strings.HasPrefix(baseURL, "https://") {
		if !strings.HasPrefix(baseURL, "http://localhost") && !strings.HasPrefix(baseURL, "http://127.0.0.1") {
			return nil, fmt.Errorf("%w (got %q)", ErrHTTPSRequired, baseURL)
		}
	}
	d := &HTTPDispatcher{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		Model:      "gpt-4",
		HTTPClient: http.DefaultClient,
		Logger:     agentport.DiscardLogger(),
		apiKeyEnv:  "OPENAI_API_KEY",
	}
	for _, o := range opts {
		o(d)
	}
	return d, nil
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type chatResponse struct {
	Choices []chatChoice `json:"choices"`
}
type chatChoice struct {
	Message chatMessage `json:"message"`
}

func (d *HTTPDispatcher) Dispatch(ctx context.Context, dctx agentport.Context) ([]byte, error) {
	prompt, err := os.ReadFile(dctx.PromptPath)
	if err != nil {
		return nil, fmt.Errorf("dispatch/http: read prompt: %w", err)
	}

	apiKey := os.Getenv(d.apiKeyEnv)
	if apiKey == "" {
		return nil, fmt.Errorf("%w: %s", ErrAPIKeyMissing, d.apiKeyEnv)
	}

	reqBody := chatRequest{
		Model:    d.Model,
		Messages: []chatMessage{{Role: "user", Content: string(prompt)}},
	}
	body, _ := json.Marshal(reqBody)

	url := d.BaseURL + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("dispatch/http: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	d.Logger.InfoContext(ctx, "dispatching HTTP request",
		slog.String(logKeyHTTPCaseID, dctx.CaseID),
		slog.String(logKeyHTTPStep, dctx.Step),
		slog.String(logKeyHTTPURL, url),
	)

	resp, err := d.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("dispatch/http: POST %s: %w", url, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("dispatch/http: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("dispatch/http: %s returned %d: %s", url, resp.StatusCode, string(respBody)) //nolint:err113 // dynamic HTTP status
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("dispatch/http: parse response: %w", err)
	}
	if len(chatResp.Choices) == 0 {
		return nil, ErrNoChoices
	}

	content := chatResp.Choices[0].Message.Content
	if err := os.WriteFile(dctx.ArtifactPath, []byte(content), 0o600); err != nil {
		return nil, fmt.Errorf("dispatch/http: write artifact: %w", err)
	}

	d.Logger.InfoContext(ctx, "HTTP dispatch complete",
		slog.String(logKeyHTTPCaseID, dctx.CaseID),
		slog.String(logKeyHTTPStep, dctx.Step),
		slog.Int(logKeyHTTPResponseSize, len(content)),
	)

	return []byte(content), nil
}
