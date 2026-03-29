package calibrate

import (
	"context"
	"fmt"
	"io/fs"
	"sync"
)

// Capturer captures a bundle for a specific schematic. The implementation
// knows how to fetch data from live sources (git repos, docs) and write
// the bundle to disk with a manifest.
type Capturer interface {
	Schematic() string
	Capture(ctx context.Context, cfg CaptureConfig) error
}

// BundleValidator validates that a bundle's on-disk state matches its
// manifest. Schematics implement this to add format-specific checks
// beyond the generic file-existence and SHA verification.
type BundleValidator interface {
	Schematic() string
	Validate(fsys fs.FS) []error
}

var (
	captureMu  sync.RWMutex
	capturers  = map[string]Capturer{}
	validators = map[string]BundleValidator{}
)

// RegisterCapturer registers a Capturer for a schematic name.
func RegisterCapturer(c Capturer) {
	captureMu.Lock()
	defer captureMu.Unlock()
	capturers[c.Schematic()] = c
}

// RegisterValidator registers a BundleValidator for a schematic name.
func RegisterValidator(v BundleValidator) {
	captureMu.Lock()
	defer captureMu.Unlock()
	validators[v.Schematic()] = v
}

// GetCapturer returns the registered Capturer for a schematic, or an error.
func GetCapturer(schematic string) (Capturer, error) {
	captureMu.RLock()
	defer captureMu.RUnlock()
	c, ok := capturers[schematic]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrNoCapturerRegisteredForSchematic, schematic)
	}
	return c, nil
}

// GetValidator returns the registered BundleValidator for a schematic, or an error.
func GetValidator(schematic string) (BundleValidator, error) {
	captureMu.RLock()
	defer captureMu.RUnlock()
	v, ok := validators[schematic]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrNoValidatorRegisteredForSchematic, schematic)
	}
	return v, nil
}

// RegisteredSchematics returns sorted names of all registered capturers.
func RegisteredSchematics() []string {
	captureMu.RLock()
	defer captureMu.RUnlock()
	names := make([]string, 0, len(capturers))
	for name := range capturers {
		names = append(names, name)
	}
	return names
}
