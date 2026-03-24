// Package curate provides domain-agnostic types and interfaces for
// curation circuits: consuming unstructured data from heterogeneous
// sources and producing structured, validated, promotable records.
//
// This package has zero consumer-domain imports.
// Domain-specific bindings are provided by adapter packages that
// map between curate.Record and their own types.
package curate

// Field is a single named value extracted from a source.
type Field struct {
	Name       string  `json:"name"`
	Value      any     `json:"value"`
	Source     string  `json:"source,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
}

// Record is a domain-agnostic container of extracted fields.
// It represents one item being curated (e.g. a test case, a document,
// a dataset entry). Fields are keyed by name for O(1) lookup.
type Record struct {
	ID     string           `json:"id"`
	Fields map[string]Field `json:"fields"`
	Tags   map[string]string `json:"tags,omitempty"`
}

// NewRecord creates an empty Record with the given ID.
func NewRecord(id string) Record {
	return Record{
		ID:     id,
		Fields: make(map[string]Field),
		Tags:   make(map[string]string),
	}
}

// Set adds or replaces a field on the record.
func (r *Record) Set(f Field) {
	if r.Fields == nil {
		r.Fields = make(map[string]Field)
	}
	r.Fields[f.Name] = f
}

// Get retrieves a field by name. Returns zero Field and false if absent.
func (r *Record) Get(name string) (Field, bool) {
	f, ok := r.Fields[name]
	return f, ok
}

// Has returns true if the record contains a non-zero-value field with the given name.
func (r *Record) Has(name string) bool {
	f, ok := r.Fields[name]
	return ok && f.Value != nil
}

// Dataset is a named collection of records with an associated schema.
type Dataset struct {
	Name    string   `json:"name"`
	Records []Record `json:"records"`
}
