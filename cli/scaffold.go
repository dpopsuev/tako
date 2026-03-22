package cli

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/kami"
	"github.com/dpopsuev/origami/lint"
	"github.com/dpopsuev/origami/observability"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
)

// AnalyzeFunc is the consumer's domain function — the reason the tool exists.
type AnalyzeFunc func(ctx context.Context, args []string) error

// DatasetStore manages datasets (list, status, import, export, review, promote).
type DatasetStore interface {
	List() ([]DatasetSummary, error)
	Status(name string) (*DatasetStatus, error)
	Import(path string) error
	Export(path string) error
	ListCandidates() ([]Candidate, error)
	Promote(caseID string) error
}

// DatasetSummary is the short view of a dataset.
type DatasetSummary struct {
	Name       string `json:"name"`
	CaseCount  int    `json:"case_count"`
	Status     string `json:"status"`
}

// DatasetStatus is the detailed view of a dataset.
type DatasetStatus struct {
	Name       string `json:"name"`
	CaseCount  int    `json:"case_count"`
	Reviewed   int    `json:"reviewed"`
	Promoted   int    `json:"promoted"`
}

// Candidate is a case that can be promoted into a dataset.
type Candidate struct {
	CaseID string `json:"case_id"`
	Source string `json:"source"`
}

// CalibrateRunner runs scorecard evaluation against a dataset.
type CalibrateRunner interface {
	Run(ctx context.Context, scenario string) error
	Report(ctx context.Context, scenario string) error
	Compare(ctx context.Context, a, b string) error
}

// ServeConfig configures the MCP server command.
type ServeConfig struct {
	StartFunc func(ctx context.Context) error
}

// DemoConfig configures the Kabuki demo server command.
type DemoConfig struct {
	StartFunc func(ctx context.Context, port int, speed float64) error
}

// ProfileConfig configures the Ouroboros profile command.
type ProfileConfig struct {
	RunFunc     func(ctx context.Context, model string) error
	ReportFunc  func(ctx context.Context) error
	CompareFunc func(ctx context.Context, a, b string) error
}

// CLIBuilder assembles a consumer CLI from Origami-provided commands.
type CLIBuilder struct {
	name        string
	description string
	version     string

	analyzeFn   AnalyzeFunc
	dataset     DatasetStore
	calibrate   CalibrateRunner
	circuits   []string
	consume     *string
	serve       *ServeConfig
	demo        *DemoConfig
	profile     *ProfileConfig
	extra        []*cobra.Command
	observers    []circuit.WalkObserver
	obsExplicit  bool
	promReg      *prometheus.Registry
}

// NewCLI creates a CLI builder for the named tool.
func NewCLI(name, description string) *CLIBuilder {
	return &CLIBuilder{name: name, description: description}
}

func (b *CLIBuilder) WithVersion(v string) *CLIBuilder {
	b.version = v
	return b
}

func (b *CLIBuilder) WithAnalyze(fn AnalyzeFunc) *CLIBuilder {
	b.analyzeFn = fn
	return b
}

func (b *CLIBuilder) WithDataset(store DatasetStore) *CLIBuilder {
	b.dataset = store
	return b
}

func (b *CLIBuilder) WithCalibrate(runner CalibrateRunner) *CLIBuilder {
	b.calibrate = runner
	return b
}

func (b *CLIBuilder) WithCircuit(defs ...string) *CLIBuilder {
	b.circuits = append(b.circuits, defs...)
	return b
}

func (b *CLIBuilder) WithConsume(circuit string) *CLIBuilder {
	b.consume = &circuit
	return b
}

func (b *CLIBuilder) WithServe(cfg ServeConfig) *CLIBuilder {
	b.serve = &cfg
	return b
}

func (b *CLIBuilder) WithDemo(cfg DemoConfig) *CLIBuilder {
	b.demo = &cfg
	return b
}

func (b *CLIBuilder) WithProfile(cfg ProfileConfig) *CLIBuilder {
	b.profile = &cfg
	return b
}

func (b *CLIBuilder) WithExtraCommand(cmd *cobra.Command) *CLIBuilder {
	b.extra = append(b.extra, cmd)
	return b
}

