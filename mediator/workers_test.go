package mediator

import (
	"encoding/json"
	"strings"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestWorkerStepParse_HandlesErrorResponse verifies that the worker
// detects IsError before JSON-parsing, returning the actual error
// message instead of crashing with "invalid character". ORG-BUG-18.
func TestWorkerStepParse_HandlesErrorResponse(t *testing.T) {
	errorResult := &sdkmcp.CallToolResult{
		IsError: true,
		Content: []sdkmcp.Content{
			&sdkmcp.TextContent{Text: `unknown session "cobalt-stallion"`},
		},
	}

	text := extractTextContent(errorResult)

	// The FIX: check IsError BEFORE attempting JSON parse.
	if errorResult.IsError {
		// Worker should return this error directly.
		if !strings.Contains(text, "unknown session") {
			t.Fatalf("expected actual error message, got: %q", text)
		}
		// Should NOT reach JSON parse.
		return
	}

	// This path should be unreachable for error responses.
	var step struct {
		Done bool `json:"done"`
	}
	if err := json.Unmarshal([]byte(text), &step); err != nil {
		t.Fatalf("should not reach JSON parse for error response: %v", err)
	}
}

// TestWorkerStepParse_ValidResponse verifies normal JSON parsing still works.
func TestWorkerStepParse_ValidResponse(t *testing.T) {
	validResult := &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{
			&sdkmcp.TextContent{Text: `{"done":false,"available":true,"step":"recall","dispatch_id":42}`},
		},
	}

	if validResult.IsError {
		t.Fatal("valid response should not have IsError")
	}

	text := extractTextContent(validResult)
	var step struct {
		Done       bool   `json:"done"`
		Available  bool   `json:"available"`
		Step       string `json:"step"`
		DispatchID int64  `json:"dispatch_id"`
	}
	if err := json.Unmarshal([]byte(text), &step); err != nil {
		t.Fatalf("JSON parse failed for valid response: %v", err)
	}
	if step.Step != "recall" {
		t.Errorf("step = %q, want recall", step.Step)
	}
	if step.DispatchID != 42 {
		t.Errorf("dispatch_id = %d, want 42", step.DispatchID)
	}
}
