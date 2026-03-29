package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/dpopsuev/origami/circuit"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func calibrateCmd(args []string) error {
	fs := flag.NewFlagSet("calibrate", flag.ContinueOnError)
	endpoint := fs.String("endpoint", "http://localhost:9300/mcp", "MCP endpoint to calibrate against")
	scenario := fs.String("scenario", "ptp", "Scenario name passed in circuit start extra")
	backend := fs.String("backend", "llm", "Backend type passed in circuit start extra")
	parallel := fs.Int("parallel", 4, "Number of parallel workers")
	timeout := fs.Duration("timeout", 30*time.Minute, "Overall calibration timeout")
	cliCommand := fs.String("cli", "claude", "CLI command for LLM processing")
	cliArgs := fs.String("cli-args", "--print", "Arguments for CLI command (space-separated)")
	mode := fs.String("mode", "offline", "Calibration mode: offline or online")
	traceLevel := fs.String("trace-level", "debug", "Trace recording level: info, debug, or trace")
	if err := fs.Parse(args); err != nil {
		return err
	}

	logger := slog.Default().With(slog.Any(circuit.LogKeyComponent, circuit.LogComponentCalibrate))

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	// Connect to the MCP server.
	transport := &sdkmcp.StreamableClientTransport{Endpoint: *endpoint}
	client := sdkmcp.NewClient(
		&sdkmcp.Implementation{Name: "origami-calibrate", Version: "v0.1.0"},
		nil,
	)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("connect to %s: %w", *endpoint, err)
	}
	defer session.Close()
	logger.InfoContext(ctx, circuit.LogConnected, slog.Any(circuit.LogKeyEndpoint, *endpoint))

	// Start circuit.
	extra := map[string]any{
		"scenario":    *scenario,
		"backend":     *backend,
		"mode":        *mode,
		"trace_level": *traceLevel,
	}
	startResult, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "circuit",
		Arguments: mustMarshalCal(map[string]any{"action": "start", "parallel": *parallel, "extra": extra}),
	})
	if err != nil {
		return fmt.Errorf("circuit/start: %w", err)
	}
	if startResult.IsError {
		return fmt.Errorf("%w: %s", ErrCircuitStart, calTextContent(startResult))
	}

	var startOut struct {
		SessionID    string `json:"session_id"`
		TotalCases   int    `json:"total_cases"`
		Scenario     string `json:"scenario"`
		WorkerPrompt string `json:"worker_prompt"`
	}
	if err := json.Unmarshal([]byte(calTextContent(startResult)), &startOut); err != nil {
		return fmt.Errorf("parse start_circuit: %w", err)
	}
	sessionID := startOut.SessionID
	logger.InfoContext(ctx, circuit.LogCircuitStarted, slog.Any(circuit.LogKeySessionID, sessionID), slog.Any(circuit.LogKeyTotalCases, startOut.TotalCases), slog.Any(circuit.LogKeyScenario, startOut.Scenario), slog.Any(circuit.LogKeyParallel, *parallel))

	// Spawn parallel workers.
	var cliArgList []string
	if *cliArgs != "" {
		cliArgList = strings.Fields(*cliArgs)
	}

	// Worker prompt contains step schemas and protocol instructions.
	// Prepend it to each step's prompt so the CLI knows the expected output format.
	workerPreamble := startOut.WorkerPrompt

	var wg sync.WaitGroup
	errCh := make(chan error, *parallel)
	stepsCompleted := 0
	var mu sync.Mutex

	for i := range *parallel {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			err := runCalibrateWorker(ctx, session, sessionID, workerID,
				*cliCommand, cliArgList, workerPreamble, logger, &mu, &stepsCompleted)
			if err != nil {
				errCh <- fmt.Errorf("worker-%d: %w", workerID, err)
			}
		}(i)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		logger.ErrorContext(ctx, circuit.LogWorkerFailed, slog.Any(circuit.LogKeyError, err))
	}

	logger.InfoContext(ctx, circuit.LogAllWorkersDone, slog.Any(circuit.LogKeyStepsCompleted, stepsCompleted))

	// Get report.
	reportResult, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "circuit",
		Arguments: mustMarshalCal(map[string]any{"action": "report", circuit.ProtoKeySessionID: sessionID}),
	})
	if err != nil {
		return fmt.Errorf("circuit/report: %w", err)
	}
	if reportResult.IsError {
		return fmt.Errorf("%w: %s", ErrCircuitReport, calTextContent(reportResult))
	}

	fmt.Println(calTextContent(reportResult))
	return nil
}