// WithObservability registers walk observers for the CLI.
// If not called, Build() uses DefaultObservability() automatically.
// Call with no arguments to disable observability entirely.
func (b *CLIBuilder) WithObservability(observers ...circuit.WalkObserver) *CLIBuilder {
	b.observers = observers
	b.obsExplicit = true
	return b
}

// CLI is the assembled command tree ready for execution.
type CLI struct {
	root           *cobra.Command
	observers      []circuit.WalkObserver
	promRegistry   *prometheus.Registry
	metricsHandler http.Handler
}

// Observers returns the configured walk observers.
func (c *CLI) Observers() []circuit.WalkObserver {
	return c.observers
}

// MetricsHandler returns a Prometheus /metrics HTTP handler, or nil
// if no Prometheus collector was configured.
func (c *CLI) MetricsHandler() http.Handler {
	return c.metricsHandler
}

// PrometheusRegistry returns the Prometheus registry used by observability,
// or nil if observability is disabled.
func (c *CLI) PrometheusRegistry() *prometheus.Registry {
	return c.promRegistry
}

// Build assembles the Cobra command tree. Returns an error if required
// configuration is missing (analyze is required).
func (b *CLIBuilder) Build() (*CLI, error) {
	root := &cobra.Command{
		Use:   b.name,
		Short: b.description,
	}

	root.PersistentFlags().BoolP("verbose", "v", false, "verbose output")
	root.PersistentFlags().Bool("debug", false, "debug-level output")
	root.PersistentFlags().StringP("output", "o", "table", "output format (json, table, markdown)")
	root.PersistentFlags().String("config", "", "config file path")

	if b.version != "" {
		root.Version = b.version
	}

	c := &CLI{root: root}

	if b.obsExplicit {
		c.observers = b.observers
	} else {
		reg := prometheus.NewRegistry()
		c.observers = observability.DefaultObservabilityWithRegistry(reg)
		b.promReg = reg
	}

	if b.promReg == nil {
		for _, obs := range c.observers {
			if pc, ok := obs.(*observability.PrometheusCollector); ok {
				b.promReg = pc.Registry
				break
			}
		}
	}

	if b.promReg != nil {
		c.promRegistry = b.promReg
		c.metricsHandler = promhttp.HandlerFor(b.promReg, promhttp.HandlerOpts{})
	}

	if b.analyzeFn != nil {
		root.AddCommand(b.buildAnalyze())
	}

	if b.dataset != nil {
		root.AddCommand(b.buildDataset())
	}
	if b.calibrate != nil {
		root.AddCommand(b.buildCalibrate())
	}
	if len(b.circuits) > 0 {
		root.AddCommand(b.buildCircuit())
	}
	if b.consume != nil {
		root.AddCommand(b.buildConsume())
	}
	if b.serve != nil {
		root.AddCommand(b.buildServe())
	}
	if b.demo != nil {
		root.AddCommand(b.buildDemo())
	}
	if b.profile != nil {
		root.AddCommand(b.buildProfile())
	}
	for _, cmd := range b.extra {
		root.AddCommand(cmd)
	}

	return c, nil
}

// Execute runs the CLI with os.Args. The standard entry point.
func (c *CLI) Execute() error {
	return c.root.Execute()
}

// Root returns the underlying Cobra command for testing or customization.
func (c *CLI) Root() *cobra.Command {
	return c.root
}

func (b *CLIBuilder) buildAnalyze() *cobra.Command {
	return &cobra.Command{
		Use:   "analyze [args...]",
		Short: "Run the domain analysis function",
		RunE: func(cmd *cobra.Command, args []string) error {
			return b.analyzeFn(cmd.Context(), args)
		},
	}
}

