package framework

// facade.go consolidates all type aliases, var forwards, function wrappers,
// and thin unexported helpers that the root "framework" package exposes.
//
// The actual implementations live in sub-packages (core/, circuit/, engine/,
// state/, finding/). This file is the single public surface of the framework
// package — everything here is either a type alias (=) or a one-line forward.
//
// Sections are ordered to match the layer numbering in doc.go:
//   1. Core Primitives          (core/, engine/)
//   2. DSL & Build              (circuit/, engine/)
//   3. Processing & Support     (core/, engine/, finding/)
//   4. Execution                (engine/, state/)

import (
	"io/fs"
	"log/slog"
	"time"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami/internal/finding"
	"github.com/dpopsuev/origami/internal/state"
)

// ---------------------------------------------------------------------------
// Section 1 — Core Primitives
// ---------------------------------------------------------------------------

// --- Node & Artifact (core/) ---

type Node = circuit.Node
type Artifact = circuit.Artifact
type CountableArtifact = circuit.CountableArtifact
type NodeContext = circuit.NodeContext

// --- Edge & Transition (core/) ---

type Edge = circuit.Edge
type ParallelEdge = circuit.ParallelEdge
type Transition = circuit.Transition

// --- ExpressionEdge (engine/) ---

type ExprContext = engine.ExprContext
type ExprState = engine.ExprState
type SignalExprHelpers = engine.SignalExprHelpers

var CompileExpressionEdge = engine.CompileExpressionEdge

// --- Graph (engine/) ---

type Graph = engine.Graph
type Zone = engine.Zone
type DefaultGraph = engine.DefaultGraph
type GraphOption = engine.GraphOption

var (
	WithDoneNode     = engine.WithDoneNode
	WithObserver     = engine.WithObserver
	WithNodeTimeouts = engine.WithNodeTimeouts
	NewGraph         = engine.NewGraph
)

// --- Walker (core/) ---

type Walker = circuit.Walker
type WalkerState = circuit.WalkerState
type StepRecord = circuit.StepRecord
type ProcessWalker = circuit.ProcessWalker

func NewWalkerState(id string) *WalkerState { return circuit.NewWalkerState(id) }
func NewProcessWalker(id string) *ProcessWalker {
	return circuit.NewProcessWalker(id)
}
func NewProcessWalkerWithIdentity(id AgentIdentity, stateID string) *ProcessWalker {
	return circuit.NewProcessWalkerWithIdentity(id, stateID)
}

// trajectoryType classifies a confidence convergence pattern.
type trajectoryType = circuit.TrajectoryType

const (
	TrajectoryUnderdamped      = circuit.TrajectoryUnderdamped
	TrajectoryOverdamped       = circuit.TrajectoryOverdamped
	TrajectoryCriticallyDamped = circuit.TrajectoryCriticallyDamped
	TrajectoryUnstable         = circuit.TrajectoryUnstable
	TrajectoryInsufficient     = circuit.TrajectoryInsufficient
)

// classifyTrajectory analyzes a confidence history to determine the convergence pattern.
func classifyTrajectory(history []float64) trajectoryType {
	return circuit.ClassifyTrajectory(history)
}

// readOnlyContext returns a shallow copy of the context map.
func readOnlyContext(ctx map[string]any) map[string]any {
	return circuit.ReadOnlyContext(ctx)
}

// --- DelegateNode (engine/) ---

type DelegateNode = engine.DelegateNode
type DelegateArtifact = engine.DelegateArtifact

// --- Element (core/) ---

type (
	Approach      = circuit.Approach
	Element       = circuit.Element
	SpeedClass    = circuit.SpeedClass
	ElementTraits = circuit.ElementTraits
)

// Approach constants.
const (
	ApproachRapid      = circuit.ApproachRapid
	ApproachAggressive = circuit.ApproachAggressive
	ApproachMethodical = circuit.ApproachMethodical
	ApproachRigorous   = circuit.ApproachRigorous
	ApproachAnalytical = circuit.ApproachAnalytical
	ApproachHolistic   = circuit.ApproachHolistic
)

// Element constants.
const (
	ElementFire      = circuit.ElementFire
	ElementLightning = circuit.ElementLightning
	ElementEarth     = circuit.ElementEarth
	ElementDiamond   = circuit.ElementDiamond
	ElementWater     = circuit.ElementWater
	ElementAir       = circuit.ElementAir
)

// SpeedClass constants.
const (
	SpeedFastest  = circuit.SpeedFastest
	SpeedFast     = circuit.SpeedFast
	SpeedSteady   = circuit.SpeedSteady
	SpeedPrecise  = circuit.SpeedPrecise
	SpeedDeep     = circuit.SpeedDeep
	SpeedHolistic = circuit.SpeedHolistic
)

