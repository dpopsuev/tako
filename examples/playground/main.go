// Framework Playground — a self-contained demo of the Origami agentic circuit framework.
//
// Run it:
//
//	go run ./examples/playground/
//
// No external services, no AI subscriptions.
// This program demonstrates every major framework concept in one flow.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	fw "github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami/cycle"
	"github.com/dpopsuev/origami/dialectic"
	"github.com/dpopsuev/origami/element"
	"github.com/dpopsuev/origami/mask"
	"github.com/dpopsuev/origami/persona"
)

func resolveNodeElement(d fw.NodeDef) element.Element {
	e, _ := element.ResolveApproach(strings.ToLower(d.Approach))
	return e
}

func main() {
	printHeader()

	section("1. ELEMENTS — Behavioral Archetypes")
	showElements()

	section("2. PERSONAS — Agent Identities")
	showPersonas()

	section("3. CIRCUIT DSL — Load and Validate YAML")
	triageDef := loadTriageCircuit()

	section("4. MERMAID — Render Circuit as Diagram")
	showMermaid("Bug Triage Circuit", triageDef)

	section("5. GRAPH WALK — Walker Traverses the Circuit")
	walkTriageCircuit(triageDef)

	section("6. MASKS — Middleware Capabilities")
	showMasks()

	section("7. ELEMENT CYCLES — Generative and Destructive Interactions")
	showCycles()

	section("8. ADVERSARIAL DIALECTIC — Thesis-Antithesis-Synthesis Circuit")
	showDialectic()

	section("9. TEAM WALK — Multi-Persona Scheduling with Live Trace")
	teamWalkDemo(triageDef)

	printFooter()
}

// ---------------------------------------------------------------------------
// Section helpers
// ---------------------------------------------------------------------------

const (
	reset  = "\033[0m"
	bold   = "\033[1m"
	dim    = "\033[2m"
	red    = "\033[31m"
	green  = "\033[32m"
	yellow = "\033[33m"
	blue   = "\033[34m"
	cyan   = "\033[36m"
	white  = "\033[37m"
)

func printHeader() {
	fmt.Println()
	fmt.Printf("%s%s=== Origami Framework Playground ===%s\n", bold, cyan, reset)
	fmt.Println()
	fmt.Printf("%sThis program demonstrates the Origami agentic circuit framework.%s\n", dim, reset)
	fmt.Printf("%sNo AI, no external services — pure graph-driven agent orchestration.%s\n\n", dim, reset)
}

func printFooter() {
	fmt.Println()
	fmt.Printf("%s%s=== End of Playground ===%s\n\n", bold, cyan, reset)
	fmt.Printf("Framework source: %sgithub.com/dpopsuev/origami%s\n", bold, reset)
	fmt.Printf("Developer guide:  %sdocs/framework-guide.md%s\n\n", bold, reset)
}

func section(title string) {
	fmt.Println()
	fmt.Printf("%s%s--- %s ---%s\n\n", bold, yellow, title, reset)
}

func indent(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = "    " + l
	}
	return strings.Join(lines, "\n")
}

// ---------------------------------------------------------------------------
// 1. Elements
// ---------------------------------------------------------------------------

func showElements() {
	fmt.Printf("The framework defines %s6 core elements%s, each an archetype governing\n", bold, reset)
	fmt.Printf("how an agent behaves: how fast it moves, how many times it retries,\n")
	fmt.Printf("when it declares convergence, and how it fails.\n\n")

	fmt.Printf("  %-12s %-10s %-6s %-8s %-10s %-5s %s\n",
		"Element", "Speed", "Loops", "Converg.", "Shortcuts", "Depth", "Failure Mode")
	fmt.Printf("  %s\n", strings.Repeat("-", 85))

	for _, el := range element.AllElements() {
		t := element.DefaultTraits(el)
		color := elementColor(el)
		fmt.Printf("  %s%-12s%s %-10s %-6d %-8.2f %-10.1f %-5d %s\n",
			color, el, reset,
			t.Speed, t.MaxLoops, t.ConvergenceThreshold,
			t.ShortcutAffinity, t.EvidenceDepth, t.FailureMode)
	}

}

