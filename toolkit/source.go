package toolkit

// SourceKind classifies what a data source represents.
type SourceKind string

const (
	SourceKindRepo SourceKind = "repo"
	SourceKindSpec SourceKind = "spec"
	SourceKindDoc  SourceKind = "doc"
	SourceKindAPI  SourceKind = "api"
)

// ReadPolicy controls when a source is included in circuit routing.
type ReadPolicy string

const (
	ReadAlways      ReadPolicy = "always"
	ReadConditional ReadPolicy = "conditional"
)

// ResolutionStatus tracks whether a source's content has been fetched.
type ResolutionStatus string

const (
	Resolved    ResolutionStatus = "resolved"
	Cached      ResolutionStatus = "cached"
	Degraded    ResolutionStatus = "degraded"
	Unavailable ResolutionStatus = "unavailable"
	Unknown     ResolutionStatus = "unknown"
)

// Source is a single data source — a repository, specification document,
// API endpoint, or other information resource available to a circuit.
type Source struct {
	Name       string            `json:"name" yaml:"name"`
	Kind       SourceKind        `json:"kind" yaml:"kind"`
	URI        string            `json:"uri" yaml:"uri"`
	Purpose    string            `json:"purpose,omitempty" yaml:"purpose,omitempty"`
	Branch     string            `json:"branch,omitempty" yaml:"branch,omitempty"`
	Tags       map[string]string `json:"tags,omitempty" yaml:"tags,omitempty"`
	ReadPolicy ReadPolicy        `json:"read_policy,omitempty" yaml:"read_policy,omitempty"`
	ReadWhen   string            `json:"read_when,omitempty" yaml:"read_when,omitempty"`
	LocalPath  string            `json:"local_path,omitempty" yaml:"local_path,omitempty"`

	Org           string   `json:"org,omitempty" yaml:"org,omitempty"`
	BranchPattern string   `json:"branch_pattern,omitempty" yaml:"branch_pattern,omitempty"`
	Exclude       []string `json:"exclude,omitempty" yaml:"exclude,omitempty"`

	Resolution      ResolutionStatus `json:"resolution,omitempty" yaml:"resolution,omitempty"`
	ResolvedContent string           `json:"resolved_content,omitempty" yaml:"-"`
}

// IsAlwaysRead returns true if this source should be included in every
// circuit run regardless of routing rules.
func (s Source) IsAlwaysRead() bool {
	return s.ReadPolicy == ReadAlways
}
