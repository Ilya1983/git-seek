package embedding

import (
	"fmt"

	"github.com/clems4ever/all-minilm-l6-v2-go/all_minilm_l6_v2"
)

const miniLMDimensions = 384

// MiniLMEmbedder implements Embedder using the all-MiniLM-L6-v2 model.
type MiniLMEmbedder struct {
	model *all_minilm_l6_v2.Model
}

// NewMiniLMEmbedder creates a new embedder using the MiniLM model.
// Requires ONNX Runtime to be installed on the system.
func NewMiniLMEmbedder() (*MiniLMEmbedder, error) {
	model, err := all_minilm_l6_v2.NewModel()
	if err != nil {
		return nil, fmt.Errorf("loading MiniLM model: %w", err)
	}
	return &MiniLMEmbedder{model: model}, nil
}

// Embed generates a vector embedding for a single text.
func (e *MiniLMEmbedder) Embed(text string) ([]float32, error) {
	return e.model.Compute(text, true)
}

// EmbedBatch generates embeddings for multiple texts efficiently.
func (e *MiniLMEmbedder) EmbedBatch(texts []string) ([][]float32, error) {
	return e.model.ComputeBatch(texts, true)
}

// Dimensions returns the size of the embedding vectors (384 for MiniLM).
func (e *MiniLMEmbedder) Dimensions() int {
	return miniLMDimensions
}

// Close releases any resources held by the embedder.
func (e *MiniLMEmbedder) Close() error {
	if e.model != nil {
		e.model.Close()
	}
	return nil
}
