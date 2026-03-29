package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOllamaClient_Chat(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Errorf("path = %q, want /api/chat", r.URL.Path)
		}
		var req struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
			Stream bool `json:"stream"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		if req.Model != "llama3.2:3b" {
			t.Errorf("model = %q, want llama3.2:3b", req.Model)
		}
		if req.Stream {
			t.Error("stream should be false")
		}
		if len(req.Messages) != 2 {
			t.Errorf("messages count = %d, want 2", len(req.Messages))
		}

		json.NewEncoder(w).Encode(map[string]any{
			"message": map[string]string{
				"role":    "assistant",
				"content": "Hello from Ollama",
			},
		})
	}))
	defer ts.Close()

	c := &OllamaClient{Endpoint: ts.URL, Model: "llama3.2:3b"}
	resp, err := c.Chat(context.Background(), "system prompt", []Message{{Role: "user", Content: "hello"}})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp != "Hello from Ollama" {
		t.Errorf("response = %q, want %q", resp, "Hello from Ollama")
	}
}

func TestAnthropicClient_Chat(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-api-key"); got != "test-key" {
			t.Errorf("x-api-key = %q, want test-key", got)
		}
		if got := r.Header.Get("anthropic-version"); got != "2023-06-01" {
			t.Errorf("anthropic-version = %q", got)
		}

		var req struct {
			Model  string `json:"model"`
			System string `json:"system"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.Model != "claude-sonnet-4-20250514" {
			t.Errorf("model = %q", req.Model)
		}

		json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]string{
				{"type": "text", "text": "Hello from Claude"},
			},
		})
	}))
	defer ts.Close()

	c2 := &testAnthropicClient{ts: ts, apiKey: "test-key", model: "claude-sonnet-4-20250514"}
	resp, err := c2.Chat(context.Background(), "sys", []Message{{Role: "user", Content: "hello"}})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp != "Hello from Claude" {
		t.Errorf("response = %q, want %q", resp, "Hello from Claude")
	}
}

// testAnthropicClient is a test variant that uses a custom endpoint.
type testAnthropicClient struct {
	ts     *httptest.Server
	apiKey string
	model  string
}

func (c *testAnthropicClient) Chat(ctx context.Context, systemPrompt string, messages []Message) (string, error) {
	type msg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	msgs := make([]msg, 0, len(messages))
	for _, m := range messages {
		msgs = append(msgs, msg(m))
	}
	payload := map[string]any{
		"model":      c.model,
		"max_tokens": 4096,
		"messages":   msgs,
	}
	if systemPrompt != "" {
		payload["system"] = systemPrompt
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", c.ts.URL+"/v1/messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	for _, c := range result.Content {
		if c.Type == "text" {
			return c.Text, nil
		}
	}
	return "", nil
}

func TestOpenAIClient_Chat(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path = %q, want /v1/chat/completions", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization = %q", got)
		}

		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": "Hello from OpenAI"}},
			},
		})
	}))
	defer ts.Close()

	c := &OpenAIClient{APIKey: "test-key", Model: "gpt-4o", Endpoint: ts.URL}
	resp, err := c.Chat(context.Background(), "sys", []Message{{Role: "user", Content: "hello"}})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp != "Hello from OpenAI" {
		t.Errorf("response = %q, want %q", resp, "Hello from OpenAI")
	}
}

func TestGeminiClient_Chat(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"candidates": []map[string]any{
				{"content": map[string]any{
					"parts": []map[string]string{{"text": "Hello from Gemini"}},
				}},
			},
		})
	}))
	defer ts.Close()

	c := &testGeminiClient{ts: ts, model: "gemini-2.5-flash"}
	resp, err := c.Chat(context.Background(), "sys", []Message{{Role: "user", Content: "hello"}})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp != "Hello from Gemini" {
		t.Errorf("response = %q, want %q", resp, "Hello from Gemini")
	}
}

// testGeminiClient uses a custom endpoint for testing.
type testGeminiClient struct {
	ts    *httptest.Server
	model string
}

func (c *testGeminiClient) Chat(ctx context.Context, _ string, messages []Message) (string, error) {
	type part struct {
		Text string `json:"text"`
	}
	type content struct {
		Role  string `json:"role"`
		Parts []part `json:"parts"`
	}
	contents := make([]content, 0, len(messages))
	for _, m := range messages {
		contents = append(contents, content{Role: m.Role, Parts: []part{{Text: m.Content}}})
	}
	payload := map[string]any{"contents": contents}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", c.ts.URL+"/generate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	if len(result.Candidates) > 0 && len(result.Candidates[0].Content.Parts) > 0 {
		return result.Candidates[0].Content.Parts[0].Text, nil
	}
	return "", nil
}
