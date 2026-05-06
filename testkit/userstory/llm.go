package userstory

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/dpopsuev/tako/assemble"
	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/shells/code"
	"github.com/dpopsuev/tangle/providers"
)

func SkipWithoutLLM(t *testing.T) {
	t.Helper()
	if _, err := detectProvider(); err != nil {
		t.Skip("no LLM credentials: ", err)
	}
}

func NewRealAgent(t *testing.T, workdir string) *assemble.Agent {
	t.Helper()

	providerName, err := detectProvider()
	if err != nil {
		t.Fatal(err)
	}

	model := os.Getenv("TAKO_TEST_MODEL")
	if model == "" {
		model = "claude-sonnet-4-6"
	}

	p, err := providers.NewProviderByName(providerName)
	if err != nil {
		t.Fatalf("provider %q: %v", providerName, err)
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

func detectProvider() (string, error) {
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		return "anthropic-api", nil
	}
	if os.Getenv("CLOUD_ML_REGION") != "" && os.Getenv("ANTHROPIC_VERTEX_PROJECT_ID") != "" {
		return "vertex-ai", nil
	}
	if os.Getenv("GOOGLE_API_KEY") != "" {
		return "gemini-api", nil
	}
	if os.Getenv("OPENROUTER_API_KEY") != "" {
		return "openrouter", nil
	}
	return "", fmt.Errorf("set ANTHROPIC_API_KEY, CLOUD_ML_REGION+ANTHROPIC_VERTEX_PROJECT_ID, GOOGLE_API_KEY, or OPENROUTER_API_KEY")
}

