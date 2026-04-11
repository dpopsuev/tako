package circuit

// Re-exports from circuit/def — maintains backward compatibility.
// New code should import circuit/def directly.

import "github.com/dpopsuev/origami/circuit/def"

// Type aliases for all definition types.
type (
	NodeName               = def.NodeName
	CircuitDef             = def.CircuitDef
	CalibrationContractDef = def.CalibrationContractDef
	CalibrationFieldDef    = def.CalibrationFieldDef
	PortDef                = def.PortDef
	WiringDef              = def.WiringDef
	ExtractorDef           = def.ExtractorDef
	WalkerDef              = def.WalkerDef
	ContextFilterDef       = def.ContextFilterDef
	ZoneDef                = def.ZoneDef
	OutputField            = def.OutputField
	NodeConfig             = def.NodeConfig
	NodeDef                = def.NodeDef
	CacheDef               = def.CacheDef
	EdgeDef                = def.EdgeDef
	AssetResolver          = def.AssetResolver
	ResolveOption          = def.ResolveOption
	StoreRegistry          = def.StoreRegistry
	StoreEngineFactory     = def.StoreEngineFactory
	StoreWiring            = def.StoreWiring
	StoreBinding           = def.StoreBinding
	StoreLifecycle         = def.StoreLifecycle
	StoreDeclaration       = def.StoreDeclaration
	StoreEngine            = def.StoreEngine
	StoreSchema            = def.StoreSchema
	StoreTableDef          = def.StoreTableDef
	StoreColumnDef         = def.StoreColumnDef
	StoreIndexDef          = def.StoreIndexDef
	SchemaProvider         = def.SchemaProvider
	Kind                   = def.Kind
	Envelope               = def.Envelope
	Metadata               = def.Metadata
	ScorecardDef           = def.ScorecardDef
	ScorecardMetric        = def.ScorecardMetric
	CostModelDef           = def.CostModelDef
	SocketDef              = def.SocketDef
	GivesDef               = def.GivesDef
	ComponentManifest      = def.ComponentManifest
	ParamDef               = def.ParamDef
	DispatchDef            = def.DispatchDef
	TopologyValidator      = def.TopologyValidator
	GraphShape             = def.GraphShape
	GraphNodeInfo          = def.GraphNodeInfo
	RichMapVocabulary      = def.RichMapVocabulary
	VocabEntry             = def.VocabEntry
	Vocabulary             = def.Vocabulary
	VocabularyFunc         = def.VocabularyFunc
	MapVocabulary          = def.MapVocabulary
	ChainVocabulary        = def.ChainVocabulary
	RichVocabulary         = def.RichVocabulary
	RichChainVocabulary    = def.RichChainVocabulary
	InstrumentManifest     = def.InstrumentManifest
	ActionDef              = def.ActionDef
	DispatchMode           = def.DispatchMode
)

// Kind constants.
const (
	KindSchematic      = def.KindSchematic
	KindComponent      = def.KindComponent
	KindBoard          = def.KindBoard
	KindCircuit        = def.KindCircuit
	KindStoreSchema    = def.KindStoreSchema
	KindScorecard      = def.KindScorecard
	KindScenario       = def.KindScenario
	KindArtifactSchema = def.KindArtifactSchema
	KindReportTemplate = def.KindReportTemplate
	KindVocabulary     = def.KindVocabulary
	KindHeuristicRules = def.KindHeuristicRules
	KindSourcePack     = def.KindSourcePack
	KindInstrument     = def.KindInstrument
	KindTuning         = def.KindTuning
	KindDataset        = def.KindDataset
	KindPrompt         = def.KindPrompt
)

// Dispatch mode constants.
const (
	DispatchCLI       = def.DispatchCLI
	DispatchMCP       = def.DispatchMCP
	DispatchContainer = def.DispatchContainer
	DispatchInproc    = def.DispatchInproc
)

// Merge strategy constants.
const (
	MergeAppend = def.MergeAppend
	MergeLatest = def.MergeLatest
	MergeCustom = def.MergeCustom
)

// Function re-exports.
var ( //nolint:dupl // intentional re-exports for backward compatibility
	LoadCircuit                 = def.LoadCircuit
	LoadCircuitWithOverlay      = def.LoadCircuitWithOverlay
	RegisterEmbeddedCircuit     = def.RegisterEmbeddedCircuit
	WithSearchDirs              = def.WithSearchDirs
	ResolveCircuitPath          = def.ResolveCircuitPath
	ClearEmbeddedCircuits       = def.ClearEmbeddedCircuits
	NewStoreRegistry            = def.NewStoreRegistry
	LoadStoreSchema             = def.LoadStoreSchema
	MergeStoreSchemas           = def.MergeStoreSchemas
	ParseKind                   = def.ParseKind
	ParseEnvelope               = def.ParseEnvelope
	LoadScorecardDef            = def.LoadScorecardDef
	MergeScorecardDefs          = def.MergeScorecardDefs
	RegisterScorecardVocab      = def.RegisterScorecardVocabulary
	LoadComponentManifest       = def.LoadComponentManifest
	InferTopology               = def.InferTopology
	LoadSubCircuitsFromFS       = def.LoadSubCircuitsFromFS
	Render                      = def.Render
	KnownKinds                  = def.KnownKinds
	DefaultTopologyValidator    = def.DefaultTopologyValidator
	RegisterTopologyValidator   = def.RegisterTopologyValidator
	NewMapVocabulary            = def.NewMapVocabulary
	NewRichMapVocabulary        = def.NewRichMapVocabulary
	NameWithCode                = def.NameWithCode
	RegisterScorecardVocabulary = def.RegisterScorecardVocabulary
	LoadInstrumentManifest      = def.LoadInstrumentManifest
	ParseInstrumentManifest     = def.ParseInstrumentManifest
)
