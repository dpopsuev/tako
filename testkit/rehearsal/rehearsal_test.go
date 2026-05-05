package rehearsal

import (
	"context"
	"testing"
	"time"
)

func TestRehearsal_TimeoutOrDefault(t *testing.T) {
	r := Rehearsal{Name: "test"}
	if r.TimeoutOrDefault() != 120*time.Second {
		t.Error("default should be 120s")
	}
	r.Timeout = 30 * time.Second
	if r.TimeoutOrDefault() != 30*time.Second {
		t.Error("custom timeout should be 30s")
	}
}

func TestScale_String(t *testing.T) {
	cases := []struct{ s Scale; want string }{
		{Single, "single"}, {Pair, "pair"}, {Fireteam, "fireteam"},
		{Squad, "squad"}, {Platoon, "platoon"},
	}
	for _, tc := range cases {
		if tc.s.String() != tc.want {
			t.Errorf("Scale(%d).String() = %s, want %s", tc.s, tc.s.String(), tc.want)
		}
	}
}

func TestScale_Implemented(t *testing.T) {
	if !Single.Implemented() {
		t.Error("Single should be implemented")
	}
	if Pair.Implemented() {
		t.Error("Pair should not be implemented yet")
	}
}

func TestScorecard_WriteRules(t *testing.T) {
	sc := BuildScorecard(Rehearsal{Name: "test", Template: Write})
	events := []string{"tool.write", "tool.write", "tool.edit", "done"}
	score, pass := sc.Score(events)
	if score != 15+15+10+10 {
		t.Errorf("score = %d, want 50", score)
	}
	if !pass {
		t.Error("should pass (50 >= 20)")
	}
}

func TestScorecard_ReadOnlyPenalizesWrites(t *testing.T) {
	sc := BuildScorecard(Rehearsal{Name: "test", Template: ReadOnly})
	events := []string{"tool.write", "tool.write"}
	score, pass := sc.Score(events)
	if score != -30 {
		t.Errorf("score = %d, want -30", score)
	}
	if pass {
		t.Error("should fail (-30 < 20)")
	}
}

func TestRunBuilder_Validation(t *testing.T) {
	_, err := NewRunBuilder().Build()
	if err != ErrMissingScenario {
		t.Errorf("expected ErrMissingScenario, got %v", err)
	}

	_, err = NewRunBuilder().WithScenario(NewStubScenario("test", "spec")).Build()
	if err != ErrMissingReferee {
		t.Errorf("expected ErrMissingReferee, got %v", err)
	}

	_, err = NewRunBuilder().
		WithScenario(NewStubScenario("test", "spec")).
		WithReferee(NewStubReferee(CheckResult{Pass: true})).
		Build()
	if err != ErrMissingActor {
		t.Errorf("expected ErrMissingActor, got %v", err)
	}
}

func TestRunner_Execute_StubReferee(t *testing.T) {
	runner, err := NewRunBuilder().
		WithScenario(NewStubScenario("test", "do something")).
		WithReferee(NewStubReferee(CheckResult{Pass: true, Score: 1.0})).
		WithActor(func(_ context.Context, _ string) (string, error) {
			return "done", nil
		}).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	metrics, err := runner.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !metrics.Pass {
		t.Error("should pass")
	}
	if metrics.Score != 1.0 {
		t.Errorf("score = %.2f, want 1.0", metrics.Score)
	}
}

func TestSetupWorkspace_Base(t *testing.T) {
	dir := SetupWorkspace(t)
	AssertCompiles(t, Result{WorkDir: dir})
	AssertTestsPass(t, Result{WorkDir: dir})
	AssertFileExists(t, Result{WorkDir: dir}, "auth/handler.go")
}

func TestSetupWorkspace_WithGitRepo(t *testing.T) {
	dir := SetupWorkspace(t, WithGitRepo())
	AssertFileExists(t, Result{WorkDir: dir}, ".git/HEAD")
}

func TestSetupWorkspace_WithFailingTest(t *testing.T) {
	dir := SetupWorkspace(t, WithFailingTest())
	AssertFileExists(t, Result{WorkDir: dir}, "auth/handler_failing_test.go")
}

func TestGoTestReferee_PassingProject(t *testing.T) {
	dir := SetupWorkspace(t)
	referee := &GoTestReferee{}
	ctx := context.Background()
	result, err := referee.Check(ctx, "test", dir)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !result.Pass {
		t.Errorf("should pass, errors: %v", result.Errors)
	}
	if result.Score != 1.0 {
		t.Errorf("score = %.2f, want 1.0", result.Score)
	}
}

func TestGoTestReferee_FailingProject(t *testing.T) {
	dir := SetupWorkspace(t, WithFailingTest())
	referee := &GoTestReferee{}
	ctx := context.Background()
	result, err := referee.Check(ctx, "test", dir)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if result.Pass {
		t.Error("should fail (failing test exists)")
	}
	if result.Score != 0.5 {
		t.Errorf("score = %.2f, want 0.5 (compiles but tests fail)", result.Score)
	}
}

func TestNoopSandbox(t *testing.T) {
	dir := t.TempDir()
	sb := &NoopSandbox{WorkDir: dir}

	ctx := context.Background()
	handle, err := sb.Create(ctx, "none")
	if err != nil {
		t.Fatal(err)
	}

	result, err := sb.Exec(ctx, handle, []string{"echo", "hello"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 0 {
		t.Errorf("exit code = %d", result.ExitCode)
	}
	if result.Stdout != "hello\n" {
		t.Errorf("stdout = %q, want 'hello\\n'", result.Stdout)
	}

	if err := sb.Destroy(ctx, handle); err != nil {
		t.Fatal(err)
	}
}
