package acceptance

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami/engine/gate"
	"github.com/dpopsuev/origami/prompt"
	"github.com/dpopsuev/origami/testkit/assertions"
	"github.com/dpopsuev/origami/testkit/stubs"
)

// ---------------------------------------------------------------------------
// SDLCGauntlet — test harness wiring
// ---------------------------------------------------------------------------

// SDLCGauntlet wires the SDLC E2E test environment: toy Scribe, approval
// store, prompt store, and a purpose-built gauntlet circuit. Each Test*
// creates its own instance so tests are independent.
type SDLCGauntlet struct {
	t           *testing.T
	Store       *stubs.MemoryApprovalStore
	Notifier    *stubs.StubNotifier
	Scribe      *stubs.ToyScribeStore
	PromptStore *prompt.LiveStore
	Graph       engine.Graph
	Walker      circuit.Walker
	TempDir     string
}

// NewGauntlet creates a fully wired gauntlet test environment.
func NewGauntlet(t *testing.T) *SDLCGauntlet {
	t.Helper()

	tempDir := t.TempDir()
	copyToyRepo(t, tempDir)

	scribe := stubs.NewToyScribeStore()
	store := stubs.NewMemoryApprovalStore()
	notifier := stubs.NewStubNotifier()
	promptStore := prompt.NewLiveStore()
	promptStore.Create("fix", "fix", "default fix prompt")

	def := loadFixture(t, "circuits/sdlc-gauntlet.yaml")

	g := &SDLCGauntlet{
		t:           t,
		Store:       store,
		Notifier:    notifier,
		Scribe:      scribe,
		PromptStore: promptStore,
		TempDir:     tempDir,
	}

	transformers := g.buildTransformers()

	graph, err := engine.BuildGraph(def, &engine.GraphRegistries{
		Transformers:     transformers,
		ApprovalStore:    store,
		ApprovalNotifier: notifier,
	})
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	g.Graph = graph
	g.Walker = circuit.NewProcessWalker("gauntlet-run")

	return g
}

// Run walks the circuit from the start node. Returns ErrWalkInterrupted at gates.
func (g *SDLCGauntlet) Run(ctx context.Context) error {
	return g.Graph.Walk(ctx, g.Walker, "scan")
}

// Resume continues the walk after a gate decision.
func (g *SDLCGauntlet) Resume(ctx context.Context) error {
	return engine.ResumeFromGate(ctx, g.Graph, g.Walker, g.Store)
}

// Approve resolves the pending approval for the given node.
func (g *SDLCGauntlet) Approve(ctx context.Context, nodeName, comment string) {
	g.t.Helper()
	parked := assertions.AssertParked(g.t, g.Store, nodeName)
	if err := g.Store.Resolve(ctx, parked.ID, gate.Decision{
		Status:   gate.ApprovalApproved,
		Comment:  comment,
		Operator: "gauntlet-test",
	}); err != nil {
		g.t.Fatalf("Approve %s: %v", nodeName, err)
	}
}

// Reject resolves the pending approval for the given node as rejected.
func (g *SDLCGauntlet) Reject(ctx context.Context, nodeName, comment string) {
	g.t.Helper()
	parked := assertions.AssertParked(g.t, g.Store, nodeName)
	if err := g.Store.Resolve(ctx, parked.ID, gate.Decision{
		Status:   gate.ApprovalRejected,
		Comment:  comment,
		Operator: "gauntlet-test",
	}); err != nil {
		g.t.Fatalf("Reject %s: %v", nodeName, err)
	}
}

// ---------------------------------------------------------------------------
// Transformer builders — closures capturing gauntlet state
// ---------------------------------------------------------------------------

func (g *SDLCGauntlet) buildTransformers() engine.TransformerRegistry {
	return engine.TransformerRegistry{
		"scan":        g.scanTransformer(),
		"poll-scribe": g.pollScribeTransformer(),
		"plan-review": g.planReviewTransformer(),
		"write-test":  g.writeTestTransformer(),
		"write-code":  g.writeCodeTransformer(),
		"build":       g.buildTransformer(),
		"test":        g.testTransformer(),
		"self-review": g.selfReviewTransformer(),
		"diff-review": g.diffReviewTransformer(),
		"mark-done":   g.markDoneTransformer(),
		"teardown":    g.teardownTransformer(),
	}
}

