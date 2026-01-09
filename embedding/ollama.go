package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	// DefaultOllamaURL is the default Ollama API endpoint
	DefaultOllamaURL = "http://localhost:11434"
	// DefaultOllamaModel is the default embedding model for Ollama
	DefaultOllamaModel = "nomic-embed-text"
	// OllamaDimension is the dimension for nomic-embed-text
	OllamaDimension = 768
	// OllamaTimeout is the timeout for Ollama requests
	OllamaTimeout = 60 * time.Second
)

// OllamaService implements Service using Ollama API
type OllamaService struct {
	baseURL   string
	model     string
	dimension int
	client    *http.Client
}

// OllamaConfig holds configuration for Ollama embedding service
type OllamaConfig struct {
	BaseURL string // Default: http://localhost:11434
	Model   string // Default: nomic-embed-text
}

// ollamaRequest represents the request body for Ollama embeddings API
type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

// ollamaResponse represents the response from Ollama embeddings API
type ollamaResponse struct {
	Embedding []float64 `json:"embedding"`
}

// NewOllamaService creates a new Ollama embedding service
func NewOllamaService(cfg OllamaConfig) (*OllamaService, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = DefaultOllamaURL
	}

	model := cfg.Model
	if model == "" {
		model = DefaultOllamaModel
	}

	// Determine dimension based on model
	dimension := OllamaDimension
	switch model {
	case "nomic-embed-text":
		dimension = 768
	case "mxbai-embed-large":
		dimension = 1024
	case "all-minilm":
		dimension = 384
	}

	return &OllamaService{
		baseURL:   baseURL,
		model:     model,
		dimension: dimension,
		client: &http.Client{
			Timeout: OllamaTimeout,
		},
	}, nil
}

// Embed generates embeddings for a single text
func (s *OllamaService) Embed(ctx context.Context, text string) ([]float32, error) {
	reqBody := ollamaRequest{
		Model:  s.model,
		Prompt: text,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.baseURL+"/api/embeddings", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to Ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama API error (status %d): %s", resp.StatusCode, string(body))
	}

	var ollamaResp ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert float64 to float32
	embedding := make([]float32, len(ollamaResp.Embedding))
	for i, v := range ollamaResp.Embedding {
		embedding[i] = float32(v)
	}

	return embedding, nil
}

// EmbedBatch generates embeddings for multiple texts
// Ollama doesn't support batch embeddings, so we process one at a time
func (s *OllamaService) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	embeddings := make([][]float32, 0, len(texts))
	for _, text := range texts {
		embedding, err := s.Embed(ctx, text)
		if err != nil {
			return nil, err
		}
		embeddings = append(embeddings, embedding)
	}

	return embeddings, nil
}

// Dimension returns the embedding dimension
func (s *OllamaService) Dimension() int {
	return s.dimension
}
