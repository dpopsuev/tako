package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	framework "github.com/dpopsuev/origami"
	"github.com/dpopsuev/origami/kami"
	"github.com/dpopsuev/origami/lint"
	originamilsp "github.com/dpopsuev/origami/lsp"
	fwmcp "github.com/dpopsuev/origami/mcp"
	"github.com/dpopsuev/origami/models"
	"github.com/dpopsuev/origami/ouroboros"
	"github.com/dpopsuev/origami/ouroboros/mcp"
	"github.com/dpopsuev/origami/ouroboros/probes"
	studiobackend "github.com/dpopsuev/origami/studio/backend"
	"github.com/dpopsuev/origami/sumi"
	"github.com/dpopsuev/origami/transformers"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
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
	case "ouroboros":
		err = ouroborosCmd(os.Args[2:])
	case "lint":
		err = lintCmd(os.Args[2:])
	case "lsp":
		err = lspCmd()
	case "kami":
		err = kamiCmd(os.Args[2:])
	case "studio":
		err = studioCmd(os.Args[2:])
	case "component":
		err = componentCmd(os.Args[2:])
	case "sumi":
		err = sumiCmd(os.Args[2:])
	case "fold":
		err = foldCmd(os.Args[2:])
	case "serve":
		err = serveCmd(os.Args[2:])
	case "autodoc":
		err = autodocCmd(os.Args[2:])
	case "capture":
		err = captureCmd(os.Args[2:])
	case "trace":
		err = traceCmd(os.Args[2:])
	case "report":
		err = reportCmd(os.Args[2:])
	case "validate-bundle":
		err = validateBundleCmd(os.Args[2:])
	case "version":
		fmt.Println("origami v1.0.0")
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
  ouroboros  Ouroboros meta-calibration tools (prompt, analyze, save, serve)
  kami       Live circuit debugger (HTTP/SSE + WS)
  kami serve Start Kami MCP server over stdio (co-starts HTTP/WS)
  studio     Visual Circuit Editor (embedded SPA + REST API)
  component  Component management (list, inspect, validate)
  sumi       Terminal circuit viewer and debugger (TUI)
  fold       Compile a YAML manifest into a standalone binary
  serve      Run the MCP gateway proxy (routes to backend engines)
  autodoc    Generate documentation tree from circuit YAML
  capture    Capture an offline bundle for a schematic (e.g. gnd)
  trace      Read and render a JSONL execution trace
  report     Read and render a run report scorecard
  validate-bundle  Validate a captured bundle against its manifest
  version    Print version`)
}

type setFlag map[string]any

func (s setFlag) String() string { return fmt.Sprintf("%v", map[string]any(s)) }
func (s setFlag) Set(v string) error {
	parts := strings.SplitN(v, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("expected key=value, got %q", v)
	}
	s[parts[0]] = parts[1]
	return nil
}

func runCmd(args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	verbose := fs.Bool("v", false, "verbose output (debug level)")
	sets := make(setFlag)
	fs.Var(sets, "set", "set circuit variable (key=value), repeatable")
	fs.Parse(args)

	if fs.NArg() < 1 {
		return fmt.Errorf("usage: origami run [-v] [--set key=value] <circuit.yaml>")
	}
	circuitPath := fs.Arg(0)

	level := slog.LevelInfo
	if *verbose {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	builtins := framework.TransformerRegistry{
		"file": transformers.NewFile(transformers.WithRootDir(filepath.Dir(circuitPath))),
	}

	opts := []framework.RunOption{
		framework.WithLogger(logger),
		framework.WithTransformers(builtins),
	}
	if len(sets) > 0 {
		opts = append(opts, framework.WithOverrides(map[string]any(sets)))
	}

	logger.Info("running circuit", "path", circuitPath)
	if err := framework.Run(ctx, circuitPath, nil, opts...); err != nil {
		return err
	}
	logger.Info("circuit completed")
	return nil
}

func validateCmd(args []string) error {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	fs.Parse(args)

	if fs.NArg() < 1 {
		return fmt.Errorf("usage: origami validate <circuit.yaml>")
	}
	circuitPath := fs.Arg(0)

	if err := framework.Validate(circuitPath); err != nil {
		return err
	}
	fmt.Printf("OK: %s is valid\n", circuitPath)
	return nil
}

// --- kami subcommand ---

func kamiCmd(args []string) error {
	if len(args) > 0 && args[0] == "serve" {
		return kamiServe(args[1:])
	}
	if len(args) > 0 && args[0] == "reset" {
		return kamiReset(args[1:])
	}

	fs := flag.NewFlagSet("kami", flag.ExitOnError)
	port := fs.Int("port", 3000, "HTTP port (WS on port+1)")
	bind := fs.String("bind", "127.0.0.1", "bind address")
	debug := fs.Bool("debug", false, "enable debug API")
	replay := fs.String("replay", "", "replay a JSONL recording file")
	speed := fs.Float64("speed", 1.0, "replay speed multiplier")
	fs.Parse(args)

	bridge := kami.NewEventBridge(nil)
	defer bridge.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	srv := kami.NewServer(kami.Config{
		Port:   *port,
		Bind:   *bind,
		Debug:  *debug,
		Bridge: bridge,
		Logger: logger,
		SPA:    kami.FrontendFS(),
	})

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if *replay != "" {
		rp, err := kami.NewReplayer(bridge, *replay, *speed)
		if err != nil {
			return err
		}
		go func() {
			if err := rp.Play(ctx.Done()); err != nil {
				logger.Error("replay error", "error", err)
			}
			logger.Info("replay complete")
		}()
	}

	return srv.Start(ctx)
}

func kamiReset(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: origami kami reset <addr> (e.g. 127.0.0.1:3001)")
	}
	addr := args[0]
	url := fmt.Sprintf("http://%s/api/store/reset", addr)

	resp, err := http.Post(url, "application/json", nil)
	if err != nil {
		return fmt.Errorf("reset failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Println(string(body))
	return nil
}

func kamiServe(args []string) error {
	fs := flag.NewFlagSet("kami serve", flag.ContinueOnError)
	port := fs.Int("port", 3000, "HTTP port for Kami server (WS on port+1)")
	bind := fs.String("bind", "127.0.0.1", "bind address")
	if err := fs.Parse(args); err != nil {
		return err
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	bridge := kami.NewEventBridge(nil)
	defer bridge.Close()

	dc := kami.NewDebugController(bridge)
	kamiSrv := kami.NewServer(kami.Config{
		Port:   *port,
		Bind:   *bind,
		Debug:  true,
		Bridge: bridge,
		Logger: logger,
		SPA:    kami.FrontendFS(),
	})

	mcpSrv := fwmcp.NewServer("origami-kami-debugger", "1.0.0")
	kami.RegisterMCPTools(mcpSrv.MCPServer, dc, kamiSrv)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	fwmcp.WatchStdin(ctx, nil, cancel)

	go func() {
		logger.Info("kami HTTP+WS server starting", "addr", fmt.Sprintf("%s:%d", *bind, *port))
		if err := kamiSrv.Start(ctx); err != nil {
			logger.Error("kami server error", "error", err)
		}
	}()

	logger.Info("starting kami MCP server over stdio")
	return mcpSrv.MCPServer.Run(ctx, &sdkmcp.StdioTransport{})
}

// --- ouroboros subcommand group ---

func ouroborosCmd(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: origami ouroboros <run|prompt|analyze|save|serve> [flags]")
	}
	switch args[0] {
	case "run":
		return ouroborosRun(args[1:])
	case "prompt":
		return ouroborosPrompt(args[1:])
	case "analyze":
		return ouroborosAnalyze(args[1:])
	case "save":
		return ouroborosSave(args[1:])
	case "serve":
		return ouroborosServe(args[1:])
	default:
		return fmt.Errorf("unknown ouroboros subcommand: %s", args[0])
	}
}

func ouroborosRun(args []string) error {
	fs := flag.NewFlagSet("ouroboros run", flag.ContinueOnError)
	seedPath := fs.String("seed", "", "path to seed YAML file")
	verbose := fs.Bool("v", false, "verbose output")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *seedPath == "" {
		return fmt.Errorf("--seed is required\nusage: origami ouroboros run --seed <path>")
	}

	seed, err := ouroboros.LoadSeed(*seedPath)
	if err != nil {
		return fmt.Errorf("load seed: %w", err)
	}

	level := slog.LevelInfo
	if *verbose {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	logger.Info("running ouroboros probe", "seed", seed.Name, "category", seed.Category)

	circuitPath := "ouroboros/circuits/ouroboros-probe.yaml"
	dispatcher := func(_ context.Context, nodeName string, prompt string) (string, error) {
		return "", fmt.Errorf("node %q: no dispatcher configured (use --serve for MCP dispatch)", nodeName)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	nodes := ouroboros.CircuitNodes(seed, dispatcher)
	if err := framework.Run(ctx, circuitPath, nil,
		framework.WithNodes(nodes),
		framework.WithLogger(logger),
	); err != nil {
		return err
	}

	logger.Info("probe completed", "seed", seed.Name)
	return nil
}

func ouroborosPrompt(args []string) error {
	fs := flag.NewFlagSet("ouroboros prompt", flag.ContinueOnError)
	excludeFile := fs.String("exclude-file", "", "JSON file with array of ModelIdentity to exclude")
	if err := fs.Parse(args); err != nil {
		return err
	}

	var exclude []framework.ModelIdentity
	if *excludeFile != "" {
		data, err := os.ReadFile(*excludeFile)
		if err != nil {
			return fmt.Errorf("read exclude file: %w", err)
		}
		if err := json.Unmarshal(data, &exclude); err != nil {
			return fmt.Errorf("parse exclude file: %w", err)
		}
	}

	fmt.Print(ouroboros.BuildFullPromptWith(exclude, probes.RefactorPrompt()))
	return nil
}

type analyzeResult struct {
	Identity framework.ModelIdentity `json:"identity"`
	Key      string                  `json:"key"`
	Code     string                  `json:"code"`
	Score    ouroboros.ProbeScore      `json:"score"`
	Known    bool                    `json:"known"`
	Wrapper  bool                    `json:"wrapper"`
}

func ouroborosAnalyze(args []string) error {
	fs := flag.NewFlagSet("ouroboros analyze", flag.ContinueOnError)
	responseFile := fs.String("response-file", "", "text file with raw subagent response (- for stdin)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *responseFile == "" {
		return fmt.Errorf("--response-file is required")
	}

	var data []byte
	var err error
	if *responseFile == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(*responseFile)
	}
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	raw := string(data)

	mi, err := ouroboros.ParseIdentityResponse(raw)
	if err != nil {
		return fmt.Errorf("parse identity: %w", err)
	}

	code, err := ouroboros.ParseProbeResponse(raw)
	if err != nil {
		return fmt.Errorf("parse code: %w", err)
	}

	score := probes.ScoreRefactorOutput(code)

	result := analyzeResult{
		Identity: mi,
		Key:      ouroboros.ModelKey(mi),
		Code:     code,
		Score:    score,
		Known:    models.IsKnownModel(mi),
		Wrapper:  models.IsWrapperName(mi.ModelName),
	}

	out, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}
	fmt.Println(string(out))
	return nil
}

const defaultRunsDir = "ouroboros/runs"

func ouroborosSave(args []string) error {
	fs := flag.NewFlagSet("ouroboros save", flag.ContinueOnError)
	reportFile := fs.String("report-file", "", "JSON file containing the RunReport (- for stdin)")
	runsDir := fs.String("runs-dir", defaultRunsDir, "directory to save run reports")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *reportFile == "" {
		return fmt.Errorf("--report-file is required")
	}

	var data []byte
	var err error
	if *reportFile == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(*reportFile)
	}
	if err != nil {
		return fmt.Errorf("read report: %w", err)
	}

	var report ouroboros.RunReport
	if err := json.Unmarshal(data, &report); err != nil {
		return fmt.Errorf("parse report: %w", err)
	}

	store, err := ouroboros.NewFileRunStore(*runsDir)
	if err != nil {
		return fmt.Errorf("create store: %w", err)
	}

	if err := store.SaveRun(report); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "saved run %q to %s\n", report.RunID, *runsDir)
	return nil
}

func ouroborosServe(args []string) error {
	fs := flag.NewFlagSet("ouroboros serve", flag.ContinueOnError)
	runsDir := fs.String("runs-dir", defaultRunsDir, "directory to save discovery run reports")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg := mcp.NewOuroborosConfig(*runsDir)
	srv := fwmcp.NewCircuitServer(cfg)
	mcp.RegisterExtraTools(srv, *runsDir)
	defer srv.Shutdown()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	fwmcp.WatchStdin(ctx, nil, cancel)

	slog.Info("starting ouroboros MCP server over stdio", "runs_dir", *runsDir)
	return srv.MCPServer.Run(ctx, &sdkmcp.StdioTransport{})
}

func lintCmd(args []string) error {
	fs := flag.NewFlagSet("lint", flag.ContinueOnError)
	profile := fs.String("profile", "moderate", "lint profile: min, basic, moderate, strict")
	format := fs.String("format", "text", "output format: text, json")
	fix := fs.Bool("fix", false, "apply auto-fixes and print diff")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return fmt.Errorf("usage: origami lint [--profile <name>] [--format text|json] [--fix] <file.yaml>...")
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
			if err := os.WriteFile(file, fixed, 0644); err != nil {
				return fmt.Errorf("write %s: %w", file, err)
			}
			for _, f := range fixes {
				fmt.Printf("fixed: %s\n", f.Finding)
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

func studioCmd(args []string) error {
	fs := flag.NewFlagSet("studio", flag.ContinueOnError)
	port := fs.Int("port", 8080, "HTTP port for Studio server")
	if err := fs.Parse(args); err != nil {
		return err
	}

	srv := studiobackend.NewStudioServer(*port, studiobackend.PlaceholderStaticFS())
	srv.Start()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	slog.Info("origami studio started", "port", *port, "url", fmt.Sprintf("http://localhost:%d", *port))
	<-ctx.Done()
	return srv.Stop(context.Background())
}

func lspCmd() error {
	srv := originamilsp.NewServer()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	stream := originamilsp.NewStdioStream(os.Stdin, os.Stdout)
	conn := originamilsp.ServeStream(ctx, srv, stream)
	srv.SetConn(conn)

	slog.Info("origami-lsp started", "transport", "stdio")
	<-ctx.Done()
	return nil
}

func sumiCmd(args []string) error {
	fs := flag.NewFlagSet("sumi", flag.ExitOnError)
	kamiAddr := fs.String("kami", "", "Kami server address for debug/agent features (e.g. 127.0.0.1:3000)")
	watch := fs.String("watch", "", "connect to running Kami SSE stream at address")
	replay := fs.String("replay", "", "replay a recorded JSONL session file")
	noColor := fs.Bool("no-color", false, "disable ANSI colors (CI/pipe-friendly)")
	compact := fs.Bool("compact", false, "reduced-width rendering")
	clean := fs.Bool("clean", false, "reset Kami store before connecting (--watch only)")
	fs.Parse(args)

	circuitPath := ""
	if fs.NArg() > 0 {
		circuitPath = fs.Arg(0)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	return sumi.Run(ctx, sumi.RunConfig{
		CircuitPath: circuitPath,
		KamiAddr:    *kamiAddr,
		WatchAddr:   *watch,
		ReplayFile:  *replay,
		NoColor:     *noColor,
		Compact:     *compact,
		Clean:       *clean,
	})
}
