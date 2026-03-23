package ouroboros

import (
	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/bugle/element"
)

// ProviderHintsContextKey is the walker context key where auto-routing
// stores the step→provider hint map.
const ProviderHintsContextKey = "ouroboros_provider_hints"

// InjectAutoRoute sets provider hints from a PersonaSheet into a walker's
// context. When the ProviderRouter encounters a dispatch with no explicit
// provider, it can check the walker context for this key and use the
// hint for the current step.
//
// providerElements maps provider names to their dominant element:
//
//	{"anthropic": ElementWater, "openai": ElementFire}
//
// The PersonaSheet's step→element mapping is inverted through this map
// to produce step→provider hints.
func InjectAutoRoute(walker circuit.Walker, sheet *PersonaSheet, providerElements map[string]element.Element) {
	hints := sheet.ProviderHints(providerElements)
	walker.State().Context[ProviderHintsContextKey] = hints
}

// LookupProviderHint retrieves the provider hint for a given step from
// a walker's context. Returns empty string if no hint exists.
func LookupProviderHint(walkerCtx map[string]any, step string) string {
	raw, ok := walkerCtx[ProviderHintsContextKey]
	if !ok {
		return ""
	}
	hints, ok := raw.(map[string]string)
	if !ok {
		return ""
	}
	return hints[step]
}
