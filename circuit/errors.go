package circuit

// Category: Processing & Support

import (
	"errors"

	"github.com/dpopsuev/origami/circuit/def"
)

// Framework sentinel errors — used by the runtime walker, observer, and engine.
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
	// ErrFindingVeto is returned by VetoHook when a FindingError targets the
	// current node. The hookingWalker intercepts this and wraps the artifact
	// with Confidence() 0.
	ErrFindingVeto = errors.New("framework: finding veto")

	// Schema validation errors.
	ErrArtifactIsNil        = errors.New("artifact is nil")
	ErrExpectedString       = errors.New("expected string")
	ErrExpectedNumber       = errors.New("expected number")
	ErrExpectedBoolean      = errors.New("expected boolean")
	ErrUnknownSchemaType    = errors.New("unknown schema type")
	ErrExpectedObject       = errors.New("expected object")
	ErrMissingRequiredField = errors.New("missing required field")
	ErrExpectedArray        = errors.New("expected array")
	ErrSchemaValidation     = errors.New("schema validation")

	// Report template errors.
	ErrReportTemplateMissingName         = errors.New("report template: missing name")
	ErrReportTemplateOverrideSection     = errors.New("report template: override section")
	ErrReportTemplateExtraColumnsSection = errors.New("report template: extra_columns section")
	ErrReportTemplateInsertAfterSection  = errors.New("report template: insert_after section")

	// Input reference errors.
	ErrInvalidInputReference = errors.New("invalid input reference")
	ErrInputReference        = errors.New("input reference: node has not produced output yet")
)

// DSL error re-exports from circuit/def — backward compatibility.
var ( //nolint:dupl // intentional re-exports for backward compatibility
	ErrNode                                      = def.ErrNode
	ErrComponentManifest                         = def.ErrComponentManifest
	ErrCircuitNameIsRequired                     = def.ErrCircuitNameIsRequired
	ErrAtLeastOneNodeIsRequired                  = def.ErrAtLeastOneNodeIsRequired
	ErrAtLeastOneEdgeIsRequired                  = def.ErrAtLeastOneEdgeIsRequired
	ErrStartNodeIsRequired                       = def.ErrStartNodeIsRequired
	ErrDoneNodeIsRequired                        = def.ErrDoneNodeIsRequired
	ErrNodeNameIsRequired                        = def.ErrNodeNameIsRequired
	ErrDuplicateNodeName                         = def.ErrDuplicateNodeName
	ErrStartNode                                 = def.ErrStartNode
	ErrEdgeIdIsRequired                          = def.ErrEdgeIdIsRequired
	ErrDuplicateEdgeId                           = def.ErrDuplicateEdgeId
	ErrEdge                                      = def.ErrEdge
	ErrZone                                      = def.ErrZone
	ErrEdgesMustBeASequence                      = def.ErrEdgesMustBeASequence
	ErrEdgeElementMustBeAStringOrMapping         = def.ErrEdgeElementMustBeAStringOrMapping
	ErrOverlayImports                            = def.ErrOverlayImports
	ErrOverlayCannotOverrideBaseNode             = def.ErrOverlayCannotOverrideBaseNode
	ErrOverlayCannotOverrideBasePort             = def.ErrOverlayCannotOverrideBasePort
	ErrCircuit                                   = def.ErrCircuit
	ErrBaseScorecardRequiredWhenOverlayHasImport = def.ErrBaseScorecardRequiredWhenOverlayHasImport
	ErrNoEngineConfiguredForStore                = def.ErrNoEngineConfiguredForStore
	ErrUnknownEngine                             = def.ErrUnknownEngine
	ErrCannotRedefineExistingColumn              = def.ErrCannotRedefineExistingColumn
)
