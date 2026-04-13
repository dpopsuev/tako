package engine

import "errors"

var (
	// ErrNode is returned for: node
	ErrNode = errors.New("node")

	// ErrExtractor is returned for: extractor
	ErrExtractor = errors.New("extractor")

	// ErrTransformer is returned for: transformer
	ErrTransformer = errors.New("transformer")

	// ErrHook is returned for: hook
	ErrHook = errors.New("hook")

	// ErrGenerator is returned for: generator
	ErrGenerator = errors.New("generator")

	// ErrEdge is returned for: edge
	ErrEdge = errors.New("edge")

	// ErrExtractorRegistryIsNil is returned for: extractor registry is nil
	ErrExtractorRegistryIsNil = errors.New("extractor registry is nil")

	// ErrJSONExtractor is returned for: JSONExtractor
	ErrJSONExtractor = errors.New("JSONExtractor")

	// ErrRegexExtractor is returned for: RegexExtractor
	ErrRegexExtractor = errors.New("RegexExtractor")

	// ErrCodeBlockExtractor is returned for: CodeBlockExtractor
	ErrCodeBlockExtractor = errors.New("CodeBlockExtractor")

	// ErrLineSplitExtractor is returned for: LineSplitExtractor
	ErrLineSplitExtractor = errors.New("LineSplitExtractor")

	// ErrHookRegistryIsNil is returned for: hook registry is nil
	ErrHookRegistryIsNil = errors.New("hook registry is nil")

	// ErrFileWriteHookNode is returned for: file-write hook: node
	ErrFileWriteHookNode = errors.New("file-write hook: node")

	// ErrMediatorCircuitStart is returned for: mediator circuit/start
	ErrMediatorCircuitStart = errors.New("mediator circuit/start")

	// ErrMediatorCircuit is returned for: mediator circuit
	ErrMediatorCircuit = errors.New("mediator circuit")

	// ErrNilResult is returned for: nil result
	ErrNilResult = errors.New("nil result")

	// ErrToolError is returned when an MCP tool call returns an error response.
	ErrToolError = errors.New("tool error")

	// ErrToolReturnedError is returned for: tool returned error
	ErrToolReturnedError = errors.New("tool returned error")

	// ErrNoTextContentInResult is returned for: no text content in result
	ErrNoTextContentInResult = errors.New("no text content in result")

	// ErrContainer is returned for: container
	ErrContainer = errors.New("container")

	// ErrRendererRegistryIsNil is returned for: renderer registry is nil
	ErrRendererRegistryIsNil = errors.New("renderer registry is nil")

	// ErrRenderer is returned for: renderer
	ErrRenderer = errors.New("renderer")

	// ErrTemplateRendererExpectedCircuitTemplateContextGot is returned for: template renderer: expected circuit.TemplateContext, got
	ErrTemplateRendererExpectedCircuitTemplateContextGot = errors.New("template renderer: expected circuit.TemplateContext, got")

	// ErrTransformerRegistryIsNil is returned for: transformer registry is nil
	ErrTransformerRegistryIsNil = errors.New("transformer registry is nil")

	// ErrUnknownElement is returned for: unknown element
	ErrUnknownElement = errors.New("unknown element")

	// ErrWalkerNameIsRequired is returned for: walker name is required
	ErrWalkerNameIsRequired = errors.New("walker name is required")

	// ErrPersona is returned for: persona
	ErrPersona = errors.New("persona")

	// ErrUnknownPersona is returned for: unknown persona
	ErrUnknownPersona = errors.New("unknown persona")

	// ErrUnknownApproach is returned for: unknown approach
	ErrUnknownApproach = errors.New("unknown approach")

	// ErrUnknownRole is returned for: unknown role
	ErrUnknownRole = errors.New("unknown role")

	// ErrMaxStepsExceeded is returned when the maximum step count is reached.
	ErrMaxStepsExceeded = errors.New("max steps exceeded")

	// ErrWalkerNotFound is returned when a checkpoint does not exist for the given walker ID.
	ErrWalkerNotFound = errors.New("walker checkpoint not found")

	// ErrWalkerNotInterrupted is returned when trying to resume a walker that is not in interrupted state.
	ErrWalkerNotInterrupted = errors.New("walker is not in interrupted state")

	// ErrInstrument is returned for instrument dispatch errors.
	ErrInstrument = errors.New("instrument")

	// ErrInstrumentDispatch is returned when an instrument dispatch fails at runtime.
	ErrInstrumentDispatch = errors.New("instrument dispatch")

	// ErrTuneFailed is returned when an instrument's preflight tune command fails.
	ErrTuneFailed = errors.New("tune failed")

	// ErrChecksumMismatch is returned when an instrument's tune output doesn't match the declared checksum.
	ErrChecksumMismatch = errors.New("checksum mismatch")

	// ErrOutputSchemaViolation is returned when instrument output doesn't match the declared schema.
	ErrOutputSchemaViolation = errors.New("output schema violation")

)