// DefaultTraits returns the canonical trait set for a given element.
func DefaultTraits(e Element) ElementTraits { return circuit.DefaultTraits(e) }

// AllElements returns the six core elements.
func AllElements() []Element { return circuit.AllElements() }

// ResolveApproach maps a user-facing approach name to an internal Element.
func ResolveApproach(name string) (Element, bool) { return circuit.ResolveApproach(name) }

// ApproachForElement returns the user-facing approach name for an element.
func ApproachForElement(e Element) Approach { return circuit.ApproachForElement(e) }

// ApproachEmoji returns the emoji for an approach.
func ApproachEmoji(a Approach) string { return circuit.ApproachEmoji(a) }

// ApproachTraits returns the ElementTraits for an approach.
func ApproachTraits(a Approach) ElementTraits { return circuit.ApproachTraits(a) }

// ApproachTraitsSummary returns a formatted multi-line summary for LSP hover.
func ApproachTraitsSummary(a Approach) string { return circuit.ApproachTraitsSummary(a) }

// AllApproaches returns the six core approaches.
func AllApproaches() []Approach { return circuit.AllApproaches() }

// --- Identity (core/) ---

type Persona = circuit.Persona
type PersonaResolver = circuit.PersonaResolver
type Color = circuit.Color
type Alignment = circuit.Alignment
type Position = circuit.Position
type MetaPhase = circuit.MetaPhase
type Role = circuit.Role
type CostProfile = circuit.CostProfile
type AgentIdentity = circuit.AgentIdentity
type ModelIdentity = circuit.ModelIdentity

const (
	AlignmentThesis     = circuit.AlignmentThesis
	AlignmentAntithesis = circuit.AlignmentAntithesis
)

const (
	PositionPG = circuit.PositionPG
	PositionSG = circuit.PositionSG
	PositionPF = circuit.PositionPF
	PositionC  = circuit.PositionC
)

const (
	MetaPhaseBk = circuit.MetaPhaseBk
	MetaPhaseFc = circuit.MetaPhaseFc
	MetaPhasePt = circuit.MetaPhasePt
)

const (
	RoleWorker   = circuit.RoleWorker
	RoleManager  = circuit.RoleManager
	RoleEnforcer = circuit.RoleEnforcer
	RoleBroker   = circuit.RoleBroker
)

var ValidRoles = circuit.ValidRoles
var DefaultPersonaResolver = circuit.DefaultPersonaResolver

func HomeZoneFor(p Position) MetaPhase { return circuit.HomeZoneFor(p) }

// ---------------------------------------------------------------------------
// Section 2 — DSL & Build
// ---------------------------------------------------------------------------

// --- CircuitDef & loading (circuit/) ---

type CircuitDef = circuit.CircuitDef
type CalibrationContractDef = circuit.CalibrationContractDef
type CalibrationFieldDef = circuit.CalibrationFieldDef
type PortDef = circuit.PortDef
type WiringDef = circuit.WiringDef
type ExtractorDef = circuit.ExtractorDef
type WalkerDef = circuit.WalkerDef
type ContextFilterDef = circuit.ContextFilterDef
type ZoneDef = circuit.ZoneDef
type OutputField = circuit.OutputField
type NodeDef = circuit.NodeDef
type CacheDef = circuit.CacheDef
type EdgeDef = circuit.EdgeDef
type AssetResolver = circuit.AssetResolver
type TopologyValidator = circuit.TopologyValidator
type GraphShape = circuit.GraphShape
type GraphNodeInfo = circuit.GraphNodeInfo

// HandlerType constants.
const (
	HandlerTypeTransformer = circuit.HandlerTypeTransformer
	HandlerTypeExtractor   = circuit.HandlerTypeExtractor
	HandlerTypeRenderer    = circuit.HandlerTypeRenderer
	HandlerTypeNode        = circuit.HandlerTypeNode
	HandlerTypeDelegate    = circuit.HandlerTypeDelegate
	HandlerTypeCircuit     = circuit.HandlerTypeCircuit
)

// Merge strategy constants.
const (
	MergeAppend = circuit.MergeAppend
	MergeLatest = circuit.MergeLatest
	MergeCustom = circuit.MergeCustom
)

// RegisterTopologyValidator sets the default topology validator.
func RegisterTopologyValidator(v TopologyValidator) {
	circuit.RegisterTopologyValidator(v)
}

// InferTopology computes shortcut and loop flags from graph topology.
func InferTopology(def *CircuitDef) { circuit.InferTopology(def) }

