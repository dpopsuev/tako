package domainfs

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

// DomainAssets provides typed, key-based access to files within an fs.FS.
// Instead of hardcoding paths like fs.ReadFile(fsys, "reports/calibration-report.yaml"),
// callers use ReadAsset("reports", "calibration-report") and get clear errors when
// the key does not exist.
type DomainAssets struct {
	fsys     fs.FS
	sections map[string]map[string]string // section -> key -> path
}

// NewDomainAssets creates a DomainAssets backed by fsys with the given section map.
// Each section maps logical keys to file paths within the filesystem.
// A nil fsys is permitted; all read operations will return an error.
func NewDomainAssets(fsys fs.FS, sections map[string]map[string]string) *DomainAssets {
	// Defensive copy so the caller cannot mutate the map after construction.
	copied := make(map[string]map[string]string, len(sections))
	for sec, keys := range sections {
		inner := make(map[string]string, len(keys))
		for k, v := range keys {
			inner[k] = v
		}
		copied[sec] = inner
	}
	return &DomainAssets{fsys: fsys, sections: copied}
}

// ReadAsset reads the file identified by (section, key). If the section or key
// is not registered, the error message lists the available alternatives.
func (d *DomainAssets) ReadAsset(section, key string) ([]byte, error) {
	if d.fsys == nil {
		return nil, fmt.Errorf("domainfs: nil filesystem")
	}
	keys, ok := d.sections[section]
	if !ok {
		return nil, fmt.Errorf(
			"domainfs: section %q not found; available sections: %s",
			section, sortedKeys(d.sections),
		)
	}
	path, ok := keys[key]
	if !ok {
		return nil, fmt.Errorf(
			"domainfs: asset %q not found in section %q; available keys: %s",
			key, section, sortedKeysFlat(keys),
		)
	}
	return fs.ReadFile(d.fsys, path)
}

// HasAsset reports whether (section, key) is registered.
func (d *DomainAssets) HasAsset(section, key string) bool {
	keys, ok := d.sections[section]
	if !ok {
		return false
	}
	_, ok = keys[key]
	return ok
}

// FS returns the underlying fs.FS for backward-compatible raw access.
func (d *DomainAssets) FS() fs.FS {
	return d.fsys
}

// Section returns a copy of the key-path map for the named section.
// Returns nil if the section does not exist.
func (d *DomainAssets) Section(name string) map[string]string {
	keys, ok := d.sections[name]
	if !ok {
		return nil
	}
	out := make(map[string]string, len(keys))
	for k, v := range keys {
		out[k] = v
	}
	return out
}

// sortedKeys returns a comma-separated list of map keys, sorted.
func sortedKeys[V any](m map[string]V) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}

// sortedKeysFlat is a non-generic helper for map[string]string.
func sortedKeysFlat(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}
