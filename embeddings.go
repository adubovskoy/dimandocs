package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"

	"dimandocs/chunking"
	"dimandocs/embedding"
	"dimandocs/mcp"
	"dimandocs/vector"
)

// EmbeddingManager handles document embedding and vector search
type EmbeddingManager struct {
	store   *vector.SQLiteStore
	embed   embedding.Service
	enabled bool
}

// NewEmbeddingManager creates a new embedding manager
func NewEmbeddingManager(cfg EmbeddingsConfig) (*EmbeddingManager, error) {
	if !cfg.Enabled {
		return &EmbeddingManager{enabled: false}, nil
	}

	// Initialize vector store
	store := vector.NewSQLiteStore(cfg.DBPath)
	if err := store.Initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize vector store: %w", err)
	}

	// Initialize embedding service
	var embedService embedding.Service
	var err error

	switch cfg.Provider {
	case "openai", "":
		embedService, err = embedding.NewOpenAIService(embedding.OpenAIConfig{
			APIKey:  cfg.APIKey,
			Model:   cfg.Model,
			BaseURL: cfg.BaseURL,
		})
	case "ollama":
		embedService, err = embedding.NewOllamaService(embedding.OllamaConfig{
			BaseURL: cfg.BaseURL,
			Model:   cfg.Model,
		})
		log.Printf("Using Ollama embedding service (model: %s, dimension: %d)", cfg.Model, embedService.Dimension())
	case "voyage", "voyageai":
		embedService, err = embedding.NewVoyageService(embedding.VoyageConfig{
			APIKey:  cfg.APIKey,
			BaseURL: cfg.BaseURL,
			Model:   cfg.Model,
		})
		if err == nil {
			log.Printf("Using Voyage AI embedding service (model: %s, dimension: %d)", cfg.Model, embedService.Dimension())
		}
	default:
		return nil, fmt.Errorf("unsupported embedding provider: %s", cfg.Provider)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create embedding service: %w", err)
	}

	// Update vector store dimension based on embedding service
	store.SetDimension(embedService.Dimension())

	return &EmbeddingManager{
		store:   store,
		embed:   embedService,
		enabled: true,
	}, nil
}

// Close closes the embedding manager
func (m *EmbeddingManager) Close() error {
	if m.store != nil {
		return m.store.Close()
	}
	return nil
}

// IsEnabled returns whether embedding is enabled
func (m *EmbeddingManager) IsEnabled() bool {
	return m.enabled
}

// IndexDocument indexes a document by chunking and embedding
// If force is true, re-index even if the document hasn't changed
func (m *EmbeddingManager) IndexDocument(ctx context.Context, doc Document, force bool) error {
	if !m.enabled {
		return nil
	}

	// Calculate content hash
	hash := sha256.Sum256([]byte(doc.Content))
	contentHash := hex.EncodeToString(hash[:])

	// Check if document needs update (unless force is set)
	if !force {
		needsUpdate, err := m.store.NeedsUpdate(doc.RelPath, contentHash)
		if err != nil {
			return fmt.Errorf("failed to check if document needs update: %w", err)
		}

		if !needsUpdate {
			log.Printf("Document %s is up to date, skipping", doc.RelPath)
			return nil
		}
	}

	log.Printf("Indexing document: %s", doc.RelPath)

	// Upsert document record
	docID, err := m.store.UpsertDocument(doc.RelPath, doc.Title, contentHash)
	if err != nil {
		return fmt.Errorf("failed to upsert document: %w", err)
	}

	// Chunk the document
	chunks := chunking.ChunkMarkdown(doc.Content, chunking.DefaultOptions())
	if len(chunks) == 0 {
		log.Printf("No chunks generated for document %s", doc.RelPath)
		return nil
	}

	// Collect chunk texts for batch embedding
	chunkTexts := make([]string, len(chunks))
	for i, chunk := range chunks {
		// Prepend document title and section for better context
		contextText := doc.Title
		if chunk.SectionTitle != "" {
			contextText += " - " + chunk.SectionTitle
		}
		contextText += "\n\n" + chunk.Text
		chunkTexts[i] = contextText
	}

	// Generate embeddings in batch
	embeddings, err := m.embed.EmbedBatch(ctx, chunkTexts)
	if err != nil {
		return fmt.Errorf("failed to generate embeddings: %w", err)
	}

	// Create vector chunks
	vectorChunks := make([]vector.Chunk, len(chunks))
	for i, chunk := range chunks {
		vectorChunks[i] = vector.Chunk{
			DocID:        docID,
			ChunkIndex:   chunk.Index,
			ChunkText:    chunk.Text,
			SectionTitle: chunk.SectionTitle,
			Embedding:    embeddings[i],
		}
	}

	// Insert chunks
	if err := m.store.InsertChunks(docID, vectorChunks); err != nil {
		return fmt.Errorf("failed to insert chunks: %w", err)
	}

	log.Printf("Indexed %d chunks for document %s", len(chunks), doc.RelPath)
	return nil
}

// Search performs semantic search
func (m *EmbeddingManager) Search(ctx context.Context, query string, limit int) ([]vector.SearchResult, error) {
	if !m.enabled {
		return nil, fmt.Errorf("embeddings not enabled")
	}

	// Generate query embedding
	queryEmbedding, err := m.embed.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	// Search
	results, err := m.store.Search(queryEmbedding, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}

	return results, nil
}

// GetVectorStore returns the vector store
func (m *EmbeddingManager) GetVectorStore() vector.Store {
	return m.store
}

// GetEmbedService returns the embedding service
func (m *EmbeddingManager) GetEmbedService() embedding.Service {
	return m.embed
}

// AppDocumentProvider implements mcp.DocumentProvider for App
type AppDocumentProvider struct {
	app *App
}

// NewAppDocumentProvider creates a new document provider for the app
func NewAppDocumentProvider(app *App) *AppDocumentProvider {
	return &AppDocumentProvider{app: app}
}

// GetDocuments returns all documents
func (p *AppDocumentProvider) GetDocuments() []mcp.DocumentInfo {
	docs := make([]mcp.DocumentInfo, len(p.app.Documents))
	for i, d := range p.app.Documents {
		docs[i] = mcp.DocumentInfo{
			Title:      d.Title,
			Path:       d.Path,
			RelPath:    d.RelPath,
			SourceName: d.SourceName,
			Overview:   d.Overview,
		}
	}
	return docs
}

// GetDocumentContent returns the content of a document by path
func (p *AppDocumentProvider) GetDocumentContent(path string) (string, error) {
	for _, doc := range p.app.Documents {
		if doc.RelPath == path {
			return doc.Content, nil
		}
	}
	return "", fmt.Errorf("document not found: %s", path)
}
