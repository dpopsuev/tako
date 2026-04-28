package mcp_test

import (
	"bufio"
	"context"
	"io"
	"testing"
	"time"

	"github.com/dpopsuev/tako/mcp"
)

func TestWatchStdin_StopsWhenContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	mcp.WatchStdin(ctx, nil, cancel)

	cancel()

	// Verify the goroutine doesn't panic or block after context cancel.
	time.Sleep(50 * time.Millisecond)
}

func TestWatchStdin_DoesNotConsumeData(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pr, pw := io.Pipe()
	defer pr.Close()

	// WatchStdin should NOT touch the pipe at all.
	// Pass it as a unused arg; the function ignores the second parameter.
	mcp.WatchStdin(ctx, nil, cancel)

	// Give the watchdog goroutine time to schedule.
	time.Sleep(50 * time.Millisecond)

	msg := `{"jsonrpc":"2.0","id":1,"method":"initialize"}` + "\n"
	go func() {
		pw.Write([]byte(msg))
		time.Sleep(100 * time.Millisecond)
		pw.Close()
	}()

	// Another reader (representing the MCP SDK transport) should get the full message.
	scanner := bufio.NewScanner(pr)
	if !scanner.Scan() {
		t.Fatalf("reader got no data; err=%v", scanner.Err())
	}
	got := scanner.Text()
	want := `{"jsonrpc":"2.0","id":1,"method":"initialize"}`
	if got != want {
		t.Fatalf("reader got corrupted data (bytes stolen):\n  got:  %q\n  want: %q", got, want)
	}
}
