package e2e

import (
	"testing"

	"github.com/dpopsuev/tako/testkit"
	"github.com/dpopsuev/tako/testkit/rehearsal"
)

func TestUserStory_HelloSettlesQuickly(t *testing.T) {
	testkit.SkipWithoutLLM(t)

	dir := rehearsal.SetupWorkspace(t)

	agent := testkit.NewRealAgent(t, dir)
	result := testkit.RunAgent(t, agent, "Hello")

	m := agent.Result()

	if !m.Sealed() {
		t.Error("molecule should be sealed")
	}

	if m.Turns() > 2 {
		t.Errorf("'Hello' should settle in 2 turns max (speak + text), got %d", m.Turns())
	}

	chain := m.Chain()
	speakCalls := 0
	for _, e := range chain.All() {
		if e.Organ == "dialog_speak" {
			speakCalls++
		}
	}

	if speakCalls > 1 {
		t.Errorf("should not call dialog_speak more than once, got %d", speakCalls)
	}

	t.Logf("Hello: turns=%d speaks=%d chain=%d result=%s",
		m.Turns(), speakCalls, chain.Len(), result[:min(len(result), 100)])
}
