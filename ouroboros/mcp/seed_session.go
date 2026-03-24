package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
	"log/slog"

	"github.com/dpopsuev/bugle/signal"
	"github.com/dpopsuev/origami/dispatch"
	fwmcp "github.com/dpopsuev/origami/mcp"
	"github.com/dpopsuev/origami/ouroboros"
	"github.com/dpopsuev/origami/schematics/toolkit"
)

const probeCircuitPath = "ouroboros/circuits/ouroboros-probe.yaml"

// NewSeedProfileConfig returns a CircuitConfig for seed-based model profiling.
// Step schemas and the terminal node are derived from the circuit definition
// so that renaming/adding nodes in the YAML is the single source of truth.
func NewSeedProfileConfig() fwmcp.CircuitConfig {
	circuitData, err := circuit.ResolveCircuitPath(probeCircuitPath)
	if err != nil {
		panic(fmt.Sprintf("resolve ouroboros circuit: %v", err))
	}
	def, err := circuit.LoadCircuit(circuitData)
	if err != nil {
		panic(fmt.Sprintf("load ouroboros circuit: %v", err))
	}

	schemas := stepSchemasFromDef(*def)

	return fwmcp.CircuitConfig{
		Name:        "ouroboros-seed",
		Version:     "dev",
		StepSchemas: schemas,
		WorkerPreamble: `You are an Ouroboros seed probe worker. For each step you receive a prompt.
Send it to the target model and return the raw response as {"response": "<text>"}.`,
		DefaultGetNextStepTimeout: 60000,
		DefaultSessionTTL:         600000,
		CreateSession: func(ctx context.Context, params fwmcp.StartParams, disp *dispatch.MuxDispatcher, bus signal.Bus) (fwmcp.RunFunc, fwmcp.SessionMeta, error) {
			return createSeedSession(params, disp, bus, *def)
		},
		FormatReport: formatSeedReport,
	}
}

func stepSchemasFromDef(def circuit.CircuitDef) []fwmcp.StepSchema {
	var schemas []fwmcp.StepSchema
	for _, node := range def.Nodes {
		if node.Name == def.Done || node.Name == "" {
			continue
		}
		schemas = append(schemas, fwmcp.StepSchema{
			Name: node.Name,
			Defs: []fwmcp.FieldDef{
				{Name: "response", Type: "string", Desc: "raw LLM response"},
			},
		})
	}
	return schemas
}

// terminalNode returns the last node before the done sentinel by walking
// edges backward from def.Done.
func terminalNode(def circuit.CircuitDef) string {
	for _, edge := range def.Edges {
		if edge.To == def.Done {
			return edge.From
		}
	}
	return ""
}

func createSeedSession(
	params fwmcp.StartParams,
	disp *dispatch.MuxDispatcher,
	bus signal.Bus,
	def circuit.CircuitDef,
) (fwmcp.RunFunc, fwmcp.SessionMeta, error) {
	seedPath := toolkit.MapStr(params.Extra, "seed_path")
	if seedPath == "" {
		return nil, fwmcp.SessionMeta{}, fmt.Errorf("seed_path is required in extra params")
	}

	seed, err := ouroboros.LoadSeed(seedPath)
	if err != nil {
		return nil, fwmcp.SessionMeta{}, fmt.Errorf("load seed: %w", err)
	}

	meta := fwmcp.SessionMeta{
		TotalCases: 1,
		Scenario:   fmt.Sprintf("seed-%s", seed.Name),
	}

	runFn := func(ctx context.Context) (any, error) {
		return runSeedCircuit(ctx, seed, disp, bus, def)
	}

	return runFn, meta, nil
}

type dispatchResponse struct {
	Response string `json:"response"`
}

func runSeedCircuit(
	ctx context.Context,
	seed *ouroboros.Seed,
	disp *dispatch.MuxDispatcher,
	bus signal.Bus,
	def circuit.CircuitDef,
) (*ouroboros.PoleResult, error) {
	log := slog.Default().With("component", "ouroboros-seed")

	dispatcher := func(ctx context.Context, nodeName string, prompt string) (string, error) {
		artifactBytes, err := disp.Dispatch(ctx, dispatch.DispatchContext{
			CaseID:        seed.Name,
			Step:          nodeName,
			PromptContent: prompt,
		})
		if err != nil {
			return "", err
		}

		var art dispatchResponse
		if err := json.Unmarshal(artifactBytes, &art); err != nil {
			return "", fmt.Errorf("parse %s artifact: %w", nodeName, err)
		}
		return art.Response, nil
	}

	nodes := ouroboros.CircuitNodes(seed, dispatcher)

	g, err := engine.BuildGraph(&def, engine.GraphRegistries{Nodes: nodes})
	if err != nil {
		return nil, fmt.Errorf("build graph: %w", err)
	}

	walker := circuit.NewProcessWalker(fmt.Sprintf("seed-%s", seed.Name))
	start := time.Now()

	if err := g.Walk(ctx, walker, def.Start); err != nil {
		return nil, fmt.Errorf("walk: %w", err)
	}

	elapsed := time.Since(start)
	log.Info("seed circuit completed", "seed", seed.Name, "elapsed", elapsed)

	lastNode := terminalNode(def)
	if lastNode == "" {
		return nil, fmt.Errorf("circuit has no edge to %s", def.Done)
	}

	finalArtifact := walker.State().Outputs[lastNode]
	if finalArtifact == nil {
		return nil, fmt.Errorf("%s node produced no artifact", lastNode)
	}

	result, ok := finalArtifact.Raw().(*ouroboros.PoleResult)
	if !ok {
		return nil, fmt.Errorf("%s artifact type %T, expected *PoleResult", lastNode, finalArtifact.Raw())
	}

	bus.Emit(&signal.Signal{Event: "seed_completed", Agent: "server", CaseID: seed.Name, Step: lastNode, Meta: map[string]string{
		"selected_pole": result.SelectedPole,
		"confidence":    fmt.Sprintf("%.2f", result.Confidence),
	}})

	return result, nil
}

func formatSeedReport(result any) (string, any, error) {
	pr, ok := result.(*ouroboros.PoleResult)
	if !ok {
		return "", nil, fmt.Errorf("expected *PoleResult, got %T", result)
	}

	summary := fmt.Sprintf("Seed Probe Result\nSelected Pole: %s\nConfidence: %.2f\nReasoning: %s\n",
		pr.SelectedPole, pr.Confidence, pr.Reasoning)

	return summary, pr, nil
}
