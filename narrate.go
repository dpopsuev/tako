package framework

// Category: Processing & Support — thin wrappers delegating to engine/.
// These remain unexported to preserve the original API surface.

import (
	"time"

	"github.com/dpopsuev/origami/engine"
)

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
