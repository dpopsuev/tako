package ouroboros

import (
	"testing"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/bugle/element"
)

func TestProviderHints_MapsElementToProvider(t *testing.T) {
	sheet := &PersonaSheet{
		SuggestedPersonas: map[string]string{
			"investigate": "Seeker",     // Water element
			"triage":      "Herald",     // Fire element
			"recall":      "Specter",    // Lightning element
		},
	}

	providerElements := map[string]element.Element{
		"anthropic": element.ElementWater,
		"openai":    element.ElementFire,
	}

	hints := sheet.ProviderHints(providerElements)

	if hints["investigate"] != "anthropic" {
		t.Errorf("investigate = %q, want anthropic", hints["investigate"])
	}
	if hints["triage"] != "openai" {
		t.Errorf("triage = %q, want openai", hints["triage"])
	}
	if _, ok := hints["recall"]; ok {
		t.Errorf("recall should have no hint (lightning has no mapped provider)")
	}
}

func TestInjectAutoRoute_SetsWalkerContext(t *testing.T) {
	walker := circuit.NewProcessWalker("test")
	sheet := &PersonaSheet{
		SuggestedPersonas: map[string]string{
			"investigate": "Seeker",  // Water element
			"triage":      "Herald",  // Fire element
		},
	}

	providerElements := map[string]element.Element{
		"anthropic": element.ElementWater,
		"openai":    element.ElementFire,
	}

	InjectAutoRoute(walker, sheet, providerElements)

	hint := LookupProviderHint(walker.State().Context, "investigate")
	if hint != "anthropic" {
		t.Errorf("investigate hint = %q, want anthropic", hint)
	}
	hint = LookupProviderHint(walker.State().Context, "triage")
	if hint != "openai" {
		t.Errorf("triage hint = %q, want openai", hint)
	}
}

func TestLookupProviderHint_EmptyContext(t *testing.T) {
	ctx := map[string]any{}
	hint := LookupProviderHint(ctx, "investigate")
	if hint != "" {
		t.Errorf("hint = %q, want empty", hint)
	}
}

func TestLookupProviderHint_MissingStep(t *testing.T) {
	ctx := map[string]any{
		ProviderHintsContextKey: map[string]string{
			"investigate": "anthropic",
		},
	}
	hint := LookupProviderHint(ctx, "recall")
	if hint != "" {
		t.Errorf("hint = %q, want empty for unmapped step", hint)
	}
}