func elementColor(e element.Element) string {
	switch e {
	case element.ElementFire:
		return red
	case element.ElementLightning:
		return yellow
	case element.ElementEarth:
		return green
	case element.ElementDiamond:
		return white
	case element.ElementWater:
		return blue
	case element.ElementAir:
		return cyan
	default:
		return dim
	}
}

// ---------------------------------------------------------------------------
// 2. Personas
// ---------------------------------------------------------------------------

func showPersonas() {
	fmt.Printf("Personas are perennial agent identities — stable across model releases. Each has a %scolor%s,\n", bold, reset)
	fmt.Printf("an %selement%s, a dialectic %sposition%s, and either %sThesis%s or %sAntithesis%s alignment.\n\n", bold, reset, bold, reset, green, reset, red, reset)

	fmt.Printf("  %sThesis (Cadai) — the investigation team:%s\n", green, reset)
	for _, p := range persona.Thesis() {
		id := p.Identity
		fmt.Printf("    %s%-12s%s %-10s %-10s %-12s %s\n",
			elementColor(id.Element), id.PersonaName, reset,
			id.Color.DisplayName, id.Element, id.Position, p.Description)
	}

	fmt.Println()
	fmt.Printf("  %sAntithesis (Cytharai) — the adversarial dialectic:%s\n", red, reset)
	for _, p := range persona.Antithesis() {
		id := p.Identity
		fmt.Printf("    %s%-12s%s %-10s %-10s %-12s %s\n",
			elementColor(id.Element), id.PersonaName, reset,
			id.Color.DisplayName, id.Element, id.Position, p.Description)
	}
}

// ---------------------------------------------------------------------------
// 3. Circuit DSL
// ---------------------------------------------------------------------------

func loadTriageCircuit() *fw.CircuitDef {
	_, thisFile, _, _ := runtime.Caller(0)
	yamlPath := filepath.Join(filepath.Dir(thisFile), "triage.yaml")
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		fmt.Printf("  %sError reading triage.yaml: %v%s\n", red, err, reset)
		os.Exit(1)
	}

	def, err := fw.LoadCircuit(data)
	if err != nil {
		fmt.Printf("  %sError parsing circuit: %v%s\n", red, err, reset)
		os.Exit(1)
	}

	if err := def.Validate(); err != nil {
		fmt.Printf("  %sValidation failed: %v%s\n", red, err, reset)
		os.Exit(1)
	}

	fmt.Printf("  Loaded circuit: %s%s%s\n", bold, def.Circuit, reset)
	fmt.Printf("  Description:     %s\n", def.Description)
	fmt.Printf("  Nodes:           %d\n", len(def.Nodes))
	fmt.Printf("  Edges:           %d\n", len(def.Edges))
	fmt.Printf("  Zones:           %d\n", len(def.Zones))
	fmt.Printf("  Start:           %s\n", def.Start)
	fmt.Printf("  Done:            %s\n", def.Done)

	fmt.Println()
	fmt.Printf("  Nodes:\n")
	for _, n := range def.Nodes {
		fmt.Printf("    %s%-14s%s approach=%-12s handler=%s\n",
			elementColor(resolveNodeElement(n)), n.Name, reset, n.Approach, n.EffectiveHandler())
	}

	fmt.Println()
	fmt.Printf("  Edges:\n")
	for _, e := range def.Edges {
		arrow := "-->"
		if e.Shortcut {
			arrow = "==>"
		}
		if e.Loop {
			arrow = "~~>"
		}
		fmt.Printf("    %-4s %-12s %s %-12s  %s%s%s\n",
			e.ID, e.From, arrow, e.To, dim, e.Condition, reset)
	}

	return def
}

// ---------------------------------------------------------------------------
// 4. Mermaid rendering
// ---------------------------------------------------------------------------

