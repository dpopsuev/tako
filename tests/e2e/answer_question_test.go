package e2e

import (
	"strings"
	"testing"

	"github.com/dpopsuev/tako/testkit"
	"github.com/dpopsuev/tako/testkit/rehearsal"
)

func TestUserStory_AnswerQuestion_Architecture(t *testing.T) {
	testkit.SkipWithoutLLM(t)

	dir := rehearsal.SetupWorkspace(t,
		rehearsal.WithExtraFiles(map[string]string{
			"server.go": `package main

import (
	"encoding/json"
	"net/http"
)

type Handler struct {
	store Store
}

type Store interface {
	Get(key string) (string, error)
	Set(key, value string) error
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		key := r.URL.Query().Get("key")
		val, err := h.store.Get(key)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"value": val})
	case http.MethodPost:
		var body struct {
			Key   string ` + "`json:\"key\"`" + `
			Value string ` + "`json:\"value\"`" + `
		}
		json.NewDecoder(r.Body).Decode(&body)
		h.store.Set(body.Key, body.Value)
		w.WriteHeader(http.StatusCreated)
	}
}
`,
			"store.go": `package main

import "fmt"

type MemoryStore struct {
	data map[string]string
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{data: make(map[string]string)}
}

func (s *MemoryStore) Get(key string) (string, error) {
	v, ok := s.data[key]
	if !ok {
		return "", fmt.Errorf("not found: %s", key)
	}
	return v, nil
}

func (s *MemoryStore) Set(key, value string) error {
	s.data[key] = value
	return nil
}
`,
		}),
	)

	agent := testkit.NewRealAgent(t, dir)
	result := testkit.RunAgent(t, agent,
		"What design pattern does this codebase use? How are the Handler and Store related? What would need to change to add a Delete endpoint?")

	lower := strings.ToLower(result)

	hasPattern := strings.Contains(lower, "interface") ||
		strings.Contains(lower, "dependency injection") ||
		strings.Contains(lower, "abstraction") ||
		strings.Contains(lower, "decouple")

	if !hasPattern {
		t.Errorf("answer should mention design pattern (interface, DI, abstraction):\n%s", result)
	}

	hasDelete := strings.Contains(lower, "delete") &&
		(strings.Contains(lower, "method") || strings.Contains(lower, "endpoint") || strings.Contains(lower, "handler"))

	if !hasDelete {
		t.Errorf("answer should explain what to change for Delete:\n%s", result)
	}

	t.Logf("PASS: answered architecture question in %d turns", agent.Result().Turns())
	t.Logf("Result: %s", result[:min(len(result), 300)])
}