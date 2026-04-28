// trace_reexport.go — type aliases for engine/trace sub-package.
// Consumers can import engine/trace directly or use these aliases
// for backward compatibility.
package engine

import "github.com/dpopsuev/tako/engine/trace"

// Trace types — definitions live in engine/trace.
type (
	TraceEvent        = trace.TraceEvent
	TraceLevel        = trace.TraceLevel
	TraceRecorder     = trace.TraceRecorder
	TraceCollector    = trace.TraceCollector
	RunRecord         = trace.RunRecord
	NarrationObserver = trace.NarrationObserver
	NarrationSink     = trace.NarrationSink
	Progress          = trace.Progress
)

// Narration option type.
type NarrationOption = trace.NarrationOption

// Trace constructors and options.
var (
	NewTraceRecorder      = trace.NewTraceRecorder
	NewLogObserver        = trace.NewLogObserver
	NewNarrationObserver  = trace.NewNarrationObserver
	SaveRunRecord         = trace.SaveRunRecord
	LoadRunRecord         = trace.LoadRunRecord
	FmtNarrateDuration    = trace.FmtNarrateDuration
	WithSink              = trace.WithSink
	WithVocabulary        = trace.WithVocabulary
	WithMilestoneInterval = trace.WithMilestoneInterval
	WithETA               = trace.WithETA
)

// Trace level constants.
const (
	LevelInfo  = trace.LevelInfo
	LevelDebug = trace.LevelDebug
	LevelTrace = trace.LevelTrace
)
