package toolkit

// SourceCatalog provides access to a collection of data sources.
// Consuming schematics depend on this interface without knowing the
// backing implementation (slice, database, remote API, filtered view).
type SourceCatalog interface {
	Sources() []Source
	AlwaysReadSources() []Source
}

// SliceCatalog is a slice-backed SourceCatalog.
type SliceCatalog struct {
	Items []Source `json:"sources" yaml:"sources"`
}

func (c *SliceCatalog) Sources() []Source {
	if c == nil {
		return nil
	}
	return c.Items
}

func (c *SliceCatalog) AlwaysReadSources() []Source {
	if c == nil {
		return nil
	}
	var out []Source
	for _, s := range c.Items {
		if s.IsAlwaysRead() {
			out = append(out, s)
		}
	}
	return out
}
