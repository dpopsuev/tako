package framework

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dpopsuev/tako/agent"
	"github.com/dpopsuev/tako/agent/corpus"
	"github.com/dpopsuev/tako/agent/shell"
	"github.com/dpopsuev/tako/artifact"
	"github.com/dpopsuev/tako/discourse"
	"github.com/dpopsuev/tako/ergograph"
	"github.com/dpopsuev/tako/fab"
	"github.com/dpopsuev/tako/memory"
	"github.com/dpopsuev/tako/render"
	"github.com/dpopsuev/tako/service/andon"
	"github.com/dpopsuev/tako/service/depo"
	"github.com/dpopsuev/tako/service/kanban"
)

var (
	ErrNoInstruments    = errors.New("tako: no instruments at station")
	ErrContractRejected = errors.New("tako: contract rejected")
)

// FabCollective is the composition root — wires all sub-systems into a running Fab.
type FabCollective struct {
	Assembly   fab.Assembly
	Kanban     kanban.Board
	Andon      andon.Signal
	Pool       ergograph.Ledger
	Inspector  ergograph.Inspector
	Canvas     render.Canvas
	Depo       depo.Depo
	Lobby      agent.Lobby
	Middleware []Middleware
	agents     []*agent.Agent
	runner     agent.Runner
	mesh       memory.Mesh
	shell      shell.Shell
	monolog    discourse.Monolog
}

// Middleware processes an Envelope as it flows through the Fab graph.
type Middleware func(next EnvelopeHandler) EnvelopeHandler

// EnvelopeHandler processes one Envelope transition.
type EnvelopeHandler func(ctx context.Context, contract fab.Contract, envelope artifact.Envelope) error

// FabCollectiveConfig holds the dependencies for constructing a FabCollective.
type FabCollectiveConfig struct {
	Assembly   fab.Assembly
	Kanban     kanban.Board
	Andon      andon.Signal
	Pool       ergograph.Ledger
	Inspector  ergograph.Inspector
	Canvas     render.Canvas
	Depo       depo.Depo
	Lobby      agent.Lobby
	Mesh       memory.Mesh
	Shell      shell.Shell
	Runner     agent.Runner
	Middleware []Middleware
}

// NewFabCollective creates a FabCollective from its dependencies.
func NewFabCollective(cfg FabCollectiveConfig) *FabCollective {
	return &FabCollective{
		Assembly:   cfg.Assembly,
		Kanban:     cfg.Kanban,
		Andon:      cfg.Andon,
		Pool:       cfg.Pool,
		Inspector:  cfg.Inspector,
		Canvas:     cfg.Canvas,
		Depo:       cfg.Depo,
		Lobby:      cfg.Lobby,
		Middleware: cfg.Middleware,
		runner:     cfg.Runner,
		mesh:       cfg.Mesh,
		shell:      cfg.Shell,
	}
}

// Run boots the FabCollective: assembles one worker agent with Corpus,
// runs the reactivity loop, processes the Fab graph from intake to terminus.
func (fc *FabCollective) Run(ctx context.Context) error {
	intake, err := fc.Assembly.Intake()
	if err != nil {
		return fmt.Errorf("tako: %w", err)
	}

	// Admit agent through Lobby (PDP-PEP: Lobby evaluates, issues Capability)
	capability, err := fc.Lobby.Admit("worker-0", agent.Worker)
	if err != nil {
		return fmt.Errorf("tako: lobby admit: %w", err)
	}

	c := corpus.New()
	_ = capability.Services

	fc.monolog = &discourse.StubMonolog{}

	a := &agent.Agent{
		Identity:   capability.Identity,
		Persona:    capability.Persona,
		Corpus:     c,
		Reactivity: &agent.StubReactivity{},
	}
	fc.agents = append(fc.agents, a)

	if err := fc.Kanban.Claim(intake.Name, a.Identity); err != nil {
		return fmt.Errorf("tako: claim station: %w", err)
	}

	names := fc.shell.Names()
	if len(names) == 0 {
		return fmt.Errorf("%w: %s", ErrNoInstruments, intake.Name)
	}

	result, err := fc.shell.Exec(ctx, names[0], json.RawMessage(`"walking-skeleton"`))
	if err != nil {
		return fmt.Errorf("tako: instrument exec: %w", err)
	}

	envelope := artifact.NewEnvelope(intake.Name, result.Text())
	envelope.ID = "env-0"
	envelope.Labels["station"] = intake.Name
	envelope.Seal()

	contracts := fc.Assembly.ContractsFrom(intake.Name)

	handler := fc.buildChain()
	for _, contract := range contracts {
		if err := handler(ctx, contract, envelope); err != nil {
			return fmt.Errorf("tako: contract evaluation: %w", err)
		}
		shelf := fc.Depo.Shelf(contract.To)
		if err := shelf.Push(envelope); err != nil {
			return fmt.Errorf("tako: depo push: %w", err)
		}
	}

	if err := fc.Pool.Append(ergograph.Record{
		Identity:  a.Identity,
		Action:    "instrument.exec",
		Timestamp: time.Now(),
		Labels:    map[string]string{"station": intake.Name, "instrument": names[0]},
		Payload:   result.Text(),
	}); err != nil {
		return fmt.Errorf("tako: ergograph append: %w", err)
	}

	if err := fc.mesh.AddNode(memory.KnowledgeNode{
		ID:        "exec-0",
		Content:   string(result.Text()),
		Tier:      memory.Knowledge,
		CreatedAt: time.Now(),
	}); err != nil {
		return fmt.Errorf("tako: memory add node: %w", err)
	}

	fc.monolog.Write(discourse.Letter{
		From:      a.Identity,
		To:        a.Identity,
		Subject:   "executed " + names[0],
		Body:      "completed station " + intake.Name,
		CreatedAt: time.Now(),
	})

	fc.Canvas.Post(render.Panel{
		ID:     "station:" + intake.Name,
		Source: "fab",
		Data:   result.Text(),
	})

	if err := fc.runner.Run(ctx, a); err != nil {
		return fmt.Errorf("tako: agent run: %w", err)
	}

	if err := fc.Inspector.Verify(fc.Pool); err != nil {
		return fmt.Errorf("tako: ergograph verify: %w", err)
	}
	oae, err := fc.Inspector.Score(fc.Pool)
	if err != nil {
		return fmt.Errorf("tako: inspector score: %w", err)
	}
	fc.Canvas.Post(render.Panel{
		ID:     "oae",
		Source: "inspector",
		Data:   []byte(fmt.Sprintf("OAE: %.2f (A=%.2f P=%.2f Q=%.2f)", oae.Score(), oae.Availability, oae.Performance, oae.Quality)),
	})

	return nil
}

func (fc *FabCollective) buildChain() EnvelopeHandler {
	terminal := func(_ context.Context, contract fab.Contract, envelope artifact.Envelope) error {
		ok, err := contract.Evaluator.Evaluate(contract, envelope)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("%w: %s→%s", ErrContractRejected, contract.From, contract.To)
		}
		return nil
	}
	handler := terminal
	for i := len(fc.Middleware) - 1; i >= 0; i-- {
		handler = fc.Middleware[i](handler)
	}
	return handler
}

// Agents returns the agents created during Run.
func (fc *FabCollective) Agents() []*agent.Agent {
	return fc.agents
}

// Monolog returns the monolog used during Run.
func (fc *FabCollective) Monolog() discourse.Monolog {
	return fc.monolog
}