func showMermaid(title string, def *fw.CircuitDef) {
	mermaid := fw.Render(def)
	fmt.Printf("  %sPaste this into any Mermaid viewer (https://mermaid.live):%s\n\n", dim, reset)
	fmt.Println(indent(mermaid))

	dialecticData, err := os.ReadFile(findCircuitsDir() + "/defect-dialectic.yaml")
	if err == nil {
		dialecticDef, err := fw.LoadCircuit(dialecticData)
		if err == nil {
			fmt.Printf("\n  %sBonus — the Adversarial Dialectic circuit:%s\n\n", dim, reset)
			fmt.Println(indent(fw.Render(dialecticDef)))
		}
	}
}

func findCircuitsDir() string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Dir(thisFile)
}

// ---------------------------------------------------------------------------
// 5. Graph Walk
// ---------------------------------------------------------------------------

func walkTriageCircuit(def *fw.CircuitDef) {
	fmt.Printf("  Building a graph from the circuit DSL, then walking it with a Herald.\n\n")

	nodeReg := engine.NodeRegistry{
		"classify":    func(d fw.NodeDef) fw.Node { return &demoNode{name: d.Name, element: resolveNodeElement(d)} },
		"investigate": func(d fw.NodeDef) fw.Node { return &demoNode{name: d.Name, element: resolveNodeElement(d)} },
		"decide":      func(d fw.NodeDef) fw.Node { return &demoNode{name: d.Name, element: resolveNodeElement(d)} },
		"close":       func(d fw.NodeDef) fw.Node { return &demoNode{name: d.Name, element: resolveNodeElement(d)} },
	}

	scenario := newDemoScenario()

	edgeFactory := engine.EdgeFactory{
		"E1": func(d fw.EdgeDef) fw.Edge { return &demoEdge{def: d, scenario: scenario} },
		"E2": func(d fw.EdgeDef) fw.Edge { return &demoEdge{def: d, scenario: scenario} },
		"E3": func(d fw.EdgeDef) fw.Edge { return &demoEdge{def: d, scenario: scenario} },
		"E4": func(d fw.EdgeDef) fw.Edge { return &demoEdge{def: d, scenario: scenario} },
		"E5": func(d fw.EdgeDef) fw.Edge { return &demoEdge{def: d, scenario: scenario} },
		"E6": func(d fw.EdgeDef) fw.Edge { return &demoEdge{def: d, scenario: scenario} },
		"E7": func(d fw.EdgeDef) fw.Edge { return &demoEdge{def: d, scenario: scenario} },
	}

	graph, err := engine.BuildGraph(def, engine.GraphRegistries{Nodes: nodeReg, Edges: edgeFactory})
	if err != nil {
		fmt.Printf("  %sBuild error: %v%s\n", red, err, reset)
		return
	}

	herald, _ := persona.ByName("Herald")
	walker := &demoWalker{
		identity: herald.Identity,
		state:    fw.NewWalkerState("demo-walk-1"),
		scenario: scenario,
	}

	fmt.Printf("  Walker: %s%s%s (element=%s, position=%s)\n\n",
		bold, herald.Identity.PersonaName, reset,
		herald.Identity.Element, herald.Identity.Position)

	err = graph.Walk(context.Background(), walker, def.Start)
	if err != nil {
		fmt.Printf("  %sWalk error: %v%s\n", red, err, reset)
		return
	}

	fmt.Println()
	fmt.Printf("  %sWalk complete!%s Status: %s\n", green, reset, walker.state.Status)
	fmt.Printf("  Steps taken: %d\n", len(walker.state.History))
	for i, step := range walker.state.History {
		fmt.Printf("    %d. node=%-14s edge=%s\n", i+1, step.Node, step.EdgeID)
	}
}

// demoScenario controls the walk path for demonstration purposes.
type demoScenario struct {
	classifyConfidence    float64
	investigateConverge   float64
	investigateLoopCount  int
	decisionApprove       bool
}

func newDemoScenario() *demoScenario {
	return &demoScenario{
		classifyConfidence:  0.65,
		investigateConverge: 0.75,
		investigateLoopCount: 0,
		decisionApprove:     true,
	}
}

// demoNode implements circuit.Node for the playground.
type demoNode struct {
	name    string
	element element.Element
}

func (n *demoNode) Name() string                 { return n.name }
func (n *demoNode) ElementAffinity() element.Element { return n.element }
func (n *demoNode) Process(ctx context.Context, nc fw.NodeContext) (fw.Artifact, error) {
	return &demoArtifact{typ: n.name, conf: 0.75}, nil
}

