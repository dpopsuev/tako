// Command llm-worker is an MCP client that processes circuit steps by
// sending prompts to an LLM provider and submitting the responses.
//
// Usage: llm-worker --gateway-endpoint http://localhost:9000/mcp --provider ollama --model llama3.2:3b
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	gatewayEndpoint := flag.String("gateway-endpoint", envOr("GATEWAY_ENDPOINT", "http://localhost:9000/mcp"), "Gateway MCP endpoint")
	provider := flag.String("provider", envOr("LLM_PROVIDER", "ollama"), "LLM provider: ollama, claude, gemini, openai")
	model := flag.String("model", envOr("LLM_MODEL", ""), "Model name")
	llmEndpoint := flag.String("llm-endpoint", envOr("LLM_ENDPOINT", ""), "LLM endpoint URL (provider-specific)")
	scenario := flag.String("scenario", envOr("SCENARIO", "ptp"), "Scenario name for circuit start")
	backend := flag.String("backend", envOr("BACKEND", "stub"), "Backend type for circuit start")
	flag.Parse()

	llm, err := NewLLMClient(*provider, *model, *llmEndpoint)
	if err != nil {
		log.Fatalf("create LLM client: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	transport := &sdkmcp.StreamableClientTransport{Endpoint: *gatewayEndpoint}
	client := sdkmcp.NewClient(
		&sdkmcp.Implementation{Name: "origami-llm-worker", Version: "v0.1.0"},
		nil,
	)

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		log.Fatalf("connect to gateway: %v", err)
	}
	defer session.Close()
	log.Printf("connected to gateway at %s", *gatewayEndpoint)

	startResult, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name: "circuit",
		Arguments: mustMarshal(map[string]any{
			"action": "start",
			"extra": map[string]any{
				"scenario": *scenario,
				"backend":  *backend,
			},
		}),
	})
	if err != nil {
		log.Fatalf("circuit/start: %v", err)
	}
	if startResult.IsError {
		log.Fatalf("circuit/start error: %s", textContent(startResult))
	}
	log.Printf("circuit started: %s", textContent(startResult))

	for {
		nextResult, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
			Name:      "circuit",
			Arguments: mustMarshal(map[string]any{"action": "step"}),
		})
		if err != nil {
			log.Fatalf("circuit/step: %v", err)
		}

		nextText := textContent(nextResult)
		var step struct {
			Done   bool   `json:"done"`
			Step   string `json:"step"`
			Prompt string `json:"prompt"`
		}
		if err := json.Unmarshal([]byte(nextText), &step); err != nil {
			log.Printf("get_next_step response: %s", nextText)
			log.Fatalf("parse get_next_step: %v", err)
		}

		if step.Done {
			log.Println("circuit complete")
			break
		}

		log.Printf("processing step: %s", step.Step)

		llmResponse, err := llm.Chat(ctx, "", []Message{
			{Role: "user", Content: step.Prompt},
		})
		if err != nil {
			log.Fatalf("LLM chat for step %s: %v", step.Step, err)
		}

		submitResult, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
			Name: "circuit",
			Arguments: mustMarshal(map[string]any{
				"action":   "submit",
				"step":     step.Step,
				"artifact": llmResponse,
			}),
		})
		if err != nil {
			log.Fatalf("circuit/submit %s: %v", step.Step, err)
		}
		if submitResult.IsError {
			log.Printf("circuit/submit %s warning: %s", step.Step, textContent(submitResult))
		} else {
			log.Printf("submitted step %s", step.Step)
		}
	}

	reportResult, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "circuit",
		Arguments: mustMarshal(map[string]any{"action": "report"}),
	})
	if err != nil {
		log.Fatalf("circuit/report: %v", err)
	}
	fmt.Println(textContent(reportResult))
}

func textContent(result *sdkmcp.CallToolResult) string {
	for _, c := range result.Content {
		if tc, ok := c.(*sdkmcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}

func mustMarshal(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
