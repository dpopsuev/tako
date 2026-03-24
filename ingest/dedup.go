package ingest

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Compile-time check that DedupIndex implements DedupStore.
var _ DedupStore = (*DedupIndex)(nil)

// DedupIndex tracks known dedup keys to prevent duplicate ingestion.
// Keys are opaque strings; the format is determined by the schematic adapter.
type DedupIndex struct {
	known map[string]bool
}

// NewDedupIndex creates an empty dedup index.
func NewDedupIndex() *DedupIndex {
	return &DedupIndex{known: make(map[string]bool)}
}

// LoadDedupIndex scans directories for existing dedup keys.
// It reads JSON files looking for "dedup_key" fields.
func LoadDedupIndex(dirs ...string) (*DedupIndex, error) {
	idx := NewDedupIndex()
	for _, dir := range dirs {
		if err := idx.scanDir(dir); err != nil {
			return nil, err
		}
	}
	return idx, nil
}

func (d *DedupIndex) scanDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var doc struct {
			DedupKey string `json:"dedup_key"`
		}
		if json.Unmarshal(data, &doc) == nil && doc.DedupKey != "" {
			d.known[doc.DedupKey] = true
		}
	}
	return nil
}

// Contains returns true if the key is already known.
func (d *DedupIndex) Contains(key string) bool {
	return d.known[key]
}

// Add marks a key as known.
func (d *DedupIndex) Add(key string) {
	d.known[key] = true
}

// Size returns the number of known keys.
func (d *DedupIndex) Size() int {
	return len(d.known)
}