// demoArtifact implements circuit.Artifact.
type demoArtifact struct {
	typ  string
	conf float64
}

func (a *demoArtifact) Type() string       { return a.typ }
func (a *demoArtifact) Confidence() float64 { return a.conf }
func (a *demoArtifact) Raw() any            { return a }

// demoWalker implements circuit.Walker with annotated output.
type demoWalker struct {
	identity fw.AgentIdentity
	state    *fw.WalkerState
	scenario *demoScenario
}

func (w *demoWalker) Identity() fw.AgentIdentity      { return w.identity }
func (w *demoWalker) SetIdentity(id fw.AgentIdentity)  { w.identity = id }
func (w *demoWalker) State() *fw.WalkerState           { return w.state }

func (w *demoWalker) Handle(ctx context.Context, node fw.Node, nc fw.NodeContext) (fw.Artifact, error) {
	color := elementColor(node.ElementAffinity())
	fmt.Printf("  %s[%s]%s %s%-14s%s processing...",
		dim, w.identity.PersonaName, reset, color, node.Name(), reset)

	var conf float64
	switch node.Name() {
	case "classify":
		conf = w.scenario.classifyConfidence
		fmt.Printf(" confidence=%.2f", conf)
		if conf >= 0.90 {
			fmt.Printf(" %s(shortcut!)%s", yellow, reset)
		} else {
			fmt.Printf(" %s(needs investigation)%s", dim, reset)
		}
	case "investigate":
		w.scenario.investigateLoopCount++
		conf = w.scenario.investigateConverge
		fmt.Printf(" convergence=%.2f loop=%d", conf, w.scenario.investigateLoopCount)
	case "decide":
		conf = 0.85
		if w.scenario.decisionApprove {
			fmt.Printf(" decision=approve")
		} else {
			fmt.Printf(" decision=reassess")
		}
	case "close":
		conf = 1.0
		fmt.Printf(" %sfinalizing report%s", green, reset)
	}
	fmt.Println()

	return &demoArtifact{typ: node.Name(), conf: conf}, nil
}

// demoEdge implements circuit.Edge with scenario-driven evaluation.
type demoEdge struct {
	def      fw.EdgeDef
	scenario *demoScenario
}

func (e *demoEdge) ID() string       { return e.def.ID }
func (e *demoEdge) From() string     { return e.def.From }
func (e *demoEdge) To() string       { return e.def.To }
func (e *demoEdge) IsShortcut() bool { return e.def.Shortcut }
func (e *demoEdge) IsLoop() bool     { return e.def.Loop }

func (e *demoEdge) Evaluate(a fw.Artifact, s *fw.WalkerState) *fw.Transition {
	switch e.def.ID {
	case "E1": // obvious-bug shortcut
		if e.scenario.classifyConfidence >= 0.90 {
			return &fw.Transition{NextNode: e.def.To, Explanation: "high confidence shortcut"}
		}
		return nil
	case "E2": // needs-investigation
		if e.scenario.classifyConfidence < 0.90 {
			return &fw.Transition{NextNode: e.def.To, Explanation: "confidence below threshold"}
		}
		return nil
	case "E3": // evidence-found
		if e.scenario.investigateConverge >= 0.70 {
			return &fw.Transition{NextNode: e.def.To, Explanation: "sufficient evidence"}
		}
		return nil
	case "E4": // keep-digging loop
		if e.scenario.investigateConverge < 0.70 && e.scenario.investigateLoopCount < 3 {
			return &fw.Transition{NextNode: e.def.To, Explanation: "keep investigating"}
		}
		return nil
	case "E5": // approved
		if e.scenario.decisionApprove {
			return &fw.Transition{NextNode: e.def.To, Explanation: "decision approved"}
		}
		return nil
	case "E6": // reassess
		if !e.scenario.decisionApprove {
			return &fw.Transition{NextNode: e.def.To, Explanation: "send back for reassessment"}
		}
		return nil
	case "E7": // done
		return &fw.Transition{NextNode: e.def.To, Explanation: "circuit complete"}
	}
	return nil
}

