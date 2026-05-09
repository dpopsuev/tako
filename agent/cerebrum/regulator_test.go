package cerebrum

import (
	"strings"
	"testing"

	"github.com/dpopsuev/tako/agent/reactivity"
	"github.com/dpopsuev/tako/agent/organ"
)

func TestDefaultRender_Basic(t *testing.T) {
	ctx := Context{
		Need:  "feed the tako",
		Phase: reactivity.IntentAtom,
	}

	result := defaultRender(ctx)

	if !strings.Contains(result, "feed the tako") {
		t.Error("should contain the need")
	}
	if !strings.Contains(result, "speak") {
		t.Error("should instruct to use speak tool")
	}
}

func TestDefaultRender_WithState(t *testing.T) {
	ctx := Context{
		Need:  "feed the tako",
		Phase: reactivity.IntentAtom,
		State: map[string]any{
			"hungry": true,
			"food":   "none",
		},
		Desired: map[string]any{
			"hungry": false,
		},
	}

	result := defaultRender(ctx)

	if !strings.Contains(result, "# Current State") {
		t.Error("should contain current state section")
	}
	if !strings.Contains(result, "hungry: true") {
		t.Error("should show hungry state")
	}
	if !strings.Contains(result, "# Desired State") {
		t.Error("should contain desired state section")
	}
	if !strings.Contains(result, "hungry: false") {
		t.Error("should show desired hungry state")
	}
}

func TestDefaultRender_WithResidual(t *testing.T) {
	ctx := Context{
		Need:  "feed the tako",
		Phase: reactivity.SelectionAtom,
		Residual: map[string]float64{
			"hungry": 1,
			"clean":  0,
		},
		Distance:     0.5,
		DeltaDistance: -0.1,
	}

	result := defaultRender(ctx)

	if !strings.Contains(result, "# Gap") {
		t.Error("should contain gap section")
	}
	if !strings.Contains(result, "hungry: UNMET") {
		t.Error("should show unmet dimension")
	}
	if !strings.Contains(result, "clean: met") {
		t.Error("should show met dimension")
	}
	if !strings.Contains(result, "Trend: improving") {
		t.Error("negative delta should show improving")
	}
}

func TestDefaultRender_WithCapabilities(t *testing.T) {
	ctx := Context{
		Need:  "feed the tako",
		Phase: reactivity.ExecutionAtom,
		Capabilities: []organ.Func{
			{
				Name:        "eat",
				Description: "eat food from plate",
				Writes:      []string{"hungry"},
			},
			{
				Name:        "look",
				Description: "look around",
			},
		},
	}

	result := defaultRender(ctx)

	if !strings.Contains(result, "# Available Actions") {
		t.Error("should contain actions section")
	}
	if !strings.Contains(result, "eat: eat food from plate [writes: hungry]") {
		t.Error("should show capability with writes")
	}
	if strings.Contains(result, "look: look around [writes:") {
		t.Error("should not show capability without writes in actions")
	}
}

func TestDefaultRender_NoStateSections(t *testing.T) {
	ctx := Context{
		Need:  "hello",
		Phase: reactivity.IntentAtom,
	}

	result := defaultRender(ctx)

	if strings.Contains(result, "# Current State") {
		t.Error("should not contain state when observer is nil")
	}
	if strings.Contains(result, "# Desired State") {
		t.Error("should not contain desired when no catalyst")
	}
	if strings.Contains(result, "# Gap") {
		t.Error("should not contain gap when no residual")
	}
}

func TestDefaultRender_Trend(t *testing.T) {
	cases := []struct {
		delta float64
		want  string
	}{
		{-0.1, "improving"},
		{0.1, "worsening"},
		{0, "stuck"},
	}
	for _, tc := range cases {
		ctx := Context{
			Need:         "test",
			Phase:        reactivity.IntentAtom,
			Residual:     map[string]float64{"x": 1},
			DeltaDistance: tc.delta,
		}
		result := defaultRender(ctx)
		if !strings.Contains(result, "Trend: "+tc.want) {
			t.Errorf("delta=%.1f: expected trend %q in output", tc.delta, tc.want)
		}
	}
}

func TestDefaultRender_Directives(t *testing.T) {
	ctx := Context{
		Need:  "test",
		Phase: reactivity.ExecutionAtom,
		Directives: []reactivity.Directive{
			"Use the eat instrument first",
		},
	}

	result := defaultRender(ctx)

	if !strings.Contains(result, "> Use the eat instrument first") {
		t.Error("should include directive")
	}
}

func TestDefaultRender_FilledContracts(t *testing.T) {
	ctx := Context{
		Need:  "test",
		Phase: reactivity.SelectionAtom,
		Desired: map[string]any{"hungry": false},
		Contracts: []reactivity.ContractInfo{
			{Phase: reactivity.IntentAtom, Contract: "discover intent"},
			{Phase: reactivity.SelectionAtom, Contract: "select approach"},
		},
		Filled: map[string]string{
			"intent": "the agent is hungry and needs food",
		},
	}

	result := defaultRender(ctx)

	if !strings.Contains(result, "[DONE] the agent is hungry") {
		t.Error("should show filled contract summary")
	}
	if !strings.Contains(result, "## Completed") {
		t.Error("should have completed section")
	}
}

