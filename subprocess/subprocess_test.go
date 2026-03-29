package subprocess_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/dpopsuev/origami/subprocess"
)

const runAsServer = "_ORIGAMI_SUBPROCESS_TEST_SERVER"

// --- Server functions (run when test binary is invoked as a subprocess) ---

type EchoParams struct {
	Message string `json:"message"`
}

func echoHandler(_ context.Context, _ *sdkmcp.CallToolRequest, args EchoParams) (*sdkmcp.CallToolResult, any, error) {
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{
			&sdkmcp.TextContent{Text: "echo: " + args.Message},
		},
	}, nil, nil
}

type AddParams struct {
	A int `json:"a"`
	B int `json:"b"`
}

func addHandler(_ context.Context, _ *sdkmcp.CallToolRequest, args AddParams) (*sdkmcp.CallToolResult, any, error) {
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{
			&sdkmcp.TextContent{Text: fmt.Sprintf("%d", args.A+args.B)},
		},
	}, nil, nil
}

var serverFuncs = map[string]func(){
	"default": runDefaultServer,
	"slow":    runSlowServer,
}

func runDefaultServer() {
	ctx := context.Background()
	server := sdkmcp.NewServer(
		&sdkmcp.Implementation{Name: "test-echo", Version: "v0.1.0"},
		nil,
	)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "echo",
		Description: "echoes input",
	}, echoHandler)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "add",
		Description: "adds two numbers",
	}, addHandler)
	if err := server.Run(ctx, &sdkmcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}

type SlowParams struct {
	DurationMS int `json:"duration_ms"`
}

func slowHandler(ctx context.Context, _ *sdkmcp.CallToolRequest, args SlowParams) (*sdkmcp.CallToolResult, any, error) {
	select {
	case <-time.After(time.Duration(args.DurationMS) * time.Millisecond):
		return &sdkmcp.CallToolResult{
			Content: []sdkmcp.Content{
				&sdkmcp.TextContent{Text: "done"},
			},
		}, nil, nil
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}
}

func runSlowServer() {
	ctx := context.Background()
	server := sdkmcp.NewServer(
		&sdkmcp.Implementation{Name: "test-slow", Version: "v0.1.0"},
		nil,
	)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "slow",
		Description: "waits for the specified duration",
	}, slowHandler)
	if err := server.Run(ctx, &sdkmcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}

// TestMain handles the fork-and-exec trick: when the env var is set,
// this binary acts as an MCP server instead of running tests.
func TestMain(m *testing.M) {
	if name := os.Getenv(runAsServer); name != "" {
		run := serverFuncs[name]
		if run == nil {
			log.Fatalf("unknown server %q", name)
		}
		os.Unsetenv(runAsServer)
		run()
		return
	}
	os.Exit(m.Run())
}

func requireExec(t *testing.T) {
	t.Helper()
	switch runtime.GOOS {
	case "darwin", "linux":
	default:
		t.Skip("unsupported OS for subprocess tests")
	}
}

func newTestServer(t *testing.T, serverName string) *subprocess.Server { //nolint:unparam // test flexibility
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return &subprocess.Server{
		BinaryPath: exe,
		Env:        []string{runAsServer + "=" + serverName},
	}
}

// --- Tests ---

func TestSubprocess_ToolCallRoundTrip(t *testing.T) {
	requireExec(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	srv := newTestServer(t, "default")
	if err := srv.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { srv.Stop(context.Background()) })

	// Call echo tool
	result, err := srv.CallTool(ctx, "echo", map[string]any{"message": "hello"})
	if err != nil {
		t.Fatalf("CallTool echo: %v", err)
	}
	got := extractText(t, result)
	if got != "echo: hello" {
		t.Errorf("echo returned %q, want %q", got, "echo: hello")
	}

	// Call add tool
	result, err = srv.CallTool(ctx, "add", map[string]any{"a": 3, "b": 7})
	if err != nil {
		t.Fatalf("CallTool add: %v", err)
	}
	got = extractText(t, result)
	if got != "10" {
		t.Errorf("add returned %q, want %q", got, "10")
	}
}

func TestSubprocess_Healthy(t *testing.T) {
	requireExec(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	srv := newTestServer(t, "default")

	// Not healthy before start
	if srv.Healthy(ctx) {
		t.Error("expected unhealthy before Start")
	}

	if err := srv.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { srv.Stop(context.Background()) })

	// Healthy after start
	if !srv.Healthy(ctx) {
		t.Error("expected healthy after Start")
	}
}