// ---------------------------------------------------------------------------
// 6. Masks
// ---------------------------------------------------------------------------

func showMasks() {
	fmt.Printf("  Masks are detachable middleware that grant powers at specific nodes.\n")
	fmt.Printf("  They wrap a node's processing: %spre -> node -> post%s.\n\n", bold, reset)

	masks := mask.DefaultThesisMasks()
	fmt.Printf("  %s4 Thesis Masks:%s\n", bold, reset)
	for name, m := range masks {
		nodes := strings.Join(m.ValidNodes(), ", ")
		fmt.Printf("    %-24s at %-14s %s%s%s\n",
			name, nodes, dim, m.Description(), reset)
	}

	fmt.Println()
	fmt.Printf("  %sExample — equipping Mask of Recall on a 'recall' node:%s\n\n", dim, reset)

	recallNode := &demoNode{name: "recall", element: element.ElementFire}
	recallMask := mask.NewRecallMask()
	masked, err := mask.Equip(recallNode, recallMask)
	if err != nil {
		fmt.Printf("    %sError: %v%s\n", red, err, reset)
		return
	}

	ctx := context.Background()
	nc := fw.NodeContext{
		WalkerState: fw.NewWalkerState("mask-demo"),
		Meta:        make(map[string]any),
	}
	artifact, _ := masked.Process(ctx, nc)
	fmt.Printf("    Processed masked node: type=%q, meta=%v\n",
		artifact.Type(), nc.Meta)
	fmt.Printf("    %sThe mask injected 'prior_rca_available=true' into the context.%s\n", dim, reset)
}

// ---------------------------------------------------------------------------
// 7. Cycles
// ---------------------------------------------------------------------------

func showCycles() {
	fmt.Printf("  Elements interact through two cycles (inspired by Wu Xing):\n\n")

	fmt.Printf("  %sGenerative (sheng) — each element strengthens the next:%s\n", green, reset)
	for _, rule := range cycle.GenerativeCycle() {
		fc := elementColor(rule.From)
		tc := elementColor(rule.To)
		fmt.Printf("    %s%-12s%s -> %s%-12s%s  %s%s%s\n",
			fc, rule.From, reset, tc, rule.To, reset, dim, rule.Interaction, reset)
	}

	fmt.Println()
	fmt.Printf("  %sDestructive (ke) — each element challenges another:%s\n", red, reset)
	for _, rule := range cycle.DestructiveCycle() {
		fc := elementColor(rule.From)
		tc := elementColor(rule.To)
		fmt.Printf("    %s%-12s%s -> %s%-12s%s  %s%s%s\n",
			fc, rule.From, reset, tc, rule.To, reset, dim, rule.Interaction, reset)
	}

	fmt.Println()
	fmt.Printf("  %sThese cycles govern agent interactions: a Fire agent%s\n", dim, reset)
	fmt.Printf("  %sgenerates work for Earth, but destructively challenges Water.%s\n", dim, reset)
}

// ---------------------------------------------------------------------------
// 8. Adversarial Dialectic
// ---------------------------------------------------------------------------

