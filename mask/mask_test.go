package mask

import (
	"context"
	"testing"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/origami/agentport"
)

type stubMaskNode struct {
	name    string
	called  bool
	element agentport.Element
}

func (n *stubMaskNode) Name() string            { return n.name }
func (n *stubMaskNode) ElementAffinity() agentport.Element { return n.element }
func (n *stubMaskNode) Process(ctx context.Context, nc circuit.NodeContext) (circuit.Artifact, error) {
	n.called = true
	return &stubMaskArtifact{meta: nc.Meta}, nil
}

type stubMaskArtifact struct {
	meta map[string]any
}

func (a *stubMaskArtifact) Type() string       { return "stub" }
func (a *stubMaskArtifact) Confidence() float64 { return 1.0 }
func (a *stubMaskArtifact) Raw() any            { return a.meta }

func TestEquip_Valid(t *testing.T) {
	node := &stubMaskNode{name: "recall"}
	m := NewRecallMask()

	mn, err := Equip(node, m)
	if err != nil {
		t.Fatalf("Equip: %v", err)
	}
	if mn.Name() != "recall" {
		t.Errorf("Name() = %q, want %q", mn.Name(), "recall")
	}
	if len(mn.Masks) != 1 {
		t.Errorf("len(Masks) = %d, want 1", len(mn.Masks))
	}
}

func TestEquip_InvalidNode(t *testing.T) {
	node := &stubMaskNode{name: "investigate"}
	m := NewRecallMask()

	_, err := Equip(node, m)
	if err == nil {
		t.Fatal("expected error for invalid node")
	}
}

func TestMaskedNode_Process_InjectsMeta(t *testing.T) {
	node := &stubMaskNode{name: "recall"}
	m := NewRecallMask()

	mn, err := Equip(node, m)
	if err != nil {
		t.Fatalf("Equip: %v", err)
	}

	nc := circuit.NodeContext{Meta: make(map[string]any)}
	artifact, err := mn.Process(context.Background(), nc)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if !node.called {
		t.Error("inner node was not called")
	}

	meta := artifact.Raw().(map[string]any)
	if meta["prior_rca_available"] != true {
		t.Error("recall mask did not inject prior_rca_available")
	}
}

func TestMaskedNode_Process_NilMetaHandled(t *testing.T) {
	node := &stubMaskNode{name: "recall"}
	m := NewRecallMask()

	mn, err := Equip(node, m)
	if err != nil {
		t.Fatalf("Equip: %v", err)
	}

	nc := circuit.NodeContext{}
	artifact, err := mn.Process(context.Background(), nc)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	meta := artifact.Raw().(map[string]any)
	if meta["prior_rca_available"] != true {
		t.Error("mask should handle nil Meta gracefully")
	}
}

type orderTrackingMask struct {
	name       string
	validNodes []string
	order      *[]string
}

func (m *orderTrackingMask) Name() string        { return m.name }
func (m *orderTrackingMask) Description() string  { return "order tracker" }
func (m *orderTrackingMask) ValidNodes() []string { return m.validNodes }
func (m *orderTrackingMask) Wrap(next NodeProcessor) NodeProcessor {
	return func(ctx context.Context, nc circuit.NodeContext) (circuit.Artifact, error) {
		*m.order = append(*m.order, m.name+":pre")
		artifact, err := next(ctx, nc)
		*m.order = append(*m.order, m.name+":post")
		return artifact, err
	}
}

func TestMiddlewareChain_Ordering(t *testing.T) {
	var order []string
	node := &stubMaskNode{name: "recall"}
	maskA := &orderTrackingMask{name: "A", validNodes: []string{"recall"}, order: &order}
	maskB := &orderTrackingMask{name: "B", validNodes: []string{"recall"}, order: &order}

	mn, err := EquipMany(node, maskA, maskB)
	if err != nil {
		t.Fatalf("EquipMany: %v", err)
	}

	nc := circuit.NodeContext{Meta: make(map[string]any)}
	if _, err := mn.Process(context.Background(), nc); err != nil {
		t.Fatalf("Process: %v", err)
	}

	expected := []string{"A:pre", "B:pre", "B:post", "A:post"}
	if len(order) != len(expected) {
		t.Fatalf("order = %v, want %v", order, expected)
	}
	for i, step := range expected {
		if order[i] != step {
			t.Errorf("order[%d] = %q, want %q", i, order[i], step)
		}
	}
}

func TestEquipMany_PartialFailure(t *testing.T) {
	node := &stubMaskNode{name: "recall"}
	validMask := NewRecallMask()
	invalidMask := NewForgeMask()

	_, err := EquipMany(node, validMask, invalidMask)
	if err == nil {
		t.Fatal("expected error when second mask is invalid for node")
	}
}

func TestEquip_StackOnMaskedNode(t *testing.T) {
	var order []string
	node := &stubMaskNode{name: "recall"}
	maskA := &orderTrackingMask{name: "A", validNodes: []string{"recall"}, order: &order}
	maskB := &orderTrackingMask{name: "B", validNodes: []string{"recall"}, order: &order}

	mn, err := Equip(node, maskA)
	if err != nil {
		t.Fatalf("first Equip: %v", err)
	}
	mn, err = Equip(mn, maskB)
	if err != nil {
		t.Fatalf("second Equip: %v", err)
	}

	if len(mn.Masks) != 2 {
		t.Errorf("len(Masks) = %d, want 2", len(mn.Masks))
	}
}

func TestDefaultThesisMasks_Registry(t *testing.T) {
	reg := DefaultThesisMasks()
	if len(reg) != 4 {
		t.Errorf("len(DefaultThesisMasks) = %d, want 4", len(reg))
	}

	expected := []string{"mask-of-recall", "mask-of-the-forge", "mask-of-correlation", "mask-of-judgment"}
	for _, name := range expected {
		if _, ok := reg[name]; !ok {
			t.Errorf("missing mask %q in registry", name)
		}
	}
}

func TestAllThesisMasks_ValidNodes(t *testing.T) {
	cases := []struct {
		mask    Mask
		node    string
		wantErr bool
	}{
		{NewRecallMask(), "recall", false},
		{NewRecallMask(), "triage", true},
		{NewForgeMask(), "investigate", false},
		{NewForgeMask(), "recall", true},
		{NewCorrelationMask(), "correlate", false},
		{NewCorrelationMask(), "review", true},
		{NewJudgmentMask(), "review", false},
		{NewJudgmentMask(), "report", true},
	}
	for _, tc := range cases {
		node := &stubMaskNode{name: tc.node}
		_, err := Equip(node, tc.mask)
		if (err != nil) != tc.wantErr {
			t.Errorf("Equip(%s, %s): err=%v, wantErr=%v", tc.mask.Name(), tc.node, err, tc.wantErr)
		}
	}
}

func TestMaskedNode_ElementAffinity(t *testing.T) {
	node := &stubMaskNode{name: "recall", element: agentport.ElementFire}
	mn, err := Equip(node, NewRecallMask())
	if err != nil {
		t.Fatalf("Equip: %v", err)
	}
	if mn.ElementAffinity() != agentport.ElementFire {
		t.Errorf("ElementAffinity() = %q, want %q", mn.ElementAffinity(), agentport.ElementFire)
	}
}