// LoadCircuit parses a YAML circuit definition and returns a CircuitDef.
func LoadCircuit(data []byte) (*CircuitDef, error) { return circuit.LoadCircuit(data) }

// LoadCircuitWithOverlay parses a consumer overlay YAML and resolves imports.
func LoadCircuitWithOverlay(overlayData []byte, resolver AssetResolver) (*CircuitDef, error) {
	return circuit.LoadCircuitWithOverlay(overlayData, resolver)
}

// --- Registry and build types (engine/) ---

type NodeRegistry = engine.NodeRegistry
type EdgeFactory = engine.EdgeFactory
type ComponentLoader = engine.ComponentLoader
type GraphRegistries = engine.GraphRegistries

var BuildGraph = engine.BuildGraph

// --- Component & manifest (circuit/, engine/) ---

type SocketDef = circuit.SocketDef
type SatisfiesDef = circuit.SatisfiesDef
type ComponentManifest = circuit.ComponentManifest

func LoadComponentManifest(path string) (*ComponentManifest, error) {
	return circuit.LoadComponentManifest(path)
}

type Component = engine.Component

var MergeComponents = engine.MergeComponents

// --- Envelope (circuit/) ---

type Envelope = circuit.Envelope
type Metadata = circuit.Metadata

// ParseEnvelope extracts just the envelope fields from raw YAML bytes.
func ParseEnvelope(data []byte) (*Envelope, error) { return circuit.ParseEnvelope(data) }

// KnownKinds enumerates the recognized kind values for Origami YAML files.
var KnownKinds = circuit.KnownKinds

// --- Render (circuit/) ---

// Render generates a Mermaid flowchart string from a circuit definition.
func Render(def *CircuitDef) string { return circuit.Render(def) }

// --- ReportTemplate (circuit/) ---

type ReportTemplate = circuit.ReportTemplate
type ReportSectionDef = circuit.ReportSectionDef

var (
	LoadReportTemplate   = circuit.LoadReportTemplate
	MergeReportTemplates = circuit.MergeReportTemplates
)

// --- Resolve (circuit/) ---

type ResolveOption = circuit.ResolveOption

func WithSearchDirs(dirs ...string) ResolveOption { return circuit.WithSearchDirs(dirs...) }

func RegisterEmbeddedCircuit(name string, content []byte) {
	circuit.RegisterEmbeddedCircuit(name, content)
}

func ResolveCircuitPath(name string, opts ...ResolveOption) ([]byte, error) {
	return circuit.ResolveCircuitPath(name, opts...)
}

// clearEmbeddedCircuits is for testing only.
func clearEmbeddedCircuits() {
	circuit.ClearEmbeddedCircuits()
}

// --- Schema (circuit/) ---

type ArtifactSchema = circuit.ArtifactSchema
type FieldSchema = circuit.FieldSchema

// ValidateArtifact checks that an artifact's Raw() value conforms to the schema.
func ValidateArtifact(schema *ArtifactSchema, artifact Artifact) error {
	return circuit.ValidateArtifact(schema, artifact)
}

// --- ScorecardOverlay (circuit/) ---

type ScorecardDef = circuit.ScorecardDef
type ScorecardMetric = circuit.ScorecardMetric
type CostModelDef = circuit.CostModelDef

func LoadScorecardDef(data []byte) (*ScorecardDef, error) { return circuit.LoadScorecardDef(data) }

func MergeScorecardDefs(base, overlay *ScorecardDef) (*ScorecardDef, error) {
	return circuit.MergeScorecardDefs(base, overlay)
}

func RegisterScorecardVocabulary(sd *ScorecardDef, v *RichMapVocabulary) {
	circuit.RegisterScorecardVocabulary(sd, v)
}

// --- StoreRegistry (circuit/) ---

type StoreRegistry = circuit.StoreRegistry
type StoreEngineFactory = circuit.StoreEngineFactory

func NewStoreRegistry(wiring *StoreWiring) *StoreRegistry {
	return circuit.NewStoreRegistry(wiring)
}

// --- StoreSchema (circuit/) ---

type StoreWiring = circuit.StoreWiring
type StoreBinding = circuit.StoreBinding
type StoreLifecycle = circuit.StoreLifecycle

const (
	LifecycleSession    = circuit.LifecycleSession
	LifecyclePersistent = circuit.LifecyclePersistent
)

type StoreDeclaration = circuit.StoreDeclaration
type StoreEngine = circuit.StoreEngine
type StoreSchema = circuit.StoreSchema
type StoreTableDef = circuit.StoreTableDef
type StoreColumnDef = circuit.StoreColumnDef
type StoreIndexDef = circuit.StoreIndexDef
type SchemaProvider = circuit.SchemaProvider

