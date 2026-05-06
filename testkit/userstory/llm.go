package userstory

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/dpopsuev/tako/assemble"
	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/shells/code"
	"github.com/dpopsuev/tangle/providers"
)

const envProvider = "TAKO_PROVIDER"

func SkipWithoutLLM(t *testing.T) {
	t.Helper()
	if os.Getenv(envProvider) == "" {
		t.Skipf("no LLM: set %s (e.g. vertex-ai, anthropic-api)", envProvider)
	}
}

func NewRealAgent(t *testing.T, workdir string) *assemble.Agent {
	t.Helper()

	p, err := providers.NewProviderFromEnv(envProvider)
	if err != nil {
		t.Fatal(err)
	}

	model := os.Getenv("TAKO_TEST_MODEL")
	if model == "" {
		model = "claude-sonnet-4-6"
	}

	completer := providers.NewCompleter(p, model, nil)

	caps := code.Capabilities(workdir)
	bp := assemble.Blueprint{
		Model:        model,
		Capabilities: caps,
		Budget: cerebrum.Budget{
			MaxTurns:    20,
			TurnTimeout: 60 * time.Second,
		},
	}

	return assemble.Assemble(bp, completer)
}

func RunAgent(t *testing.T, agent *assemble.Agent, task string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	result, err := agent.Run(ctx, task)
	if err != nil {
		t.Fatalf("agent.Run: %v", err)
	}
	return result
}

