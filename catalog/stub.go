package catalog

// StubCatalog returns a hardcoded list of built-in instruments.
type StubCatalog struct {
	entries []Entry
}

var _ Catalog = (*StubCatalog)(nil)

func NewStubCatalog() *StubCatalog {
	return &StubCatalog{
		entries: []Entry{
			{Name: "echo", Module: "builtin", Version: "0.1.0", TrustLayer: Builtin, Signature: "echo(input []byte) -> Result"},
		},
	}
}

func (c *StubCatalog) List() []Entry {
	return append([]Entry(nil), c.entries...)
}

func (c *StubCatalog) Resolve(name string) (Entry, error) {
	for _, e := range c.entries {
		if e.Name == name {
			return e, nil
		}
	}
	return Entry{}, ErrNotFound
}
