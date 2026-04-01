package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"

	"github.com/dpopsuev/jericho/bugle"
	"github.com/dpopsuev/jericho/orchestrate"
)

const (
	logKeyOrchestrateEndpoint = "endpoint"
	logKeyOrchestrateSession  = "session"
	logKeyOrchestrateWorker   = "worker"
	logKeyOrchestrateError    = "error"
)

func orchestrateCmd(args []string) error {
	fs := flag.NewFlagSet("orchestrate", flag.ContinueOnError)
	endpoint := fs.String("endpoint", envOr("BUGLE_ENDPOINT", "http://localhost:9000/mcp"), "MCP endpoint")
	session := fs.String("session", "", "session ID (required)")
	workerID := fs.String("worker", "worker-1", "worker identity")
	tool := fs.String("tool", "bugle", "MCP tool name")
	pullAction := fs.String("pull-action", "pull", "action name for pulling work")
	pushAction := fs.String("push-action", "push", "action name for submitting results")
	sessionKey := fs.String("session-key", "session_id", "session key name")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if *session == "" {
		slog.ErrorContext(ctx, "--session is required")
		fs.Usage()
		return flag.ErrHelp
	}

	// Connect to MCP endpoint.
	mcpSession, err := orchestrate.ConnectEndpoint(ctx, *endpoint, *workerID)
	if err != nil {
		return err
	}
	defer mcpSession.Close()

	slog.InfoContext(ctx, "orchestrate worker starting",
		slog.String(logKeyOrchestrateEndpoint, *endpoint),
		slog.String(logKeyOrchestrateSession, *session),
		slog.String(logKeyOrchestrateWorker, *workerID),
	)

	// Use a simple echo responder — consumers override via MCP tool.
	responder := &echoResponder{}

	cfg := orchestrate.WorkerConfig{
		ToolName:   *tool,
		PullAction: *pullAction,
		PushAction: *pushAction,
		SessionKey: *sessionKey,
	}

	return orchestrate.RunWorker(ctx, mcpSession, responder, *session, *workerID, cfg)
}

// echoResponder is the default responder — echoes the prompt back.
// Consumers replace this by providing their own Responder via the MCP tool.
type echoResponder struct{}

func (e *echoResponder) RespondTo(_ context.Context, prompt string) (string, error) {
	return prompt, nil
}

// Compile-time check.
var _ bugle.Responder = (*echoResponder)(nil)
