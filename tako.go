package framework

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dpopsuev/tako/agent"
	"github.com/dpopsuev/tako/agent/corpus"
	"github.com/dpopsuev/tako/agent/organ"
	"github.com/dpopsuev/tako/artifact"
	"github.com/dpopsuev/tako/discourse"
	"github.com/dpopsuev/tako/ergograph"
	"github.com/dpopsuev/tako/fab"
	"github.com/dpopsuev/tako/memory"
	"github.com/dpopsuev/tako/render"
	"github.com/dpopsuev/tako/service/andon"
	"github.com/dpopsuev/tako/service/depo"
	"github.com/dpopsuev/tako/service/kanban"
	"github.com/dpopsuev/tako/workstation"
)

var (
	ErrNoInstruments    = errors.New("tako: no instruments at station")
	ErrContractRejected = errors.New("tako: contract rejected")
)

// FabCollective is the composition root — wires all sub-systems into a running Fab.
type FabCollective struct {
	Assembly    fab.Assembly
	Kanban      kanban.Board
	Andon       andon.Signal
	Pool        ergograph.Pool
	Inspector   ergograph.Inspector
	Canvas      render.Canvas
	Depo        depo.Depo
	Lobby       agent.Lobby
	Middleware  []Middleware
	agents      []*agent.Agent
	runner      agent.Runner
	mesh        memory.Mesh
	workstation workstation.Workstation
	monolog   discourse.Monolog
}

// Middleware processes an Envelope as it flows through the Fab graph.
type Middleware func(next EnvelopeHandler) EnvelopeHandler

// EnvelopeHandler processes one Envelope transition.
type EnvelopeHandler func(ctx context.Context, contract fab.Contract, envelope artifact.Envelope) error

// FabCollectiveConfig holds the dependencies for constructing a FabCollective.
type FabCollectiveConfig struct {
	Assembly    fab.Assembly
	Kanban      kanban.Board
	Andon       andon.Signal
	Pool        ergograph.Pool
	Inspector   ergograph.Inspector
	Canvas      render.Canvas
	Depo        depo.Depo
	Lobby       agent.Lobby
	Mesh        memory.Mesh
	Workstation workstation.Workstation
	Runner      agent.Runner
	Middleware  []Middleware
}

// NewFabCollective creates a FabCollective from its dependencies.
func NewFabCollective(cfg FabCollectiveConfig) *FabCollective {
	return &FabCollective{
		Assembly:    cfg.Assembly,
		Kanban:      cfg.Kanban,
		Andon:       cfg.Andon,
		Pool:        cfg.Pool,
		Inspector:   cfg.Inspector,
		Canvas:      cfg.Canvas,
		Depo:        cfg.Depo,
		Lobby:       cfg.Lobby,
		Middleware:  cfg.Middleware,
		runner:      cfg.Runner,
		mesh:        cfg.Mesh,
		workstation: cfg.Workstation,
	}
}

// Run boots the FabCollective: assembles one worker agent with Corpus,
// runs the reactivity loop, processes the Fab graph from intake to terminus.
func (fc *FabCollective) Run(ctx context.Context) error {
	intake, err := fc.Assembly.Intake()
	if err != nil {
		return fmt.Errorf("tako: %w", err)
	}

	if err := fc.workstation.Provision(intake); err != nil {
		return fmt.Errorf("tako: provision workstation: %w", err)
	}

	// Admit agent through Lobby (PDP-PEP: Lobby evaluates, issues Capability)
	capability, err := fc.Lobby.Admit("worker-0", agent.Worker)
	if err != nil {
		return fmt.Errorf("tako: lobby admit: %w", err)
	}

	// Assemble Corpus from Capability blueprint (Tangled builds, agent never self-assembles)
	c := corpus.New()
	for _, organName := range capability.Organs {
		c.Attach(organ.NewStubOrgan(organName))
	}

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

	shell := fc.workstation.Shell()
	names := shell.Names()
	if len(names) == 0 {
		return fmt.Errorf("%w: %s", ErrNoInstruments, intake.Name)
	}

	result, err := shell.Exec(ctx, names[0], []byte("walking-skeleton"))
	if err != nil {
		return fmt.Errorf("tako: instrument exec: %w", err)
	}

	envelope := artifact.NewEnvelope(intake.Name, result.Content)
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
		Payload:   result.Content,
	}); err != nil {
		return fmt.Errorf("tako: ergograph append: %w", err)
	}

	if err := fc.mesh.AddNode(memory.KnowledgeNode{
		ID:        "exec-0",
		Content:   string(result.Content),
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
		Data:   result.Content,
	})

	if err := fc.runner.Run(ctx, a); err != nil {
		return fmt.Errorf("tako: agent run: %w", err)
	}

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
