package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

const (
	// DefaultVoyageURL is the default Voyage AI API endpoint
	DefaultVoyageURL = "https://api.voyageai.com/v1/embeddings"
	// DefaultVoyageModel is the default embedding model for Voyage AI
	DefaultVoyageModel = "voyage-3"
	// VoyageTimeout is the timeout for Voyage AI requests
	VoyageTimeout = 60 * time.Second
	// VoyageMaxBatchSize is the maximum batch size for Voyage AI
	VoyageMaxBatchSize = 128
	// VoyageMaxRetries is the maximum number of retries
	VoyageMaxRetries = 5
)

// VoyageService implements Service using Voyage AI API
type VoyageService struct {
	apiKey    string
	baseURL   string
	model     string
	dimension int
	client    *http.Client
}

// VoyageConfig holds configuration for Voyage AI embedding service
type VoyageConfig struct {
	APIKey  string
	BaseURL string // Default: https://api.voyageai.com/v1/embeddings
	Model   string // Default: voyage-3
}

// voyageRequest represents the request body for Voyage AI embeddings API
type voyageRequest struct {
	Input     []string `json:"input"`
	Model     string   `json:"model"`
	InputType string   `json:"input_type,omitempty"` // "document" or "query"
}

// voyageResponse represents the response from Voyage AI embeddings API
type voyageResponse struct {
	Data  []voyageEmbedding `json:"data"`
	Usage voyageUsage       `json:"usage"`
}

type voyageEmbedding struct {
	Embedding []float64 `json:"embedding"`
	Index     int       `json:"index"`
}

type voyageUsage struct {
	TotalTokens int `json:"total_tokens"`
}

// NewVoyageService creates a new Voyage AI embedding service
func NewVoyageService(cfg VoyageConfig) (*VoyageService, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("Voyage AI API key is required")
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = DefaultVoyageURL
	}

	model := cfg.Model
	if model == "" {
		model = DefaultVoyageModel
	}

	// Determine dimension based on model
	dimension := 1024 // default for voyage-3
	switch model {
	case "voyage-3", "voyage-code-3":
		dimension = 1024
	case "voyage-3-lite":
		dimension = 512
	case "voyage-large-2", "voyage-2":
		dimension = 1536
	case "voyage-code-2":
		dimension = 1536
	}

	return &VoyageService{
		apiKey:    cfg.APIKey,
		baseURL:   baseURL,
		model:     model,
		dimension: dimension,
		client: &http.Client{
			Timeout: VoyageTimeout,
		},
	}, nil
}

// Embed generates embeddings for a single text
func (s *VoyageService) Embed(ctx context.Context, text string) ([]float32, error) {
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
func (s *VoyageService) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	var allEmbeddings [][]float32

	// Process in batches
	for i := 0; i < len(texts); i += VoyageMaxBatchSize {
		end := i + VoyageMaxBatchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[i:end]

		embeddings, err := s.embedBatchWithRetry(ctx, batch)
		if err != nil {
			return nil, err
		}
		allEmbeddings = append(allEmbeddings, embeddings...)
	}

	return allEmbeddings, nil
}

func (s *VoyageService) embedBatchWithRetry(ctx context.Context, texts []string) ([][]float32, error) {
	reqBody := voyageRequest{
		Input:     texts,
		Model:     s.model,
		InputType: "document",
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	var resp *http.Response
	backoff := 10 * time.Second

	for retry := 0; retry <= VoyageMaxRetries; retry++ {
		req, err := http.NewRequestWithContext(ctx, "POST", s.baseURL, bytes.NewBuffer(jsonBody))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+s.apiKey)

		resp, err = s.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to send request to Voyage AI: %w", err)
		}

		// Check for rate limit
		if resp.StatusCode == http.StatusTooManyRequests && retry < VoyageMaxRetries {
			resp.Body.Close()
			log.Printf("Voyage AI rate limit hit, retrying in %v (attempt %d/%d)", backoff, retry+1, VoyageMaxRetries)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= 2
			if backoff > 120*time.Second {
				backoff = 120 * time.Second
			}
			continue
		}

		break
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Voyage AI API error (status %d): %s", resp.StatusCode, string(body))
	}

	var voyageResp voyageResponse
	if err := json.NewDecoder(resp.Body).Decode(&voyageResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Sort by index and convert to float32
	embeddings := make([][]float32, len(voyageResp.Data))
	for _, data := range voyageResp.Data {
		embedding := make([]float32, len(data.Embedding))
		for j, v := range data.Embedding {
			embedding[j] = float32(v)
		}
		embeddings[data.Index] = embedding
	}

	return embeddings, nil
}

// Dimension returns the embedding dimension
func (s *VoyageService) Dimension() int {
	return s.dimension
}

// isVoyageRateLimitError checks if the error is a rate limit error
func isVoyageRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "429") ||
		strings.Contains(errStr, "rate") ||
		strings.Contains(errStr, "quota")
}
