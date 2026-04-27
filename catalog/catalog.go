package catalog

import "errors"

var ErrNotFound = errors.New("catalog: instrument not found")

// TrustLayer indicates the provenance trust level of an instrument.
type TrustLayer int

const (
	Builtin   TrustLayer = iota // shipped with Tako
	Verified                    // signed by known publisher
	Community                   // unverified third-party
)

// Entry describes a discoverable instrument in the Catalog.
type Entry struct {
	Module     string
	Name       string
	Version    string
	TrustLayer TrustLayer
	Signature  string
}

// Catalog discovers and resolves instruments.
type Catalog interface {
	List() []Entry
	Resolve(name string) (Entry, error)
}
