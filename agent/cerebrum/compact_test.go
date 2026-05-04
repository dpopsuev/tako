package cerebrum

import (
	"testing"

	"github.com/dpopsuev/tako/agent/reactivity"
	tangle "github.com/dpopsuev/tangle"
)

func TestSummaryCompactor_PreservesToolMessages(t *testing.T) {
	history := []tangle.Message{
		{Role: "user", Content: "what food is available?"},
		{Role: "assistant", Content: "Let me check the fridge."},
		{Role: "assistant", Content: "", ToolCalls: []tangle.ToolCall{{ID: "tc1", Name: "look_fridge"}}},
		{Role: "tool", Content: "eggs, milk, cheese", ToolCallID: "tc1"},
		{Role: "user", Content: "good, now plan cooking"},
		{Role: "assistant", Content: "I'll cook eggs."},
	}

	c := SummaryCompactor{}
	result := c.Compact(history, reactivity.ThinkTriad)

	toolCount := 0
	for _, m := range result {
		if m.Role == "tool" || len(m.ToolCalls) > 0 {
			toolCount++
		}
	}
	if toolCount != 2 {
		t.Errorf("expected 2 tool messages preserved, got %d", toolCount)
	}

	if result[0].Role != "user" {
		t.Errorf("expected summary as first message, got role=%s", result[0].Role)
	}
	if len(result) >= len(history) {
		t.Errorf("expected fewer messages after compaction, got %d (was %d)", len(result), len(history))
	}
}

func TestSummaryCompactor_TruncatesLongSummary(t *testing.T) {
	long := ""
	for i := 0; i < 200; i++ {
		long += "word "
	}
	history := []tangle.Message{
		{Role: "user", Content: long},
		{Role: "assistant", Content: long},
	}

	c := SummaryCompactor{}
	result := c.Compact(history, reactivity.ComposeTriad)

	if len(result) != 1 {
		t.Fatalf("expected 1 summary message, got %d", len(result))
	}
	if len(result[0].Content) > 600 {
		t.Errorf("expected truncated summary, got len=%d", len(result[0].Content))
	}
}

func TestSummaryCompactor_EmptyHistory(t *testing.T) {
	c := SummaryCompactor{}
	result := c.Compact(nil, reactivity.ThinkTriad)
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d", len(result))
	}
}
