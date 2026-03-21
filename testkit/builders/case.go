package builders

import framework "github.com/dpopsuev/origami"

// BatchCaseBuilder constructs a framework.BatchCase incrementally for tests.
type BatchCaseBuilder struct {
	bc framework.BatchCase
}

// NewBatchCase creates a new BatchCaseBuilder with the given case ID.
func NewBatchCase(id string) *BatchCaseBuilder {
	return &BatchCaseBuilder{
		bc: framework.BatchCase{
			ID:      id,
			Context: make(map[string]any),
		},
	}
}

// WithInput sets a context key-value pair on the batch case.
func (b *BatchCaseBuilder) WithInput(key string, val any) *BatchCaseBuilder {
	b.bc.Context[key] = val
	return b
}

// WithExpected sets an expected value in the context under the "expected" sub-map.
// This follows the convention of storing ground truth under context["expected"].
func (b *BatchCaseBuilder) WithExpected(key string, val any) *BatchCaseBuilder {
	expected, ok := b.bc.Context["expected"].(map[string]any)
	if !ok {
		expected = make(map[string]any)
		b.bc.Context["expected"] = expected
	}
	expected[key] = val
	return b
}

// WithComponent adds a component to the batch case.
func (b *BatchCaseBuilder) WithComponent(c *framework.Component) *BatchCaseBuilder {
	b.bc.Components = append(b.bc.Components, c)
	return b
}

// Build returns the constructed BatchCase.
func (b *BatchCaseBuilder) Build() framework.BatchCase {
	return b.bc
}
