package def

// Log component names — used as slog "component" field values.
const (
	LogComponentDSL = "dsl"
)

// Log event names — used as slog msg values.
const (
	LogOverlayMerge         = "overlay merge"
	LogOverlayMergeComplete = "overlay merge complete"
	LogSubCircuitLoaded     = "sub-circuit loaded"
	LogSubCircuitSkipped    = "skipping sub-circuit with unresolved import"
)

// Log field keys — used as slog attribute keys.
const (
	LogKeyComponent    = "component"
	LogKeyCircuit      = "circuit"
	LogKeyImport       = "import"
	LogKeyNodes        = "nodes"
	LogKeyBase         = "base"
	LogKeyBaseNodes    = "base_nodes"
	LogKeyOverlayNodes = "overlay_nodes"
	LogKeyOverlayEdges = "overlay_edges"
	LogKeyMergedNodes  = "merged_nodes"
	LogKeyMergedEdges  = "merged_edges"
	LogKeyStart        = "start"
	LogKeyDone         = "done"
)
