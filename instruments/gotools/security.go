package gotools

import (
	"context"
	"sync"

	"github.com/dpopsuev/origami/engine"
	"github.com/dpopsuev/origami/engine/trace"
	"github.com/dpopsuev/origami/simulate/sdlc/sdlctype"
)

// SecurityScanTransformer runs `govulncheck ./...` on the target repository.
type SecurityScanTransformer struct {
	repoPath string

	mu             sync.Mutex
	lastStationLog trace.StationLogger
}

// NewSecurityScanTransformer creates a security scan transformer.
func NewSecurityScanTransformer(repoPath string) *SecurityScanTransformer {
	return &SecurityScanTransformer{repoPath: repoPath}
}

// Name implements engine.Instrument.
func (s *SecurityScanTransformer) Name() string { return "security-scan" }

// LastStationLog implements engine.StationLoggable.
func (s *SecurityScanTransformer) LastStationLog() trace.StationLogger {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastStationLog
}

// Transform implements engine.Instrument.
func (s *SecurityScanTransformer) Transform(ctx context.Context, _ *engine.InstrumentContext) (any, error) {
	r := runCommand(ctx, s.repoPath, "govulncheck", "./...")
	s.mu.Lock()
	s.lastStationLog = buildStationLog(r)
	s.mu.Unlock()

	return &sdlctype.SecurityScanResult{
		Clean: r.pass,
	}, nil
}

var _ engine.Instrument = (*SecurityScanTransformer)(nil)
