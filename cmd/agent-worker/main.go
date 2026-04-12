// Command agent-worker is an MCP client that processes circuit steps by
// piping prompts to a Bugle ACP agent and submitting the responses.
// It does NOT start circuits — the orchestrator does that.
//
// Usage:
//
//	agent-worker --gateway http://localhost:9000/mcp --agent cursor
//	ORIGAMI_AGENT=cursor GATEWAY_ENDPOINT=http://localhost:9000/mcp agent-worker
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/troupe"
	"github.com/dpopsuev/troupe/broker"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	gateway := flag.String("gateway", envOr("GATEWAY_ENDPOINT", "http://localhost:9000/mcp"), "MCP gateway endpoint")
	agent := flag.String("agent", envOr("ORIGAMI_AGENT", "cursor"), "ACP agent name (CLI command)")
	sessionID := flag.String("session", envOr("ORIGAMI_SESSION", ""), "session ID to join (required)")
	flag.Parse()

	if *sessionID == "" {
		slog.ErrorContext(context.Background(), "session ID required: set --session or ORIGAMI_SESSION")
		os.Exit(1)
	}

	if err := run(*gateway, *agent, *sessionID); err != nil {
		slog.ErrorContext(context.Background(), "agent-worker failed", slog.Any(circuit.LogKeyError, err))
		os.Exit(1)
	}
}

func run(gateway, agentName, sessionID string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Spawn ACP agent via Bugle.
	// ACP launcher absorbed into Broker
	broker := broker.New("")
	actor, err := broker.Spawn(ctx, troupe.ActorConfig{
		Model: agentName,
		Role:  "worker",
	})
	if err != nil {
		return fmt.Errorf("spawn agent %q: %w", agentName, err)
	}
	slog.InfoContext(ctx, "agent spawned", slog.Any(circuit.LogKeyAgent, agentName))

	// Connect to MCP gateway.
	transport := &sdkmcp.StreamableClientTransport{Endpoint: gateway}
	client := sdkmcp.NewClient(
		&sdkmcp.Implementation{Name: "origami-agent-worker", Version: "v0.1.0"},
		nil,
	)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("connect to gateway: %w", err)
	}
	defer session.Close()
	slog.InfoContext(ctx, "connected to gateway", slog.Any(circuit.LogKeyEndpoint, gateway))

	// Step/submit loop — pull steps, pipe to agent, submit artifacts.
	for {
		nextResult, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
			Name: "circuit",
			Arguments: mustMarshal(map[string]any{
				"action":     "step",
				"session_id": sessionID,
			}),
		})
		if err != nil {
			return fmt.Errorf("circuit/step: %w", err)
		}

		nextText := textContent(nextResult)
		var step struct {
			Done      bool   `json:"done"`
			Available bool   `json:"available"`
			Step      string `json:"step"`
			Prompt    string `json:"prompt"`
		}
		if err := json.Unmarshal([]byte(nextText), &step); err != nil {
			return fmt.Errorf("parse step response: %w", err)
		}

		if step.Done {
			slog.InfoContext(ctx, "circuit complete")
			break
		}
		if !step.Available {
			continue
		}

		slog.InfoContext(ctx, "processing step",
			slog.Any(circuit.LogKeyStep, step.Step))

		// Pipe prompt to ACP agent.
		response, err := actor.Perform(ctx, step.Prompt)
		if err != nil {
			slog.ErrorContext(ctx, "agent failed",
				slog.Any(circuit.LogKeyStep, step.Step),
				slog.Any(circuit.LogKeyError, err))
			continue
		}

		// Submit artifact.
		submitResult, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
			Name: "circuit",
			Arguments: mustMarshal(map[string]any{
				"action":     "submit",
				"session_id": sessionID,
				"step":       step.Step,
				"fields":     json.RawMessage(response),
			}),
		})
		if err != nil {
			return fmt.Errorf("circuit/submit %s: %w", step.Step, err)
		}
		if submitResult.IsError {
			slog.WarnContext(ctx, "submit warning",
				slog.Any(circuit.LogKeyStep, step.Step),
				slog.Any(circuit.LogKeyDetail, textContent(submitResult)))
		} else {
			slog.InfoContext(ctx, "step submitted",
				slog.Any(circuit.LogKeyStep, step.Step))
		}
	}

	return nil
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
