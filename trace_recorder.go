package framework

// Category: Processing & Support — aliases to engine/ package.

import "github.com/dpopsuev/origami/engine"

type TraceLevel = engine.TraceLevel

const (
	LevelInfo  = engine.LevelInfo
	LevelDebug = engine.LevelDebug
	LevelTrace = engine.LevelTrace
)

type TraceEvent = engine.TraceEvent
type TraceRecorder = engine.TraceRecorder

var NewTraceRecorder = engine.NewTraceRecorder
