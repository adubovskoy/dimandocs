package embedding

import "context"

// Service defines the interface for embedding generation
type Service interface {
	// Embed generates embeddings for a single text
	Embed(ctx context.Context, text string) ([]float32, error)

	// EmbedBatch generates embeddings for multiple texts
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)

	// Dimension returns the embedding dimension
	Dimension() int
}
