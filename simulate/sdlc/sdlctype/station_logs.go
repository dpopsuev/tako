package sdlctype

import "github.com/dpopsuev/origami/engine/trace"

// Station log type names.
const (
	stationLogTypeScan  = "scan"
	stationLogTypeFix   = "fix"
	stationLogTypeBuild = "build"
	stationLogTypeTest  = "test"
)

// ScanStationLog records scan instrument internals for FlightRecorder.
type ScanStationLog struct {
	FindingsCount int
	Categories    map[string]int // rule -> count
}

// StationLogType implements trace.StationLogger.
func (s *ScanStationLog) StationLogType() string { return stationLogTypeScan }

// FixStationLog records fix instrument internals for FlightRecorder.
type FixStationLog struct {
	PromptLen     int
	ResponseLen   int
	FilesModified []string
	ParsedChanges int
	DryRun        bool
}

// StationLogType implements trace.StationLogger.
func (f *FixStationLog) StationLogType() string { return stationLogTypeFix }

// BuildStationLog records build instrument internals for FlightRecorder.
type BuildStationLog struct {
	Pass          bool
	OutputSnippet string // first 500 chars
	DurationMs    int64
}

// StationLogType implements trace.StationLogger.
func (b *BuildStationLog) StationLogType() string { return stationLogTypeBuild }

// TestStationLog records test instrument internals for FlightRecorder.
type TestStationLog struct {
	Pass          bool
	Total         int
	Failed        int
	OutputSnippet string
	DurationMs    int64
}

// StationLogType implements trace.StationLogger.
func (t *TestStationLog) StationLogType() string { return stationLogTypeTest }

// Compile-time interface checks.
var (
	_ trace.StationLogger = (*ScanStationLog)(nil)
	_ trace.StationLogger = (*FixStationLog)(nil)
	_ trace.StationLogger = (*BuildStationLog)(nil)
	_ trace.StationLogger = (*TestStationLog)(nil)
)
