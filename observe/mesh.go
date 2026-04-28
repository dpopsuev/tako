package observe

import (
	"context"

	"github.com/dpopsuev/tako/ergograph"
	"github.com/dpopsuev/tako/memory"
	"go.opentelemetry.io/otel/trace"
)

type Mesh struct {
	inner  memory.Mesh
	pool   ergograph.Pool
	tracer trace.Tracer
	name   string
}

var _ memory.Mesh = (*Mesh)(nil)

func NewMesh(inner memory.Mesh, pool ergograph.Pool, tracer trace.Tracer, name string) *Mesh {
	return &Mesh{inner: inner, pool: pool, tracer: tracer, name: name}
}

func (m *Mesh) AddNode(node memory.KnowledgeNode) error {
	ctx, span := m.tracer.Start(context.Background(), "mesh.add_node")
	defer span.End()
	err := m.inner.AddNode(node)
	spanError(span, err)
	record(ctx, m.pool, "mesh.add_node", map[string]string{"mesh": m.name, "node": node.ID})
	return err
}

func (m *Mesh) AddEdge(edge memory.Edge) error {
	ctx, span := m.tracer.Start(context.Background(), "mesh.add_edge")
	defer span.End()
	err := m.inner.AddEdge(edge)
	spanError(span, err)
	record(ctx, m.pool, "mesh.add_edge", map[string]string{"mesh": m.name, "from": edge.From, "to": edge.To})
	return err
}

func (m *Mesh) Node(id string) (memory.KnowledgeNode, error) {
	_, span := m.tracer.Start(context.Background(), "mesh.node")
	defer span.End()
	n, err := m.inner.Node(id)
	spanError(span, err)
	return n, err
}

func (m *Mesh) Neighbors(id string) ([]memory.KnowledgeNode, error) {
	_, span := m.tracer.Start(context.Background(), "mesh.neighbors")
	defer span.End()
	n, err := m.inner.Neighbors(id)
	spanError(span, err)
	return n, err
}

func (m *Mesh) Walk(startID string, fn memory.WalkFunc) error {
	_, span := m.tracer.Start(context.Background(), "mesh.walk")
	defer span.End()
	err := m.inner.Walk(startID, fn)
	spanError(span, err)
	return err
}

func (m *Mesh) Nodes() []memory.KnowledgeNode {
	_, span := m.tracer.Start(context.Background(), "mesh.nodes")
	defer span.End()
	return m.inner.Nodes()
}

func (m *Mesh) Edges() []memory.Edge {
	_, span := m.tracer.Start(context.Background(), "mesh.edges")
	defer span.End()
	return m.inner.Edges()
}
