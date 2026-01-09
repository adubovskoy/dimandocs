package main

import (
	"regexp"
)

// DirectoryConfig represents a directory configuration with path, name, and file pattern
type DirectoryConfig struct {
	Path        string `json:"path"`
	Name        string `json:"name"`
	FilePattern string `json:"file_pattern"`
}

// EmbeddingsConfig represents embedding service configuration
type EmbeddingsConfig struct {
	Enabled  bool   `json:"enabled"`
	Provider string `json:"provider"` // "openai" or "ollama"
	Model    string `json:"model"`
	APIKey   string `json:"api_key"` // Supports ${ENV_VAR} syntax
	BaseURL  string `json:"base_url,omitempty"`
	DBPath   string `json:"db_path"` // Path to embeddings database
}

// MCPConfig represents MCP server configuration
type MCPConfig struct {
	Enabled   bool   `json:"enabled"`
	Transport string `json:"transport"` // "stdio" or "http"
	Port      int    `json:"port,omitempty"`
}

// Config represents the application configuration
type Config struct {
	Directories    []DirectoryConfig `json:"directories"`
	Port           string            `json:"port"`
	Title          string            `json:"title"`
	IgnorePatterns []string          `json:"ignore_patterns"`
	Embeddings     EmbeddingsConfig  `json:"embeddings,omitempty"`
	MCP            MCPConfig         `json:"mcp,omitempty"`
}

// Document represents a parsed markdown document
type Document struct {
	Title      string `json:"Title"`
	Path       string `json:"-"`
	Content    string `json:"-"`
	RelPath    string `json:"RelPath"`
	DirName    string `json:"DirName"`
	SourceDir  string `json:"-"`
	SourceName string `json:"SourceName"`
	AbsPath    string `json:"AbsPath"`
	Overview   string `json:"Overview"`
}

// DirectoryGroup represents a group of documents from the same directory
type DirectoryGroup struct {
	Name      string     `json:"Name"`
	Documents []Document `json:"Documents"`
}

// App represents the main application
type App struct {
	Config           Config
	Documents        []Document
	IgnoreRegexes    []*regexp.Regexp
	FileRegexes      map[string]*regexp.Regexp
	WorkingDir       string
	EmbeddingManager *EmbeddingManager // Optional, for vector search
}

// IndexData represents data for the API index response
type IndexData struct {
	Title          string           `json:"Title"`
	Groups         []DirectoryGroup `json:"Groups"`
	TotalDocuments int              `json:"TotalDocuments"`
}