// scan: detects planted lint issue, files task in Scribe.
func (g *SDLCGauntlet) scanTransformer() engine.Transformer {
	return engine.TransformerFunc("scan", func(ctx context.Context, _ *engine.TransformerContext) (any, error) {
		mainPath := filepath.Join(g.TempDir, "main.go")
		data, err := os.ReadFile(mainPath)
		if err != nil {
			return nil, err
		}

		// Check for unused import — simplistic string match.
		hasIssue := len(data) > 0 && strings.Contains(string(data), "var _ = fmt.Sprintf")
		if !hasIssue {
			return map[string]any{"clean": true, "findings": []string{}}, nil
		}

		// File a task in toy Scribe.
		result, err := g.Scribe.Handle(ctx, stubs.ScribeInput("create", map[string]string{
			"title": "Fix unused import in main.go",
			"kind":  "task",
			"scope": "toy-repo",
		}))
		if err != nil {
			return nil, err
		}
		resultMap, _ := result.(map[string]string)
		taskID := resultMap["id"]

		// Attach findings as section.
		g.Scribe.Handle(ctx, stubs.ScribeInput("attach_section", map[string]string{
			"id":   taskID,
			"name": "findings",
			"text": "unused import \"fmt\" in main.go (var _ = fmt.Sprintf)",
		}))

		return map[string]any{
			"clean":    false,
			"findings": []string{"unused import fmt in main.go"},
			"task_id":  taskID,
		}, nil
	})
}

// poll-scribe: checks Scribe for mature tasks, allocates if found.
func (g *SDLCGauntlet) pollScribeTransformer() engine.Transformer {
	return engine.TransformerFunc("poll-scribe", func(ctx context.Context, _ *engine.TransformerContext) (any, error) {
		items := g.Scribe.List("mature")
		if len(items) == 0 {
			return map[string]any{"has_task": true, "task_id": ""}, nil
		}

		task := items[0]
		// Allocate the task.
		g.Scribe.Handle(ctx, stubs.ScribeInput("set", map[string]string{
			"id": task.ID, "field": "status", "value": "allocated",
		}))

		return map[string]any{"has_task": true, "task_id": task.ID}, nil
	})
}

// plan-review: returns approved=true. The gate handles parking.
func (g *SDLCGauntlet) planReviewTransformer() engine.Transformer {
	return engine.TransformerFunc("plan-review", func(_ context.Context, _ *engine.TransformerContext) (any, error) {
		return map[string]any{"approved": true}, nil
	})
}

// write-test: creates a test file in the temp worktree (RED phase).
func (g *SDLCGauntlet) writeTestTransformer() engine.Transformer {
	return engine.TransformerFunc("write-test", func(_ context.Context, _ *engine.TransformerContext) (any, error) {
		testContent := `package main

import "testing"

func TestMain_Exits(t *testing.T) {
	// Verifies main package compiles.
	t.Log("main package compiles")
}
`
		testPath := filepath.Join(g.TempDir, "main_test.go")
		if err := os.WriteFile(testPath, []byte(testContent), 0o644); err != nil {
			return nil, err
		}
		return map[string]any{"test_file": "main_test.go"}, nil
	})
}

// write-code: applies the fix patch to the temp worktree (GREEN phase).
func (g *SDLCGauntlet) writeCodeTransformer() engine.Transformer {
	return engine.TransformerFunc("write-code", func(_ context.Context, _ *engine.TransformerContext) (any, error) {
		// Apply the fix: rewrite main.go without the unused import.
		fixedContent := `package main

import (
	"os"
)

func main() {
	os.Exit(0)
}
`
		mainPath := filepath.Join(g.TempDir, "main.go")
		if err := os.WriteFile(mainPath, []byte(fixedContent), 0o644); err != nil {
			return nil, err
		}
		return map[string]any{"files_changed": []string{"main.go"}}, nil
	})
}

