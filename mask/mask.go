// Package mask provides detachable capability modifiers (masks) that wrap
// a Node's processing as middleware. Masks grant powers at specific nodes
// without changing the agent's core identity.
package mask

import (
	"context"
	"fmt"

	"github.com/dpopsuev/origami/circuit"
	"github.com/dpopsuev/bugle/element"
)

// NodeProcessor is the function signature for processing a node.
type NodeProcessor func(ctx context.Context, nc circuit.NodeContext) (circuit.Artifact, error)

// Mask is a detachable capability modifier that wraps a Node's processing.
// Masks grant powers at specific nodes without changing the agent's core identity.
type Mask interface {
	Name() string
	Description() string
	ValidNodes() []string
	Wrap(next NodeProcessor) NodeProcessor
}

// Registry holds available masks indexed by name.
type Registry map[string]Mask

// MaskedNode wraps a Node with one or more Masks applied as middleware.
type MaskedNode struct {
	Inner circuit.Node
	Masks []Mask
}

func (mn *MaskedNode) Name() string            { return mn.Inner.Name() }
func (mn *MaskedNode) ElementAffinity() element.Element { return mn.Inner.ElementAffinity() }

// Process executes the node with all masks applied as a middleware chain.
// First equipped = outermost wrapper: A.pre -> B.pre -> Node -> B.post -> A.post.
func (mn *MaskedNode) Process(ctx context.Context, nc circuit.NodeContext) (circuit.Artifact, error) {
	var processor NodeProcessor = mn.Inner.Process
	for i := len(mn.Masks) - 1; i >= 0; i-- {
		processor = mn.Masks[i].Wrap(processor)
	}
	return processor(ctx, nc)
}

// Equip adds a mask to a node. Returns error if the mask is not valid
// for this node's name. If the node is already a MaskedNode, the mask is
// appended to the existing chain.
func Equip(node circuit.Node, m Mask) (*MaskedNode, error) {
	if !isValidNode(node.Name(), m.ValidNodes()) {
		return nil, fmt.Errorf("mask %q cannot be equipped at node %q (valid: %v)",
			m.Name(), node.Name(), m.ValidNodes())
	}

	if mn, ok := node.(*MaskedNode); ok {
		mn.Masks = append(mn.Masks, m)
		return mn, nil
	}

	return &MaskedNode{Inner: node, Masks: []Mask{m}}, nil
}

// EquipMany adds multiple masks to a node. First mask = outermost wrapper.
func EquipMany(node circuit.Node, masks ...Mask) (*MaskedNode, error) {
	var result *MaskedNode
	for _, m := range masks {
		var err error
		if result == nil {
			result, err = Equip(node, m)
		} else {
			result, err = Equip(result, m)
		}
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func isValidNode(nodeName string, validNodes []string) bool {
	for _, vn := range validNodes {
		if vn == nodeName {
			return true
		}
	}
	return false
}

// --- Skeleton mask implementations for the Thesis circuit ---

type recallMask struct{}

func (m *recallMask) Name() string        { return "mask-of-recall" }
func (m *recallMask) Description() string  { return "Injects prior RCA database context" }
func (m *recallMask) ValidNodes() []string { return []string{"recall"} }
func (m *recallMask) Wrap(next NodeProcessor) NodeProcessor {
	return func(ctx context.Context, nc circuit.NodeContext) (circuit.Artifact, error) {
		if nc.Meta == nil {
			nc.Meta = make(map[string]any)
		}
		nc.Meta["prior_rca_available"] = true
		return next(ctx, nc)
	}
}

// NewRecallMask returns the Mask of Recall (valid at "recall" node).
func NewRecallMask() Mask { return &recallMask{} }

type forgeMask struct{}

func (m *forgeMask) Name() string        { return "mask-of-the-forge" }
func (m *forgeMask) Description() string  { return "Injects GND source context" }
func (m *forgeMask) ValidNodes() []string { return []string{"investigate"} }
func (m *forgeMask) Wrap(next NodeProcessor) NodeProcessor {
	return func(ctx context.Context, nc circuit.NodeContext) (circuit.Artifact, error) {
		if nc.Meta == nil {
			nc.Meta = make(map[string]any)
		}
		nc.Meta["gnd_sources_available"] = true
		return next(ctx, nc)
	}
}

// NewForgeMask returns the Mask of the Forge (valid at "investigate" node).
func NewForgeMask() Mask { return &forgeMask{} }

type correlationMask struct{}

func (m *correlationMask) Name() string        { return "mask-of-correlation" }
func (m *correlationMask) Description() string  { return "Enables cross-case pattern matching" }
func (m *correlationMask) ValidNodes() []string { return []string{"correlate"} }
func (m *correlationMask) Wrap(next NodeProcessor) NodeProcessor {
	return func(ctx context.Context, nc circuit.NodeContext) (circuit.Artifact, error) {
		if nc.Meta == nil {
			nc.Meta = make(map[string]any)
		}
		nc.Meta["cross_case_matching"] = true
		return next(ctx, nc)
	}
}

// NewCorrelationMask returns the Mask of Correlation (valid at "correlate" node).
func NewCorrelationMask() Mask { return &correlationMask{} }

type judgmentMask struct{}

func (m *judgmentMask) Name() string        { return "mask-of-judgment" }
func (m *judgmentMask) Description() string  { return "Grants authority to approve/reject/reassess" }
func (m *judgmentMask) ValidNodes() []string { return []string{"review"} }
func (m *judgmentMask) Wrap(next NodeProcessor) NodeProcessor {
	return func(ctx context.Context, nc circuit.NodeContext) (circuit.Artifact, error) {
		if nc.Meta == nil {
			nc.Meta = make(map[string]any)
		}
		nc.Meta["review_authority"] = true
		return next(ctx, nc)
	}
}

// NewJudgmentMask returns the Mask of Judgment (valid at "review" node).
func NewJudgmentMask() Mask { return &judgmentMask{} }

// DefaultThesisMasks returns the 4 Thesis circuit masks in a registry.
func DefaultThesisMasks() Registry {
	masks := []Mask{NewRecallMask(), NewForgeMask(), NewCorrelationMask(), NewJudgmentMask()}
	reg := make(Registry, len(masks))
	for _, m := range masks {
		reg[m.Name()] = m
	}
	return reg
}
