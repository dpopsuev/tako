package contextual

import "sort"

// StubAssembler is an in-memory assembler that returns blocks within budget.
type StubAssembler struct {
	blocks []Block
}

var _ Assembler = (*StubAssembler)(nil)

func (s *StubAssembler) Add(block Block) {
	s.blocks = append(s.blocks, block)
}

func (s *StubAssembler) Assemble(budget int) []Block {
	sort.Slice(s.blocks, func(i, j int) bool {
		return s.blocks[i].Weight > s.blocks[j].Weight
	})
	out := make([]Block, 0, len(s.blocks))
	used := 0
	for _, b := range s.blocks {
		size := len(b.Content)
		if used+size > budget {
			continue
		}
		out = append(out, b)
		used += size
	}
	return out
}

func (s *StubAssembler) Reset() {
	s.blocks = nil
}
