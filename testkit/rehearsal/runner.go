package rehearsal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

type ActorFunc func(ctx context.Context, prompt string) (string, error)

type ActorFactory func(workspace string) (ActorFunc, error)

var (
	ErrMissingScenario = errors.New("rehearsal: scenario is required")
	ErrMissingReferee  = errors.New("rehearsal: referee is required")
	ErrMissingActor    = errors.New("rehearsal: actor or actor factory is required")
)

type Runner struct {
	scenario     Scenario
	referee      Referee
	operator     Operator
	actor        ActorFunc
	actorFactory ActorFactory
	sandbox      Sandbox
	setup        []SetupOption
	workspace    string
	log          *slog.Logger
}

func (r *Runner) Execute(ctx context.Context) (*RunMetrics, error) {
	log := r.log
	if log == nil {
		log = slog.Default()
	}

	workspace := r.workspace
	if workspace == "" {
		var err error
		workspace, err = os.MkdirTemp("", "rehearsal-"+r.scenario.ID()+"-")
		if err != nil {
			return nil, fmt.Errorf("rehearsal: create workspace: %w", err)
		}
		defer os.RemoveAll(workspace)
	}

	if r.sandbox != nil {
		handle, err := r.sandbox.Create(ctx, "none")
		if err != nil {
			return nil, fmt.Errorf("rehearsal: sandbox create: %w", err)
		}
		defer r.sandbox.Destroy(ctx, handle)
	}

	log.InfoContext(ctx, "rehearsal.start",
		slog.String("scenario", r.scenario.ID()),
		slog.String("workspace", workspace))

	actor := r.actor
	if r.actorFactory != nil {
		a, err := r.actorFactory(workspace)
		if err != nil {
			return nil, fmt.Errorf("rehearsal: actor factory: %w", err)
		}
		actor = a
	}

	start := time.Now()

	prompt := r.scenario.Spec()
	if r.operator != nil {
		resp, err := r.operator.Perform(ctx, prompt)
		if err != nil {
			return nil, fmt.Errorf("rehearsal: operator: %w", err)
		}
		if resp != "" {
			prompt = resp
		}
	}

	_, actorErr := actor(ctx, prompt)
	if actorErr != nil {
		log.WarnContext(ctx, "rehearsal.actor_error",
			slog.String("error", actorErr.Error()))
	}

	check, checkErr := r.referee.Check(ctx, r.scenario.ID(), workspace)
	if checkErr != nil {
		return nil, fmt.Errorf("rehearsal: referee: %w", checkErr)
	}

	elapsed := time.Since(start)
	artifacts := dumpWorkspace(workspace)

	metrics := &RunMetrics{
		ScenarioID:  r.scenario.ID(),
		TimeElapsed: elapsed,
		Pass:        check.Pass,
		Score:       check.Score,
		Artifacts:   artifacts,
	}

	metricsJSON, _ := json.Marshal(metrics)
	log.InfoContext(ctx, "rehearsal.complete", slog.String("metrics", string(metricsJSON)))

	return metrics, nil
}

func dumpWorkspace(dir string) map[string]string {
	artifacts := make(map[string]string)
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if info.Name() == ".git" || filepath.Base(filepath.Dir(path)) == ".git" {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		if data, err := os.ReadFile(path); err == nil && len(data) < 10000 {
			artifacts[rel] = string(data)
		}
		return nil
	})
	return artifacts
}

type RunBuilder struct {
	scenario     Scenario
	referee      Referee
	operator     Operator
	actor        ActorFunc
	actorFactory ActorFactory
	sandbox      Sandbox
	setup        []SetupOption
	workspace    string
	log          *slog.Logger
}

func NewRunBuilder() *RunBuilder {
	return &RunBuilder{}
}

func (b *RunBuilder) WithScenario(s Scenario) *RunBuilder     { b.scenario = s; return b }
func (b *RunBuilder) WithReferee(r Referee) *RunBuilder       { b.referee = r; return b }
func (b *RunBuilder) WithOperator(o Operator) *RunBuilder     { b.operator = o; return b }
func (b *RunBuilder) WithActor(a ActorFunc) *RunBuilder       { b.actor = a; return b }
func (b *RunBuilder) WithActorFactory(f ActorFactory) *RunBuilder { b.actorFactory = f; return b }
func (b *RunBuilder) WithSandbox(s Sandbox) *RunBuilder       { b.sandbox = s; return b }
func (b *RunBuilder) WithWorkspace(path string) *RunBuilder    { b.workspace = path; return b }
func (b *RunBuilder) WithSetup(opts ...SetupOption) *RunBuilder { b.setup = opts; return b }
func (b *RunBuilder) WithLogger(l *slog.Logger) *RunBuilder   { b.log = l; return b }

func (b *RunBuilder) Build() (*Runner, error) {
	if b.scenario == nil {
		return nil, ErrMissingScenario
	}
	if b.referee == nil {
		return nil, ErrMissingReferee
	}
	if b.actor == nil && b.actorFactory == nil {
		return nil, ErrMissingActor
	}
	return &Runner{
		scenario:     b.scenario,
		referee:      b.referee,
		operator:     b.operator,
		actor:        b.actor,
		actorFactory: b.actorFactory,
		sandbox:      b.sandbox,
		setup:        b.setup,
		workspace:    b.workspace,
		log:          b.log,
	}, nil
}
