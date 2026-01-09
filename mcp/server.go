package mcp

import (
	"context"
	"fmt"
	"strings"

	"dimandocs/embedding"
	"dimandocs/vector"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// DocumentProvider interface for accessing documents
type DocumentProvider interface {
	GetDocuments() []DocumentInfo
	GetDocumentContent(path string) (string, error)
}

// DocumentInfo represents basic document information
type DocumentInfo struct {
	Title      string
	Path       string
	RelPath    string
	SourceName string
	Overview   string
}

// Server represents the MCP server for DimanDocs
type Server struct {
	mcpServer    *server.MCPServer
	vectorStore  vector.Store
	embedService embedding.Service
	docProvider  DocumentProvider
}

// Config holds MCP server configuration
type Config struct {
	Name         string
	Version      string
	VectorStore  vector.Store
	EmbedService embedding.Service
	DocProvider  DocumentProvider
}

// NewServer creates a new MCP server
func NewServer(cfg Config) (*Server, error) {
	if cfg.Name == "" {
		cfg.Name = "dimandocs"
	}
	if cfg.Version == "" {
		cfg.Version = "1.0.0"
	}

	s := &Server{
		vectorStore:  cfg.VectorStore,
		embedService: cfg.EmbedService,
		docProvider:  cfg.DocProvider,
	}

	// Create MCP server
	mcpServer := server.NewMCPServer(
		cfg.Name,
		cfg.Version,
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(true, false),
	)

	// Register tools
	s.registerTools(mcpServer)

	// Register resources
	s.registerResources(mcpServer)

	s.mcpServer = mcpServer
	return s, nil
}

// registerTools registers MCP tools
func (s *Server) registerTools(srv *server.MCPServer) {
	// Tool: search_docs - semantic search across documentation
	searchTool := mcp.NewTool("search_docs",
		mcp.WithDescription("Search documentation using semantic similarity. Returns relevant document chunks based on the query."),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("The search query to find relevant documentation"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of results to return (default: 5, max: 20)"),
		),
	)
	srv.AddTool(searchTool, s.handleSearchDocs)

	// Tool: get_document - retrieve full document content
	getDocTool := mcp.NewTool("get_document",
		mcp.WithDescription("Get the full content of a specific document by its path."),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("The relative path of the document"),
		),
	)
	srv.AddTool(getDocTool, s.handleGetDocument)

	// Tool: list_documents - list all available documents
	listDocsTool := mcp.NewTool("list_documents",
		mcp.WithDescription("List all available documents in the documentation."),
		mcp.WithString("source",
			mcp.Description("Optional: filter documents by source directory name"),
		),
	)
	srv.AddTool(listDocsTool, s.handleListDocuments)
}

// registerResources registers MCP resources
func (s *Server) registerResources(srv *server.MCPServer) {
	// Resource: docs://index - document index
	indexResource := mcp.NewResource(
		"docs://index",
		"Documentation Index",
		mcp.WithResourceDescription("List of all documentation files with their titles and paths"),
		mcp.WithMIMEType("application/json"),
	)
	srv.AddResource(indexResource, s.handleIndexResource)

	// Resource template: docs://{path} - individual document
	docTemplate := mcp.NewResourceTemplate(
		"docs://{path}",
		"Document Content",
		mcp.WithTemplateDescription("Individual document content by path"),
		mcp.WithTemplateMIMEType("text/markdown"),
	)
	srv.AddResourceTemplate(docTemplate, s.handleDocumentResource)
}

// handleSearchDocs handles the search_docs tool
func (s *Server) handleSearchDocs(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query := request.GetString("query", "")
	if query == "" {
		return mcp.NewToolResultError("query parameter is required"), nil
	}

	limit := request.GetInt("limit", 5)
	if limit > 20 {
		limit = 20
	}
	if limit < 1 {
		limit = 1
	}

	// Generate embedding for query
	queryEmbedding, err := s.embedService.Embed(ctx, query)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to generate query embedding: %v", err)), nil
	}

	// Search vector store
	results, err := s.vectorStore.Search(queryEmbedding, limit)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to search: %v", err)), nil
	}

	if len(results) == 0 {
		return mcp.NewToolResultText("No results found for the query."), nil
	}

	// Format results
	var output strings.Builder
	for i, r := range results {
		output.WriteString(fmt.Sprintf("## Result %d (score: %.4f)\n", i+1, r.Score))
		output.WriteString(fmt.Sprintf("**Document:** %s\n", r.Document.Title))
		output.WriteString(fmt.Sprintf("**Path:** %s\n", r.Document.Path))
		if r.Chunk.SectionTitle != "" {
			output.WriteString(fmt.Sprintf("**Section:** %s\n", r.Chunk.SectionTitle))
		}
		output.WriteString(fmt.Sprintf("\n%s\n\n---\n\n", r.Chunk.ChunkText))
	}

	return mcp.NewToolResultText(output.String()), nil
}

// handleGetDocument handles the get_document tool
func (s *Server) handleGetDocument(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path := request.GetString("path", "")
	if path == "" {
		return mcp.NewToolResultError("path parameter is required"), nil
	}

	content, err := s.docProvider.GetDocumentContent(path)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get document: %v", err)), nil
	}

	return mcp.NewToolResultText(content), nil
}

// handleListDocuments handles the list_documents tool
func (s *Server) handleListDocuments(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sourceFilter := request.GetString("source", "")

	docs := s.docProvider.GetDocuments()
	var output strings.Builder

	for _, doc := range docs {
		if sourceFilter != "" && doc.SourceName != sourceFilter {
			continue
		}
		output.WriteString(fmt.Sprintf("- **%s** (%s)\n", doc.Title, doc.RelPath))
		if doc.Overview != "" {
			output.WriteString(fmt.Sprintf("  %s\n", truncateString(doc.Overview, 150)))
		}
	}

	if output.Len() == 0 {
		return mcp.NewToolResultText("No documents found."), nil
	}

	return mcp.NewToolResultText(output.String()), nil
}

// handleIndexResource handles the docs://index resource
func (s *Server) handleIndexResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	docs := s.docProvider.GetDocuments()

	var output strings.Builder
	output.WriteString("# Documentation Index\n\n")
	for _, doc := range docs {
		output.WriteString(fmt.Sprintf("- [%s](docs://%s) - %s\n", doc.Title, doc.RelPath, doc.SourceName))
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "text/markdown",
			Text:     output.String(),
		},
	}, nil
}

// handleDocumentResource handles the docs://{path} resource template
func (s *Server) handleDocumentResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	// Extract path from URI (remove "docs://" prefix)
	path := request.Params.URI
	if len(path) > 7 {
		path = path[7:] // Remove "docs://"
	}

	content, err := s.docProvider.GetDocumentContent(path)
	if err != nil {
		return nil, fmt.Errorf("document not found: %s", path)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "text/markdown",
			Text:     content,
		},
	}, nil
}

// ServeStdio starts the MCP server over stdio
func (s *Server) ServeStdio() error {
	return server.ServeStdio(s.mcpServer)
}

// truncateString truncates a string to maxLen and adds "..." if truncated
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