func LoadStoreSchema(data []byte) (*StoreSchema, error) { return circuit.LoadStoreSchema(data) }

func MergeStoreSchemas(base, overlay *StoreSchema) (*StoreSchema, error) {
	return circuit.MergeStoreSchemas(base, overlay)
}

// --- SubCircuit (circuit/) ---

func LoadSubCircuitsFromFS(fsys fs.FS, resolvers map[string]AssetResolver) map[string]*CircuitDef {
	return circuit.LoadSubCircuitsFromFS(fsys, resolvers)
}

// --- Vars (circuit/) ---

type TemplateContext = circuit.TemplateContext

func ResolveInput(input string, outputs map[string]Artifact) (Artifact, error) {
	return circuit.ResolveInput(input, outputs)
}

func RenderPrompt(tmplContent string, tc TemplateContext) (string, error) {
	return circuit.RenderPrompt(tmplContent, tc)
}

func MergeVars(base map[string]any, overrides map[string]any) map[string]any {
	return circuit.MergeVars(base, overrides)
}

// --- Vocabulary (circuit/) ---

type Vocabulary = circuit.Vocabulary
type VocabularyFunc = circuit.VocabularyFunc
type MapVocabulary = circuit.MapVocabulary
type VocabEntry = circuit.VocabEntry
type RichVocabulary = circuit.RichVocabulary
type RichMapVocabulary = circuit.RichMapVocabulary

func NewMapVocabulary() *MapVocabulary        { return circuit.NewMapVocabulary() }
func NewRichMapVocabulary() *RichMapVocabulary { return circuit.NewRichMapVocabulary() }
func NameWithCode(v Vocabulary, code string) string {
	return circuit.NameWithCode(v, code)
}

// chainVocabulary tries multiple vocabularies in order.
type chainVocabulary = circuit.ChainVocabulary

// richChainVocabulary tries multiple RichVocabulary implementations in order.
type richChainVocabulary = circuit.RichChainVocabulary

// --- WalkerBuild (engine/) ---

// ValidateElement checks that name is a recognized element and returns it.
var ValidateElement = engine.ValidateElement

// BuildWalkersFromDef constructs Walker instances from YAML walker definitions.
var BuildWalkersFromDef = engine.BuildWalkersFromDef

// ---------------------------------------------------------------------------
// Section 3 — Processing & Support
// ---------------------------------------------------------------------------

// --- Transformer (engine/) ---

type Transformer = engine.Transformer
type DeterministicTransformer = engine.DeterministicTransformer
type TypedTransformer = engine.TypedTransformer
type TransformerContext = engine.TransformerContext
type TransformerRegistry = engine.TransformerRegistry

var (
	IsDeterministic     = engine.IsDeterministic
	TransformerFunc     = engine.TransformerFunc
	IsTransformerNode   = engine.IsTransformerNode
	TransformerNodeName = engine.TransformerNodeName
)

const (
	BuiltinTransformerGoTemplate  = engine.BuiltinTransformerGoTemplate
	BuiltinTransformerPassthrough = engine.BuiltinTransformerPassthrough
)

// --- Extractor (engine/) ---

type Extractor = engine.Extractor
type ExtractorRegistry = engine.ExtractorRegistry

const (
	BuiltinExtractorJSONSchema = engine.BuiltinExtractorJSONSchema
	BuiltinExtractorRegex      = engine.BuiltinExtractorRegex
)

type JSONSchemaExtractor = engine.JSONSchemaExtractor

var NewRegexExtractor = engine.NewRegexExtractor
var MustRegexExtractor = engine.MustRegexExtractor

// --- Extractors (engine/) ---

// NewJSONExtractor parses JSON bytes into a typed Go struct.
// Generic function — cannot be aliased via var, so forwarded explicitly.
func NewJSONExtractor[T any](name string) Extractor {
	return engine.NewJSONExtractor[T](name)
}

var (
	NewCodeBlockExtractor = engine.NewCodeBlockExtractor
	NewLineSplitExtractor = engine.NewLineSplitExtractor
)

// --- Hook (engine/) ---

var (
	WithWalkerState       = engine.WithWalkerState
	WalkerStateFromContext = engine.WalkerStateFromContext
)

type Hook = engine.Hook
type HookRegistry = engine.HookRegistry
type HookFunc = engine.HookFunc

var NewHookFunc = engine.NewHookFunc

const BuiltinHookFileWrite = engine.BuiltinHookFileWrite

type FileWriteHook = engine.FileWriteHook

// --- Renderer (engine/) ---