func showDialectic() {
	fmt.Printf("  When the Thesis circuit's confidence is uncertain (0.50-0.85),\n")
	fmt.Printf("  the adversarial dialectic activates for thesis-antithesis-synthesis review.\n\n")

	cfg := dialectic.DefaultConfig()
	cfg.Enabled = true
	fmt.Printf("  Dialectic config: contradiction_threshold=%.2f, max_turns=%d, max_negations=%d, ttl=%s\n\n",
		cfg.ContradictionThreshold, cfg.MaxTurns, cfg.MaxNegations, cfg.TTL)

	fmt.Printf("  Antithesis activation check:\n")
	for _, conf := range []float64{0.40, 0.55, 0.75, 0.90} {
		activated := cfg.NeedsAntithesis(conf)
		marker := dim + "no" + reset
		if activated {
			marker = red + "YES" + reset
		}
		fmt.Printf("    confidence=%.2f  needs antithesis? %s\n", conf, marker)
	}

	dialecticData, err := os.ReadFile(findCircuitsDir() + "/defect-dialectic.yaml")
	if err != nil {
		fmt.Printf("\n  %sCould not load defect-dialectic.yaml: %v%s\n", dim, err, reset)
		return
	}

	dialecticDef, err := fw.LoadCircuit(dialecticData)
	if err != nil {
		fmt.Printf("\n  %sCould not parse dialectic circuit: %v%s\n", dim, err, reset)
		return
	}

	fmt.Println()
	fmt.Printf("  %sAdversarial Dialectic circuit (D0-D4):%s\n", bold, reset)
	for _, n := range dialecticDef.Nodes {
		fmt.Printf("    %s%-14s%s approach=%-12s\n",
			elementColor(resolveNodeElement(n)), n.Name, reset, n.Approach)
	}

	fmt.Println()
	fmt.Printf("  %sSynthesis decisions:%s\n", bold, reset)
	decisions := []struct {
		name string
		desc string
	}{
		{"affirm", "Original classification stands"},
		{"amend", "Classification changed based on evidence"},
		{"acquit", "Insufficient evidence — produce gap brief"},
		{"remand", "Send back to Thesis path for reinvestigation"},
		{"unresolved", "Irreconcilable contradiction — turn limit or arbiter declares"},
	}
	for _, d := range decisions {
		fmt.Printf("    %-12s %s%s%s\n", d.name, dim, d.desc, reset)
	}

	fmt.Println()
	fmt.Printf("  %sThe dialectic uses typed artifacts (ThesisChallenge, AntithesisResponse,\n", dim)
	fmt.Printf("  DialecticRecord, Synthesis) and HD1-HD12 heuristic edges — the same Edge\n")
	fmt.Printf("  interface used by the Thesis circuit. Antithesis is just another graph walk.%s\n", reset)
}

// ---------------------------------------------------------------------------
// 9. Team Walk — multi-persona scheduling with live trace
// ---------------------------------------------------------------------------

