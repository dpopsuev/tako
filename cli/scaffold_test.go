package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/dpopsuev/origami/circuit"
	"github.com/spf13/cobra"
)

func TestBuild_NoAnalyzeOK(t *testing.T) {
	c, err := NewCLI("test", "test tool").Build()
	if err != nil {
		t.Fatalf("Build without WithAnalyze should succeed: %v", err)
	}
	if findSubcommand(c.Root(), "analyze") != nil {
		t.Error("analyze should not be registered when WithAnalyze is not called")
	}
}

func TestBuild_MinimalCLI(t *testing.T) {
	c, err := NewCLI("test", "test tool").
		WithAnalyze(func(_ context.Context, _ []string) error { return nil }).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	root := c.Root()
	if root.Use != "test" {
		t.Errorf("root.Use = %q, want %q", root.Use, "test")
	}

	found := findSubcommand(root, "analyze")
	if found == nil {
		t.Fatal("missing analyze subcommand")
	}
}

func TestBuild_AllTiers(t *testing.T) {
	c, err := NewCLI("full", "full tool").
		WithVersion("1.0.0").
		WithAnalyze(func(_ context.Context, _ []string) error { return nil }).
		WithDataset(&mockDataset{}).
		WithCalibrate(&mockCalibrate{}).
		WithCircuit("a.yaml", "b.yaml").
		WithConsume("ingest.yaml").
		WithServe(ServeConfig{StartFunc: func(_ context.Context) error { return nil }}).
		WithDemo(DemoConfig{StartFunc: func(_ context.Context, _ int, _ float64) error { return nil }}).
		WithProfile(ProfileConfig{
			RunFunc:     func(_ context.Context, _ string) error { return nil },
			ReportFunc:  func(_ context.Context) error { return nil },
			CompareFunc: func(_ context.Context, _, _ string) error { return nil },
		}).
		WithExtraCommand(&cobra.Command{Use: "push", Short: "Push results"}).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	root := c.Root()
	expected := []string{
		"analyze", "dataset", "calibrate", "circuit",
		"consume", "serve", "demo", "profile", "push",
	}
	for _, name := range expected {
		if findSubcommand(root, name) == nil {
			t.Errorf("missing subcommand %q", name)
		}
	}
}

func TestBuild_DatasetSubcommands(t *testing.T) {
	c, err := NewCLI("test", "test").
		WithAnalyze(func(_ context.Context, _ []string) error { return nil }).
		WithDataset(&mockDataset{}).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	ds := findSubcommand(c.Root(), "dataset")
	if ds == nil {
		t.Fatal("missing dataset command")
	}

	subs := []string{"list", "status", "import", "export", "review", "promote"}
	for _, name := range subs {
		if findSubcommand(ds, name) == nil {
			t.Errorf("missing dataset subcommand %q", name)
		}
	}
}

func TestBuild_CalibrateSubcommands(t *testing.T) {
	c, err := NewCLI("test", "test").
		WithAnalyze(func(_ context.Context, _ []string) error { return nil }).
		WithCalibrate(&mockCalibrate{}).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	cal := findSubcommand(c.Root(), "calibrate")
	if cal == nil {
		t.Fatal("missing calibrate command")
	}

	subs := []string{"run", "report", "compare"}
	for _, name := range subs {
		if findSubcommand(cal, name) == nil {
			t.Errorf("missing calibrate subcommand %q", name)
		}
	}
}

func TestBuild_CircuitSubcommands(t *testing.T) {
	c, err := NewCLI("test", "test").
		WithAnalyze(func(_ context.Context, _ []string) error { return nil }).
		WithCircuit("rca.yaml").
		Build()
	if err != nil {
		t.Fatal(err)
	}

	pl := findSubcommand(c.Root(), "circuit")
	if pl == nil {
		t.Fatal("missing circuit command")
	}

	subs := []string{"list", "validate", "render", "replay"}
	for _, name := range subs {
		if findSubcommand(pl, name) == nil {
			t.Errorf("missing circuit subcommand %q", name)
		}
	}
}

func TestBuild_ProfileSubcommands(t *testing.T) {
	c, err := NewCLI("test", "test").
		WithAnalyze(func(_ context.Context, _ []string) error { return nil }).
		WithProfile(ProfileConfig{
			RunFunc:     func(_ context.Context, _ string) error { return nil },
			ReportFunc:  func(_ context.Context) error { return nil },
			CompareFunc: func(_ context.Context, _, _ string) error { return nil },
		}).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	prof := findSubcommand(c.Root(), "profile")
	if prof == nil {
		t.Fatal("missing profile command")
	}

	subs := []string{"run", "report", "compare"}
	for _, name := range subs {
		if findSubcommand(prof, name) == nil {
			t.Errorf("missing profile subcommand %q", name)
		}
	}
}

