package contextual

// Block is the atomic unit of assembled context.
type Block struct {
	Role    string
	Content string
	Weight  int
}

// Assembler builds context from Blocks for an LLM call.
type Assembler interface {
	Add(block Block)
	Assemble(budget int) []Block
	Reset()
}
