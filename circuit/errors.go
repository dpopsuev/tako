package circuit

// Category: Processing & Support

import "errors"

var (
	// ErrNodeNotFound is returned when a referenced node does not exist in the graph.
	ErrNodeNotFound = errors.New("framework: node not found")

	// ErrNoEdge is returned when no edge matches from the current node,
	// indicating the walk has reached a terminal state or a graph definition gap.
	ErrNoEdge = errors.New("framework: no matching edge from node")

	// ErrMaxLoops is returned when a loop edge's counter exceeds the configured maximum.
	ErrMaxLoops = errors.New("framework: max loop iterations exceeded")

	// ErrFanOutMerge is returned when parallel branches disagree on merge target or no merge is found.
	ErrFanOutMerge = errors.New("framework: fan-out merge error")

	// ErrEscalate is returned by RunOperator when Evaluate returns ActionEscalate.
	// The caller (e.g. a Broker) should handle the escalation.
	ErrEscalate = errors.New("framework: operator escalation requested")

	// ErrMaxIterations is returned by RunOperator when the iteration limit is reached
	// without the goal being met.
	ErrMaxIterations = errors.New("framework: operator max iterations exceeded")

	// ErrFindingVeto is returned by VetoHook when a FindingError targets the
	// current node. The hookingWalker intercepts this and wraps the artifact
	// with Confidence() 0.
	ErrFindingVeto = errors.New("framework: finding veto")
	// ErrNode is returned when a node validation fails.
	ErrNode = errors.New("node")

	// ErrComponentManifest is returned for: component manifest
	ErrComponentManifest = errors.New("component manifest")

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

	// ErrReportTemplateMissingName is returned for: report template: missing name
	ErrReportTemplateMissingName = errors.New("report template: missing name")

	// ErrReportTemplateOverrideSection is returned for: report template: override section
	ErrReportTemplateOverrideSection = errors.New("report template: override section")

	// ErrReportTemplateExtraColumnsSection is returned for: report template: extra_columns section
	ErrReportTemplateExtraColumnsSection = errors.New("report template: extra_columns section")

	// ErrReportTemplateInsertAfterSection is returned for: report template: insert_after section
	ErrReportTemplateInsertAfterSection = errors.New("report template: insert_after section")

	// ErrCircuit is returned for: circuit
	ErrCircuit = errors.New("circuit")

	// ErrArtifactIsNil is returned for: artifact is nil
	ErrArtifactIsNil = errors.New("artifact is nil")

	// ErrExpectedString is returned when a schema expects a string value.
	ErrExpectedString = errors.New("expected string")

	// ErrExpectedNumber is returned when a schema expects a number value.
	ErrExpectedNumber = errors.New("expected number")

	// ErrExpectedBoolean is returned when a schema expects a boolean value.
	ErrExpectedBoolean = errors.New("expected boolean")

	// ErrUnknownSchemaType is returned when a schema has an unrecognized type.
	ErrUnknownSchemaType = errors.New("unknown schema type")

	// ErrExpectedObject is returned when a schema expects an object value.
	ErrExpectedObject = errors.New("expected object")

	// ErrMissingRequiredField is returned when a required field is absent.
	ErrMissingRequiredField = errors.New("missing required field")

	// ErrExpectedArray is returned when a schema expects an array value.
	ErrExpectedArray = errors.New("expected array")

	// ErrSchemaValidation is returned when schema validation fails with multiple errors.
	ErrSchemaValidation = errors.New("schema validation")

	// ErrBaseScorecardRequiredWhenOverlayHasImport is returned for: base scorecard required when overlay has import
	ErrBaseScorecardRequiredWhenOverlayHasImport = errors.New("base scorecard required when overlay has import")

	// ErrNoEngineConfiguredForStore is returned for: no engine configured for store
	ErrNoEngineConfiguredForStore = errors.New("no engine configured for store")

	// ErrUnknownEngine is returned for: unknown engine
	ErrUnknownEngine = errors.New("unknown engine")

	// ErrCannotRedefineExistingColumn is returned for: cannot redefine existing column
	ErrCannotRedefineExistingColumn = errors.New("cannot redefine existing column")

	// ErrInvalidInputReference is returned for: invalid input reference
	ErrInvalidInputReference = errors.New("invalid input reference")

	// ErrInputReference is returned when a referenced node has not produced output yet.
	ErrInputReference = errors.New("input reference: node has not produced output yet")
)
