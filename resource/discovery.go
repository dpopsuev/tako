package resource

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dpopsuev/tako/circuit"
)

// ResourceIndex holds discovered resources indexed by kind and name.
type ResourceIndex struct {
	byKind map[circuit.Kind][]*Resource
	byKey  map[string]*Resource // "kind/name" → Resource
	all    []*Resource
}

// DiscoverResources walks a directory tree and loads all YAML files that
// have a kind: field with a registered handler. Non-YAML files and files
// without kind: are silently skipped.
func DiscoverResources(reg *KindRegistry, root string) (*ResourceIndex, error) {
	idx := &ResourceIndex{
		byKind: make(map[circuit.Kind][]*Resource),
		byKey:  make(map[string]*Resource),
	}

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !isYAML(path) {
			return nil
		}

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil // skip unreadable files
		}

		// Quick envelope check before full parse.
		env, envErr := circuit.ParseEnvelope(data)
		if envErr != nil || env.Kind == "" {
			return nil // not a resource
		}
		if !reg.Has(env.Kind) {
			return nil // unregistered kind
		}

		res, _, loadErr := Load(reg, data, path)
		if loadErr != nil {
			return nil // skip unparseable resources
		}

		idx.byKind[res.Kind] = append(idx.byKind[res.Kind], res)
		key := string(res.Kind) + "/" + res.Metadata.Name
		idx.byKey[key] = res
		idx.all = append(idx.all, res)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return idx, nil
}

// ByKind returns all resources of the given kind, sorted by name.
func (idx *ResourceIndex) ByKind(k circuit.Kind) []*Resource {
	result := idx.byKind[k]
	sort.Slice(result, func(i, j int) bool {
		return result[i].Metadata.Name < result[j].Metadata.Name
	})
	return result
}

// Get returns a resource by kind and name, or nil if not found.
func (idx *ResourceIndex) Get(kind circuit.Kind, name string) *Resource {
	return idx.byKey[string(kind)+"/"+name]
}

// All returns all discovered resources sorted by kind then name.
func (idx *ResourceIndex) All() []*Resource {
	result := make([]*Resource, len(idx.all))
	copy(result, idx.all)
	sort.Slice(result, func(i, j int) bool {
		if result[i].Kind != result[j].Kind {
			return result[i].Kind < result[j].Kind
		}
		return result[i].Metadata.Name < result[j].Metadata.Name
	})
	return result
}

// Count returns the total number of discovered resources.
func (idx *ResourceIndex) Count() int { return len(idx.all) }

// KindCounts returns a map of kind → count.
func (idx *ResourceIndex) KindCounts() map[circuit.Kind]int {
	counts := make(map[circuit.Kind]int)
	for k, v := range idx.byKind {
		counts[k] = len(v)
	}
	return counts
}

func isYAML(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".yaml" || ext == ".yml"
}
