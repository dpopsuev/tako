package engine

import (
	"context"
	"log/slog"
	"sort"
	"strings"

	"github.com/dpopsuev/origami/circuit"
)

// runBuildDiagnostics performs post-build static analysis on the circuit graph.
// All diagnostics emit slog.Warn — they do not fail the build.
func runBuildDiagnostics(def *circuit.CircuitDef, reg *GraphRegistries) {
	diagUnreferencedHooks(def, reg)
	diagMissingHookRefs(def, reg)
	diagCircuitMediatorFallback(def, reg)
}

// diagCircuitMediatorFallback (D3): handler_type:circuit nodes that fall
// back to MCPCircuitTransformer via mediator endpoint instead of resolving
// to a local circuit definition. This is often unintentional and means
// sub-circuit transformers run on a remote registry (which may be sparse).
func diagCircuitMediatorFallback(def *circuit.CircuitDef, reg *GraphRegistries) {
	if reg.MediatorEndpoint == "" {
		return
	}
	for i := range def.Nodes {
		nd := &def.Nodes[i]
		ht := nd.EffectiveHandlerType(def.HandlerType)
		if ht != circuit.HandlerTypeCircuit {
			continue
		}
		handler := nd.Handler
		if handler == "" {
			continue
		}
		// Check if this would resolve locally or fall back to mediator.
		locallyResolved := false
		if reg.Circuits != nil {
			if _, ok := reg.Circuits[handler]; ok {
				locallyResolved = true
			}
		}
		if !locallyResolved {
			slog.WarnContext(context.Background(), circuit.LogCircuitMediatorFallback,
				slog.Any(circuit.LogKeyComponent, circuit.LogComponentBuild),
				slog.Any(circuit.LogKeyDiagnostic, "D3"),
				slog.Any(circuit.LogKeyNode, string(nd.Name)),
				slog.Any(circuit.LogKeyHandler, handler),
				slog.Any(circuit.LogKeyEndpoint, reg.MediatorEndpoint),
				slog.Any(circuit.LogKeyCircuit, def.Circuit))
		}
	}
}

// diagUnreferencedHooks (D1): hooks registered in HookRegistry but not
// referenced by any node's before: or after: field.
func diagUnreferencedHooks(def *circuit.CircuitDef, reg *GraphRegistries) {
	if len(reg.Hooks) == 0 {
		return
	}

	referenced := collectReferencedHooks(def)

	for name := range reg.Hooks {
		if !referenced[name] {
			slog.WarnContext(context.Background(), circuit.LogUnreferencedHook, slog.Any(circuit.LogKeyComponent, circuit.LogComponentBuild), slog.Any(circuit.LogKeyDiagnostic, "D1"), slog.Any(circuit.LogKeyHook, name), slog.Any(circuit.LogKeyCircuit, def.Circuit))
		}
	}
}

// diagMissingHookRefs (D2+D4): nodes reference hooks that don't exist in the registry.
func diagMissingHookRefs(def *circuit.CircuitDef, reg *GraphRegistries) {
	for i := range def.Nodes {
		nd := &def.Nodes[i]
		checkHookList(string(nd.Name), "before", nd.Before, reg, def.Circuit)
		checkHookList(string(nd.Name), "after", nd.After, reg, def.Circuit)
	}
}

func checkHookList(nodeName, phase string, hooks []string, reg *GraphRegistries, circuitName string) {
	if len(hooks) == 0 {
		return
	}

	var missing []string
	for _, hookName := range hooks {
		if reg.Hooks == nil {
			missing = append(missing, hookName)
			continue
		}
		if _, err := reg.Hooks.Get(hookName); err != nil {
			missing = append(missing, hookName)
		}
	}

	if len(missing) == 0 {
		return
	}

	available := registeredHookNames(reg)
	slog.WarnContext(context.Background(), circuit.LogMissingHookRefs, slog.Any(circuit.LogKeyComponent, circuit.LogComponentBuild), slog.Any(circuit.LogKeyDiagnostic, "D2"), slog.Any(circuit.LogKeyNode, nodeName), slog.Any(circuit.LogKeyPhase, phase), slog.Any(circuit.LogKeyMissing, strings.Join(missing, ", ")), slog.Any(circuit.LogKeyMissingCount, len(missing)), slog.Any(circuit.LogKeyDeclaredCount, len(hooks)), slog.Any(circuit.LogKeyAvailable, strings.Join(available, ", ")), slog.Any(circuit.LogKeyCircuit, circuitName))
}

func collectReferencedHooks(def *circuit.CircuitDef) map[string]bool {
	refs := make(map[string]bool)
	for i := range def.Nodes {
		for _, h := range def.Nodes[i].Before {
			refs[h] = true
		}
		for _, h := range def.Nodes[i].After {
			refs[h] = true
		}
	}
	return refs
}

func registeredHookNames(reg *GraphRegistries) []string {
	if reg.Hooks == nil {
		return nil
	}
	names := make([]string, 0, len(reg.Hooks))
	for name := range reg.Hooks {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
