package framework

import (
	"log/slog"
	"sort"
	"strings"
)

// runBuildDiagnostics performs post-build static analysis on the circuit graph.
// All diagnostics emit slog.Warn — they do not fail the build.
func runBuildDiagnostics(def *CircuitDef, reg GraphRegistries) {
	diagUnreferencedHooks(def, reg)
	diagMissingHookRefs(def, reg)
}

// diagUnreferencedHooks (D1): hooks registered in HookRegistry but not
// referenced by any node's before: or after: field.
func diagUnreferencedHooks(def *CircuitDef, reg GraphRegistries) {
	if len(reg.Hooks) == 0 {
		return
	}

	referenced := collectReferencedHooks(def)

	for name := range reg.Hooks {
		if !referenced[name] {
			slog.Warn("unreferenced hook",
				"component", "build",
				"diagnostic", "D1",
				"hook", name,
				"circuit", def.Circuit,
			)
		}
	}
}

// diagMissingHookRefs (D2+D4): nodes reference hooks that don't exist in the registry.
func diagMissingHookRefs(def *CircuitDef, reg GraphRegistries) {
	for _, nd := range def.Nodes {
		checkHookList(nd.Name, "before", nd.Before, reg, def.Circuit)
		checkHookList(nd.Name, "after", nd.After, reg, def.Circuit)
	}
}

func checkHookList(nodeName, phase string, hooks []string, reg GraphRegistries, circuit string) {
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
	slog.Warn("missing hook references",
		"component", "build",
		"diagnostic", "D2",
		"node", nodeName,
		"phase", phase,
		"missing", strings.Join(missing, ", "),
		"missing_count", len(missing),
		"declared_count", len(hooks),
		"available", strings.Join(available, ", "),
		"circuit", circuit,
	)
}

func collectReferencedHooks(def *CircuitDef) map[string]bool {
	refs := make(map[string]bool)
	for _, nd := range def.Nodes {
		for _, h := range nd.Before {
			refs[h] = true
		}
		for _, h := range nd.After {
			refs[h] = true
		}
	}
	return refs
}

func registeredHookNames(reg GraphRegistries) []string {
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