// build: returns pass=true.
func (g *SDLCGauntlet) buildTransformer() engine.Transformer {
	return engine.TransformerFunc("build", func(_ context.Context, _ *engine.TransformerContext) (any, error) {
		return map[string]any{"pass": true, "output": "build ok"}, nil
	})
}

// test: returns pass=true.
func (g *SDLCGauntlet) testTransformer() engine.Transformer {
	return engine.TransformerFunc("test", func(_ context.Context, _ *engine.TransformerContext) (any, error) {
		return map[string]any{"pass": true, "total": 1, "failed": 0}, nil
	})
}

// self-review: returns all_verified=true with stamps.
func (g *SDLCGauntlet) selfReviewTransformer() engine.Transformer {
	return engine.TransformerFunc("self-review", func(_ context.Context, _ *engine.TransformerContext) (any, error) {
		return map[string]any{
			"all_verified": true,
			"stamps": []map[string]string{
				{"requirement": "remove unused import", "file": "main.go", "line": "3"},
			},
		}, nil
	})
}

// diff-review: returns approved=true. The gate handles parking.
func (g *SDLCGauntlet) diffReviewTransformer() engine.Transformer {
	return engine.TransformerFunc("diff-review", func(_ context.Context, tc *engine.TransformerContext) (any, error) {
		out := map[string]any{"approved": true}
		// Capture rejection feedback if present (Story 6 verification).
		if tc.WalkerState != nil {
			if fb, ok := tc.WalkerState.Context[gate.ContextKeyRejectionFeedback]; ok {
				out["rejection_feedback_seen"] = fb
			}
		}
		return out, nil
	})
}

// mark-done: marks the Scribe task as done.
func (g *SDLCGauntlet) markDoneTransformer() engine.Transformer {
	return engine.TransformerFunc("mark-done", func(ctx context.Context, _ *engine.TransformerContext) (any, error) {
		// Find the allocated task and mark it done.
		for _, status := range []string{"allocated", "in_progress", "mature", "draft"} {
			items := g.Scribe.List(status)
			if len(items) > 0 {
				g.Scribe.Handle(ctx, stubs.ScribeInput("set", map[string]string{
					"id": items[0].ID, "field": "status", "value": "done",
				}))
				return map[string]any{"updated": true}, nil
			}
		}
		return map[string]any{"updated": false}, nil
	})
}

// teardown: no-op for tests.
func (g *SDLCGauntlet) teardownTransformer() engine.Transformer {
	return engine.TransformerFunc("teardown", func(_ context.Context, _ *engine.TransformerContext) (any, error) {
		return map[string]any{"cleaned": []string{}}, nil
	})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// copyToyRepo copies testdata/toy-repo/ into dst.
func copyToyRepo(t *testing.T, dst string) {
	t.Helper()
	src := testdataPath(t, "toy-repo")
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatalf("read toy-repo: %v", err)
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			continue // skip subdirectories (patches/) — not needed in worktree
		}
		data, err := os.ReadFile(srcPath)
		if err != nil {
			t.Fatalf("read %s: %v", srcPath, err)
		}
		if err := os.WriteFile(dstPath, data, 0o644); err != nil {
			t.Fatalf("write %s: %v", dstPath, err)
		}
	}
}

// ---------------------------------------------------------------------------
// Story 1: Audit Intake
// ---------------------------------------------------------------------------

func TestGauntlet_Story1_AuditIntake(t *testing.T) {
	if testing.Short() {
		t.Log("structural tier")
	}

	g := NewGauntlet(t)
	ctx := context.Background()

	// Walk — scan will detect the planted issue and file a Scribe task.
	// Walk parks at plan-review gate.
	err := g.Run(ctx)
	if !errors.Is(err, engine.ErrWalkInterrupted) {
		t.Fatalf("Run: expected ErrWalkInterrupted, got %v", err)
	}

	// Verify: Scribe has 1 task with status=draft filed by scan.
	tasks := g.Scribe.List("draft")
	if len(tasks) != 1 {
		t.Fatalf("draft tasks = %d, want 1", len(tasks))
	}
	if tasks[0].Title != "Fix unused import in main.go" {
		t.Errorf("title = %q", tasks[0].Title)
	}
	if tasks[0].Kind != "task" {
		t.Errorf("kind = %q", tasks[0].Kind)
	}

	// Verify: findings section attached.
	task := g.Scribe.Get(tasks[0].ID)
	if task.Sections["findings"] == "" {
		t.Error("findings section is empty")
	}

	// Verify: plan-review gate is parked.
	assertions.AssertParked(t, g.Store, "plan-review")
}