type Renderer = engine.Renderer
type RendererRegistry = engine.RendererRegistry
type TemplateRenderer = engine.TemplateRenderer

const BuiltinRendererTemplate = engine.BuiltinRendererTemplate

// --- Observer (core/, engine/) ---

type WalkEventType = circuit.WalkEventType

const (
	EventNodeEnter        = circuit.EventNodeEnter
	EventNodeExit         = circuit.EventNodeExit
	EventEdgeEvaluate     = circuit.EventEdgeEvaluate
	EventTransition       = circuit.EventTransition
	EventWalkerSwitch     = circuit.EventWalkerSwitch
	EventFanOutStart      = circuit.EventFanOutStart
	EventFanOutEnd        = circuit.EventFanOutEnd
	EventWalkComplete     = circuit.EventWalkComplete
	EventWalkError        = circuit.EventWalkError
	EventWalkInterrupted  = circuit.EventWalkInterrupted
	EventWalkResumed      = circuit.EventWalkResumed
	EventCheckpointSaved  = circuit.EventCheckpointSaved
	EventProviderFallback = circuit.EventProviderFallback
	EventCircuitOpen      = circuit.EventCircuitOpen
	EventCircuitClose     = circuit.EventCircuitClose
	EventRateLimit        = circuit.EventRateLimit
	EventThermalWarning   = circuit.EventThermalWarning
	EventDelegateStart    = circuit.EventDelegateStart
	EventDelegateEnd      = circuit.EventDelegateEnd
)

type WalkEvent = circuit.WalkEvent
type WalkObserver = circuit.WalkObserver
type WalkObserverFunc = circuit.WalkObserverFunc
type MultiObserver = circuit.MultiObserver

// TraceCollector accumulates walk events in memory for post-walk analysis.
type TraceCollector = engine.TraceCollector

// NewLogObserver creates a WalkObserver that logs events using the given logger.
func NewLogObserver(logger *slog.Logger) WalkObserver { return engine.NewLogObserver(logger) }

// emitEvent is a helper to safely emit an event to a possibly-nil observer.
// Duplicated from core/ because it is unexported and used by root-package code.
func emitEvent(obs WalkObserver, e WalkEvent) {
	if obs != nil {
		obs.OnEvent(e)
	}
}

// --- Narration (engine/) ---

// narrationSink receives a single human-readable narration line.
type narrationSink = engine.NarrationSink

// narrationOption configures a narrationObserver.
type narrationOption = engine.NarrationOption

// withVocabulary sets the vocabulary for translating node/edge names.
func withVocabulary(v Vocabulary) narrationOption { return engine.WithVocabulary(v) }

// withSink sets the output destination for narration lines.
func withSink(s narrationSink) narrationOption { return engine.WithSink(s) }

// withMilestoneInterval sets how often milestone summaries are emitted.
func withMilestoneInterval(every int) narrationOption { return engine.WithMilestoneInterval(every) }

// withETA enables or disables ETA estimation in narration output.
func withETA(enabled bool) narrationOption { return engine.WithETA(enabled) }

// progress captures a snapshot of walk progress.
type progress = engine.Progress

// narrationObserver is a WalkObserver that produces human-readable narration.
type narrationObserver = engine.NarrationObserver

// newNarrationObserver creates a narration observer with sensible defaults.
func newNarrationObserver(opts ...narrationOption) *narrationObserver {
	return engine.NewNarrationObserver(opts...)
}

// fmtNarrateDuration formats a duration for narration output.
func fmtNarrateDuration(d time.Duration) string { return engine.FmtNarrateDuration(d) }

// --- Capture (core/, state/, engine/) ---

// ArtifactCapture provides access to artifacts captured during a walk.
// Obtain one via NewCapture() and use the returned WalkObserver during the walk.
type ArtifactCapture = circuit.ArtifactCapture

// outputCapture collects artifacts produced at each node during a walk.
type outputCapture = state.OutputCapture

// newOutputCapture creates an outputCapture ready for use.
func newOutputCapture() *outputCapture { return state.NewOutputCapture() }

// NewCapture returns a WalkObserver that captures artifacts and an ArtifactCapture
// to read them after the walk. Use the observer with MultiObserver or run config.
func NewCapture() (WalkObserver, ArtifactCapture) {
	return state.NewCapture()
}

// withOutputCapture attaches an outputCapture as a walk observer.
// If another observer is already set, both are composed via MultiObserver.
func withOutputCapture(capture *outputCapture) RunOption {
	return engine.WithOutputCapture(capture)
}

// --- Determinism (engine/) ---

