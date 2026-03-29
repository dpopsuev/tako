package subprocess

import (
	"context"
	"encoding/json"
	"fmt"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// CallToolTyped calls a tool and unmarshals the JSON text result into T.
// It expects the tool to return a single TextContent with valid JSON.
func CallToolTyped[T any](ctx context.Context, caller ToolCaller, toolName string, args map[string]any) (T, error) {
	var zero T

	result, err := caller.CallTool(ctx, toolName, args)
	if err != nil {
		return zero, fmt.Errorf("call %s: %w", toolName, err)
	}
	if result.IsError {
		text := extractFirstText(result)
		return zero, fmt.Errorf("%w: %s returned error: %s", ErrTool, toolName, text)
	}

	text := extractFirstText(result)
	if text == "" {
		return zero, fmt.Errorf("%w: %s returned no text content", ErrTool, toolName)
	}

	var v T
	if err := json.Unmarshal([]byte(text), &v); err != nil {
		return zero, fmt.Errorf("unmarshal %s result: %w", toolName, err)
	}
	return v, nil
}

func extractFirstText(result *sdkmcp.CallToolResult) string {
	for _, c := range result.Content {
		if tc, ok := c.(*sdkmcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}
