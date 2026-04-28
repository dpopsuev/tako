package selfreview_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dpopsuev/tako/circuit"
	"github.com/dpopsuev/tako/engine/handler"
	"github.com/dpopsuev/tako/instruments/selfreview"
	"github.com/dpopsuev/tako/simulate/sdlc/sdlctype"
	"github.com/dpopsuev/tako/testkit/stubs"
	"github.com/dpopsuev/tako/tool"
	battmcp "github.com/dpopsuev/tako/tool/mcp"
)

// testArtifact is a helper to create a walker state artifact wrapping a value.
type testArtifact struct{ val any }

func (a testArtifact) Type() string        { return "test" }
func (a testArtifact) Confidence() float64 { return 1.0 }
func (a testArtifact) Raw() any            { return a.val }

// setupTransformer creates a SelfReviewTransformer wired to a ToyScribeStore via MCPAdapter.
func setupTransformer(t *testing.T, scribe *stubs.ToyScribeStore, repoPath string) *selfreview.SelfReviewTransformer {
	t.Helper()

	registry := tool.NewRegistry()
	adapter := battmcp.NewMCPAdapter(registry)

	transport := scribe.Serve(t)
	ctx := context.Background()
	if err := adapter.RegisterMCP(ctx, "scribe", transport); err != nil {
		t.Fatalf("register MCP: %v", err)
	}

	return selfreview.New(registry, repoPath)
}

// writeFile creates a file with the given content in the temp dir.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// walkerStateWithTask creates a WalkerState with a prior node output containing task_id and fixed files.
func walkerStateWithTask(taskID string, fixedFiles []string) *circuit.WalkerState {
	ws := circuit.NewWalkerState("test")
	ws.Outputs["poll-scribe"] = testArtifact{val: map[string]any{
		"task_id": taskID,
	}}
	ws.Outputs["fix"] = testArtifact{val: &sdlctype.FixResult{
		Fixed:   fixedFiles,
		Applied: "test fix",
	}}
	return ws
}

func TestSelfReview_ThreeRequirements_TwoCovered_Fails(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create source files that address 2 of 3 requirements.
	writeFile(t, tmpDir, "handler.go", `package main
// HandleRequest processes incoming API requests.
func HandleRequest(r *Request) (*Response, error) {
	return &Response{Status: 200}, nil
}
`)
	writeFile(t, tmpDir, "validator.go", `package main
// ValidateInput checks that input fields are within bounds.
func ValidateInput(input *Input) error {
	if input.Name == "" {
		return ErrMissingName
	}
	return nil
}
`)

	// Seed Scribe with a task that has 3 requirements (title + 2 sections).
	// Requirement 1 (title): "implement request handler" — covered by handler.go
	// Requirement 2 (section "validation"): "validate input fields" — covered by validator.go
	// Requirement 3 (section "logging"): "structured logging with slog" — NOT covered
	store := stubs.NewToyScribeStore()
	store.Seed(&stubs.ToyArtifact{
		ID:     "TSK-42",
		Kind:   "task",
		Title:  "implement request handler",
		Status: "in_progress",
		Sections: map[string]string{
			"validation": "validate input fields within bounds",
			"logging":    "add structured logging with slog for all endpoints",
		},
	})

	tr := setupTransformer(t, store, tmpDir)
	ctx := context.Background()

	tc := &handler.InstrumentContext{
		WalkerState: walkerStateWithTask("TSK-42", []string{"handler.go", "validator.go"}),
		Config:      map[string]any{},
	}

	result, err := tr.Transform(ctx, tc)
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}

	sr, ok := result.(*sdlctype.SelfReviewResult)
	if !ok {
		t.Fatalf("result type = %T, want *SelfReviewResult", result)
	}

	// Must fail: only 2 of 3 requirements have evidence.
	if sr.AllVerified {
		t.Fatal("expected all_verified=false (logging requirement not covered)")
	}

	// Count verified stamps.
	verified := 0
	unverified := 0
	for _, s := range sr.Stamps {
		switch s.Status {
		case selfreview.StampVerified:
			verified++
			if s.Evidence == "" {
				t.Errorf("verified stamp %q has empty evidence", s.Field)
			}
		case selfreview.StampUnverified:
			unverified++
		}
	}
	if verified < 2 {
		t.Errorf("verified = %d, want >= 2", verified)
	}
	if unverified < 1 {
		t.Errorf("unverified = %d, want >= 1", unverified)
	}

	// Stamps should be attached to Scribe artifact.
	art := store.Get("TSK-42")
	if art == nil {
		t.Fatal("artifact not found in store")
	}
	if art.Sections["stamps"] == "" {
		t.Fatal("stamps section not attached to Scribe artifact")
	}
}