// isCircuitDeterministic delegates to engine package.
func isCircuitDeterministic(def *CircuitDef, reg TransformerRegistry) bool {
	return engine.IsCircuitDeterministic(def, reg)
}

// --- TraceRecorder (engine/) ---

type TraceLevel = engine.TraceLevel

const (
	LevelInfo  = engine.LevelInfo
	LevelDebug = engine.LevelDebug
	LevelTrace = engine.LevelTrace
)

type TraceEvent = engine.TraceEvent
type TraceRecorder = engine.TraceRecorder

var NewTraceRecorder = engine.NewTraceRecorder

// --- Finding (core/, finding/) ---

type FindingSeverity = circuit.FindingSeverity

const (
	FindingInfo    = circuit.FindingInfo
	FindingWarning = circuit.FindingWarning
	FindingError   = circuit.FindingError
)

type Finding = circuit.Finding
type FindingCollector = circuit.FindingCollector

const FindingCollectorKey = circuit.FindingCollectorKey

func SeverityAtOrAbove(have, threshold FindingSeverity) bool {
	return circuit.SeverityAtOrAbove(have, threshold)
}

type InMemoryFindingCollector = finding.InMemoryFindingCollector

// --- FindingHook (finding/) ---

type VetoHook = finding.VetoHook

func NewVetoHook(collector FindingCollector) *VetoHook {
	return finding.NewVetoHook(collector)
}

// --- FindingRouter (finding/) ---

type RouteTarget = finding.RouteTarget

const (
	TargetManager = finding.TargetManager
	TargetBroker  = finding.TargetBroker
	TargetLog     = finding.TargetLog
)

type RouteRule = finding.RouteRule
type FindingHandlers = finding.FindingHandlers
type FindingRouter = finding.FindingRouter

func NewFindingRouter(rules []RouteRule, handlers FindingHandlers) *FindingRouter {
	return finding.NewFindingRouter(rules, handlers)
}

// --- Errors (core/) ---

var (
	ErrNodeNotFound  = circuit.ErrNodeNotFound
	ErrNoEdge        = circuit.ErrNoEdge
	ErrMaxLoops      = circuit.ErrMaxLoops
	ErrFanOutMerge   = circuit.ErrFanOutMerge
	ErrEscalate      = circuit.ErrEscalate
	ErrMaxIterations = circuit.ErrMaxIterations
	ErrFindingVeto   = circuit.ErrFindingVeto
)

// --- Log constants (core/) ---

const (
	LogComponentWalk      = circuit.LogComponentWalk
	LogComponentDSL       = circuit.LogComponentDSL
	LogComponentCalibrate = circuit.LogComponentCalibrate
	LogComponentBatch     = circuit.LogComponentBatch
	LogComponentTransform = circuit.LogComponentTransform
)

const (
	LogNodeEnter        = circuit.LogNodeEnter
	LogNodeExit         = circuit.LogNodeExit
	LogEdgeTaken        = circuit.LogEdgeTaken
	LogEdgeNoMatch      = circuit.LogEdgeNoMatch
	LogLoopIncremented  = circuit.LogLoopIncremented
	LogWalkComplete     = circuit.LogWalkComplete
	LogWalkError        = circuit.LogWalkError
	LogDelegateStart    = circuit.LogDelegateStart
	LogDelegateComplete = circuit.LogDelegateComplete

	LogOverlayMerge         = circuit.LogOverlayMerge
	LogOverlayMergeComplete = circuit.LogOverlayMergeComplete
	LogSubCircuitLoaded     = circuit.LogSubCircuitLoaded

	LogRunStart       = circuit.LogRunStart
	LogCaseComplete   = circuit.LogCaseComplete
	LogAllCasesFailed = circuit.LogAllCasesFailed
)

const (
	LogKeyComponent = circuit.LogKeyComponent
	LogKeyNode      = circuit.LogKeyNode
	LogKeyEdge      = circuit.LogKeyEdge
	LogKeyFrom      = circuit.LogKeyFrom
	LogKeyTo        = circuit.LogKeyTo
	LogKeyWalker    = circuit.LogKeyWalker
	LogKeyElapsed   = circuit.LogKeyElapsed
	LogKeyLoop      = circuit.LogKeyLoop
	LogKeyShortcut  = circuit.LogKeyShortcut
	LogKeyCount     = circuit.LogKeyCount
	LogKeyError     = circuit.LogKeyError
	LogKeyCaseID    = circuit.LogKeyCaseID
	LogKeyCircuit   = circuit.LogKeyCircuit
)

// --- Context-key & protocol constants (core/) ---

const (
	ContextKeyTraceID = circuit.ContextKeyTraceID
)

