package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dpopsuev/origami/agentport"
	"github.com/dpopsuev/origami/circuit"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	errSessionRequired   = errors.New("--session is required (session ID or heraldic alias)")
	errWorkersFailed     = errors.New("workers failed")
	errCircuitStepFailed = errors.New("circuit step failed")
)

func workersCmd(args []string) error {
	fs := flag.NewFlagSet("workers", flag.ContinueOnError)
	gateway := fs.String("gateway", envOr("GATEWAY_ENDPOINT", "http://localhost:9000/mcp"), "MCP gateway endpoint")
	sessionID := fs.String("session", envOr("ORIGAMI_SESSION", ""), "session ID or alias (required)")
	agentName := fs.String("agent", envOr("ORIGAMI_AGENT", "claude"), "agent CLI name")
	count := fs.Int("count", 4, "number of workers to spawn")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *sessionID == "" {
		return errSessionRequired
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	slog.InfoContext(ctx, "spawning workers",
		slog.Any(circuit.LogKeySessionID, *sessionID),
		slog.Any(circuit.LogKeyAgent, *agentName),
		slog.Any(circuit.LogKeyCount, *count),
		slog.Any(circuit.LogKeyEndpoint, *gateway))

	var wg sync.WaitGroup
	var completed atomic.Int64
	var errCount atomic.Int64
	start := time.Now()

	for i := range *count {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			workerName := fmt.Sprintf("worker-%d", workerID+1)
			err := runWorker(ctx, *gateway, *agentName, *sessionID, workerName)
			if err != nil {
				errCount.Add(1)
				slog.ErrorContext(ctx, "worker failed",
					slog.Any(circuit.LogKeyWorker, workerName),
					slog.Any(circuit.LogKeyError, err))
			} else {
				completed.Add(1)
				slog.InfoContext(ctx, "worker done",
					slog.Any(circuit.LogKeyWorker, workerName))
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	slog.InfoContext(ctx, "all workers finished",
		slog.Any(circuit.LogKeyStepsCompleted, completed.Load()),
		slog.Any(circuit.LogKeyError, errCount.Load()),
		slog.Any(circuit.LogKeyElapsedDur, elapsed.Round(time.Second).String()))

	if errCount.Load() > 0 {
		return fmt.Errorf("%w: %d/%d", errWorkersFailed, errCount.Load(), *count)
	}
	return nil
}

func runWorker(ctx context.Context, gateway, agentName, sessionID, workerName string) error {
	// ACP launcher absorbed into Broker
	broker := agentport.NewBroker("")
	actor, err := broker.Spawn(ctx, agentport.ActorConfig{
		Model: agentName,
		Role:  "worker",
	})
	if err != nil {
		return fmt.Errorf("spawn agent %q: %w", agentName, err)
	}
	defer actor.Kill(ctx) //nolint:errcheck // best-effort cleanup

	slog.InfoContext(ctx, "agent spawned",
		slog.Any(circuit.LogKeyWorker, workerName),
		slog.Any(circuit.LogKeyAgent, agentName))

	transport := &sdkmcp.StreamableClientTransport{Endpoint: gateway}
	client := sdkmcp.NewClient(
		&sdkmcp.Implementation{Name: "origami-" + workerName, Version: "v0.1.0"},
		nil,
	)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("connect to gateway: %w", err)
	}
	defer session.Close()

	steps := 0
	for {
		nextResult, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
			Name:      "circuit",
			Arguments: workerMarshal(map[string]any{"action": "step", "session_id": sessionID, "timeout_ms": 30000}),
		})
		if err != nil {
			return fmt.Errorf("circuit/step: %w", err)
		}

		if nextResult.IsError {
			return fmt.Errorf("%w: %s", errCircuitStepFailed, workerTextContent(nextResult))
		}

		nextText := workerTextContent(nextResult)
		var step struct {
			Done       bool   `json:"done"`
			Available  bool   `json:"available"`
			Step       string `json:"step"`
			Prompt     string `json:"prompt_content"`
			DispatchID int64  `json:"dispatch_id"`
		}
		if err := json.Unmarshal([]byte(nextText), &step); err != nil {
			return fmt.Errorf("parse step: %w", err)
		}

		if step.Done {
			slog.InfoContext(ctx, "circuit done",
				slog.Any(circuit.LogKeyWorker, workerName),
				slog.Any(circuit.LogKeyStepsCompleted, steps))
			return nil
		}
		if !step.Available {
			continue
		}

		response, err := actor.Perform(ctx, step.Prompt)
		if err != nil {
			slog.ErrorContext(ctx, "agent ask failed",
				slog.Any(circuit.LogKeyWorker, workerName),
				slog.Any(circuit.LogKeyStep, step.Step),
				slog.Any(circuit.LogKeyError, err))
			continue
		}

		_, err = session.CallTool(ctx, &sdkmcp.CallToolParams{
			Name: "circuit",
			Arguments: workerMarshal(map[string]any{
				"action":      "submit",
				"session_id":  sessionID,
				"dispatch_id": step.DispatchID,
				"step":        step.Step,
				"fields":      json.RawMessage(response),
			}),
		})
		if err != nil {
			slog.WarnContext(ctx, "submit failed",
				slog.Any(circuit.LogKeyWorker, workerName),
				slog.Any(circuit.LogKeyStep, step.Step),
				slog.Any(circuit.LogKeyError, err))
		}
		steps++
	}
}

func workerMarshal(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func workerTextContent(result *sdkmcp.CallToolResult) string {
	for _, c := range result.Content {
		if tc, ok := c.(*sdkmcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
