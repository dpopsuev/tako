package fold

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dpopsuev/tako/circuit"
	"github.com/dpopsuev/tako/circuit/def"
)

// ValidateExports checks that every symbol declared in a ComponentManifest
// (session_factory, resolver, socket options) exists as an exported Go function
// in the module's source files. This is the "trait bound not satisfied" check —
// component.yaml declares contracts, this function enforces them.
// ErrExportNotFound is returned when component.yaml declares a symbol not exported by the module.
var ErrExportNotFound = errors.New("declared symbol not exported")

func ValidateExports(cm *def.ComponentManifest, moduleDir string) error {
	// Components (kind: Component) only declare gives, not session_factory/resolver/options.
	if cm.Kind == "Component" {
		return nil
	}

	exports, err := scanExportedFuncs(moduleDir)
	if err != nil {
		return fmt.Errorf("scan exports in %s: %w", moduleDir, err)
	}

	var missing []string

	// Check session_factory symbol.
	if cm.SessionFactory != "" {
		sym := extractSymbolName(cm.SessionFactory)
		if sym != "" && !exports[sym] {
			slog.WarnContext(context.Background(), circuit.LogSymbolNotExported,
				slog.Any(circuit.LogKeyComponent, circuit.LogComponentFold),
				slog.Any(circuit.LogKeyName, cm.Component),
				slog.Any(circuit.LogKeyHandler, sym),
				slog.Any(circuit.LogKeyPhase, "session_factory"),
			)
			missing = append(missing, fmt.Sprintf("session_factory %q: func %s not found in %s", cm.SessionFactory, sym, moduleDir))
		}
	}

	// Check resolver symbol.
	if cm.Resolver != "" {
		if !exports[cm.Resolver] {
			slog.WarnContext(context.Background(), circuit.LogSymbolNotExported,
				slog.Any(circuit.LogKeyComponent, circuit.LogComponentFold),
				slog.Any(circuit.LogKeyName, cm.Component),
				slog.Any(circuit.LogKeyHandler, cm.Resolver),
				slog.Any(circuit.LogKeyPhase, "resolver"),
			)
			missing = append(missing, fmt.Sprintf("resolver %q: func %s not found in %s", cm.Resolver, cm.Resolver, moduleDir))
		}
	}

	// Check socket option functions.
	allSockets := collectSockets(cm)
	for _, sock := range allSockets {
		if sock.Option == "" {
			continue
		}
		if !exports[sock.Option] {
			slog.WarnContext(context.Background(), circuit.LogSocketContractNotSatisfied,
				slog.Any(circuit.LogKeyComponent, circuit.LogComponentFold),
				slog.Any(circuit.LogKeyName, cm.Component),
				slog.Any(circuit.LogKeyNode, sock.Name),
				slog.Any(circuit.LogKeyHandler, sock.Option),
			)
			missing = append(missing, fmt.Sprintf("socket %q option %q: func %s not found in %s", sock.Name, sock.Option, sock.Option, moduleDir))
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("%w: component %q: %d declared symbol(s) not exported:\n  %s",
			ErrExportNotFound, cm.Component, len(missing), strings.Join(missing, "\n  "))
	}

	slog.InfoContext(context.Background(), circuit.LogExportValidationComplete,
		slog.Any(circuit.LogKeyComponent, circuit.LogComponentFold),
		slog.Any(circuit.LogKeyName, cm.Component),
		slog.Any(circuit.LogKeyCount, len(allSockets)),
	)
	return nil
}

// scanExportedFuncs reads all .go files in a directory (non-recursive, excludes
// _test.go) and returns the set of exported function names.
func scanExportedFuncs(dir string) (map[string]bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	funcPattern := regexp.MustCompile(`^func\s+(\p{Lu}\w*)\s*[\[(]`)
	exports := make(map[string]bool)

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			if matches := funcPattern.FindStringSubmatch(strings.TrimSpace(line)); len(matches) > 1 {
				exports[matches[1]] = true
			}
		}
	}
	return exports, nil
}

// extractSymbolName extracts the function name from a symbol like "Factory()" or "alpha.Factory()".
func extractSymbolName(symbol string) string {
	// Strip parentheses: "Factory()" → "Factory"
	sym := strings.TrimSuffix(symbol, "()")
	// Strip package prefix: "alpha.Factory" → "Factory"
	if idx := strings.LastIndex(sym, "."); idx >= 0 {
		sym = sym[idx+1:]
	}
	return sym
}

// collectSockets gathers all socket definitions from all needs sections.
func collectSockets(cm *def.ComponentManifest) []def.SocketDef {
	var all []def.SocketDef
	all = append(all, cm.Needs.Transports...)
	all = append(all, cm.Needs.Sources...)
	all = append(all, cm.Needs.Storage...)
	return all
}
