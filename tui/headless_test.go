package tui

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dpopsuev/tako/tui/widgets"
)

func send(m Model, msg tea.Msg) (Model, tea.Cmd) {
	updated, cmd := m.Update(msg)
	return updated.(Model), cmd
}

func drain(m Model, cmd tea.Cmd) Model {
	if cmd == nil {
		return m
	}
	msg := cmd()
	if msg == nil {
		return m
	}
	m, next := send(m, msg)
	return drain(m, next)
}

type syncRunner struct {
	mu     sync.Mutex
	task   string
	called atomic.Bool
	done   chan struct{}
}

func newSyncRunner() *syncRunner {
	return &syncRunner{done: make(chan struct{})}
}

func (r *syncRunner) Run(_ context.Context, task string) (string, error) {
	r.mu.Lock()
	r.task = task
	r.mu.Unlock()
	r.called.Store(true)
	close(r.done)
	return "done", nil
}

func (r *syncRunner) waitDone() {
	<-r.done
}

func TestHeadless_SubmitDispatchesRunner(t *testing.T) {
	runner := newSyncRunner()
	m := NewModel(runner, "stub-model")

	m, _ = send(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m, cmd := send(m, widgets.SubmitMsg{Text: "explain main.go"})
	drain(m, cmd)

	runner.waitDone()

	if !runner.called.Load() {
		t.Fatal("runner was not called")
	}
	runner.mu.Lock()
	task := runner.task
	runner.mu.Unlock()
	if task != "explain main.go" {
		t.Errorf("task = %q, want %q", task, "explain main.go")
	}
}

func TestHeadless_AgentDoneUpdatesView(t *testing.T) {
	runner := newSyncRunner()
	m := NewModel(runner, "stub-model")

	m, _ = send(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m, cmd := send(m, widgets.SubmitMsg{Text: "test"})
	m = drain(m, cmd)

	m, _ = send(m, widgets.PhaseChangeMsg{Phase: "assessment", Turn: 1})
	m, _ = send(m, widgets.ToolCallStartMsg{Name: "file_read", Input: `{"path":"main.go"}`})
	m, _ = send(m, widgets.ToolCallResultMsg{Name: "file_read", Result: "package main"})
	m, _ = send(m, widgets.StreamTokenMsg("The answer "))
	m, _ = send(m, widgets.StreamTokenMsg("is 42."))
	m, _ = send(m, widgets.AgentDoneMsg{
		Sealed:   true,
		Distance: 0.3,
		Turns:    2,
		Result:   "The answer is 42.",
	})

	if m.running {
		t.Error("should not be running after AgentDoneMsg")
	}

	view := m.View()
	if !strings.Contains(view, "done") {
		t.Errorf("view should contain 'done' after seal, got:\n%s", truncateView(view))
	}
}

func TestHeadless_ErrorRecovery(t *testing.T) {
	runner := newSyncRunner()
	m := NewModel(runner, "stub-model")

	m, _ = send(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := send(m, widgets.SubmitMsg{Text: "do something"})
	m = drain(m, cmd)
	m, _ = send(m, widgets.ErrorMsg{Err: context.DeadlineExceeded})

	if m.running {
		t.Error("should not be running after ErrorMsg")
	}

	view := m.View()
	if !strings.Contains(view, "ERROR") {
		t.Errorf("view should contain 'ERROR', got:\n%s", truncateView(view))
	}
}

func TestHeadless_DoubleSubmitBlocked(t *testing.T) {
	runner := newSyncRunner()
	m := NewModel(runner, "stub-model")

	m, _ = send(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := send(m, widgets.SubmitMsg{Text: "first task"})
	m = drain(m, cmd)

	if !m.running {
		t.Fatal("should be running after first submit")
	}

	m, _ = send(m, widgets.SubmitMsg{Text: "second task"})

	runner.waitDone()
	runner.mu.Lock()
	task := runner.task
	runner.mu.Unlock()
	if task != "first task" {
		t.Errorf("second submit should be blocked, but runner got %q", task)
	}
}

func truncateView(s string) string {
	if len(s) > 500 {
		return s[:500] + "..."
	}
	return s
}
