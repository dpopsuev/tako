package cerebrum

import (
	"context"
	"crypto/sha256"
	"math"
)

type Embedder interface {
	Embed(ctx context.Context, text string) ([]float64, error)
}

// EXPERIMENTAL: StubEmbedder is hash-based — needs real embedding model for production (TSK-436)
type StubEmbedder struct {
	Dims int
}

func (e StubEmbedder) Embed(_ context.Context, text string) ([]float64, error) {
	dims := e.Dims
	if dims == 0 {
		dims = 64
	}
	hash := sha256.Sum256([]byte(text))
	vec := make([]float64, dims)
	for i := range vec {
		b := hash[i%len(hash)]
		vec[i] = (float64(b) - 128.0) / 128.0
	}
	var norm float64
	for _, v := range vec {
		norm += v * v
	}
	norm = math.Sqrt(norm)
	if norm > 0 {
		for i := range vec {
			vec[i] /= norm
		}
	}
	return vec, nil
}

func CosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return dot / denom
}

var _ Embedder = StubEmbedder{}
