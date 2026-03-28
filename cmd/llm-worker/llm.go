package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	defaultClaudeModel    = "claude-sonnet-4-20250514"
	defaultOpenAIEndpoint = "https://api.openai.com"
	contentTypeText       = "text"
)

// Message represents a chat message with a role and content.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// LLMClient sends chat completions to an LLM provider.
type LLMClient interface {
	Chat(ctx context.Context, systemPrompt string, messages []Message) (string, error)
}

// NewLLMClient creates the appropriate client for the given provider.
func NewLLMClient(provider, model, endpoint string) (LLMClient, error) {
	switch provider {
	case "ollama":
		if endpoint == "" {
			endpoint = "http://localhost:11434"
		}
		return &OllamaClient{Endpoint: endpoint, Model: model}, nil
	case "claude":
		key := os.Getenv("ANTHROPIC_API_KEY")
		if key == "" {
			return nil, fmt.Errorf("ANTHROPIC_API_KEY required for claude provider")
		}
		if model == "" {
			model = defaultClaudeModel
		}
		return &AnthropicClient{APIKey: key, Model: model}, nil
	case "gemini":
		key := os.Getenv("GEMINI_API_KEY")
		if key == "" {
			return nil, fmt.Errorf("GEMINI_API_KEY required for gemini provider")
		}
		if model == "" {
			model = "gemini-2.5-flash"
		}
		return &GeminiClient{APIKey: key, Model: model}, nil
	case "openai":
		key := os.Getenv("OPENAI_API_KEY")
		if key == "" {
			return nil, fmt.Errorf("OPENAI_API_KEY required for openai provider")
		}
		if model == "" {
			model = "gpt-4o"
		}
		endpoint := os.Getenv("OPENAI_API_BASE")
		if endpoint == "" {
			endpoint = defaultOpenAIEndpoint
		}
		return &OpenAIClient{APIKey: key, Model: model, Endpoint: endpoint}, nil
	default:
		return nil, fmt.Errorf("unknown provider %q (known: ollama, claude, gemini, openai)", provider)
	}
}

var httpClient = &http.Client{Timeout: 5 * time.Minute}

// --- Ollama ---

// OllamaClient calls the Ollama /api/chat endpoint.
type OllamaClient struct {
	Endpoint string
	Model    string
}

func (c *OllamaClient) Chat(ctx context.Context, systemPrompt string, messages []Message) (string, error) {
	type ollamaMsg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	msgCap := len(messages)
	if systemPrompt != "" {
		msgCap++
	}
	msgs := make([]ollamaMsg, 0, msgCap)
	if systemPrompt != "" {
		msgs = append(msgs, ollamaMsg{Role: "system", Content: systemPrompt})
	}
	for _, m := range messages {
		msgs = append(msgs, ollamaMsg(m))
	}

	body, _ := json.Marshal(map[string]any{
		"model":    c.Model,
		"messages": msgs,
		"stream":   false,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", c.Endpoint+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama: status %d: %s", resp.StatusCode, b)
	}

	var result struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("ollama: decode: %w", err)
	}
	return result.Message.Content, nil
}

// --- Anthropic ---

// AnthropicClient calls the Anthropic Messages API.
type AnthropicClient struct {
	APIKey string
	Model  string
}

func (c *AnthropicClient) Chat(ctx context.Context, systemPrompt string, messages []Message) (string, error) {
	type anthropicMsg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	msgs := make([]anthropicMsg, 0, len(messages))
	for _, m := range messages {
		msgs = append(msgs, anthropicMsg(m))
	}

	payload := map[string]any{
		"model":      c.Model,
		"max_tokens": 4096,
		"messages":   msgs,
	}
	if systemPrompt != "" {
		payload["system"] = systemPrompt
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("anthropic: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("anthropic: status %d: %s", resp.StatusCode, b)
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("anthropic: decode: %w", err)
	}

	texts := make([]string, 0, len(result.Content))
	for _, c := range result.Content {
		if c.Type == contentTypeText {
			texts = append(texts, c.Text)
		}
	}
	return strings.Join(texts, ""), nil
}

// --- Gemini ---

// GeminiClient calls the Google AI generateContent API.
type GeminiClient struct {
	APIKey string
	Model  string
}

func (c *GeminiClient) Chat(ctx context.Context, systemPrompt string, messages []Message) (string, error) {
	type part struct {
		Text string `json:"text"`
	}
	type content struct {
		Role  string `json:"role"`
		Parts []part `json:"parts"`
	}

	contents := make([]content, 0, len(messages))
	for _, m := range messages {
		role := m.Role
		if role == "assistant" {
			role = "model"
		}
		contents = append(contents, content{
			Role:  role,
			Parts: []part{{Text: m.Content}},
		})
	}

	payload := map[string]any{"contents": contents}
	if systemPrompt != "" {
		payload["systemInstruction"] = map[string]any{
			"parts": []part{{Text: systemPrompt}},
		}
	}

	body, _ := json.Marshal(payload)
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", c.Model, c.APIKey)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("gemini: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("gemini: status %d: %s", resp.StatusCode, b)
	}

	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("gemini: decode: %w", err)
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("gemini: empty response")
	}
	return result.Candidates[0].Content.Parts[0].Text, nil
}

// --- OpenAI ---

// OpenAIClient calls the OpenAI Chat Completions API.
type OpenAIClient struct {
	APIKey   string
	Model    string
	Endpoint string
}

func (c *OpenAIClient) Chat(ctx context.Context, systemPrompt string, messages []Message) (string, error) {
	type openaiMsg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	msgCap := len(messages)
	if systemPrompt != "" {
		msgCap++
	}
	msgs := make([]openaiMsg, 0, msgCap)
	if systemPrompt != "" {
		msgs = append(msgs, openaiMsg{Role: "system", Content: systemPrompt})
	}
	for _, m := range messages {
		msgs = append(msgs, openaiMsg(m))
	}

	body, _ := json.Marshal(map[string]any{
		"model":    c.Model,
		"messages": msgs,
	})

	endpoint := c.Endpoint
	if endpoint == "" {
		endpoint = defaultOpenAIEndpoint
	}
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("openai: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("openai: status %d: %s", resp.StatusCode, b)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("openai: decode: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("openai: empty response")
	}
	return result.Choices[0].Message.Content, nil
}
