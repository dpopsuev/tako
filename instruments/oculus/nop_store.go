package oculus

import (
	"context"

	"github.com/dpopsuev/oculus/arch"
	"github.com/dpopsuev/oculus/port"
)

// nopStore implements port.Store with no persistence — every scan is fresh.
type nopStore struct{}

// ReportStore
func (n *nopStore) GetReport(_ context.Context, _, _ string) (*arch.ContextReport, bool, error) {
	return nil, false, nil
}
func (n *nopStore) PutReport(_ context.Context, _, _ string, _ *arch.ContextReport) error {
	return nil
}
func (n *nopStore) Invalidate(_ context.Context, _ string) error { return nil }

// HistoryStore
func (n *nopStore) RecordScan(_ context.Context, _, _, _ string, _ *arch.ContextReport) error {
	return nil
}
func (n *nopStore) ListHistory(_ context.Context, _ string, _ int) ([]port.HistoryEntry, error) {
	return nil, nil
}
func (n *nopStore) GetHistoryReport(_ context.Context, _ string, _ int) (*arch.ContextReport, error) {
	return nil, nil
}

// GitResolver
func (n *nopStore) ResolveHEAD(_ string) string {
	return ""
}

func (n *nopStore) ResolveBranch(_, _ string) (string, error) {
	return "", nil
}

// ProjectStore
func (n *nopStore) ListProjects(_ context.Context) ([]port.ProjectInfo, error) { return nil, nil }
func (n *nopStore) UpsertProject(_ context.Context, _ port.ProjectInfo) error  { return nil }

// ComponentStore
func (n *nopStore) PutComponentMeta(_ context.Context, _, _ string, _ []port.ComponentMeta) error {
	return nil
}
func (n *nopStore) ListComponentMeta(_ context.Context, _, _ string) ([]port.ComponentMeta, error) {
	return nil, nil
}
func (n *nopStore) SearchComponents(_ context.Context, _, _, _ string) ([]port.ComponentMeta, error) {
	return nil, nil
}

// DesiredStateStore
func (n *nopStore) GetDesiredState(_ context.Context, _ string) (*port.DesiredState, error) {
	return nil, nil
}
func (n *nopStore) PutDesiredState(_ context.Context, _ string, _ *port.DesiredState) error {
	return nil
}

// Close
func (n *nopStore) Close() error { return nil }
