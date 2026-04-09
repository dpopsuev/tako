package def

import "errors"

// DSL sentinel errors — used by definition parsing, loading, and validation.
var (
	// ErrNode is returned when a node validation fails.
	ErrNode = errors.New("node")

	// ErrComponentManifest is returned for: component manifest
	ErrComponentManifest = errors.New("component manifest")

	// ErrInstrumentManifest is returned for: instrument manifest
	ErrInstrumentManifest = errors.New("instrument manifest")

	// ErrCircuitNameIsRequired is returned for: circuit name is required
	ErrCircuitNameIsRequired = errors.New("circuit name is required")

	// ErrAtLeastOneNodeIsRequired is returned for: at least one node is required
	ErrAtLeastOneNodeIsRequired = errors.New("at least one node is required")

	// ErrAtLeastOneEdgeIsRequired is returned for: at least one edge is required
	ErrAtLeastOneEdgeIsRequired = errors.New("at least one edge is required")

	// ErrStartNodeIsRequired is returned for: start node is required
	ErrStartNodeIsRequired = errors.New("start node is required")

	// ErrDoneNodeIsRequired is returned for: done node is required
	ErrDoneNodeIsRequired = errors.New("done node is required")

	// ErrNodeNameIsRequired is returned for: node name is required
	ErrNodeNameIsRequired = errors.New("node name is required")

	// ErrDuplicateNodeName is returned for: duplicate node name
	ErrDuplicateNodeName = errors.New("duplicate node name")

	// ErrStartNode is returned for: start node
	ErrStartNode = errors.New("start node")

	// ErrEdgeIdIsRequired is returned for: edge id is required
	ErrEdgeIdIsRequired = errors.New("edge id is required")

	// ErrDuplicateEdgeId is returned for: duplicate edge id
	ErrDuplicateEdgeId = errors.New("duplicate edge id")

	// ErrEdge is returned for: edge
	ErrEdge = errors.New("edge")

	// ErrZone is returned for: zone
	ErrZone = errors.New("zone")

	// ErrEdgesMustBeASequence is returned for: edges must be a sequence
	ErrEdgesMustBeASequence = errors.New("edges must be a sequence")

	// ErrEdgeElementMustBeAStringOrMapping is returned for: edge element must be a string or mapping
	ErrEdgeElementMustBeAStringOrMapping = errors.New("edge element must be a string or mapping")

	// ErrOverlayImports is returned for: overlay imports
	ErrOverlayImports = errors.New("overlay imports")

	// ErrOverlayCannotOverrideBaseNode is returned for: overlay cannot override base node
	ErrOverlayCannotOverrideBaseNode = errors.New("overlay cannot override base node")

	// ErrOverlayCannotOverrideBasePort is returned for: overlay cannot override base port
	ErrOverlayCannotOverrideBasePort = errors.New("overlay cannot override base port")

	// ErrCircuit is returned for: circuit
	ErrCircuit = errors.New("circuit")

	// ErrBaseScorecardRequiredWhenOverlayHasImport is returned for: base scorecard required when overlay has import
	ErrBaseScorecardRequiredWhenOverlayHasImport = errors.New("base scorecard required when overlay has import")

	// ErrNoEngineConfiguredForStore is returned for: no engine configured for store
	ErrNoEngineConfiguredForStore = errors.New("no engine configured for store")

	// ErrUnknownEngine is returned for: unknown engine
	ErrUnknownEngine = errors.New("unknown engine")

	// ErrCannotRedefineExistingColumn is returned for: cannot redefine existing column
	ErrCannotRedefineExistingColumn = errors.New("cannot redefine existing column")

	// ErrUnknownKind is returned when a YAML kind value is not registered in KnownKinds.
	ErrUnknownKind = errors.New("unknown kind")
)
