package vector

import (
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"
	"unsafe"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	_ "github.com/mattn/go-sqlite3"
)

const (
	DefaultEmbeddingDimension = 3072 // text-embedding-3-large
)

// Chunk represents a document chunk with its embedding
type Chunk struct {
	ID           int64
	DocID        int64
	ChunkIndex   int
	ChunkText    string
	SectionTitle string
	Embedding    []float32
}

// DocumentRecord represents a document in the vector store
type DocumentRecord struct {
	ID          int64
	Path        string
	Title       string
	ContentHash string
	UpdatedAt   time.Time
}

// SearchResult represents a search result with similarity score
type SearchResult struct {
	Chunk    Chunk
	Document DocumentRecord
	Score    float32
}

// Store defines the interface for vector storage operations
type Store interface {
	// Initialize creates or opens the database
	Initialize() error

	// Close closes the database connection
	Close() error

	// UpsertDocument inserts or updates a document record
	UpsertDocument(path, title, contentHash string) (int64, error)

	// GetDocument retrieves a document by path
	GetDocument(path string) (*DocumentRecord, error)

	// DeleteDocument removes a document and its chunks
	DeleteDocument(path string) error

	// InsertChunks inserts chunks for a document (deletes existing first)
	InsertChunks(docID int64, chunks []Chunk) error

	// Search performs semantic similarity search
	Search(queryEmbedding []float32, limit int) ([]SearchResult, error)

	// GetChunksByDocument retrieves all chunks for a document
	GetChunksByDocument(docID int64) ([]Chunk, error)

	// NeedsUpdate checks if document needs re-embedding based on content hash
	NeedsUpdate(path, contentHash string) (bool, error)
}

// SQLiteStore implements Store using SQLite with sqlite-vec
type SQLiteStore struct {
	db        *sql.DB
	path      string
	dimension int
	mu        sync.RWMutex
}

// NewSQLiteStore creates a new SQLite vector store
func NewSQLiteStore(dbPath string) *SQLiteStore {
	return &SQLiteStore{
		path:      dbPath,
		dimension: DefaultEmbeddingDimension,
	}
}

// SetDimension sets the embedding dimension and recreates the chunks table if needed
func (s *SQLiteStore) SetDimension(dim int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check stored dimension in metadata
	var storedDim int
	err := s.db.QueryRow("SELECT value FROM metadata WHERE key = 'dimension'").Scan(&storedDim)
	if err == nil && storedDim == dim {
		s.dimension = dim
		return nil
	}

	log.Printf("Embedding dimension changed to %d, re-indexing all documents...", dim)
	s.dimension = dim

	// Drop existing chunks table and recreate with new dimension
	_, err = s.db.Exec("DROP TABLE IF EXISTS chunks")
	if err != nil {
		return fmt.Errorf("failed to drop chunks table: %w", err)
	}

	// Clear documents table to force re-indexing with new dimension
	_, err = s.db.Exec("DELETE FROM documents")
	if err != nil {
		return fmt.Errorf("failed to clear documents table: %w", err)
	}

	// Create virtual table for vector search with new dimension
	_, err = s.db.Exec(fmt.Sprintf(`
		CREATE VIRTUAL TABLE IF NOT EXISTS chunks USING vec0 (
			embedding float[%d],
			doc_id INTEGER,
			chunk_index INTEGER,
			chunk_text TEXT,
			section_title TEXT
		)
	`, dim))
	if err != nil {
		return fmt.Errorf("failed to create chunks virtual table: %w", err)
	}

	// Store dimension in metadata
	_, err = s.db.Exec(`
		INSERT INTO metadata (key, value) VALUES ('dimension', ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, dim)
	if err != nil {
		return fmt.Errorf("failed to store dimension: %w", err)
	}

	return nil
}

// Initialize creates the database and tables
func (s *SQLiteStore) Initialize() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Register sqlite-vec extension
	sqlite_vec.Auto()

	db, err := sql.Open("sqlite3", s.path)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	s.db = db

	// Create metadata table for storing configuration like dimension
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS metadata (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create metadata table: %w", err)
	}

	// Create documents table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS documents (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			path TEXT UNIQUE NOT NULL,
			title TEXT NOT NULL,
			content_hash TEXT NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create documents table: %w", err)
	}

	// Create index on path
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_documents_path ON documents(path)`)
	if err != nil {
		return fmt.Errorf("failed to create path index: %w", err)
	}

	// Create virtual table for vector search
	_, err = db.Exec(fmt.Sprintf(`
		CREATE VIRTUAL TABLE IF NOT EXISTS chunks USING vec0 (
			embedding float[%d],
			doc_id INTEGER,
			chunk_index INTEGER,
			chunk_text TEXT,
			section_title TEXT
		)
	`, s.dimension))
	if err != nil {
		return fmt.Errorf("failed to create chunks virtual table: %w", err)
	}

	return nil
}