func teamWalkDemo(def *fw.CircuitDef) {
	fmt.Printf("  The same circuit, but now %smultiple agents%s collaborate.\n", bold, reset)
	fmt.Printf("  A %sScheduler%s picks the best walker per node based on affinity.\n", bold, reset)
	fmt.Printf("  An %sObserver%s traces every event in real time.\n\n", bold, reset)

	nodeReg := engine.NodeRegistry{
		"classify":    func(d fw.NodeDef) fw.Node { return &demoNode{name: d.Name, element: resolveNodeElement(d)} },
		"investigate": func(d fw.NodeDef) fw.Node { return &demoNode{name: d.Name, element: resolveNodeElement(d)} },
		"decide":      func(d fw.NodeDef) fw.Node { return &demoNode{name: d.Name, element: resolveNodeElement(d)} },
		"close":       func(d fw.NodeDef) fw.Node { return &demoNode{name: d.Name, element: resolveNodeElement(d)} },
	}

	scenario := newDemoScenario()

	edgeFactory := engine.EdgeFactory{
		"E1": func(d fw.EdgeDef) fw.Edge { return &demoEdge{def: d, scenario: scenario} },
		"E2": func(d fw.EdgeDef) fw.Edge { return &demoEdge{def: d, scenario: scenario} },
		"E3": func(d fw.EdgeDef) fw.Edge { return &demoEdge{def: d, scenario: scenario} },
		"E4": func(d fw.EdgeDef) fw.Edge { return &demoEdge{def: d, scenario: scenario} },
		"E5": func(d fw.EdgeDef) fw.Edge { return &demoEdge{def: d, scenario: scenario} },
		"E6": func(d fw.EdgeDef) fw.Edge { return &demoEdge{def: d, scenario: scenario} },
		"E7": func(d fw.EdgeDef) fw.Edge { return &demoEdge{def: d, scenario: scenario} },
	}

	graph, err := engine.BuildGraph(def, engine.GraphRegistries{Nodes: nodeReg, Edges: edgeFactory})
	if err != nil {
		fmt.Printf("  %sBuild error: %v%s\n", red, err, reset)
		return
	}

	herald, _ := persona.ByName("Herald")
	seeker, _ := persona.ByName("Seeker")
	sentinel, _ := persona.ByName("Sentinel")
	weaver, _ := persona.ByName("Weaver")

	walkers := []fw.Walker{
		&demoWalker{identity: herald.Identity, state: fw.NewWalkerState("herald-team"), scenario: scenario},
		&demoWalker{identity: seeker.Identity, state: fw.NewWalkerState("seeker-team"), scenario: scenario},
		&demoWalker{identity: sentinel.Identity, state: fw.NewWalkerState("sentinel-team"), scenario: scenario},
		&demoWalker{identity: weaver.Identity, state: fw.NewWalkerState("weaver-team"), scenario: scenario},
	}

	fmt.Printf("  %sTeam roster:%s\n", bold, reset)
	for _, w := range walkers {
		id := w.Identity()
		color := elementColor(id.Element)
		fmt.Printf("    %s%-12s%s element=%-10s position=%s\n",
			color, id.PersonaName, reset, id.Element, id.Position)
	}
	fmt.Println()

	liveObserver := fw.WalkObserverFunc(func(e fw.WalkEvent) {
		switch e.Type {
		case fw.EventWalkerSwitch:
			fmt.Printf("  %s⟳ scheduler%s  → %s%s%s now active at %s\n",
				cyan, reset, bold, e.Walker, reset, e.Node)
		case fw.EventNodeEnter:
			color := yellow
			fmt.Printf("  %s▸ enter%s      %s%-14s%s  walker=%s\n",
				color, reset, bold, e.Node, reset, e.Walker)
		case fw.EventNodeExit:
			if e.Error != nil {
				fmt.Printf("  %s✗ exit%s       %-14s  %serror=%v%s\n",
					red, reset, e.Node, red, e.Error, reset)
			} else {
				fmt.Printf("  %s✓ exit%s       %-14s  elapsed=%s  artifact=%s\n",
					green, reset, e.Node, e.Elapsed, e.Artifact.Type())
			}
		case fw.EventEdgeEvaluate:
			fmt.Printf("  %s? edge%s       %-14s  evaluating %s\n",
				dim, reset, e.Node, e.Edge)
		case fw.EventTransition:
			fmt.Printf("  %s→ transition%s %-14s  via %s\n",
				blue, reset, e.Node, e.Edge)
		case fw.EventWalkComplete:
			fmt.Printf("  %s★ complete%s   walk finished (last walker: %s)\n",
				green, reset, e.Walker)
		case fw.EventWalkError:
			fmt.Printf("  %s✗ walk error%s %v\n", red, reset, e.Error)
		}
	})

	trace := &engine.TraceCollector{}

	team := &engine.Team{
		Walkers:   walkers,
		Scheduler: &engine.AffinityScheduler{},
		Observer:  fw.MultiObserver{liveObserver, trace},
		MaxSteps:  20,
	}

	fmt.Printf("  %sLive trace:%s\n", bold, reset)
	err = graph.WalkTeam(context.Background(), team, def.Start)
	if err != nil {
		fmt.Printf("\n  %sTeam walk error: %v%s\n", red, err, reset)
		return
	}

	fmt.Printf("\n  %sPost-walk summary (from TraceCollector):%s\n", bold, reset)
	events := trace.Events()
	enters := trace.EventsOfType(fw.EventNodeEnter)
	switches := trace.EventsOfType(fw.EventWalkerSwitch)
	fmt.Printf("    Total events:     %d\n", len(events))
	fmt.Printf("    Nodes visited:    %d\n", len(enters))
	fmt.Printf("    Walker switches:  %d\n", len(switches))

	fmt.Println()
	fmt.Printf("  %sThe AffinityScheduler picked agents by StepAffinity score.\n", dim)
	fmt.Printf("  Fire-element Herald handles 'classify'; Water-element Seeker handles\n")
	fmt.Printf("  'investigate'; each agent is automatically matched to the node where\n")
	fmt.Printf("  it performs best. The observer traced every micro-event for post-hoc\n")
	fmt.Printf("  analysis, profiling, and debugging.%s\n", reset)
}