func TestDefaultRegulate(t *testing.T) {
	m := reactivity.NewMoleculeWithCatalyst("test", reactivity.Catalyst{
		Need:    "feed",
		Desired: map[string]any{"hungry": false},
	})

	state := map[string]any{"hungry": true}
	raw := RawContext{
		Need:     []byte("feed"),
		Observer: func() map[string]any { return state },
		Molecule: m,
		Capabilities: []organ.Func{
			{Name: "eat", Description: "eat food", Writes: []string{"hungry"}},
		},
		Domain: Complicated,
		Turn:   3,
	}

	ctx := defaultRegulate(raw)

	if ctx.Need != "feed" {
		t.Error("should pass through need")
	}
	if ctx.State["hungry"] != true {
		t.Error("should call observer")
	}
	if ctx.Desired["hungry"] != false {
		t.Error("should extract desired from catalyst")
	}
	if ctx.Residual["hungry"] != 1 {
		t.Error("should extract residual from molecule")
	}
	if ctx.Turn != 3 {
		t.Error("should pass through turn")
	}
}

func TestDefaultRegulate_NoObserver(t *testing.T) {
	m := reactivity.NewMolecule("test")
	raw := RawContext{
		Need:     []byte("hello"),
		Molecule: m,
	}

	ctx := defaultRegulate(raw)

	if ctx.State != nil {
		t.Error("state should be nil without observer")
	}
	if ctx.Desired != nil {
		t.Error("desired should be nil without catalyst")
	}
}

func TestCerebrum_Assemble_WithObserver(t *testing.T) {
	state := map[string]any{"hungry": true}
	observer := func() map[string]any { return state }

	completer := &stubCompleter{response: "done"}
	reactor := reactivity.NewReactor()
	cb := New(reactor, completer,
		WithObserver(observer),
		WithCapabilities([]organ.Func{
			{Name: "eat", Description: "eat food", Writes: []string{"hungry"}},
		}),
	)

	m := reactivity.NewMoleculeWithCatalyst("test", reactivity.Catalyst{
		Need:    "feed",
		Desired: map[string]any{"hungry": false},
	})

	prompt := cb.assemble(m, []byte("feed"), Complicated, 0)

	if !strings.Contains(prompt, "hungry: true") {
		t.Error("should contain observed state")
	}
	if !strings.Contains(prompt, "hungry: false") {
		t.Error("should contain desired state")
	}
	if !strings.Contains(prompt, "hungry: UNMET") {
		t.Error("should contain residual gap")
	}
	if !strings.Contains(prompt, "eat: eat food [writes: hungry]") {
		t.Error("should contain capability with writes")
	}
}

func TestCerebrum_Assemble_NoObserver(t *testing.T) {
	completer := &stubCompleter{response: "done"}
	reactor := reactivity.NewReactor()
	cb := New(reactor, completer)

	m := reactivity.NewMolecule("test")
	prompt := cb.assemble(m, []byte("hello"), Complicated, 0)

	if !strings.Contains(prompt, "hello") {
		t.Error("should contain the need")
	}
	if strings.Contains(prompt, "# Current State") {
		t.Error("should not contain state section without observer")
	}
}

func TestCerebrum_Assemble_CustomRegulator(t *testing.T) {
	custom := stubRegulator(func(raw RawContext) Context {
		return Context{
			Need:  "REGULATED: " + string(raw.Need),
			Phase: raw.Molecule.Phase(),
		}
	})

	completer := &stubCompleter{response: "done"}
	reactor := reactivity.NewReactor()
	cb := New(reactor, completer, WithRegulator(custom))

	m := reactivity.NewMolecule("test")
	prompt := cb.assemble(m, []byte("hello"), Complicated, 0)

	if !strings.Contains(prompt, "REGULATED: hello") {
		t.Errorf("expected custom thalamus output, got: %s", prompt)
	}
}

func TestCerebrum_Assemble_CustomAssembler(t *testing.T) {
	custom := stubAssembler(func(ctx Context) string {
		return "CORTEX: " + ctx.Need
	})

	completer := &stubCompleter{response: "done"}
	reactor := reactivity.NewReactor()
	cb := New(reactor, completer, WithAssembler(custom))

	m := reactivity.NewMolecule("test")
	prompt := cb.assemble(m, []byte("hello"), Complicated, 0)

	if prompt != "CORTEX: hello" {
		t.Errorf("expected custom cortex output, got: %s", prompt)
	}
}

type stubRegulator func(RawContext) Context

func (f stubRegulator) Regulate(raw RawContext) Context { return f(raw) }

type stubAssembler func(Context) string

func (f stubAssembler) Assemble(ctx Context) string { return f(ctx) }
