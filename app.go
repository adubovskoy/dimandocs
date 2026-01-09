package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/russross/blackfriday/v2"
)

//go:embed frontend/dist/*
var frontendFS embed.FS

// NewApp creates a new application instance
func NewApp() *App {
	return &App{
		FileRegexes: make(map[string]*regexp.Regexp),
	}
}

// Initialize sets up the application
func (a *App) Initialize(configFile string) error {
	// Get working directory
	workingDir, err := GetWorkingDirectory()
	if err != nil {
		return err
	}
	a.WorkingDir = workingDir

	// Load configuration
	if err := a.LoadConfig(configFile); err != nil {
		return err
	}

	// Scan directories for documents
	if err := a.ScanDirectories(); err != nil {
		return err
	}

	return nil
}

// ScanDirectories scans all configured directories for documents
func (a *App) ScanDirectories() error {
	for _, dirConfig := range a.Config.Directories {
		if err := a.scanDirectory(dirConfig.Path, dirConfig.Name, a.FileRegexes[dirConfig.Path]); err != nil {
			return fmt.Errorf("failed to scan directory %s: %w", dirConfig.Path, err)
		}
	}
	return nil
}

// scanDirectory scans a single directory for matching files
func (a *App) scanDirectory(rootDir string, sourceName string, fileRegex *regexp.Regexp) error {
	return filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if a.shouldIgnorePath(path) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if !info.IsDir() {
			filename := info.Name()
			if fileRegex.MatchString(filename) {
				if err := a.processFile(path, rootDir, sourceName); err != nil {
					log.Printf("Failed to process file %s: %v", path, err)
				}
			}
		}

		return nil
	})
}

// extractOverviewParagraph extracts the first paragraph after "## Overview" heading
func extractOverviewParagraph(content string) string {
	lines := strings.Split(content, "\n")
	foundOverview := false
	var paragraphLines []string

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Check if we found the Overview heading
		if strings.HasPrefix(trimmedLine, "## Overview") {
			foundOverview = true
			continue
		}

		// If we found Overview, start collecting paragraph lines
		if foundOverview {
			// Skip empty lines after the heading
			if trimmedLine == "" && len(paragraphLines) == 0 {
				continue
			}

			// Stop if we hit another heading or empty line after content
			if (strings.HasPrefix(trimmedLine, "#") || trimmedLine == "") && len(paragraphLines) > 0 {
				break
			}

			// Collect non-empty lines
			if trimmedLine != "" {
				paragraphLines = append(paragraphLines, trimmedLine)
			}
		}
	}

	return strings.Join(paragraphLines, " ")
}

// processFile processes a single markdown file
func (a *App) processFile(path, rootDir, sourceName string) error {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	relPath, _ := filepath.Rel(rootDir, path)
	dirName := filepath.Dir(relPath)
	if dirName == "." {
		dirName = "Root"
	}

	// Include filename in directory name
	filename := filepath.Base(path)
	if dirName == "Root" {
		dirName = filename
	} else {
		dirName = dirName + "/" + filename
	}

	absPath, _ := filepath.Abs(path)
	absDir := filepath.Dir(absPath)
	relAbsDir, _ := filepath.Rel(a.WorkingDir, absDir)

	// If path starts with ../, replace it with /
	if strings.HasPrefix(relAbsDir, "../") {
		relAbsDir = "/" + strings.TrimPrefix(relAbsDir, "../")
	}

	title := dirName
	if strings.Contains(string(content), "# ") {
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "# ") {
				title = strings.TrimPrefix(line, "# ")
				break
			}
		}
	}

	// Extract overview paragraph
	overview := extractOverviewParagraph(string(content))

	doc := Document{
		Title:      title,
		Path:       path,
		Content:    string(content),
		RelPath:    relPath,
		DirName:    dirName,
		SourceDir:  rootDir,
		SourceName: sourceName,
		AbsPath:    relAbsDir,
		Overview:   overview,
	}

	a.Documents = append(a.Documents, doc)
	return nil
}

// shouldIgnorePath checks if a path should be ignored
func (a *App) shouldIgnorePath(path string) bool {
	for _, regex := range a.IgnoreRegexes {
		if regex.MatchString(path) {
			return true
		}
	}
	return false
}

// GroupDocumentsByDirectory groups documents by their source directory
func (a *App) GroupDocumentsByDirectory() []DirectoryGroup {
	groupMap := make(map[string][]Document)

	for _, doc := range a.Documents {
		groupMap[doc.SourceName] = append(groupMap[doc.SourceName], doc)
	}

	var groups []DirectoryGroup
	for name, docs := range groupMap {
		groups = append(groups, DirectoryGroup{
			Name:      name,
			Documents: docs,
		})
	}

	return groups
}

// SetupRoutes sets up HTTP routes
func (a *App) SetupRoutes() {
	// API routes
	http.HandleFunc("/api/index", a.handleAPIIndex)
	http.HandleFunc("/api/doc/", a.handleAPIDocument)
	http.HandleFunc("/api/search", a.handleSearch)

	// Static files and SPA fallback
	http.HandleFunc("/", a.handleSPA)
}

// handleAPIIndex returns index data as JSON
func (a *App) handleAPIIndex(w http.ResponseWriter, r *http.Request) {
	groups := a.GroupDocumentsByDirectory()

	data := IndexData{
		Title:          a.Config.Title,
		Groups:         groups,
		TotalDocuments: len(a.Documents),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, fmt.Sprintf("Failed to encode response: %v", err), http.StatusInternalServerError)
	}
}