// ---------------------------------------------------------------------------
// Story 2: Task Pickup
// ---------------------------------------------------------------------------

func TestGauntlet_Story2_TaskPickup(t *testing.T) {
	if testing.Short() {
		t.Log("structural tier")
	}

	g := NewGauntlet(t)
	ctx := context.Background()

	// Pre-seed: Scribe has a mature task (simulating external maturation).
	g.Scribe.Seed(&stubs.ToyArtifact{
		ID: "TSK-100", Title: "Existing mature task", Status: "mature", Kind: "task",
	})

	// Walk — scan finds the planted issue (files another task),
	// poll-scribe finds the mature task and allocates it.
	err := g.Run(ctx)
	if !errors.Is(err, engine.ErrWalkInterrupted) {
		t.Fatalf("Run: expected ErrWalkInterrupted, got %v", err)
	}

	// Verify: mature task was allocated by poll-scribe.
	task := g.Scribe.Get("TSK-100")
	if task == nil {
		t.Fatal("TSK-100 not found")
	}
	if task.Status != "allocated" {
		t.Errorf("TSK-100 status = %q, want allocated", task.Status)
	}

	// Verify: parked at plan-review gate.
	assertions.AssertParked(t, g.Store, "plan-review")
}

// ---------------------------------------------------------------------------
// Story 3+4: TDD Coding + Quality Gates
// ---------------------------------------------------------------------------

func TestGauntlet_Story3_4_TDDAndQualityGates(t *testing.T) {
	if testing.Short() {
		t.Log("structural tier")
	}

	g := NewGauntlet(t)
	ctx := context.Background()

	// Seed a mature task for poll-scribe to find.
	g.Scribe.Seed(&stubs.ToyArtifact{
		ID: "TSK-100", Title: "Fix lint", Status: "mature", Kind: "task",
	})

	// Walk to plan-review gate.
	err := g.Run(ctx)
	if !errors.Is(err, engine.ErrWalkInterrupted) {
		t.Fatalf("Run: expected ErrWalkInterrupted at plan-review, got %v", err)
	}

	// Approve the plan.
	g.Approve(ctx, "plan-review", "LGTM — proceed with fix")

	// Resume — walks through write-test, write-code, build, test, self-review.
	// Parks at diff-review gate.
	err = g.Resume(ctx)
	if !errors.Is(err, engine.ErrWalkInterrupted) {
		t.Fatalf("Resume: expected ErrWalkInterrupted at diff-review, got %v", err)
	}

	// Verify: test file was created (RED→GREEN).
	testFile := filepath.Join(g.TempDir, "main_test.go")
	if _, err := os.Stat(testFile); err != nil {
		t.Errorf("test file not created: %v", err)
	}

	// Verify: main.go was patched (unused import removed).
	mainData, _ := os.ReadFile(filepath.Join(g.TempDir, "main.go"))
	if strings.Contains(string(mainData), "var _ = fmt.Sprintf") {
		t.Error("main.go still has planted lint issue after write-code")
	}

	// Verify: diff-review gate is parked.
	assertions.AssertParked(t, g.Store, "diff-review")
}

// ---------------------------------------------------------------------------
// Story 5: Approval + Merge
// ---------------------------------------------------------------------------

