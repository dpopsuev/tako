package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami/lint"
	originamilsp "github.com/dpopsuev/origami/lsp"
	"github.com/dpopsuev/origami/transformers"
)

// Build-time variables injected via -ldflags.
// Example: go build -ldflags "-X main.version=v0.1.0 -X main.commit=$(git rev-parse --short HEAD) -X main.date=$(date -u +%Y-%m-%d)"
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	var err error
	switch os.Args[1] {
	case "run":
		err = runCmd(os.Args[2:])
	case "validate":
		err = validateCmd(os.Args[2:])
	case "skill":
		err = skillCmd(os.Args[2:])
	case "lint":
		err = lintCmd(os.Args[2:])
	case "lsp":
		err = lspCmd()
	case "component":
		err = componentCmd(os.Args[2:])
	case "fold":
		err = foldCmd(os.Args[2:])
	case "autodoc":
		err = autodocCmd(os.Args[2:])
	case "capture":
		err = captureCmd(os.Args[2:])
	case "trace":
		err = traceCmd(os.Stdout, os.Args[2:])
	case "report":
		err = reportCmd(os.Stdout, os.Args[2:])
	case "diff":
		err = diffCmd(os.Stdout, os.Args[2:])
	case "validate-bundle":
		err = validateBundleCmd(os.Args[2:])
	case "calibrate":
		err = calibrateCmd(os.Args[2:])
	case "version", "--version":
		fmt.Printf("origami %s (%s, %s)\n", version, commit, date)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `Usage: origami <command> [flags]

Commands:
  run        Execute a circuit YAML
  validate   Validate a circuit YAML without executing
  lint       Static analysis for circuit YAML (rules, profiles, auto-fix)
  lsp        Language Server for circuit YAML (diagnostics, completion, hover)
  skill      Skill scaffolding (scaffold SKILL.md from circuit YAML)
  component  Component management (list, inspect, validate)
  fold       Compile a YAML manifest into a standalone binary
  autodoc    Generate documentation tree from circuit YAML
  capture    Capture an offline bundle for a schematic (e.g. gnd)
  trace      Read and render a JSONL execution trace
  report     Read and render a run report scorecard
  diff       Compare metrics between two runs
  validate-bundle  Validate a captured bundle against its manifest
  version    Print version`)
}

type setFlag map[string]any

func (s setFlag) String() string { return fmt.Sprintf("%v", map[string]any(s)) }
func (s setFlag) Set(v string) error {
	parts := strings.SplitN(v, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("%w: %q", ErrExpectedKeyValueGot, v)
	}
	s[parts[0]] = parts[1]
	return nil
}

func runCmd(args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	verbose := fs.Bool("v", false, "verbose output (debug level)")
	sets := make(setFlag)
	fs.Var(sets, "set", "set circuit variable (key=value), repeatable")
	_ = fs.Parse(args)

	if fs.NArg() < 1 {
		return ErrUsageOrigamiRunVSetKeyValueCircuitYaml
	}
	circuitPath := fs.Arg(0)

	level := slog.LevelInfo
	if *verbose {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	builtins := engine.TransformerRegistry{
		"file": transformers.NewFile(transformers.WithRootDir(filepath.Dir(circuitPath))),
	}

	opts := []engine.RunOption{
		engine.WithLogger(logger),
		engine.WithTransformers(builtins),
	}
	if len(sets) > 0 {
		opts = append(opts, engine.WithOverrides(map[string]any(sets)))
	}

	logger.InfoContext(ctx, "running circuit", slog.Any("path", circuitPath))
	if err := engine.Run(ctx, circuitPath, nil, opts...); err != nil {
		return err
	}
	logger.InfoContext(ctx, "circuit completed")
	return nil
}

func validateCmd(args []string) error {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	_ = fs.Parse(args)

	if fs.NArg() < 1 {
		return ErrUsageOrigamiValidateCircuitYaml
	}
	circuitPath := fs.Arg(0)

	if err := engine.Validate(circuitPath); err != nil {
		return err
	}
	fmt.Printf("OK: %s is valid\n", circuitPath)
	return nil
}

//nolint:gocyclo // CLI command with flag parsing, file I/O, and output formatting
func lintCmd(args []string) error {
	fs := flag.NewFlagSet("lint", flag.ContinueOnError)
	profile := fs.String("profile", "moderate", "lint profile: min, basic, moderate, strict")
	format := fs.String("format", "text", "output format: text, json")
	fix := fs.Bool("fix", false, "apply auto-fixes and print diff")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return ErrUsageOrigamiLintProfileNameFormatTextJsonFixFileYaml
	}

	p := lint.Profile(*profile)
	exitCode := 0

	// Build project file index for cross-reference validation.
	// Collect all YAML files from directories containing the lint targets.
	projectRaw := make(map[string][]byte)
	seen := make(map[string]bool)
	for _, file := range fs.Args() {
		dir := filepath.Dir(file)
		if seen[dir] {
			continue
		}
		seen[dir] = true
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			if e.IsDir() || !(strings.HasSuffix(e.Name(), ".yaml") || strings.HasSuffix(e.Name(), ".yml")) {
				continue
			}
			fp := filepath.Join(dir, e.Name())
			if data, err := os.ReadFile(fp); err == nil {
				projectRaw[fp] = data
			}
		}
	}
	projectFiles := lint.LoadProjectFiles(projectRaw)

	lintOpts := []lint.Option{
		lint.WithProjectFiles(projectFiles),
	}

	for _, file := range fs.Args() {
		raw, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %s: %v\n", file, err)
			exitCode = 2
			continue
		}

		if *fix {
			fixed, fixes, err := lint.ApplyFixes(raw, file, lint.WithProfile(p))
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %s: %v\n", file, err)
				exitCode = 2
				continue
			}
			if len(fixes) == 0 {
				fmt.Fprintf(os.Stderr, "%s: no fixes to apply\n", file)
				continue
			}
			if err := os.WriteFile(file, fixed, 0o600); err != nil {
				return fmt.Errorf("write %s: %w", file, err)
			}
			for j := range fixes {
				fmt.Printf("fixed: %s\n", fixes[j].Finding)
			}
			continue
		}

		opts := append([]lint.Option{lint.WithProfile(p)}, lintOpts...)
		findings, err := lint.Run(raw, file, opts...)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %s: %v\n", file, err)
			exitCode = 2
			continue
		}

		if *format == "json" {
			data, _ := json.MarshalIndent(findings, "", "  ")
			fmt.Println(string(data))
		} else {
			for _, f := range findings {
				fmt.Println(f.String())
			}
		}

		if lint.HasErrors(findings) && exitCode < 2 {
			exitCode = 2
		} else if lint.HasWarnings(findings) && exitCode < 1 {
			exitCode = 1
		}
	}

	if exitCode > 0 {
		os.Exit(exitCode)
	}
	return nil
}

func lspCmd() error {
	srv := originamilsp.NewServer()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	stream := originamilsp.NewStdioStream(os.Stdin, os.Stdout)
	conn := originamilsp.ServeStream(ctx, srv, stream)
	srv.SetConn(conn)

	slog.InfoContext(ctx, "origami-lsp started", slog.Any("transport", "stdio"))
	<-ctx.Done()
	return nil
}