func (b *CLIBuilder) buildDataset() *cobra.Command {
	ds := &cobra.Command{
		Use:   "dataset",
		Short: "Manage datasets",
	}

	ds.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List available datasets",
		RunE: func(_ *cobra.Command, _ []string) error {
			items, err := b.dataset.List()
			if err != nil {
				return err
			}
			for _, d := range items {
				fmt.Printf("%-20s %d cases  [%s]\n", d.Name, d.CaseCount, d.Status)
			}
			return nil
		},
	})

	ds.AddCommand(&cobra.Command{
		Use:   "status <name>",
		Short: "Show dataset status",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			st, err := b.dataset.Status(args[0])
			if err != nil {
				return err
			}
			fmt.Printf("Dataset: %s\nCases: %d\nReviewed: %d\nPromoted: %d\n",
				st.Name, st.CaseCount, st.Reviewed, st.Promoted)
			return nil
		},
	})

	importCmd := &cobra.Command{
		Use:   "import <path>",
		Short: "Import a dataset from path",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return b.dataset.Import(args[0])
		},
	}
	ds.AddCommand(importCmd)

	ds.AddCommand(&cobra.Command{
		Use:   "export <path>",
		Short: "Export dataset to path",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return b.dataset.Export(args[0])
		},
	})

	ds.AddCommand(&cobra.Command{
		Use:   "review",
		Short: "List candidates for review",
		RunE: func(_ *cobra.Command, _ []string) error {
			candidates, err := b.dataset.ListCandidates()
			if err != nil {
				return err
			}
			for _, c := range candidates {
				fmt.Printf("%-30s  %s\n", c.CaseID, c.Source)
			}
			return nil
		},
	})

	ds.AddCommand(&cobra.Command{
		Use:   "promote <case-id>",
		Short: "Promote a case to the dataset",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return b.dataset.Promote(args[0])
		},
	})

	return ds
}

func (b *CLIBuilder) buildCalibrate() *cobra.Command {
	cal := &cobra.Command{
		Use:   "calibrate",
		Short: "Run scorecard evaluation",
	}

	cal.AddCommand(&cobra.Command{
		Use:   "run <scenario>",
		Short: "Run calibration against a scenario",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return b.calibrate.Run(cmd.Context(), args[0])
		},
	})

	cal.AddCommand(&cobra.Command{
		Use:   "report <scenario>",
		Short: "Show calibration report",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return b.calibrate.Report(cmd.Context(), args[0])
		},
	})

	cal.AddCommand(&cobra.Command{
		Use:   "compare <a> <b>",
		Short: "Compare two calibration runs",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return b.calibrate.Compare(cmd.Context(), args[0], args[1])
		},
	})

	return cal
}

func (b *CLIBuilder) buildCircuit() *cobra.Command {
	pl := &cobra.Command{
		Use:   "circuit",
		Short: "Circuit operations",
	}

	pl.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List registered circuits",
		RunE: func(_ *cobra.Command, _ []string) error {
			for _, p := range b.circuits {
				fmt.Println(p)
			}
			return nil
		},
	})

	validateCmd := &cobra.Command{
		Use:   "validate <circuit>",
		Short: "Validate a circuit YAML (lint + structural check)",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			raw, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("read %s: %w", args[0], err)
			}
			findings, err := lint.Run(raw, args[0])
			if err != nil {
				return fmt.Errorf("lint %s: %w", args[0], err)
			}
			if lint.HasErrors(findings) {
				for _, f := range findings {
					fmt.Fprintln(os.Stderr, f.String())
				}
				return fmt.Errorf("%s has %d error(s)", args[0], len(findings))
			}
			if lint.HasWarnings(findings) {
				for _, f := range findings {
					fmt.Fprintln(os.Stderr, f.String())
				}
			}
			fmt.Printf("OK: %s is valid\n", args[0])
			return nil
		},
	}
	pl.AddCommand(validateCmd)

	renderCmd := &cobra.Command{
		Use:   "render <circuit>",
		Short: "Render circuit as text or DOT graph",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			raw, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("read %s: %w", args[0], err)
			}
			def, err := circuit.LoadCircuit(raw)
			if err != nil {
				return fmt.Errorf("parse %s: %w", args[0], err)
			}
			format, _ := cmd.Flags().GetString("format")
			if format == "dot" {
				fmt.Println("digraph circuit {")
				fmt.Println("  rankdir=LR;")
				for _, n := range def.Nodes {
					label := n.Name
					if n.Approach != "" {
						label += fmt.Sprintf(" [%s]", n.Approach)
					}
					fmt.Printf("  %q [label=%q];\n", n.Name, label)
				}
				for _, e := range def.Edges {
					label := e.Name
					if label == "" {
						label = e.ID
					}
					fmt.Printf("  %q -> %q [label=%q];\n", e.From, e.To, label)
				}
				fmt.Println("}")
			} else {
				fmt.Printf("Circuit: %s\n", def.Circuit)
				fmt.Printf("Start:    %s\n", def.Start)
				fmt.Printf("Done:     %s\n\n", def.Done)
				fmt.Println("Nodes:")
				for _, n := range def.Nodes {
					elem := ""
					if n.Approach != "" {
						elem = fmt.Sprintf(" [%s]", n.Approach)
					}
					fmt.Printf("  %-20s%s\n", n.Name, elem)
				}
				fmt.Println("\nEdges:")
				for _, e := range def.Edges {
					fmt.Printf("  %s → %s", e.From, e.To)
					if e.When != "" {
						fmt.Printf("  (when: %s)", e.When)
					}
					fmt.Println()
				}
			}
			return nil
		},
	}
	renderCmd.Flags().String("format", "text", "output format: text, dot")
	pl.AddCommand(renderCmd)

	replayCmd := &cobra.Command{
		Use:   "replay <recording.jsonl>",
		Short: "Replay a circuit recording via Kami",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			speed, _ := cmd.Flags().GetFloat64("speed")
			port, _ := cmd.Flags().GetInt("port")

			bridge := kami.NewEventBridge(nil)
			defer bridge.Close()

			logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

			srv := kami.NewServer(kami.Config{
				Port:   port,
				Bridge: bridge,
				Logger: logger,
				SPA:    kami.FrontendFS(),
			})

			ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt)
			defer cancel()

			rp, err := kami.NewReplayer(bridge, args[0], speed)
			if err != nil {
				return fmt.Errorf("create replayer: %w", err)
			}
			go func() {
				if err := rp.Play(ctx.Done()); err != nil {
					logger.Error("replay error", "error", err)
				}
				logger.Info("replay complete")
			}()

			return srv.Start(ctx)
		},
	}
	replayCmd.Flags().Float64("speed", 1.0, "replay speed multiplier")
	replayCmd.Flags().Int("port", 3000, "Kami server port")
	pl.AddCommand(replayCmd)

	return pl
}

