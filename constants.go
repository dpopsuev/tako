package framework

// Aliases to core/ package.

import "github.com/dpopsuev/origami/core"

const (
	ContextKeyTraceID = core.ContextKeyTraceID
)

const (
	ExtraKeyCircuitType = core.ExtraKeyCircuitType
	ExtraKeyTraceID     = core.ExtraKeyTraceID
)

const (
	TraceMetaDelegation = core.TraceMetaDelegation
	TraceMetaSource     = core.TraceMetaSource
)

const (
	ProtoKeySessionID     = core.ProtoKeySessionID
	ProtoKeyDone          = core.ProtoKeyDone
	ProtoKeyAvailable     = core.ProtoKeyAvailable
	ProtoKeyStep          = core.ProtoKeyStep
	ProtoKeyDispatchID    = core.ProtoKeyDispatchID
	ProtoKeyPromptContent = core.ProtoKeyPromptContent
	ProtoKeyCaseID        = core.ProtoKeyCaseID
	ProtoKeyArtifactPath  = core.ProtoKeyArtifactPath
	ProtoKeyFields        = core.ProtoKeyFields
	ProtoKeyExtra         = core.ProtoKeyExtra
	ProtoKeyError         = core.ProtoKeyError
	ProtoKeyStatus        = core.ProtoKeyStatus
	ProtoKeyStructured    = core.ProtoKeyStructured
	ProtoKeyTimeoutMS     = core.ProtoKeyTimeoutMS
)
