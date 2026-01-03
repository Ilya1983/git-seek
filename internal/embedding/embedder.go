package embedding

// Embedder defines the interface for text embedding.
// Used for both indexing commits and embedding search queries.
type Embedder interface {
	// Embed generates a vector embedding for a single text.
	Embed(text string) ([]float32, error)

	// EmbedBatch generates embeddings for multiple texts efficiently.
	EmbedBatch(texts []string) ([][]float32, error)

	// Dimensions returns the size of the embedding vectors.
	Dimensions() int

	// Close releases any resources held by the embedder.
	Close() error
}
