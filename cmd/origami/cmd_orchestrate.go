package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"

	"github.com/dpopsuev/bugle/orchestrate"
)

const (
	logKeyOrchestrateError    = "error"
	logKeyOrchestrateEndpoint = "endpoint"
	logKeyOrchestrateMode     = "mode"
)

func orchestrateCmd(args []string) error {
	fs := flag.NewFlagSet("orchestrate", flag.ContinueOnError)
	endpoint := fs.String("endpoint", envOr("BUGLE_ENDPOINT", "http://localhost:9000/mcp"), "MCP endpoint to connect workers to")
	session := fs.String("session", "", "auto-start workers for this session (optional)")
	agentName := fs.String("agent", envOr("ORIGAMI_AGENT", "claude"), "agent CLI name")
	count := fs.Int("count", 4, "number of workers (for auto-start)")
	tool := fs.String("tool", "circuit", "MCP tool name for step/submit")
	pullAction := fs.String("pull-action", "step", "action name for pulling work")
	pushAction := fs.String("push-action", "submit", "action name for submitting results")
	sessionKey := fs.String("session-key", "session_id", "session key name in arguments")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	cfg := orchestrate.WorkerConfig{
		ToolName:   *tool,
		PullAction: *pullAction,
		PushAction: *pushAction,
		SessionKey: *sessionKey,
	}
	mgr := orchestrate.NewManager(*endpoint, cfg)

	if *session != "" {
		if err := mgr.Start(ctx, *session, *agentName, *count); err != nil {
			slog.ErrorContext(ctx, "auto-start failed", slog.Any(logKeyOrchestrateError, err))
			return err
		}
	}

	slog.InfoContext(ctx, "orchestrator starting",
		slog.String(logKeyOrchestrateEndpoint, *endpoint),
		slog.String(logKeyOrchestrateMode, "stdio"))

	return orchestrate.ServeStdio(ctx, mgr)
}
