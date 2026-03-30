package resource

import (
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"

	"github.com/dpopsuev/origami/circuit"

	"gopkg.in/yaml.v3"
)

// Load parses raw YAML into a Resource envelope and dispatches to the
// registered KindHandler for typed parsing. Returns the envelope and
// the kind-specific typed object.
func Load(reg *KindRegistry, data []byte, source string) (*Resource, any, error) {
	// Extract envelope to determine kind.
	env, err := circuit.ParseEnvelope(data)
	if err != nil {
		return nil, nil, fmt.Errorf("parse envelope: %w", err)
	}
	if env.Kind == "" {
		return nil, nil, fmt.Errorf("%w: %s", ErrNoKindField, source)
	}

	handler := reg.Lookup(env.Kind)
	if handler == nil {
		return nil, nil, fmt.Errorf("%w: %q in %s", ErrUnknownKind, env.Kind, source)
	}

	// Parse via kind handler.
	typed, err := handler.Parse(data)
	if err != nil {
		return nil, nil, fmt.Errorf("parse %s %q: %w", env.Kind, source, err)
	}

	// Build Resource envelope.
	res := &Resource{
		Kind:    env.Kind,
		Version: env.Version,
		Metadata: Metadata{
			Name:        env.Metadata.Name,
			Description: env.Metadata.Description,
		},
		Raw:    data,
		Source: source,
	}

	// Try to extract apiVersion from raw YAML (not in circuit.Envelope).
	var rawMap map[string]any
	if yamlErr := yaml.Unmarshal(data, &rawMap); yamlErr == nil {
		if av, ok := rawMap["apiVersion"].(string); ok {
			res.APIVersion = av
		}
		if spec, ok := rawMap["spec"].(map[string]any); ok {
			res.Spec = spec
		}
	}

	return res, typed, nil
}

// LoadFile reads a file and calls Load.
func LoadFile(reg *KindRegistry, path string) (*Resource, any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read resource: %w", err)
	}
	return Load(reg, data, path)
}

// Validate runs the kind's validator on a parsed resource.
func Validate(reg *KindRegistry, res *Resource, parsed any) error {
	handler := reg.Lookup(res.Kind)
	if handler == nil {
		return fmt.Errorf("%w: %q", ErrUnknownKind, res.Kind)
	}
	return handler.Validate(parsed)
}

// Merge applies an overlay resource onto a base resource via the kind's merger.
func Merge(reg *KindRegistry, kind circuit.Kind, base, overlay any) (any, error) {
	handler := reg.Lookup(kind)
	if handler == nil {
		return nil, fmt.Errorf("%w: %q", ErrUnknownKind, kind)
	}
	return handler.Merge(base, overlay)
}

// Diff compares two resources and returns structural differences.
// Operates on the raw spec maps for kind-agnostic comparison.
func Diff(a, b *Resource) []DiffEntry {
	var entries []DiffEntry
	diffMaps("", toMap(a), toMap(b), &entries)
	return entries
}

func toMap(r *Resource) map[string]any {
	m := make(map[string]any)
	if r.Raw != nil {
		_ = yaml.Unmarshal(r.Raw, &m)
	}
	return m
}

func diffMaps(prefix string, a, b map[string]any, entries *[]DiffEntry) {
	keys := make(map[string]bool)
	for k := range a {
		keys[k] = true
	}
	for k := range b {
		keys[k] = true
	}

	sorted := make([]string, 0, len(keys))
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)

	for _, k := range sorted {
		path := k
		if prefix != "" {
			path = prefix + "." + k
		}
		va, oka := a[k]
		vb, okb := b[k]

		if !oka {
			*entries = append(*entries, DiffEntry{Path: path, A: nil, B: vb})
			continue
		}
		if !okb {
			*entries = append(*entries, DiffEntry{Path: path, A: va, B: nil})
			continue
		}

		// Recurse into nested maps.
		ma, aIsMap := va.(map[string]any)
		mb, bIsMap := vb.(map[string]any)
		if aIsMap && bIsMap {
			diffMaps(path, ma, mb, entries)
			continue
		}

		if !reflect.DeepEqual(va, vb) {
			// Truncate large values for readability.
			*entries = append(*entries, DiffEntry{Path: path, A: truncate(va), B: truncate(vb)})
		}
	}
}

func truncate(v any) any {
	if s, ok := v.(string); ok && len(s) > 200 {
		return s[:200] + "..."
	}
	return v
}

// Summary returns a compact string representation of a resource.
func (r *Resource) Summary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s/%s", r.Kind, r.Metadata.Name)
	if r.Version != "" {
		fmt.Fprintf(&b, " (%s)", r.Version)
	}
	if r.Source != "" {
		fmt.Fprintf(&b, " [%s]", r.Source)
	}
	return b.String()
}
