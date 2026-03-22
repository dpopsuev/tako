package framework

// Category: DSL & Build — aliases to circuit/ and engine/ packages.

import (
	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/engine"
)

// Type aliases — definitions live in circuit/ sub-package.
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

// Registry and build types — aliases to engine/ package.
type NodeRegistry = engine.NodeRegistry
type EdgeFactory = engine.EdgeFactory
type ComponentLoader = engine.ComponentLoader
type GraphRegistries = engine.GraphRegistries

var BuildGraph = engine.BuildGraph