func TestSelfReview_AllCovered_Passes(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create a file that covers both requirements.
	writeFile(t, tmpDir, "main.go", `package main
import "log/slog"
// HandleRequest with structured logging.
func HandleRequest() {
	slog.Info("handling request")
}
`)

	store := stubs.NewToyScribeStore()
	store.Seed(&stubs.ToyArtifact{
		ID:     "TSK-1",
		Kind:   "task",
		Title:  "implement request handler",
		Status: "in_progress",
		Sections: map[string]string{
			"logging": "add slog structured logging",
		},
	})

	tr := setupTransformer(t, store, tmpDir)
	ctx := context.Background()

	tc := &handler.InstrumentContext{
		WalkerState: walkerStateWithTask("TSK-1", []string{"main.go"}),
		Config:      map[string]any{},
	}

	result, err := tr.Transform(ctx, tc)
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}

	sr := result.(*sdlctype.SelfReviewResult)
	if !sr.AllVerified {
		t.Fatalf("expected all_verified=true, stamps: %+v", sr.Stamps)
	}
}

func TestSelfReview_NoModifiedFiles_AllUnverified(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	store := stubs.NewToyScribeStore()
	store.Seed(&stubs.ToyArtifact{
		ID:     "TSK-2",
		Kind:   "task",
		Title:  "refactor database layer",
		Status: "in_progress",
		Sections: map[string]string{
			"migration": "add migration for new schema",
		},
	})

	tr := setupTransformer(t, store, tmpDir)
	ctx := context.Background()

	// Walker state with task_id but no fix output.
	ws := circuit.NewWalkerState("test")
	ws.Outputs["poll-scribe"] = testArtifact{val: map[string]any{
		"task_id": "TSK-2",
	}}

	tc := &handler.InstrumentContext{
		WalkerState: ws,
		Config:      map[string]any{},
	}

	result, err := tr.Transform(ctx, tc)
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}

	sr := result.(*sdlctype.SelfReviewResult)
	if sr.AllVerified {
		t.Fatal("expected all_verified=false when no files modified")
	}
	for _, s := range sr.Stamps {
		if s.Status != selfreview.StampUnverified {
			t.Errorf("stamp %q status = %q, want unverified", s.Field, s.Status)
		}
	}
}

func TestSelfReview_NoTaskID_Error(t *testing.T) {
	t.Parallel()

	store := stubs.NewToyScribeStore()
	tr := setupTransformer(t, store, t.TempDir())
	ctx := context.Background()

	tc := &handler.InstrumentContext{
		WalkerState: circuit.NewWalkerState("test"),
		Config:      map[string]any{},
	}

	_, err := tr.Transform(ctx, tc)
	if err == nil {
		t.Fatal("expected error when no task_id")
	}
	if !errors.Is(err, selfreview.ErrNoTaskID) {
		t.Errorf("error = %v, want ErrNoTaskID", err)
	}
}

func TestSelfReview_TaskIDFromConfig(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	writeFile(t, tmpDir, "fix.go", `package main
// FixDatabase repairs corrupted records.
func FixDatabase() {}
`)

	store := stubs.NewToyScribeStore()
	store.Seed(&stubs.ToyArtifact{
		ID:     "TSK-99",
		Kind:   "task",
		Title:  "fix database corruption",
		Status: "in_progress",
	})

	tr := setupTransformer(t, store, tmpDir)
	ctx := context.Background()

	ws := circuit.NewWalkerState("test")
	ws.Outputs["fix"] = testArtifact{val: &sdlctype.FixResult{
		Fixed: []string{"fix.go"},
	}}

	tc := &handler.InstrumentContext{
		WalkerState: ws,
		Config:      map[string]any{"task_id": "TSK-99"},
	}

	result, err := tr.Transform(ctx, tc)
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}

	sr := result.(*sdlctype.SelfReviewResult)
	if !sr.AllVerified {
		t.Errorf("expected all_verified=true, stamps: %+v", sr.Stamps)
	}
}

func TestSelfReview_StationLog(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	writeFile(t, tmpDir, "main.go", `package main
func main() {}
`)

	store := stubs.NewToyScribeStore()
	store.Seed(&stubs.ToyArtifact{
		ID:     "TSK-3",
		Kind:   "task",
		Title:  "implement main entrypoint",
		Status: "in_progress",
	})

	tr := setupTransformer(t, store, tmpDir)
	ctx := context.Background()

	tc := &handler.InstrumentContext{
		WalkerState: walkerStateWithTask("TSK-3", []string{"main.go"}),
		Config:      map[string]any{},
	}

	_, err := tr.Transform(ctx, tc)
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}

	log := tr.LastStationLog()
	if log == nil {
		t.Fatal("station log is nil")
	}
	if log.StationLogType() != "self-review" {
		t.Errorf("station log type = %q, want self-review", log.StationLogType())
	}
}