const (
	ExtraKeyCircuitType = circuit.ExtraKeyCircuitType
	ExtraKeyTraceID     = circuit.ExtraKeyTraceID
)

const (
	TraceMetaDelegation = circuit.TraceMetaDelegation
	TraceMetaSource     = circuit.TraceMetaSource
)

const (
	ProtoKeySessionID     = circuit.ProtoKeySessionID
	ProtoKeyDone          = circuit.ProtoKeyDone
	ProtoKeyAvailable     = circuit.ProtoKeyAvailable
	ProtoKeyStep          = circuit.ProtoKeyStep
	ProtoKeyDispatchID    = circuit.ProtoKeyDispatchID
	ProtoKeyPromptContent = circuit.ProtoKeyPromptContent
	ProtoKeyCaseID        = circuit.ProtoKeyCaseID
	ProtoKeyArtifactPath  = circuit.ProtoKeyArtifactPath
	ProtoKeyFields        = circuit.ProtoKeyFields
	ProtoKeyExtra         = circuit.ProtoKeyExtra
	ProtoKeyError         = circuit.ProtoKeyError
	ProtoKeyStatus        = circuit.ProtoKeyStatus
	ProtoKeyStructured    = circuit.ProtoKeyStructured
	ProtoKeyTimeoutMS     = circuit.ProtoKeyTimeoutMS
)

// --- Defaults (engine/) ---

// DefaultWalker returns a zero-config Walker suitable for consumers that
// don't need persona or element customization.
var DefaultWalker = engine.DefaultWalker

// DefaultWalkerWithElement returns a default Walker with a custom element.
var DefaultWalkerWithElement = engine.DefaultWalkerWithElement

// ---------------------------------------------------------------------------
// Section 4 — Execution
// ---------------------------------------------------------------------------

// --- Run (engine/) ---

type RunOption = engine.RunOption

var (
	WithTransformers       = engine.WithTransformers
	WithHooks              = engine.WithHooks
	WithExtractors         = engine.WithExtractors
	WithNodes              = engine.WithNodes
	WithEdges              = engine.WithEdges
	WithComponents         = engine.WithComponents
	WithOverrides          = engine.WithOverrides
	WithWalker             = engine.WithWalker
	WithTeam               = engine.WithTeam
	WithRunObserver        = engine.WithRunObserver
	WithLogger             = engine.WithLogger
	WithMemory             = engine.WithMemory
	WithTaggedMemory       = engine.WithTaggedMemory
	WithNodeCache          = engine.WithNodeCache
	WithCheckpointer       = engine.WithCheckpointer
	WithResume             = engine.WithResume
	WithResumeInput        = engine.WithResumeInput
	WithOffsetCompensation = engine.WithOffsetCompensation

	Run      = engine.Run
	Validate = engine.Validate
)

// --- Runner (engine/) ---

type Interrupt = engine.Interrupt
type Runner = engine.Runner

var (
	IsInterrupt          = engine.IsInterrupt
	AsInterrupt          = engine.AsInterrupt
	NewRunner            = engine.NewRunner
	NewRunnerWith        = engine.NewRunnerWith
	WrapWithCheckpointer = engine.WrapWithCheckpointer
)

// --- Scheduler (engine/) ---

type SchedulerContext = engine.SchedulerContext
type Scheduler = engine.Scheduler
type SingleScheduler = engine.SingleScheduler
type AffinityScheduler = engine.AffinityScheduler

// --- Executor (engine/) ---

type Executor = engine.Executor
type InProcessExecutor = engine.InProcessExecutor
type ExecutorFunc = engine.ExecutorFunc

// --- BatchWalk (engine/) ---

type BatchCase = engine.BatchCase
type BatchWalkResult = engine.BatchWalkResult
type BatchWalkConfig = engine.BatchWalkConfig

var BatchWalk = engine.BatchWalk

// --- Checkpoint (core/, state/) ---

// Checkpointer persists and restores WalkerState between nodes, enabling
// resume-from-failure and crash recovery. Implementations must be safe
// for concurrent use by multiple walkers with distinct IDs.
type Checkpointer = circuit.Checkpointer

// JSONCheckpointer persists WalkerState to a JSON file between nodes,
// enabling resume-from-failure for circuits.
type JSONCheckpointer = state.JSONCheckpointer

// NewJSONCheckpointer creates a checkpointer that writes to the given directory.
func NewJSONCheckpointer(dir string) (*JSONCheckpointer, error) {
	return state.NewJSONCheckpointer(dir)
}

// --- Cache (core/, state/) ---

// NodeCache stores and retrieves node output artifacts by cache key.
type NodeCache = circuit.NodeCache

