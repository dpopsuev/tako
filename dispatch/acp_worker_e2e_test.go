//go:build integration

package dispatch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/dpopsuev/tangle"
	"github.com/dpopsuev/tangle/signal"
)

// ollamaAvailable checks if Ollama is reachable.
func ollamaAvailable(endpoint string) bool {
	resp, err := http.Get(endpoint + "/api/tags") //nolint:noctx // test helper
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// TestACPWorker_OllamaE2E validates the full A2A dispatch path with a real
// local LLM. Flow: MuxDispatcher.Dispatch → ACPWorkerDispatcher.workerLoop →
// Broker.Pick+Spawn → Actor.Perform (Ollama) → SubmitArtifact → response.
func TestACPWorker_OllamaE2E(t *testing.T) {
	const endpoint = "http://localhost:11434"
	if !ollamaAvailable(endpoint) {
		t.Skip("Ollama not available at " + endpoint)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	mux := NewMuxDispatcher(ctx)
	bus := signal.NewMemBus()

	// Use a mock broker with an Ollama-backed actor.
	broker := &ollamaBroker{endpoint: endpoint, model: "qwen3:1.7b"}

	acpDisp := NewACPWorkerDispatcher(mux, broker, "worker", 1,
		WithACPWorkerBus(bus),
	)
	go func() {
		if err := acpDisp.Run(ctx); err != nil {
			t.Logf("ACP dispatcher: %v", err)
		}
	}()

	// Dispatch a prompt and wait for the response.
	var response []byte
	var dispatchErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		response, dispatchErr = mux.Dispatch(ctx, Context{
			CaseID:        "e2e-1",
			Step:          "generate",
			PromptContent: "Reply with exactly one word: hello",
		})
	}()

	wg.Wait()

	if dispatchErr != nil {
		t.Fatalf("Dispatch: %v", dispatchErr)
	}
	if len(response) == 0 {
		t.Fatal("empty response from Ollama")
	}

	t.Logf("Ollama response (%d bytes): %s", len(response), string(response))
}

// ollamaBroker is a minimal Broker that creates ollamaActors.
type ollamaBroker struct {
	endpoint string
	model    string
}

func (b *ollamaBroker) Pick(_ context.Context, prefs troupe.Preferences) ([]troupe.ActorConfig, error) {
	count := prefs.Count
	if count <= 0 {
		count = 1
	}
	configs := make([]troupe.ActorConfig, count)
	for i := range count {
		configs[i] = troupe.ActorConfig{Model: b.model, Role: prefs.Role}
	}
	return configs, nil
}

func (b *ollamaBroker) Spawn(_ context.Context, cfg troupe.ActorConfig) (troupe.Actor, error) {
	return &ollamaActor{endpoint: b.endpoint, model: cfg.Model}, nil
}

func (b *ollamaBroker) Discover(_ string) []troupe.AgentInfo {
	return nil
}

// ollamaActor calls Ollama's /api/generate endpoint.
type ollamaActor struct {
	endpoint string
	model    string
}

func (a *ollamaActor) Perform(ctx context.Context, prompt string) (string, error) {
	payload := map[string]any{
		"model":  a.model,
		"prompt": prompt,
		"stream": false,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.endpoint+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Response string `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("ollama decode: %w", err)
	}
	return result.Response, nil
}

func (a *ollamaActor) Ready() bool                  { return true }
func (a *ollamaActor) Kill(_ context.Context) error { return nil }
