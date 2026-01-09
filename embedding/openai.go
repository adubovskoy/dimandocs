package embedding

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
)

const (
	// DefaultModel is the default embedding model
	DefaultModel = openai.LargeEmbedding3
	// DefaultDimension is the dimension for text-embedding-3-large
	DefaultDimension = 3072
	// MaxBatchSize is the maximum number of texts per API call
	MaxBatchSize = 2048
	// MaxRetries is the maximum number of retries for rate-limited requests
	MaxRetries = 5
	// InitialBackoff is the initial backoff duration
	InitialBackoff = 10 * time.Second
	// MaxBackoff is the maximum backoff duration
	MaxBackoff = 120 * time.Second
)

// OpenAIService implements Service using OpenAI API
type OpenAIService struct {
	client    *openai.Client
	model     openai.EmbeddingModel
	dimension int
}

// OpenAIConfig holds configuration for OpenAI embedding service
type OpenAIConfig struct {
	APIKey    string
	Model     string
	Dimension int
	BaseURL   string // Optional: for proxies or compatible APIs
}

// NewOpenAIService creates a new OpenAI embedding service
func NewOpenAIService(cfg OpenAIConfig) (*OpenAIService, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required")
	}

	config := openai.DefaultConfig(cfg.APIKey)
	if cfg.BaseURL != "" {
		config.BaseURL = cfg.BaseURL
	}

	client := openai.NewClientWithConfig(config)

	model := DefaultModel
	if cfg.Model != "" {
		model = openai.EmbeddingModel(cfg.Model)
	}

	dimension := DefaultDimension
	if cfg.Dimension > 0 {
		dimension = cfg.Dimension
	}

	return &OpenAIService{
		client:    client,
		model:     model,
		dimension: dimension,
	}, nil
}

// Embed generates embeddings for a single text
func (s *OpenAIService) Embed(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := s.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}
	return embeddings[0], nil
}

// EmbedBatch generates embeddings for multiple texts with retry logic
func (s *OpenAIService) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	var allEmbeddings [][]float32

	// Process in batches
	for i := 0; i < len(texts); i += MaxBatchSize {
		end := i + MaxBatchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[i:end]

		req := openai.EmbeddingRequest{
			Input:      batch,
			Model:      s.model,
			Dimensions: s.dimension,
		}

		// Retry with exponential backoff
		var resp openai.EmbeddingResponse
		var err error
		backoff := InitialBackoff

		for retry := 0; retry <= MaxRetries; retry++ {
			resp, err = s.client.CreateEmbeddings(ctx, req)
			if err == nil {
				break
			}

			// Check if it's a rate limit error (429)
			if isRateLimitError(err) && retry < MaxRetries {
				log.Printf("Rate limit hit, retrying in %v (attempt %d/%d)", backoff, retry+1, MaxRetries)
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(backoff):
				}
				// Exponential backoff with cap
				backoff *= 2
				if backoff > MaxBackoff {
					backoff = MaxBackoff
				}
				continue
			}

			return nil, fmt.Errorf("failed to create embeddings: %w", err)
		}

		for _, data := range resp.Data {
			allEmbeddings = append(allEmbeddings, data.Embedding)
		}
	}

	return allEmbeddings, nil
}

// isRateLimitError checks if the error is a rate limit (429) or quota error
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "429") ||
		strings.Contains(errStr, "rate") ||
		strings.Contains(errStr, "quota")
}

// Dimension returns the embedding dimension
func (s *OpenAIService) Dimension() int {
	return s.dimension
}