// InMemoryCache is a thread-safe in-memory NodeCache with TTL-based lazy eviction.
type InMemoryCache = state.InMemoryCache

// NewInMemoryCache creates a new in-memory node cache.
func NewInMemoryCache() *InMemoryCache { return state.NewInMemoryCache() }

// cachePolicy configures caching behavior for a node via the DSL.
type cachePolicy struct {
	TTL     time.Duration            `yaml:"ttl,omitempty"`
	KeyFunc func(NodeContext) string `yaml:"-"`
}

// eventNodeCacheHit is emitted when a cached artifact is returned instead of
// processing the node.
const eventNodeCacheHit WalkEventType = "node_cache_hit"

// --- Memory (core/, state/) ---

// MemoryStore provides cross-walk, identity-scoped key-value persistence.
type MemoryStore = circuit.MemoryStore

// MemoryItem represents a stored memory entry with metadata.
type MemoryItem = circuit.MemoryItem

// Conventional namespace constants for the three memory types.
const (
	NamespaceSemantic   = circuit.NamespaceSemantic
	NamespaceEpisodic   = circuit.NamespaceEpisodic
	NamespaceProcedural = circuit.NamespaceProcedural
)

// InMemoryStore is a thread-safe in-process MemoryStore with namespace support.
type InMemoryStore = state.InMemoryStore

// NewInMemoryStore creates a ready-to-use InMemoryStore.
func NewInMemoryStore() *InMemoryStore { return state.NewInMemoryStore() }

// taggedSetter is implemented by MemoryStore backends that support tagged writes.
type taggedSetter = state.TaggedSetter

// taggedMemoryStore wraps a MemoryStore and auto-appends tags to every SetNS call.
type taggedMemoryStore = state.TaggedMemoryStore

// --- Memory type helper functions (unexported, used by root tests) ---

// setFact stores a semantic fact about a walker.
func setFact(store MemoryStore, walkerID, key string, value any) {
	store.SetNS(NamespaceSemantic, walkerID, key, value)
}

// recordEpisode stores an episodic memory (a walk summary).
func recordEpisode(store MemoryStore, walkerID, walkID string, summary string) {
	store.SetNS(NamespaceEpisodic, walkerID, walkID, summary)
}

// updateInstruction stores a procedural memory (a prompt refinement).
func updateInstruction(store MemoryStore, walkerID, key string, instruction string) {
	store.SetNS(NamespaceProcedural, walkerID, key, instruction)
}

// --- Operator (engine/) ---

type EvalAction = engine.EvalAction

const (
	ActionContinue = engine.ActionContinue
	ActionEscalate = engine.ActionEscalate
	ActionDone     = engine.ActionDone
)

type Goal = engine.Goal
type SystemState = engine.SystemState
type Evaluation = engine.Evaluation
type WalkResult = engine.WalkResult
type Operator = engine.Operator
type OperatorObserver = engine.OperatorObserver
type OperatorOption = engine.OperatorOption

type ContainerStatus = engine.ContainerStatus

const (
	StatusPending   = engine.StatusPending
	StatusRunning   = engine.StatusRunning
	StatusSucceeded = engine.StatusSucceeded
	StatusFailed    = engine.StatusFailed
	StatusAborted   = engine.StatusAborted
)

type CircuitContainer = engine.CircuitContainer
type InMemoryContainer = engine.InMemoryContainer

var (
	WithMaxIterations    = engine.WithMaxIterations
	WithOperatorObserver = engine.WithOperatorObserver
	WithWalkObserver     = engine.WithWalkObserver
	NewInMemoryContainer = engine.NewInMemoryContainer
	RunOperator          = engine.RunOperator
)

// --- FanOut (engine/) ---

type ListArtifact = engine.ListArtifact

// --- FindingParallel (engine/) ---

const ArtifactStoreKey = engine.ArtifactStoreKey

type ArtifactStore = engine.ArtifactStore
type ParallelEnforcerConfig = engine.ParallelEnforcerConfig

var RunWithEnforcer = engine.RunWithEnforcer

// --- Team (engine/) ---

type Team = engine.Team

// --- Thermal (engine/) ---

var WithThermalBudget = engine.WithThermalBudget

// --- RunRecord (engine/) ---

type RunRecord = engine.RunRecord

var (
	SaveRunRecord = engine.SaveRunRecord
	LoadRunRecord = engine.LoadRunRecord
)

// --- MediatorDelegate (engine/) ---

type PromptRelayer = engine.PromptRelayer
type PromptRelayContext = engine.PromptRelayContext

const ContextKeyPromptRelayer = engine.ContextKeyPromptRelayer