func runCalibrateWorker(
	ctx context.Context,
	session *sdkmcp.ClientSession,
	sessionID string,
	workerID int,
	cliCommand string, cliArgs []string,
	workerPreamble string,
	logger *slog.Logger,
	mu *sync.Mutex, stepsCompleted *int,
) error {
	wlog := logger.With(slog.Any(circuit.LogKeyWorkerID, workerID))

	// Emit worker_started signal.
	_, _ = session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name: "signal",
		Arguments: mustMarshalCal(map[string]any{
			"action":                  "emit",
			circuit.ProtoKeySessionID: sessionID,
			"event":                   "worker_started",
			"agent":                   "worker",
			"meta":                    map[string]any{"worker_id": fmt.Sprintf("w%d", workerID)},
		}),
	})

	defer func() {
		_, _ = session.CallTool(ctx, &sdkmcp.CallToolParams{
			Name: "signal",
			Arguments: mustMarshalCal(map[string]any{
				"action":                  "emit",
				circuit.ProtoKeySessionID: sessionID,
				"event":                   "worker_stopped",
				"agent":                   "worker",
				"meta":                    map[string]any{"worker_id": fmt.Sprintf("w%d", workerID)},
			}),
		})
	}()

	for {
		// Pull next step.
		nextResult, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
			Name: "circuit",
			Arguments: mustMarshalCal(map[string]any{
				"action":                  "step",
				circuit.ProtoKeySessionID: sessionID,
				circuit.ProtoKeyTimeoutMS: 30000,
			}),
		})
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("circuit/step: %w", err)
		}

		var step struct {
			Done          bool   `json:"done"`
			Available     bool   `json:"available"`
			CaseID        string `json:"case_id"`
			Step          string `json:"step"`
			PromptContent string `json:"prompt_content"`
			DispatchID    int64  `json:"dispatch_id"`
			Error         string `json:"error"`
		}
		if err := json.Unmarshal([]byte(calTextContent(nextResult)), &step); err != nil {
			return fmt.Errorf("parse get_next_step: %w", err)
		}

		if step.Done {
			if step.Error != "" {
				wlog.WarnContext(ctx, circuit.LogCircuitDoneErr, slog.Any(circuit.LogKeyError, step.Error))
			}
			return nil
		}

		if !step.Available {
			continue
		}

		wlog.InfoContext(ctx, circuit.LogProcessing, slog.Any(circuit.LogKeyCaseID, step.CaseID), slog.Any(circuit.LogKeyStep, step.Step), slog.Any(circuit.LogKeyDispatchID, step.DispatchID))

		// Prepend worker preamble (step schemas + output format instructions)
		// so the CLI knows what JSON fields to produce.
		fullPrompt := step.PromptContent
		if workerPreamble != "" {
			fullPrompt = workerPreamble + "\n---\n\n## Current Step: " + step.Step + "\n\n" + step.PromptContent
		}

		// Execute CLI with prompt.
		artifact, err := execCLI(ctx, cliCommand, cliArgs, fullPrompt)
		if err != nil {
			wlog.ErrorContext(ctx, circuit.LogCLIFailed, slog.Any(circuit.LogKeyCaseID, step.CaseID), slog.Any(circuit.LogKeyStep, step.Step), slog.Any(circuit.LogKeyError, err))
			continue
		}

		// Parse artifact as JSON fields.
		var fields map[string]any
		cleaned := cleanArtifactJSON(artifact)
		if err := json.Unmarshal(cleaned, &fields); err != nil {
			fields = map[string]any{"content": string(artifact)}
		}

		// Submit.
		submitResult, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
			Name: "circuit",
			Arguments: mustMarshalCal(map[string]any{
				"action":                   "submit",
				circuit.ProtoKeySessionID:  sessionID,
				circuit.ProtoKeyDispatchID: step.DispatchID,
				circuit.ProtoKeyStep:       step.Step,
				circuit.ProtoKeyFields:     fields,
			}),
		})
		if err != nil {
			return fmt.Errorf("circuit/submit %s/%s: %w", step.CaseID, step.Step, err)
		}
		if submitResult.IsError {
			wlog.WarnContext(ctx, circuit.LogSubmitRejected, slog.Any(circuit.LogKeyCaseID, step.CaseID), slog.Any(circuit.LogKeyStep, step.Step), slog.Any(circuit.LogKeyError, calTextContent(submitResult)))
			continue
		}

		mu.Lock()
		*stepsCompleted++
		mu.Unlock()
		wlog.InfoContext(ctx, circuit.LogSubmitted, slog.Any(circuit.LogKeyCaseID, step.CaseID), slog.Any(circuit.LogKeyStep, step.Step))
	}
}

func execCLI(ctx context.Context, command string, args []string, prompt string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stdin = strings.NewReader(prompt)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("exec %s: %w", command, err)
	}
	return out, nil
}

func cleanArtifactJSON(data []byte) []byte {
	s := bytes.TrimSpace(data)
	if bytes.HasPrefix(s, []byte("```")) {
		if idx := bytes.IndexByte(s, '\n'); idx >= 0 {
			s = s[idx+1:]
		}
		if bytes.HasSuffix(s, []byte("```")) {
			s = s[:len(s)-3]
		}
		s = bytes.TrimSpace(s)
	}
	return s
}

func calTextContent(result *sdkmcp.CallToolResult) string {
	for _, c := range result.Content {
		if tc, ok := c.(*sdkmcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}

func mustMarshalCal(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
