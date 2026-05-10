package cerebrum

import (
	"context"
	"os"
	"testing"
)

func TestHugotEmbedder_RealModel(t *testing.T) {
	if os.Getenv("TAKO_TEST_HUGOT") == "" {
		t.Skip("set TAKO_TEST_HUGOT=1 to test real model download")
	}

	embedder, err := NewHugotEmbedder()
	if err != nil {
		t.Fatalf("NewHugotEmbedder: %v", err)
	}
	defer embedder.Close()

	ctx := context.Background()

	hello, err := embedder.Embed(ctx, "hello world")
	if err != nil {
		t.Fatalf("embed hello: %v", err)
	}
	if len(hello) == 0 {
		t.Fatal("empty embedding")
	}
	t.Logf("dims=%d", len(hello))

	similar, err := embedder.Embed(ctx, "hi there")
	if err != nil {
		t.Fatalf("embed similar: %v", err)
	}

	different, err := embedder.Embed(ctx, "quantum mechanics equations")
	if err != nil {
		t.Fatalf("embed different: %v", err)
	}

	simScore := CosineSimilarity(hello, similar)
	diffScore := CosineSimilarity(hello, different)

	t.Logf("hello vs hi_there: %.4f", simScore)
	t.Logf("hello vs quantum:  %.4f", diffScore)

	if simScore <= diffScore {
		t.Errorf("similar text should score higher than different: sim=%.4f diff=%.4f", simScore, diffScore)
	}
	if simScore < 0.3 {
		t.Errorf("similar greetings should have higher cosine similarity than random, got %.4f", simScore)
	}
}
