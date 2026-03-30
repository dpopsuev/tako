package mediator

import (
	"encoding/json"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestWorkerStepParse_CrashesOnErrorResponse reproduces ORG-BUG-18:
// when the mediator returns an IsError response (plain text, not JSON),
// the worker's JSON unmarshal fails with "invalid character" instead of
// surfacing the actual error message.
func TestWorkerStepParse_CrashesOnErrorResponse(t *testing.T) {
	// Simulate the error response the mediator returns for unknown session.
	errorResult := &sdkmcp.CallToolResult{
		IsError: true,
		Content: []sdkmcp.Content{
			&sdkmcp.TextContent{Text: `unknown session "cobalt-stallion"`},
		},
	}

	// Extract text — this works fine.
	text := extractTextContent(errorResult)
	if text == "" {
		t.Fatal("extractTextContent returned empty")
	}

	// This is exactly what runMediatorWorker does at line 192:
	// it does NOT check IsError, just tries to JSON parse.
	var step struct {
		Done       bool  `json:"done"`
		Available  bool  `json:"available"`
		DispatchID int64 `json:"dispatch_id"`
	}
	err := json.Unmarshal([]byte(text), &step)

	// The bug: this produces "invalid character 'u'" instead of the
	// actual error message "unknown session cobalt-stallion".
	if err == nil {
		t.Fatal("expected JSON parse error for non-JSON error text")
	}

	// ORG-BUG-18: the error message is useless — it's a JSON parse error,
	// not the actual mediator error. The worker should have checked
	// IsError BEFORE attempting JSON parse.
	if !errorResult.IsError {
		t.Fatal("IsError should be true")
	}

	// This test PASSES (demonstrating the bug exists) — the JSON parse
	// fails as expected. The FIX would add an IsError check before
	// line 192 in workers.go, making this JSON parse path unreachable
	// for error responses.
	t.Logf("Bug confirmed: JSON parse error = %q (actual error was: %q)", err, text)
}
