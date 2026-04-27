package render

// Renderable is implemented by anything that can project itself onto a Canvas.
type Renderable interface {
	Render() []byte
}

// Node is a tree element in the Canvas (i3/Wayland prior art).
type Node struct {
	ID       string
	Type     string
	Data     []byte
	Children []*Node
}

// DamageRegion marks a Canvas subtree that needs re-rendering.
type DamageRegion struct {
	NodeID string
}

// Canvas is the shared surface for Operator and Avatar.
type Canvas interface {
	Mount(node *Node)
	Damage(region DamageRegion)
	Render() []byte
}