func TestSubprocess_Restart(t *testing.T) {
	requireExec(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	srv := newTestServer(t, "default")
	if err := srv.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { srv.Stop(context.Background()) })

	// Verify working before restart
	result, err := srv.CallTool(ctx, "echo", map[string]any{"message": "before"})
	if err != nil {
		t.Fatalf("CallTool before restart: %v", err)
	}
	if got := extractText(t, result); got != "echo: before" {
		t.Errorf("before restart: got %q, want %q", got, "echo: before")
	}

	// Restart
	if err := srv.Restart(ctx); err != nil {
		t.Fatalf("Restart: %v", err)
	}

	// Verify working after restart
	result, err = srv.CallTool(ctx, "echo", map[string]any{"message": "after"})
	if err != nil {
		t.Fatalf("CallTool after restart: %v", err)
	}
	if got := extractText(t, result); got != "echo: after" {
		t.Errorf("after restart: got %q, want %q", got, "echo: after")
	}

	// Health check after restart
	if !srv.Healthy(ctx) {
		t.Error("expected healthy after Restart")
	}
}

func TestSubprocess_StopIdempotent(t *testing.T) {
	requireExec(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	srv := newTestServer(t, "default")
	if err := srv.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if err := srv.Stop(ctx); err != nil {
		t.Fatalf("first Stop: %v", err)
	}

	// Second stop should be a no-op
	if err := srv.Stop(ctx); err != nil {
		t.Fatalf("second Stop: %v", err)
	}

	// Not healthy after stop
	if srv.Healthy(ctx) {
		t.Error("expected unhealthy after Stop")
	}
}

func TestSubprocess_CallToolAfterStop(t *testing.T) {
	requireExec(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	srv := newTestServer(t, "default")
	if err := srv.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := srv.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	_, err := srv.CallTool(ctx, "echo", map[string]any{"message": "hello"})
	if err == nil {
		t.Fatal("expected error calling tool after Stop")
	}
}

func TestSubprocess_DoubleStart(t *testing.T) {
	requireExec(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	srv := newTestServer(t, "default")
	if err := srv.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { srv.Stop(context.Background()) })

	err := srv.Start(ctx)
	if err == nil {
		t.Fatal("expected error on double Start")
	}
}

func TestSubprocess_GracefulShutdown(t *testing.T) {
	requireExec(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	srv := newTestServer(t, "default")
	if err := srv.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Make a tool call to verify working
	result, err := srv.CallTool(ctx, "echo", map[string]any{"message": "final"})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if got := extractText(t, result); got != "echo: final" {
		t.Errorf("got %q, want %q", got, "echo: final")
	}

	// Stop should complete without error (graceful shutdown via stdin close)
	if err := srv.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

func TestSubprocess_MultipleConcurrentCalls(t *testing.T) {
	requireExec(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	srv := newTestServer(t, "default")
	if err := srv.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { srv.Stop(context.Background()) })

	const n = 10
	errs := make(chan error, n)
	for i := range n {
		go func(i int) {
			result, err := srv.CallTool(ctx, "add", map[string]any{"a": i, "b": 1})
			if err != nil {
				errs <- fmt.Errorf("call %d: %w", i, err)
				return
			}
			want := fmt.Sprintf("%d", i+1)
			if got := extractTextNoFail(result); got != want {
				errs <- fmt.Errorf("call %d: got %q, want %q", i, got, want)
				return
			}
			errs <- nil
		}(i)
	}

	for range n {
		if err := <-errs; err != nil {
			t.Error(err)
		}
	}
}

// --- Raw CommandTransport tests (verifying SDK behavior directly) ---

func TestCommandTransport_DirectRoundTrip(t *testing.T) {
	requireExec(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(exe)
	cmd.Env = append(os.Environ(), runAsServer+"=default")

	client := sdkmcp.NewClient(
		&sdkmcp.Implementation{Name: "test-client", Version: "v0.1.0"},
		nil,
	)
	session, err := client.Connect(ctx, &sdkmcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer session.Close()

	result, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "echo",
		Arguments: map[string]any{"message": "direct"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	got := extractText(t, result)
	if got != "echo: direct" {
		t.Errorf("got %q, want %q", got, "echo: direct")
	}
}

// --- Helpers ---

func extractText(t *testing.T, result *sdkmcp.CallToolResult) string {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("empty content in result")
	}
	tc, ok := result.Content[0].(*sdkmcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	return tc.Text
}

func extractTextNoFail(result *sdkmcp.CallToolResult) string {
	if len(result.Content) == 0 {
		return ""
	}
	tc, ok := result.Content[0].(*sdkmcp.TextContent)
	if !ok {
		return ""
	}
	return tc.Text
}
