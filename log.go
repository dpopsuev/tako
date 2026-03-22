package framework

// Aliases to core/ package.

import "github.com/dpopsuev/origami/core"

const (
	LogComponentWalk      = core.LogComponentWalk
	LogComponentDSL       = core.LogComponentDSL
	LogComponentCalibrate = core.LogComponentCalibrate
	LogComponentBatch     = core.LogComponentBatch
	LogComponentTransform = core.LogComponentTransform
)

const (
	LogNodeEnter        = core.LogNodeEnter
	LogNodeExit         = core.LogNodeExit
	LogEdgeTaken        = core.LogEdgeTaken
	LogEdgeNoMatch      = core.LogEdgeNoMatch
	LogLoopIncremented  = core.LogLoopIncremented
	LogWalkComplete     = core.LogWalkComplete
	LogWalkError        = core.LogWalkError
	LogDelegateStart    = core.LogDelegateStart
	LogDelegateComplete = core.LogDelegateComplete

	LogOverlayMerge         = core.LogOverlayMerge
	LogOverlayMergeComplete = core.LogOverlayMergeComplete
	LogSubCircuitLoaded     = core.LogSubCircuitLoaded

	LogRunStart       = core.LogRunStart
	LogCaseComplete   = core.LogCaseComplete
	LogAllCasesFailed = core.LogAllCasesFailed
)

const (
	LogKeyComponent = core.LogKeyComponent
	LogKeyNode      = core.LogKeyNode
	LogKeyEdge      = core.LogKeyEdge
	LogKeyFrom      = core.LogKeyFrom
	LogKeyTo        = core.LogKeyTo
	LogKeyWalker    = core.LogKeyWalker
	LogKeyElapsed   = core.LogKeyElapsed
	LogKeyLoop      = core.LogKeyLoop
	LogKeyShortcut  = core.LogKeyShortcut
	LogKeyCount     = core.LogKeyCount
	LogKeyError     = core.LogKeyError
	LogKeyCaseID    = core.LogKeyCaseID
	LogKeyCircuit   = core.LogKeyCircuit
)
