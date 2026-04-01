package def

// Category: DSL & Build — circuit path resolution.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	embeddedCircuits   = map[string][]byte{}
	embeddedCircuitsMu sync.RWMutex
)

// RegisterEmbeddedCircuit registers a go:embed circuit by name.
// Consumers call this in init() to make circuits resolvable by name
// regardless of the working directory.
//
//	//go:embed circuits/achilles.yaml
//	var circuitYAML []byte
//
//	func init() {
//	    RegisterEmbeddedCircuit("achilles", circuitYAML)
//	}
func RegisterEmbeddedCircuit(name string, content []byte) {
	embeddedCircuitsMu.Lock()
	defer embeddedCircuitsMu.Unlock()
	embeddedCircuits[strings.ToLower(name)] = content
}

// ResolveOption configures circuit path resolution.
type ResolveOption func(*resolveConfig)

type resolveConfig struct {
	searchDirs []string
}

// WithSearchDirs adds directories to the search path.
func WithSearchDirs(dirs ...string) ResolveOption {
	return func(c *resolveConfig) { c.searchDirs = append(c.searchDirs, dirs...) }
}

// ResolveCircuitPath resolves a circuit by name, returning the YAML content.
// Resolution order:
//  1. Embedded registry (RegisterEmbeddedCircuit)
//  2. $ORIGAMI_CIRCUITS directory
//  3. Additional search dirs (from WithSearchDirs)
//  4. Current working directory
//
// Returns the raw YAML bytes and nil error on success.
func ResolveCircuitPath(name string, opts ...ResolveOption) ([]byte, error) {
	cfg := &resolveConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	key := strings.ToLower(name)

	embeddedCircuitsMu.RLock()
	if content, ok := embeddedCircuits[key]; ok {
		embeddedCircuitsMu.RUnlock()
		return content, nil
	}
	embeddedCircuitsMu.RUnlock()

	candidates := []string{name}
	if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
		candidates = append(candidates, name+".yaml", name+".yml")
	}

	searched := make([]string, 0, len(candidates)*2)

	if envDir := os.Getenv("ORIGAMI_CIRCUITS"); envDir != "" {
		for _, c := range candidates {
			p := filepath.Join(envDir, c)
			searched = append(searched, p)
			if data, err := os.ReadFile(p); err == nil {
				return data, nil
			}
		}
	}

	for _, dir := range cfg.searchDirs {
		for _, c := range candidates {
			p := filepath.Join(dir, c)
			searched = append(searched, p)
			if data, err := os.ReadFile(p); err == nil {
				return data, nil
			}
		}
	}

	for _, c := range candidates {
		searched = append(searched, c)
		if data, err := os.ReadFile(c); err == nil {
			return data, nil
		}
	}

	return nil, fmt.Errorf("%w: %q not found; searched: %s", ErrCircuit, name, strings.Join(searched, ", "))
}

// ClearEmbeddedCircuits is for testing only.
func ClearEmbeddedCircuits() {
	embeddedCircuitsMu.Lock()
	embeddedCircuits = map[string][]byte{}
	embeddedCircuitsMu.Unlock()
}