func (b *CLIBuilder) buildConsume() *cobra.Command {
	consume := &cobra.Command{
		Use:   "consume",
		Short: "Ingest data from sources",
	}
	consume.AddCommand(&cobra.Command{
		Use:   "run",
		Short: "Run ingestion circuit",
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Printf("Running consume circuit %s...\n", *b.consume)
			return nil
		},
	})
	consume.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show last ingestion status",
		RunE: func(_ *cobra.Command, _ []string) error {
			info, err := os.Stat(*b.consume)
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Println("No ingestion runs recorded yet.")
					return nil
				}
				return fmt.Errorf("check circuit %s: %w", *b.consume, err)
			}
			fmt.Printf("Circuit:  %s\n", *b.consume)
			fmt.Printf("Last run:  %s\n", info.ModTime().Format(time.RFC3339))
			return nil
		},
	})
	return consume
}

func (b *CLIBuilder) buildServe() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Run as MCP server",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return b.serve.StartFunc(cmd.Context())
		},
	}
}

func (b *CLIBuilder) buildDemo() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "demo",
		Short: "Start Kabuki presentation server",
		RunE: func(cmd *cobra.Command, _ []string) error {
			port, _ := cmd.Flags().GetInt("port")
			speed, _ := cmd.Flags().GetFloat64("speed")
			return b.demo.StartFunc(cmd.Context(), port, speed)
		},
	}
	cmd.Flags().Int("port", 3000, "server port")
	cmd.Flags().Float64("speed", 1.0, "presentation speed")
	return cmd
}

func (b *CLIBuilder) buildProfile() *cobra.Command {
	prof := &cobra.Command{
		Use:   "profile",
		Short: "Ouroboros model discovery",
	}
	prof.AddCommand(&cobra.Command{
		Use:   "run <model>",
		Short: "Run discovery probes on a model",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return b.profile.RunFunc(cmd.Context(), args[0])
		},
	})
	prof.AddCommand(&cobra.Command{
		Use:   "report",
		Short: "Show discovery report",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return b.profile.ReportFunc(cmd.Context())
		},
	})
	prof.AddCommand(&cobra.Command{
		Use:   "compare <a> <b>",
		Short: "Compare two model profiles",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return b.profile.CompareFunc(cmd.Context(), args[0], args[1])
		},
	})
	return prof
}