// handleAPIDocument returns a single document as JSON with rendered HTML
func (a *App) handleAPIDocument(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/doc/")

	var doc *Document
	for _, d := range a.Documents {
		if d.RelPath == path {
			doc = &d
			break
		}
	}

	if doc == nil {
		http.NotFound(w, r)
		return
	}

	html := blackfriday.Run([]byte(doc.Content))

	data := struct {
		Title    string `json:"Title"`
		AppTitle string `json:"AppTitle"`
		DirName  string `json:"DirName"`
		AbsPath  string `json:"AbsPath"`
		Content  string `json:"Content"`
	}{
		Title:    doc.Title,
		AppTitle: a.Config.Title,
		DirName:  doc.DirName,
		AbsPath:  doc.AbsPath,
		Content:  string(html),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, fmt.Sprintf("Failed to encode response: %v", err), http.StatusInternalServerError)
	}
}

// SearchResult represents a search result with optional score
type SearchResultJSON struct {
	Document
	Score       float32 `json:"Score,omitempty"`
	ChunkText   string  `json:"ChunkText,omitempty"`
	SectionTitle string `json:"SectionTitle,omitempty"`
	IsVectorSearch bool `json:"IsVectorSearch"`
}

// handleSearch handles search API requests
func (a *App) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))

	if query == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]SearchResultJSON{})
		return
	}

	// Try vector search first if embedding manager is available
	if a.EmbeddingManager != nil && a.EmbeddingManager.IsEnabled() {
		results, err := a.vectorSearch(query)
		if err != nil {
			log.Printf("Vector search failed, falling back to text search: %v", err)
		} else {
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(results); err != nil {
				http.Error(w, fmt.Sprintf("Failed to encode results: %v", err), http.StatusInternalServerError)
			}
			return
		}
	}

	// Fallback to text search
	results := a.textSearch(query)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(results); err != nil {
		http.Error(w, fmt.Sprintf("Failed to encode results: %v", err), http.StatusInternalServerError)
	}
}

// vectorSearch performs semantic search using embeddings
func (a *App) vectorSearch(query string) ([]SearchResultJSON, error) {
	ctx := context.Background()
	results, err := a.EmbeddingManager.Search(ctx, query, 20)
	if err != nil {
		return nil, err
	}

	var searchResults []SearchResultJSON
	seenDocs := make(map[string]bool)

	for _, r := range results {
		// Find the full document
		var doc *Document
		for i := range a.Documents {
			if a.Documents[i].RelPath == r.Document.Path {
				doc = &a.Documents[i]
				break
			}
		}

		if doc == nil {
			continue
		}

		// Deduplicate by document path, keeping highest score
		if seenDocs[doc.RelPath] {
			continue
		}
		seenDocs[doc.RelPath] = true

		searchResults = append(searchResults, SearchResultJSON{
			Document:      *doc,
			Score:         r.Score,
			ChunkText:     r.Chunk.ChunkText,
			SectionTitle:  r.Chunk.SectionTitle,
			IsVectorSearch: true,
		})
	}

	return searchResults, nil
}

// textSearch performs traditional text-based search
func (a *App) textSearch(query string) []SearchResultJSON {
	queryLower := strings.ToLower(query)
	var results []SearchResultJSON

	for _, doc := range a.Documents {
		// Search in title, content, and overview (case-insensitive)
		if strings.Contains(strings.ToLower(doc.Title), queryLower) ||
			strings.Contains(strings.ToLower(doc.Content), queryLower) ||
			strings.Contains(strings.ToLower(doc.Overview), queryLower) {
			results = append(results, SearchResultJSON{
				Document:      doc,
				IsVectorSearch: false,
			})
		}
	}

	return results
}

// handleSPA serves the frontend SPA
func (a *App) handleSPA(w http.ResponseWriter, r *http.Request) {
	// Get the sub-filesystem for frontend/dist
	distFS, err := fs.Sub(frontendFS, "frontend/dist")
	if err != nil {
		http.Error(w, "Failed to access frontend files", http.StatusInternalServerError)
		return
	}

	// Try to serve the requested file
	path := r.URL.Path
	if path == "/" {
		path = "/index.html"
	}

	// Remove leading slash for file lookup
	filePath := strings.TrimPrefix(path, "/")

	// Check if the file exists
	file, err := distFS.Open(filePath)
	if err == nil {
		file.Close()
		// File exists, serve it
		http.FileServer(http.FS(distFS)).ServeHTTP(w, r)
		return
	}

	// File doesn't exist, serve index.html for SPA routing
	indexFile, err := distFS.Open("index.html")
	if err != nil {
		http.Error(w, "Frontend not found", http.StatusNotFound)
		return
	}
	defer indexFile.Close()

	content, err := ioutil.ReadAll(indexFile)
	if err != nil {
		http.Error(w, "Failed to read index.html", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write(content)
}

// Start starts the HTTP server
func (a *App) Start() error {
	port := a.Config.Port
	if port == "" {
		port = "8080"
	}

	a.SetupRoutes()

	fmt.Printf("Starting server on port %s\n", port)
	fmt.Printf("Found %d documents\n", len(a.Documents))

	return http.ListenAndServe(":"+port, nil)
}