func TestBuild_CommonFlags(t *testing.T) {
	c, err := NewCLI("test", "test").
		WithAnalyze(func(_ context.Context, _ []string) error { return nil }).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	flags := []string{"verbose", "debug", "output", "config"}
	for _, name := range flags {
		f := c.Root().PersistentFlags().Lookup(name)
		if f == nil {
			t.Errorf("missing persistent flag %q", name)
		}
	}
}

func TestBuild_OptionalCommandsNotRegistered(t *testing.T) {
	c, err := NewCLI("test", "test").
		WithAnalyze(func(_ context.Context, _ []string) error { return nil }).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	absent := []string{"dataset", "calibrate", "circuit", "consume", "serve", "demo", "profile"}
	for _, name := range absent {
		if findSubcommand(c.Root(), name) != nil {
			t.Errorf("command %q should not be registered", name)
		}
	}
}

func TestBuild_HelpOutput(t *testing.T) {
	c, err := NewCLI("asterisk", "Root-cause analysis tool").
		WithAnalyze(func(_ context.Context, _ []string) error { return nil }).
		WithDataset(&mockDataset{}).
		WithCalibrate(&mockCalibrate{}).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	var buf strings.Builder
	c.Root().SetOut(&buf)
	c.Root().SetArgs([]string{"--help"})
	_ = c.Execute()

	help := buf.String()
	for _, sub := range []string{"analyze", "dataset", "calibrate"} {
		if !strings.Contains(help, sub) {
			t.Errorf("help output missing %q", sub)
		}
	}
}

func TestBuild_DefaultObservability(t *testing.T) {
	c, err := NewCLI("test", "test tool").Build()
	if err != nil {
		t.Fatal(err)
	}

	obs := c.Observers()
	if len(obs) != 2 {
		t.Fatalf("default observers count = %d, want 2", len(obs))
	}
	if c.MetricsHandler() == nil {
		t.Error("MetricsHandler should be non-nil with default observability")
	}
	if c.PrometheusRegistry() == nil {
		t.Error("PrometheusRegistry should be non-nil with default observability")
	}
}

func TestBuild_WithObservabilityEmpty(t *testing.T) {
	c, err := NewCLI("test", "test tool").
		WithObservability().
		Build()
	if err != nil {
		t.Fatal(err)
	}

	obs := c.Observers()
	if len(obs) != 0 {
		t.Fatalf("empty WithObservability should yield 0 observers, got %d", len(obs))
	}
	if c.MetricsHandler() != nil {
		t.Error("MetricsHandler should be nil when observability is disabled")
	}
}

func TestBuild_WithObservabilityCustom(t *testing.T) {
	var events []string
	custom := circuit.WalkObserverFunc(func(e circuit.WalkEvent) {
		events = append(events, string(e.Type))
	})

	c, err := NewCLI("test", "test tool").
		WithObservability(custom).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	obs := c.Observers()
	if len(obs) != 1 {
		t.Fatalf("observers count = %d, want 1", len(obs))
	}

	obs[0].OnEvent(circuit.WalkEvent{Type: circuit.EventNodeEnter})
	if len(events) != 1 {
		t.Errorf("custom observer should have received 1 event, got %d", len(events))
	}
}

// --- test helpers ---

func findSubcommand(parent *cobra.Command, name string) *cobra.Command {
	for _, cmd := range parent.Commands() {
		if cmd.Name() == name {
			return cmd
		}
	}
	return nil
}

type mockDataset struct{}

func (m *mockDataset) List() ([]DatasetSummary, error) {
	return []DatasetSummary{{Name: "test", CaseCount: 5, Status: "active"}}, nil
}
func (m *mockDataset) Status(_ string) (*DatasetStatus, error) {
	return &DatasetStatus{Name: "test", CaseCount: 5}, nil
}
func (m *mockDataset) Import(_ string) error          { return nil }
func (m *mockDataset) Export(_ string) error          { return nil }
func (m *mockDataset) ListCandidates() ([]Candidate, error) { return nil, nil }
func (m *mockDataset) Promote(_ string) error         { return nil }

type mockCalibrate struct{}

func (m *mockCalibrate) Run(_ context.Context, _ string) error      { return nil }
func (m *mockCalibrate) Report(_ context.Context, _ string) error   { return nil }
func (m *mockCalibrate) Compare(_ context.Context, _, _ string) error { return nil }
