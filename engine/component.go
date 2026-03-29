package engine

// Category: DSL & Build — component types.

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/dpopsuev/origami/circuit"
)

// Manifest type aliases — definitions live in circuit/ sub-package.

func LoadComponentManifest(path string) (*circuit.ComponentManifest, error) {
	return circuit.LoadComponentManifest(path)
}

// Component bundles reusable plumbing (transformers, extractors, hooks) under
// a namespace. Consumers merge components into their registries at build time.
type Component struct {
	Namespace    string
	Name         string
	Version      string
	Description  string
	Transformers TransformerRegistry
	Extractors   ExtractorRegistry
	Hooks        HookRegistry
}

// MergeComponents merges one or more components into a base GraphRegistries.
func MergeComponents(base *GraphRegistries, components ...*Component) (*GraphRegistries, error) {
	merged := &GraphRegistries{
		Transformers:     cloneMap(base.Transformers),
		Extractors:       cloneMap(base.Extractors),
		Hooks:            cloneMap(base.Hooks),
		Nodes:            base.Nodes,
		Edges:            base.Edges,
		Circuits:         base.Circuits,
		MediatorEndpoint: base.MediatorEndpoint,
	}

	slog.DebugContext(context.Background(), "merge components", slog.Any("component", "registry"), slog.Any("base_circuits", len(base.Circuits)), slog.Any("mediator_endpoint", base.MediatorEndpoint), slog.Any("components", len(components)))

	for _, a := range components {
		if err := mergeTransformers(merged.Transformers, a); err != nil {
			return nil, err
		}
		if err := mergeExtractors(merged.Extractors, a); err != nil {
			return nil, err
		}
		if err := mergeHooks(merged.Hooks, a); err != nil {
			return nil, err
		}
	}
	return merged, nil
}

func mergeTransformers(dst TransformerRegistry, a *Component) error {
	for name, t := range a.Transformers {
		fqcn := a.Namespace + "." + name
		if _, exists := dst[fqcn]; exists {
			return fmt.Errorf("%w: %q collision (component %s)", ErrTransformer, fqcn, a.Namespace)
		}
		dst[fqcn] = t
		if _, exists := dst[name]; !exists {
			dst[name] = t
		}
	}
	return nil
}

func mergeExtractors(dst ExtractorRegistry, a *Component) error {
	for name, e := range a.Extractors {
		fqcn := a.Namespace + "." + name
		if _, exists := dst[fqcn]; exists {
			return fmt.Errorf("%w: %q collision (component %s)", ErrExtractor, fqcn, a.Namespace)
		}
		dst[fqcn] = e
		if _, exists := dst[name]; !exists {
			dst[name] = e
		}
	}
	return nil
}

func mergeHooks(dst HookRegistry, a *Component) error {
	for name, h := range a.Hooks {
		fqcn := a.Namespace + "." + name
		if _, exists := dst[fqcn]; exists {
			return fmt.Errorf("%w: %q collision (component %s)", ErrHook, fqcn, a.Namespace)
		}
		dst[fqcn] = h
		if _, exists := dst[name]; !exists {
			dst[name] = h
		}
	}
	return nil
}

func cloneMap[K comparable, V any](src map[K]V) map[K]V {
	if src == nil {
		return make(map[K]V)
	}
	dst := make(map[K]V, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
