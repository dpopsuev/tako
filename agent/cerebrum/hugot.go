package cerebrum

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/knights-analytics/hugot"
	"github.com/knights-analytics/hugot/pipelines"
)

const (
	hugotModel   = "sentence-transformers/all-MiniLM-L6-v2"
	hugotPipeline = "tako-embedder"
)

type HugotEmbedder struct {
	session  *hugot.Session
	pipeline *pipelines.FeatureExtractionPipeline
	mu       sync.Mutex
}

func NewHugotEmbedder() (*HugotEmbedder, error) {
	session, err := hugot.NewGoSession()
	if err != nil {
		return nil, fmt.Errorf("hugot session: %w", err)
	}

	modelDir := hugotModelDir()
	opts := hugot.NewDownloadOptions()
	opts.OnnxFilePath = "onnx/model.onnx"
	modelPath, err := hugot.DownloadModel(hugotModel, modelDir, opts)
	if err != nil {
		session.Destroy()
		return nil, fmt.Errorf("hugot download %s: %w", hugotModel, err)
	}

	pipeline, err := hugot.NewPipeline(session, hugot.FeatureExtractionConfig{
		ModelPath: modelPath,
		Name:      hugotPipeline,
	})
	if err != nil {
		session.Destroy()
		return nil, fmt.Errorf("hugot pipeline: %w", err)
	}

	return &HugotEmbedder{
		session:  session,
		pipeline: pipeline,
	}, nil
}

func (h *HugotEmbedder) Embed(_ context.Context, text string) ([]float64, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	result, err := h.pipeline.RunPipeline([]string{text})
	if err != nil {
		return nil, fmt.Errorf("hugot embed: %w", err)
	}

	if len(result.Embeddings) == 0 || len(result.Embeddings[0]) == 0 {
		return nil, fmt.Errorf("hugot: empty embedding result")
	}

	embedding := make([]float64, len(result.Embeddings[0]))
	for i, v := range result.Embeddings[0] {
		embedding[i] = float64(v)
	}
	return embedding, nil
}

func (h *HugotEmbedder) Close() error {
	if h.session != nil {
		return h.session.Destroy()
	}
	return nil
}

func hugotModelDir() string {
	if d := os.Getenv("TAKO_MODEL_DIR"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "tako", "models")
}

var _ Embedder = (*HugotEmbedder)(nil)