// Close closes the database connection
func (s *SQLiteStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// UpsertDocument inserts or updates a document record
func (s *SQLiteStore) UpsertDocument(path, title, contentHash string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	result, err := s.db.Exec(`
		INSERT INTO documents (path, title, content_hash, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(path) DO UPDATE SET
			title = excluded.title,
			content_hash = excluded.content_hash,
			updated_at = CURRENT_TIMESTAMP
	`, path, title, contentHash)
	if err != nil {
		return 0, fmt.Errorf("failed to upsert document: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		// If LastInsertId fails, query for the ID
		var docID int64
		err = s.db.QueryRow("SELECT id FROM documents WHERE path = ?", path).Scan(&docID)
		if err != nil {
			return 0, fmt.Errorf("failed to get document id: %w", err)
		}
		return docID, nil
	}

	return id, nil
}

// GetDocument retrieves a document by path
func (s *SQLiteStore) GetDocument(path string) (*DocumentRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var doc DocumentRecord
	err := s.db.QueryRow(`
		SELECT id, path, title, content_hash, updated_at
		FROM documents WHERE path = ?
	`, path).Scan(&doc.ID, &doc.Path, &doc.Title, &doc.ContentHash, &doc.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get document: %w", err)
	}

	return &doc, nil
}

// DeleteDocument removes a document and its chunks
func (s *SQLiteStore) DeleteDocument(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get document ID
	var docID int64
	err := s.db.QueryRow("SELECT id FROM documents WHERE path = ?", path).Scan(&docID)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get document id: %w", err)
	}

	// Delete chunks
	_, err = s.db.Exec("DELETE FROM chunks WHERE doc_id = ?", docID)
	if err != nil {
		return fmt.Errorf("failed to delete chunks: %w", err)
	}

	// Delete document
	_, err = s.db.Exec("DELETE FROM documents WHERE id = ?", docID)
	if err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}

	return nil
}

// InsertChunks inserts chunks for a document (deletes existing first)
func (s *SQLiteStore) InsertChunks(docID int64, chunks []Chunk) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Delete existing chunks for this document
	_, err := s.db.Exec("DELETE FROM chunks WHERE doc_id = ?", docID)
	if err != nil {
		return fmt.Errorf("failed to delete existing chunks: %w", err)
	}

	// Insert new chunks
	stmt, err := s.db.Prepare(`
		INSERT INTO chunks (embedding, doc_id, chunk_index, chunk_text, section_title)
		VALUES (?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare insert statement: %w", err)
	}
	defer stmt.Close()

	for _, chunk := range chunks {
		// Convert embedding to blob format for sqlite-vec
		embeddingBlob := float32SliceToBlob(chunk.Embedding)
		_, err = stmt.Exec(embeddingBlob, docID, chunk.ChunkIndex, chunk.ChunkText, chunk.SectionTitle)
		if err != nil {
			return fmt.Errorf("failed to insert chunk: %w", err)
		}
	}

	return nil
}

// Search performs semantic similarity search
func (s *SQLiteStore) Search(queryEmbedding []float32, limit int) ([]SearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	queryBlob := float32SliceToBlob(queryEmbedding)

	// sqlite-vec requires k = ? for KNN queries
	rows, err := s.db.Query(`
		SELECT
			c.rowid,
			c.doc_id,
			c.chunk_index,
			c.chunk_text,
			c.section_title,
			c.distance,
			d.id,
			d.path,
			d.title,
			d.content_hash,
			d.updated_at
		FROM chunks c
		JOIN documents d ON c.doc_id = d.id
		WHERE c.embedding MATCH ? AND k = ?
		ORDER BY c.distance
	`, queryBlob, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var result SearchResult
		err := rows.Scan(
			&result.Chunk.ID,
			&result.Chunk.DocID,
			&result.Chunk.ChunkIndex,
			&result.Chunk.ChunkText,
			&result.Chunk.SectionTitle,
			&result.Score,
			&result.Document.ID,
			&result.Document.Path,
			&result.Document.Title,
			&result.Document.ContentHash,
			&result.Document.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan result: %w", err)
		}
		results = append(results, result)
	}

	return results, nil
}

// GetChunksByDocument retrieves all chunks for a document
func (s *SQLiteStore) GetChunksByDocument(docID int64) ([]Chunk, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT rowid, doc_id, chunk_index, chunk_text, section_title
		FROM chunks
		WHERE doc_id = ?
		ORDER BY chunk_index
	`, docID)
	if err != nil {
		return nil, fmt.Errorf("failed to get chunks: %w", err)
	}
	defer rows.Close()

	var chunks []Chunk
	for rows.Next() {
		var chunk Chunk
		err := rows.Scan(&chunk.ID, &chunk.DocID, &chunk.ChunkIndex, &chunk.ChunkText, &chunk.SectionTitle)
		if err != nil {
			return nil, fmt.Errorf("failed to scan chunk: %w", err)
		}
		chunks = append(chunks, chunk)
	}

	return chunks, nil
}

// NeedsUpdate checks if document needs re-embedding based on content hash
func (s *SQLiteStore) NeedsUpdate(path, contentHash string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var existingHash string
	err := s.db.QueryRow("SELECT content_hash FROM documents WHERE path = ?", path).Scan(&existingHash)
	if err == sql.ErrNoRows {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check content hash: %w", err)
	}

	return existingHash != contentHash, nil
}

// float32SliceToBlob converts a float32 slice to a byte slice for sqlite-vec
func float32SliceToBlob(vec []float32) []byte {
	blob := make([]byte, len(vec)*4)
	for i, v := range vec {
		bits := *(*uint32)(unsafe.Pointer(&v))
		// Little-endian encoding
		blob[i*4] = byte(bits)
		blob[i*4+1] = byte(bits >> 8)
		blob[i*4+2] = byte(bits >> 16)
		blob[i*4+3] = byte(bits >> 24)
	}
	return blob
}
