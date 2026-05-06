package e2e

import (
	"strings"
	"testing"

	"github.com/dpopsuev/tako/testkit/rehearsal"
	"github.com/dpopsuev/tako/testkit"
)

func TestUserStory_ReadAndExplain(t *testing.T) {
	testkit.SkipWithoutLLM(t)

	dir := rehearsal.SetupWorkspace(t,
		rehearsal.WithExtraFiles(map[string]string{
			"pipeline.go": `package main

import (
	"fmt"
	"strings"
)

type Record struct {
	Name  string
	Value int
}

func Process(records []Record) map[string]int {
	result := make(map[string]int)
	for _, r := range records {
		key := strings.ToLower(r.Name)
		if existing, ok := result[key]; ok {
			result[key] = existing + r.Value
		} else {
			result[key] = r.Value
		}
	}
	return result
}

func FormatReport(data map[string]int) string {
	var lines []string
	for k, v := range data {
		lines = append(lines, fmt.Sprintf("%s: %d", k, v))
	}
	return strings.Join(lines, "\n")
}
`,
		}),
	)

	agent := testkit.NewRealAgent(t, dir)
	result := testkit.RunAgent(t, agent, "Read pipeline.go and explain what the Process function does. Be specific about the aggregation logic.")

	if len(result) < 20 {
		t.Fatalf("response too short, agent didn't explain: %q", result)
	}

	lower := strings.ToLower(result)
	hasRelevantContent := strings.Contains(lower, "record") ||
		strings.Contains(lower, "aggregat") ||
		strings.Contains(lower, "group") ||
		strings.Contains(lower, "sum") ||
		strings.Contains(lower, "lowercase")

	if !hasRelevantContent {
		t.Errorf("response doesn't mention key concepts (record, aggregate, group, sum, lowercase):\n%s", result)
	}

	t.Logf("PASS: agent explained Process in %d turns", agent.Result().Turns())
	t.Logf("Result: %s", result)
}
