package oculus

import (
	"context"

	"github.com/dpopsuev/oculus/arch"
	"github.com/dpopsuev/oculus/port"
)

// NopStore implements port.Store with no persistence — every scan is fresh.
type NopStore struct{}

// ReportStore
func (n *NopStore) GetReport(_ context.Context, _, _ string) (*arch.ContextReport, bool, error) {
	return nil, false, nil
}
func (n *NopStore) PutReport(_ context.Context, _, _ string, _ *arch.ContextReport) error {
	return nil
}
func (n *NopStore) Invalidate(_ context.Context, _ string) error { return nil }

// HistoryStore
func (n *NopStore) RecordScan(_ context.Context, _, _, _ string, _ *arch.ContextReport) error {
	return nil
}
func (n *NopStore) ListHistory(_ context.Context, _ string, _ int) ([]port.HistoryEntry, error) {
	return nil, nil
}
func (n *NopStore) GetHistoryReport(_ context.Context, _ string, _ int) (*arch.ContextReport, error) {
	return nil, nil
}

// GitResolver
func (n *NopStore) ResolveHEAD(_ string) string {
	return ""
}

func (n *NopStore) ResolveBranch(_, _ string) (string, error) {
	return "", nil
}

// ProjectStore
func (n *NopStore) ListProjects(_ context.Context) ([]port.ProjectInfo, error) { return nil, nil }
func (n *NopStore) UpsertProject(_ context.Context, _ port.ProjectInfo) error  { return nil }

// ComponentStore
func (n *NopStore) PutComponentMeta(_ context.Context, _, _ string, _ []port.ComponentMeta) error {
	return nil
}
func (n *NopStore) ListComponentMeta(_ context.Context, _, _ string) ([]port.ComponentMeta, error) {
	return nil, nil
}
func (n *NopStore) SearchComponents(_ context.Context, _, _, _ string) ([]port.ComponentMeta, error) {
	return nil, nil
}

// DesiredStateStore
func (n *NopStore) GetDesiredState(_ context.Context, _ string) (*port.DesiredState, error) {
	return nil, nil
}
func (n *NopStore) PutDesiredState(_ context.Context, _ string, _ *port.DesiredState) error {
	return nil
}

// Close
func (n *NopStore) Close() error { return nil }
