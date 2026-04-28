package fold

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/dpopsuev/tako/circuit/def"
)

// LoadedInstrument holds a parsed instrument manifest alongside its source path.
type LoadedInstrument struct {
	Name     string
	Path     string
	Manifest *def.InstrumentManifest
}

// LoadInstruments loads and validates all instrument manifests declared in
// the board manifest. Paths are resolved relative to baseDir.
func LoadInstruments(instruments map[string]string, baseDir string) ([]LoadedInstrument, error) {
	if len(instruments) == 0 {
		return nil, nil
	}

	names := make([]string, 0, len(instruments))
	for name := range instruments {
		names = append(names, name)
	}
	sort.Strings(names)

	loaded := make([]LoadedInstrument, 0, len(names))
	for _, name := range names {
		relPath := instruments[name]
		absPath := filepath.Join(baseDir, relPath)

		m, err := def.LoadInstrumentManifest(absPath)
		if err != nil {
			return nil, fmt.Errorf("%w: instrument %q at %s: %w", ErrInstrument, name, relPath, err)
		}

		if m.Name != name {
			return nil, fmt.Errorf("%w: instrument %q: manifest name is %q — must match board declaration", ErrInstrument, name, m.Name)
		}

		loaded = append(loaded, LoadedInstrument{
			Name:     name,
			Path:     relPath,
			Manifest: m,
		})
	}

	return loaded, nil
}