func TestGauntlet_Story5_ApprovalAndMerge(t *testing.T) {
	if testing.Short() {
		t.Log("structural tier")
	}

	g := NewGauntlet(t)
	ctx := context.Background()

	g.Scribe.Seed(&stubs.ToyArtifact{
		ID: "TSK-100", Title: "Fix lint", Status: "mature", Kind: "task",
	})

	// Walk to plan-review, approve, resume to diff-review.
	g.Run(ctx)
	g.Approve(ctx, "plan-review", "approved")
	g.Resume(ctx)

	// Approve the diff.
	g.Approve(ctx, "diff-review", "Ship it")

	// Resume — mark-done runs, circuit completes.
	err := g.Resume(ctx)
	if err != nil {
		t.Fatalf("final Resume: %v", err)
	}

	// Verify: Scribe task is done.
	task := g.Scribe.Get("TSK-100")
	if task.Status != "done" {
		t.Errorf("task status = %q, want done", task.Status)
	}

	// Verify: no pending approvals remain.
	assertions.AssertNoPending(t, g.Store)
}

// ---------------------------------------------------------------------------
// Story 6: Rejection + Feedback
// ---------------------------------------------------------------------------

func TestGauntlet_Story6_RejectionAndFeedback(t *testing.T) {
	if testing.Short() {
		t.Log("structural tier")
	}

	g := NewGauntlet(t)
	ctx := context.Background()

	g.Scribe.Seed(&stubs.ToyArtifact{
		ID: "TSK-100", Title: "Fix lint", Status: "mature", Kind: "task",
	})

	// Walk to plan-review, approve, resume to diff-review.
	g.Run(ctx)
	g.Approve(ctx, "plan-review", "approved")
	g.Resume(ctx)

	// Reject the diff with feedback.
	g.Reject(ctx, "diff-review", "don't change public API")

	// Resume — ResumeFromGate injects rejection_feedback, re-walks diff-review.
	// The diff-review transformer captures the feedback. Walk parks again.
	err := g.Resume(ctx)
	if !errors.Is(err, engine.ErrWalkInterrupted) {
		t.Fatalf("Resume after reject: expected ErrWalkInterrupted, got %v", err)
	}

	// Verify: rejection_feedback is in walker context.
	state := g.Walker.State()
	fb, ok := state.Context[gate.ContextKeyRejectionFeedback]
	if !ok {
		t.Fatal("rejection_feedback not in walker context")
	}
	if fb != "don't change public API" {
		t.Errorf("rejection_feedback = %q, want %q", fb, "don't change public API")
	}

	// Verify: a new approval item is parked (different from the rejected one).
	assertions.AssertParked(t, g.Store, "diff-review")

	// Approve the second attempt, circuit completes.
	g.Approve(ctx, "diff-review", "looks good now")
	err = g.Resume(ctx)
	if err != nil {
		t.Fatalf("final Resume: %v", err)
	}

	assertions.AssertNoPending(t, g.Store)
}

// ---------------------------------------------------------------------------
// Story 7: Self-Evolving Prompt
// ---------------------------------------------------------------------------

func TestGauntlet_Story7_SelfEvolvingPrompt(t *testing.T) {
	if testing.Short() {
		t.Log("structural tier")
	}

	// Test that the prompt store supports the auto-tune lifecycle:
	// create → get (version 1) → update → get (version 2).
	ps := prompt.NewLiveStore()

	// Seed with default prompt.
	p, err := ps.Create("fix", "fix", "default fix prompt")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if p.Version != 1 {
		t.Errorf("initial version = %d, want 1", p.Version)
	}
	if p.Content != "default fix prompt" {
		t.Errorf("initial content = %q", p.Content)
	}

	// Simulate agent updating the prompt after learning.
	updated, err := ps.Update("fix", "improved fix prompt with rejection context and coding rules")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Version != 2 {
		t.Errorf("updated version = %d, want 2", updated.Version)
	}

	// Verify: Get returns the updated content.
	got, err := ps.Get("fix")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Content != "improved fix prompt with rejection context and coding rules" {
		t.Errorf("content after update = %q", got.Content)
	}
	if got.Version != 2 {
		t.Errorf("version after update = %d, want 2", got.Version)
	}

	// Verify: rollback works.
	rolled, err := ps.Rollback("fix", 1)
	if err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if rolled.Content != "default fix prompt" {
		t.Errorf("rollback content = %q", rolled.Content)
	}
	if rolled.Version != 3 {
		t.Errorf("rollback version = %d, want 3 (new version from rollback)", rolled.Version)
	}
}
