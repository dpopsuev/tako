package render

import "sync"

// StubCanvas is a headless canvas — accepts damage, renders to buffer.
type StubCanvas struct {
	mu      sync.Mutex
	root    *Node
	damaged []DamageRegion
	buf     []byte
}

var _ Canvas = (*StubCanvas)(nil)

func (c *StubCanvas) Mount(node *Node) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.root = node
}

func (c *StubCanvas) Damage(region DamageRegion) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.damaged = append(c.damaged, region)
}

func (c *StubCanvas) Render() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.root == nil {
		return nil
	}
	c.buf = c.root.Data
	c.damaged = nil
	return c.buf
}

// DamageCount returns how many damage notifications were received.
func (c *StubCanvas) DamageCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.damaged)
}
